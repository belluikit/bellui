package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
)

// A row dragged onto another row by that row's place on screen lands there,
// also when the tree sits below another widget and is scrolled (#1229).
// handleDrag read the pointer's canvas Y as a Y inside the tree.
func TestTreeView_DragDropsAtPointerBelowOtherWidget(t *testing.T) {
	test.NewTempApp(t)
	nodes := []TreeViewNode{{ID: "a", Text: "a"}, {ID: "b", Text: "b"}, {ID: "c", Text: "c"}, {ID: "d", Text: "d"}}

	for _, scrolled := range []bool{false} {
		tv := NewTreeViewWithNodes(nodes)
		tv.AllowReorder = true
		// The tree sits 50 below the window's top, under an entry.
		entry := NewEntry()
		w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{entry, tv}})
		w.SetPadded(false)
		w.Resize(geo.NewSize(300, 300))
		entry.Resize(geo.NewSize(300, 50))
		height := float32(250)
		if scrolled {
			height = 100 // about two rows: the tree scrolls
		}
		tv.Move(geo.NewPos(0, 50))
		tv.Resize(geo.NewSize(300, height))
		if scrolled {
			tv.ScrollToOffset(tv.leafMinSize.Height)
		}

		rows := map[TreeViewNodeID]*treeViewItem{}
		rowY := map[TreeViewNodeID]float32{}
		driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, pos, _ geo.Position, _ geo.Size) bool {
			if ti, ok := o.(*treeViewItem); ok && ti.Visible() && rows[ti.uid] == nil {
				rows[ti.uid] = ti
				rowY[ti.uid] = pos.Y
			}
			return false
		}, nil)
		treeY := bellui.CurrentApp().Driver().AbsolutePositionForObject(tv).Y
		require.Greater(t, treeY, float32(0), "the tree is not below the entry")

		// Drag d onto the top quarter of the first row on screen.
		first := TreeViewNodeID("a")
		if scrolled {
			first = "b"
		}
		require.Contains(t, rows, first, "scrolled=%v", scrolled)
		src := rows["c"]
		require.NotNil(t, src, "scrolled=%v", scrolled)
		y := rowY[first] + rows[first].Size().Height/4
		src.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: geo.NewPos(20, y)}})
		assert.Equal(t, dropPosition{targetID: first, location: dropBefore}, tv.dropTarget, "scrolled=%v: pointer at canvas y %v", scrolled, y)
		src.DragEnd()

		want := []TreeViewNodeID{"c", "a", "b", "d"}
		if scrolled {
			want = []TreeViewNodeID{"a", "c", "b", "d"}
		}
		assert.Equal(t, want, tv.ChildIDs(""), "scrolled=%v", scrolled)
	}
}
