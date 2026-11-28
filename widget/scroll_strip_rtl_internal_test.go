package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	iwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drawnBoxes records where every visible label, text run, row and separator
// on w is drawn: its left edge and width on the canvas, keyed by what it is
// and how far down it is. Objects with no size draw nothing and are left out.
func drawnBoxes(w bellui.Window) map[string][2]float32 {
	boxes := map[string][2]float32{}
	driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, pos, _ geo.Position, _ geo.Size) bool {
		var key string
		switch v := o.(type) {
		case *Label:
			key = "label " + v.Text
		case *canvas.Text:
			key = "text " + v.Text
		case *listItem, *leaf, *branch, *treeViewItem:
			key = fmt.Sprintf("row %T", o)
		case *Separator:
			key = "separator"
		default:
			return false
		}
		if o.Size().IsZero() {
			return false // draws nothing: a List's first separator is never placed
		}
		boxes[fmt.Sprintf("%s @%.1f", key, pos.Y)] = [2]float32{pos.X, o.Size().Width}
		return false
	}, nil)
	return boxes
}

// List, Tree and TreeView rows, and a scrolling RichText's lines, are laid
// out across the whole widget, so left-to-right they reach under the strip
// the Scroll keeps for its vertical bar on the right. Right-to-left that
// strip is on the left (#1496), and each of them is its left-to-right layout
// mirrored: every label, text run, row and separator as far from the right
// edge as it is from the left one left-to-right. They used to start at the
// content's left edge, after the strip, and run off the right edge by its
// width, clipping right-aligned text.
func TestScrollingWidgets_RTLMirrorAcrossBarStrip(t *testing.T) {
	long := strings.Repeat("A paragraph long enough to wrap across the width of the window several times. ", 6)
	widgets := []struct {
		name string
		make func() core.CanvasObject
	}{
		{"List", func() core.CanvasObject {
			l := NewList(func() int { return 50 }, func() core.CanvasObject { return NewLabel("item") },
				func(i ListItemID, o core.CanvasObject) { o.(*Label).SetText(fmt.Sprintf("item %d", i)) })
			l.Select(1)
			return l
		}},
		{"Tree", func() core.CanvasObject {
			kids := map[string][]string{"": {"a"}, "a": nil}
			for i := 0; i < 20; i++ {
				kids["a"] = append(kids["a"], fmt.Sprintf("a%d", i))
			}
			tr := NewTreeWithStrings(kids)
			tr.OpenBranch("a")
			return tr
		}},
		{"TreeView", func() core.CanvasObject {
			tv := NewTreeViewWithNodes([]TreeViewNode{{ID: "a", Text: "a"}})
			for i := 0; i < 20; i++ {
				id := TreeViewNodeID(fmt.Sprintf("a%d", i))
				if err := tv.AddNode("a", TreeViewNode{ID: id, Text: string(id)}); err != nil {
					panic(err)
				}
			}
			tv.ShowGuideLines = true
			tv.OpenBranch("a")
			return tv
		}},
		{"RichText", func() core.CanvasObject {
			rt := NewRichTextWithText(long)
			rt.Wrapping = bellui.TextWrapWord
			rt.Scroll = iwidget.ScrollVerticalOnly
			return rt
		}},
	}
	for _, wd := range widgets {
		t.Run(wd.name, func(t *testing.T) {
			drawn := map[string]map[string][2]float32{}
			var width float32
			treeDirections(t, func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window) {
				w := show(wd.make())
				width = w.Canvas().Size().Width
				drawn[strings.TrimPrefix(t.Name(), "TestScrollingWidgets_RTLMirrorAcrossBarStrip/"+wd.name+"/")] = drawnBoxes(w)
			})
			ltr := drawn["ltr"]
			require.NotEmpty(t, ltr, "something is drawn")
			for _, name := range []string{"rtl", "window"} {
				got := drawn[name]
				for k, want := range ltr {
					if g, ok := got[k]; assert.True(t, ok, "%s: %s is drawn", name, k) {
						assert.InDelta(t, width-want[0]-want[1], g[0], 0.5, "%s: %s is its left-to-right place mirrored", name, k)
					}
				}
				assert.Len(t, got, len(ltr), "%s: the same parts are drawn", name)
			}
		})
	}
}

