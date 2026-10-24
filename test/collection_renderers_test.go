package test_test

import (
	"fmt"
	"runtime"
	"runtime/debug"
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

// keepPooledItems makes a widget's pool of unused items, a sync.Pool, keep
// what it is given for the rest of the test: a GC empties the pool, and a Get
// on another P than the Put's misses the item. An item the pool loses keeps
// its renderer (#740), so a test that asserts nothing is left needs it.
func keepPooledItems(t *testing.T) {
	gc := debug.SetGCPercent(-1)
	procs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() {
		runtime.GOMAXPROCS(procs)
		debug.SetGCPercent(gc)
	})
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
// separators are checked here, except for a List: its Refresh drains the
// pool and drops those items, so it must leave nothing, where the pool keeps
// what it is given. A GridWrap has no separators.
func TestDestroyRenderers_CollectionWidgetsShrunk(t *testing.T) {
	for name, build := range collectionWidgets() {
		if name == "GridWrap" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			strict := name == "List" && !raceEnabled
			if strict {
				keepPooledItems(t)
			}
			test.NewTempApp(t)
			before := cache.RenderedWidgets()

			w := build()
			c := renderCollection(w)
			c.Resize(geo.NewSize(150, 150))
			w.Refresh()
			c.Capture()
			test.DestroyRenderers(t, w)

			if strict {
				assert.Empty(t, rendererLeftovers(before), "renderers left behind")
				return
			}
			assert.NotContains(t, rendererLeftovers(before), "*widget.Separator")
		})
	}
}

// TestDestroyRenderers_ListScrolledRefresh scrolls a List so that rows it
// stops showing go to its pool of unused items, then refreshes it: a full
// Refresh drains the pool and drops what it held, so the dropped rows must
// not keep their renderers (#722).
func TestDestroyRenderers_ListScrolledRefresh(t *testing.T) {
	if raceEnabled {
		t.Skip("the pool of unused items is a sync.Pool, which -race empties at random")
	}
	keepPooledItems(t)
	test.NewTempApp(t)
	before := cache.RenderedWidgets()

	l := collectionWidgets()["List"]().(*widget.List)
	c := renderCollection(l)
	c.Resize(geo.NewSize(150, 150))
	l.ScrollToBottom()
	c.Capture()
	l.ScrollToTop()
	c.Capture()
	l.Refresh()
	c.Capture()
	test.DestroyRenderers(t, l)

	assert.Empty(t, rendererLeftovers(before), "renderers left behind")
}

// TestDestroyRenderers_RichTextTable checks that a Markdown table leaves no
// renderer behind for the labels it builds only to measure its columns, or
// for the cells it replaces when the table changes shape (#722).
func TestDestroyRenderers_RichTextTable(t *testing.T) {
	test.NewTempApp(t)
	before := cache.RenderedWidgets()

	rt := widget.NewRichTextFromMarkdown("| a | b |\n|---|---|\n| 1 | 2 |\n")
	renderCollection(rt)
	rt.ParseMarkdown("| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n| 4 | 5 | 6 |\n")
	c := renderCollection(rt)
	// A changed row count rebuilds the cells of the same table.
	for _, seg := range rt.Segments {
		if table, ok := seg.(*widget.TableSegment); ok {
			table.Rows = append(table.Rows, []string{"7", "8", "9"})
		}
	}
	rt.Refresh()
	assert.Contains(t, typesOf(cache.RenderedWidgets()), "*widget.richTable", "the table was never rendered")
	c.Capture()
	test.DestroyRenderers(t, rt)

	assert.Empty(t, rendererLeftovers(before), "renderers left behind")
}
