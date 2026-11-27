package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// treeDirections runs f left-to-right, right-to-left under the app-wide
// direction, and right-to-left as a window's own direction in a
// left-to-right app. The last is what a DirectionScope also gives: a
// direction event handlers must look up, as it is not the global one. show
// puts the object on a window and returns it.
func treeDirections(t *testing.T, f func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window)) {
	for _, c := range []struct {
		name   string
		global bellui.Direction
		scoped bool
	}{
		{"ltr", bellui.DirectionLTR, false},
		{"rtl", bellui.DirectionRTL, false},
		{"window", bellui.DirectionLTR, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(c.global)
			defer bellui.SetLayoutDirection(bellui.DirectionAuto)
			show := func(o core.CanvasObject) bellui.Window {
				w := test.NewTempWindow(t, o)
				if c.scoped {
					w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
				}
				w.Resize(geo.NewSize(300, 200))
				return w
			}
			f(t, c.name != "ltr", show)
		})
	}
}

// labelBoxes maps each visible Label's text to its box on the canvas.
func labelBoxes(w bellui.Window) map[string][2]float32 {
	boxes := map[string][2]float32{}
	driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, pos, _ geo.Position, _ geo.Size) bool {
		if l, ok := o.(*Label); ok {
			boxes[l.Text] = [2]float32{pos.X, pos.X + l.Size().Width}
		}
		return false
	}, nil)
	return boxes
}

// Right-to-left, a Tree row indents from the right: the expand icon is at
// the right, and a child's label ends an indent before its parent's. It
// used to put the label on the right but leave the icon on the left and
// lose the indent (#779).
func TestTree_RTL(t *testing.T) {
	treeDirections(t, func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window) {
		tree := NewTreeWithStrings(map[string][]string{"": {"a", "b"}, "a": {"a1", "a2"}})
		tree.OpenBranch("a")
		w := show(tree)
		if !rtl {
			test.AssertRendersToImage(t, "tree/rtl779_ltr.png", w.Canvas())
		} else if bellui.CurrentLayoutDirection() == bellui.DirectionRTL {
			test.AssertRendersToImage(t, "tree/rtl779_rtl.png", w.Canvas())
		}

		boxes := labelBoxes(w)
		parent, child := boxes["a"], boxes["a1"]
		require.NotZero(t, parent[1], "the label of a is on screen")
		indent := tree.Theme().Size(theme.SizeNameInlineIcon) + tree.Theme().Size(theme.SizeNamePadding)
		if rtl {
			assert.InDelta(t, parent[1]-indent, child[1], 0.5, "a child's label ends an indent left of its parent's")
		} else {
			assert.InDelta(t, parent[0]+indent, child[0], 0.5, "a child's label starts an indent right of its parent's")
		}

		// The branch's expand icon is beside its label, on the leading side.
		var iconX float32 = -1
		driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, pos, _ geo.Position, _ geo.Size) bool {
			if _, ok := o.(*branchIcon); ok && iconX < 0 {
				iconX = pos.X
			}
			return false
		}, nil)
		if rtl {
			assert.Greater(t, iconX, parent[1], "the expand icon is right of the label")
		} else {
			assert.Less(t, iconX, parent[0], "the expand icon is left of the label")
		}

		// Hover lays the row out again, outside a layout walk, and must keep
		// it in the row's own direction.
		var row *leaf
		driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			if l, ok := o.(*leaf); ok && l.uid == "a1" {
				row = l
			}
			return false
		}, nil)
		require.NotNil(t, row, "the a1 row is on screen")
		row.HoverIn(&event.HoverEvent{})
		require.True(t, row.hovered, "the row took the hover")
		assert.Equal(t, child, labelBoxes(w)["a1"], "a hovered row stays where it was")
	})
}