// A TreeView's guide line, load-more spinner and drop line are mirrored
// across its content too, so right-to-left they are where they are
// left-to-right, mirrored, beside the strip the Scroll keeps for its bar on
// the left (#1496).
func TestTreeView_RTLChromeMirrorsAcrossBarStrip(t *testing.T) {
	drawn := map[string]map[string][2]float32{}
	var width float32
	treeDirections(t, func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window) {
		tv := NewTreeViewWithNodes([]TreeViewNode{{ID: "a", Text: "a"}})
		for i := 0; i < 6; i++ {
			id := TreeViewNodeID(fmt.Sprintf("a%d", i))
			require.NoError(t, tv.AddNode("a", TreeViewNode{ID: id, Text: string(id)}))
		}
		tv.ShowGuideLines = true
		tv.SetHasMore("a", true)
		tv.OpenBranch("a")
		w := show(tv)
		width = tv.Size().Width

		var content *treeViewContent
		driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			if c, ok := o.(*treeViewContent); ok {
				content = c
			}
			return false
		}, nil)
		require.NotNil(t, content)
		if bellui.CurrentDevice().IsMobile() {
			// A touch device's bar overlays the content and reserves nothing;
			// the chrome mirrors all the same.
			require.Equal(t, width, content.Size().Width, "a mobile scroll reserves no strip for its bar")
		} else {
			require.Less(t, content.Size().Width, width, "the scroll keeps a strip for its bar")
		}
		cr := test.TempWidgetRenderer(t, content).(*treeViewContentRenderer)
		tv.dropTarget = dropPosition{targetID: "a2", location: dropAfter}
		tv.updateDropIndicator()

		// All three are the content's own objects, placed in its coordinates;
		// the drop line joins the tree only at the next layout, so it is
		// measured through the content's position rather than looked up.
		d := bellui.CurrentApp().Driver()
		contentX := d.AbsolutePositionForObject(content).X - d.AbsolutePositionForObject(tv).X
		at := func(o core.CanvasObject) [2]float32 {
			return [2]float32{contentX + o.Position().X, o.Size().Width}
		}
		got := map[string][2]float32{}
		for i, l := range cr.guideLines {
			if l.Visible() {
				got[fmt.Sprint("guide line ", i)] = at(l)
			}
		}
		for id, a := range cr.loadMoreIndicators {
			got["load more "+string(id)] = at(a)
		}
		require.NotNil(t, cr.dropIndicator)
		got["drop line"] = at(cr.dropIndicator)
		// A second drag shows the line again after the first one hid it.
		tv.hideDropIndicator()
		tv.dropTarget = dropPosition{targetID: "a3", location: dropAfter}
		tv.updateDropIndicator()
		got["drop line, second drag"] = at(cr.dropIndicator)
		drawn[t.Name()[strings.LastIndex(t.Name(), "/")+1:]] = got
	})
	ltr := drawn["ltr"]
	assert.Contains(t, ltr, "guide line 0", "a guide line is drawn")
	assert.Contains(t, ltr, "load more a", "the load-more spinner is drawn")
	for _, name := range []string{"rtl", "window"} {
		got := drawn[name]
		for k, want := range ltr {
			if g, ok := got[k]; assert.True(t, ok, "%s: %s is drawn", name, k) {
				assert.InDelta(t, width-want[0]-want[1], g[0], 0.5, "%s: %s is its left-to-right place mirrored", name, k)
			}
		}
		assert.Len(t, got, len(ltr), "%s: the same parts are drawn", name)
	}
}
