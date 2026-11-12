package widget_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// A collection widget that puts the row holding the focus back in its pool,
// as a scroll that takes the row out of view does, drops the focus there and
// then: the pool hands the same widget out for another item, which the keys
// typed next would edit (#1175). The test canvas never walks, so nothing else
// unfocuses it.
func TestCollectionsUnfocusRecycledRow(t *testing.T) {
	type built struct {
		obj    core.CanvasObject
		first  func() *widget.Entry // the entry showing the first item
		scroll func()
	}
	entryUpdated := func(first **widget.Entry, isFirst bool, o core.CanvasObject) {
		if isFirst {
			*first = o.(*widget.Entry)
		} else if *first == o {
			*first = nil // reused for another item
		}
	}
	newEntry := func() core.CanvasObject { return widget.NewEntry() }

	for name, build := range map[string]func() built{
		"List": func() built {
			var first *widget.Entry
			l := widget.NewList(func() int { return 200 }, newEntry,
				func(id widget.ListItemID, o core.CanvasObject) { entryUpdated(&first, id == 0, o) })
			return built{l, func() *widget.Entry { return first }, l.ScrollToBottom}
		},
		"GridWrap": func() built {
			var first *widget.Entry
			g := widget.NewGridWrap(func() int { return 400 }, newEntry,
				func(id widget.GridWrapItemID, o core.CanvasObject) { entryUpdated(&first, id == 0, o) })
			return built{g, func() *widget.Entry { return first }, g.ScrollToBottom}
		},
		"Table": func() built {
			var first *widget.Entry
			tb := widget.NewTable(func() (int, int) { return 200, 2 }, newEntry,
				func(id widget.TableCellID, o core.CanvasObject) { entryUpdated(&first, id.Row == 0 && id.Col == 0, o) })
			return built{tb, func() *widget.Entry { return first }, tb.ScrollToBottom}
		},
		"Tree": func() built {
			var first *widget.Entry
			ids := make([]widget.TreeNodeID, 200)
			for i := range ids {
				ids[i] = fmt.Sprint("n", i)
			}
			tr := widget.NewTree(
				func(id widget.TreeNodeID) []widget.TreeNodeID {
					if id == "" {
						return ids
					}
					return nil
				},
				func(id widget.TreeNodeID) bool { return id == "" },
				func(bool) core.CanvasObject { return widget.NewEntry() },
				func(id widget.TreeNodeID, _ bool, o core.CanvasObject) { entryUpdated(&first, id == "n0", o) },
			)
			return built{tr, func() *widget.Entry { return first }, tr.ScrollToBottom}
		},
	} {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			b := build()
			w := test.NewTempWindow(t, b.obj)
			w.Resize(geo.NewSize(300, 300))
			entry := b.first()
			require.NotNil(t, entry, "precondition: a row shows the first item")
			w.Canvas().Focus(entry)
			require.Equal(t, event.Focusable(entry), w.Canvas().Focused())

			b.scroll()
			assert.Nil(t, w.Canvas().Focused(), "the recycled row kept the focus")
		})
	}
}

// A TreeView row renaming its node is recycled the same way: the rename
// editor it holds loses the focus.
func TestTreeViewUnfocusRecycledRenamingRow(t *testing.T) {
	test.NewTempApp(t)
	nodes := make([]widget.TreeViewNode, 200)
	for i := range nodes {
		nodes[i] = widget.TreeViewNode{ID: widget.TreeViewNodeID(fmt.Sprint("n", i)), Text: fmt.Sprint("node ", i)}
	}
	tv := widget.NewTreeViewWithNodes(nodes)
	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 300))

	tv.BeginRename("n0")
	_, editing := w.Canvas().Focused().(*widget.Entry)
	require.True(t, editing, "precondition: the rename editor has the focus")

	tv.ScrollToBottom()
	assert.Nil(t, w.Canvas().Focused(), "the recycled row's editor kept the focus")
}