// Right-to-left, the arrow pointing at a row's indent closes its branch or
// goes to its parent, and the other opens it: Right closes and Left opens
// (#779). The same holds for TreeView.
func TestTree_RTLKeys(t *testing.T) {
	treeDirections(t, func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window) {
		closeKey, openKey := event.KeyLeft, event.KeyRight
		if rtl {
			closeKey, openKey = event.KeyRight, event.KeyLeft
		}

		tree := NewTreeWithStrings(map[string][]string{"": {"a", "b"}, "a": {"a1", "a2"}})
		show(tree)
		tree.currentFocus = "a"
		tree.TypedKey(&event.KeyEvent{Name: openKey})
		assert.True(t, tree.IsBranchOpen("a"), "Tree: the open key opens the branch")
		assert.Equal(t, "a1", tree.currentFocus, "Tree: and moves to its first child")
		tree.TypedKey(&event.KeyEvent{Name: closeKey})
		assert.Equal(t, "a", tree.currentFocus, "Tree: the close key goes to the parent")
		tree.TypedKey(&event.KeyEvent{Name: closeKey})
		assert.False(t, tree.IsBranchOpen("a"), "Tree: then closes it")

		tv := NewTreeViewWithNodes([]TreeViewNode{{ID: "a", Text: "a"}, {ID: "b", Text: "b"}})
		require.NoError(t, tv.AddNode("a", TreeViewNode{ID: "a1", Text: "a1"}))
		show(tv)
		tv.setItemFocus("a")
		tv.TypedKey(&event.KeyEvent{Name: openKey})
		assert.True(t, tv.IsBranchOpen("a"), "TreeView: the open key opens the branch")
		tv.TypedKey(&event.KeyEvent{Name: closeKey})
		assert.False(t, tv.IsBranchOpen("a"), "TreeView: the close key closes it")
	})
}

// Right-to-left, a TreeView row mirrors too: the chevron and indent are at
// the right, a guide line runs down the right-hand indent, and a hovered
// row's rename and delete buttons are at the left (#779).
func TestTreeView_RTL(t *testing.T) {
	treeDirections(t, func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window) {
		tv := NewTreeViewWithNodes([]TreeViewNode{{ID: "a", Text: "a"}, {ID: "b", Text: "b"}})
		require.NoError(t, tv.AddNode("a", TreeViewNode{ID: "a1", Text: "a1"}))
		tv.AllowRename, tv.AllowDelete, tv.ShowGuideLines = true, true, true
		tv.SetHasMore("a", true)
		tv.OpenBranch("a")
		w := show(tv)

		boxes := labelBoxes(w)
		parent, child := boxes["a"], boxes["a1"]
		require.NotZero(t, parent[1], "the label of a is on screen")
		if rtl {
			assert.Less(t, child[1], parent[1], "a child's label ends left of its parent's")
		} else {
			assert.Greater(t, child[0], parent[0], "a child's label starts right of its parent's")
		}

		var item *treeViewItem
		var content *treeViewContent
		var guide float32 = -1
		driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			if ti, ok := o.(*treeViewItem); ok && ti.uid == "a" {
				item = ti
			}
			if c, ok := o.(*treeViewContent); ok {
				content = c
			}
			return false
		}, nil)
		require.NotNil(t, content)
		require.NotNil(t, item)
		r := test.TempWidgetRenderer(t, item).(*treeViewItemRenderer)
		chevronX := r.chevron.Position().X
		contentX := r.content.Position().X
		if rtl {
			assert.Greater(t, chevronX, contentX, "the chevron is right of the content")
		} else {
			assert.Less(t, chevronX, contentX, "the chevron is left of the content")
		}

		cr := test.TempWidgetRenderer(t, content).(*treeViewContentRenderer)
		for _, l := range cr.guideLines {
			if l.Visible() {
				guide = l.Position().X
				break
			}
		}
		require.GreaterOrEqual(t, guide, float32(0), "a guide line is drawn")
		if rtl {
			assert.Greater(t, guide, content.Size().Width/2, "the guide line is at the right")
		} else {
			assert.Less(t, guide, content.Size().Width/2, "the guide line is at the left")
		}

		// The load-more indicator under a is indented like a's children.
		more := cr.loadMoreIndicators["a"]
		require.NotNil(t, more, "a shows a load-more indicator")
		left := more.Position().X
		right := content.Size().Width - more.Position().X - more.Size().Width
		if rtl {
			assert.Greater(t, right, left, "the load-more indicator is indented from the right")
		} else {
			assert.Greater(t, left, right, "the load-more indicator is indented from the left")
		}

		item.HoverIn(&event.HoverEvent{})
		del := r.deleteBtn.Position().X
		require.True(t, r.deleteBtn.Visible(), "a hovered row shows its delete button")
		if rtl {
			assert.Less(t, del, contentX, "the delete button is left of the content")
		} else {
			assert.Greater(t, del, contentX, "the delete button is right of the content")
		}
	})
}
