package test_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// collectionWidgets builds each collection widget that measures an item
// template it never shows. Each table is tall and wide enough to need several
// dividers, so that shrinking it drops some.
func collectionWidgets() map[string]func() core.Widget {
	label := func() core.CanvasObject { return widget.NewLabel("template") }
	setLabel := func(id int, o core.CanvasObject) { o.(*widget.Label).SetText(fmt.Sprint(id)) }
	tree := map[string][]string{"": {"a", "b", "c", "d", "e", "f"}, "a": {"a1", "a2", "a3"}, "b": {"b1", "b2"}}
	return map[string]func() core.Widget{
		"List": func() core.Widget {
			return widget.NewList(func() int { return 50 }, label,
				func(id widget.ListItemID, o core.CanvasObject) { setLabel(id, o) })
		},
		"Table": func() core.Widget {
			return widget.NewTable(func() (int, int) { return 50, 10 }, label,
				func(id widget.TableCellID, o core.CanvasObject) { setLabel(id.Row, o) })
		},
		"TableWithHeaders": func() core.Widget {
			return widget.NewTableWithHeaders(func() (int, int) { return 50, 10 }, label,
				func(id widget.TableCellID, o core.CanvasObject) { setLabel(id.Row, o) })
		},
		"GridWrap": func() core.Widget {
			return widget.NewGridWrap(func() int { return 50 }, label,
				func(id widget.GridWrapItemID, o core.CanvasObject) { setLabel(id, o) })
		},
		"Tree": func() core.Widget {
			t := widget.NewTreeWithStrings(tree)
			t.OpenAllBranches()
			return t
		},
		"TreeView": func() core.Widget {
			var nodes []widget.TreeViewNode
			for i := range 20 {
				nodes = append(nodes, widget.TreeViewNode{ID: widget.TreeViewNodeID(fmt.Sprint(i)), Text: "node"})
			}
			return widget.NewTreeViewWithNodes(nodes)
		},
	}
}

// renderCollection shows w on a painted canvas, refreshes it a few times, as
// an app does when its data changes, and returns the canvas. Each refresh
// measures the item template again.
func renderCollection(w core.Widget) test.WindowlessCanvas {
	c := test.NewCanvasWithPainter(software.NewPainter())
	c.SetContent(w)
	c.Resize(geo.NewSize(600, 600))
	// Painting gives every visible object a renderer, as a window does, so
	// that DestroyRenderers can reach the whole tree.
	c.Capture()
	for range 3 {
		w.Refresh()
	}
	c.Capture()
	return c
}

// rendererLeftovers lists the types of the widgets that have a renderer now
// but did not have one in before.
func rendererLeftovers(before map[core.Widget]struct{}) []string {
	left := map[core.Widget]struct{}{}
	for w := range cache.RenderedWidgets() {
		if _, ok := before[w]; !ok {
			left[w] = struct{}{}
		}
	}
	return typesOf(left)
}

// TestDestroyRenderers_CollectionWidgets checks that a collection widget
// leaves no renderer behind for the item templates it measures, or for the
// separators and table dividers it builds and stops showing: DestroyRenderers
// reaches only what the widget's tree holds, so anything else would stay
// cached (#722).
func TestDestroyRenderers_CollectionWidgets(t *testing.T) {
	for name, build := range collectionWidgets() {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			before := cache.RenderedWidgets()

			w := build()
			renderCollection(w)
			assert.True(t, cache.IsRendered(w), "the widget was never rendered")
			test.DestroyRenderers(t, w)

			assert.Empty(t, rendererLeftovers(before), "renderers left behind")
		})
	}
}

// TestDestroyRenderers_CollectionWidgetsShrunk shrinks each widget after
// showing it, so that it drops separators and dividers it no longer needs,
// and checks that none of their renderers is left. The items the widget
// moves to its pool of unused items do keep theirs (#740), so only
// separators are checked here. A GridWrap has none.
func TestDestroyRenderers_CollectionWidgetsShrunk(t *testing.T) {
	for name, build := range collectionWidgets() {
		if name == "GridWrap" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			before := cache.RenderedWidgets()

			w := build()
			c := renderCollection(w)
			c.Resize(geo.NewSize(150, 150))
			w.Refresh()
			c.Capture()
			test.DestroyRenderers(t, w)

			assert.NotContains(t, rendererLeftovers(before), "*widget.Separator")
		})
	}
}
