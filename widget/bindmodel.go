package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/data/model"
)

// BindListModel drives a [List] from a [model.Model], setting its Length,
// CreateItem and UpdateItem callbacks and refreshing the list whenever the model
// changes. Because the model may be a [model.SortProxy] or [model.FilterProxy]
// (or a composition of them), this is how a list gets locale-aware sorting and
// live filtering without the app hand-rolling index bookkeeping.
//
// updateItem receives the typed item for a row rather than a positional id, so
// callers never index back into their data. The returned function detaches the
// binding; call it when the list is discarded.
//
// Mutate the underlying model on the main goroutine (use [bellui.Do] from a
// background goroutine), the same rule the widgets already follow.
//
// Since: 2.8
func BindListModel[T any](list *List, m model.Model[T],
	createItem func() core.CanvasObject,
	updateItem func(item T, row core.CanvasObject),
) (unbind func()) {
	list.Length = m.Len
	list.CreateItem = createItem
	list.UpdateItem = func(id ListItemID, row core.CanvasObject) {
		if id < 0 || id >= m.Len() {
			return
		}
		updateItem(m.Item(id), row)
	}

	l := binding.NewDataListener(list.Refresh)
	m.AddListener(l) // fires immediately, rendering current contents
	return func() { m.RemoveListener(l) }
}

// BindTableModel drives a [Table] whose rows come from a [model.Model], with a
// fixed number of columns. updateCell receives the row's typed item plus the
// column index, so the app maps (item, col) to cell content. The returned
// function detaches the binding.
//
// For header-click sorting over a [model.SortProxy], pair this with
// [BindTableSort] on the same proxy.
//
// Since: 2.8
func BindTableModel[T any](t *Table, m model.Model[T], cols int,
	createCell func() core.CanvasObject,
	updateCell func(item T, col int, cell core.CanvasObject),
) (unbind func()) {
	t.Length = func() (int, int) { return m.Len(), cols }
	t.CreateCell = createCell
	t.UpdateCell = func(id TableCellID, cell core.CanvasObject) {
		if id.Row < 0 || id.Row >= m.Len() || id.Col < 0 || id.Col >= cols {
			return
		}
		updateCell(m.Item(id.Row), id.Col, cell)
	}

	l := binding.NewDataListener(t.Refresh)
	m.AddListener(l)
	return func() { m.RemoveListener(l) }
}

// BindTreeModel drives a [Tree] from a [model.TreeProxy], setting its Root,
// ChildUIDs and IsBranch callbacks and refreshing the tree whenever the proxy's
// view changes. Because the proxy applies ancestor-keeping filtering and
// per-sibling-group sorting, this is how a tree gets locale-aware ordering and
// live filtering without the app re-deriving the visible hierarchy itself.
//
// The app supplies the tree's CreateNode/UpdateNode as usual. Change the view by
// calling the proxy's SetLess/SetPredicate/Refresh on the main goroutine (use
// [bellui.Do] from a background goroutine), the same rule the widgets already
// follow. The returned function detaches the binding.
//
// Since: 2.8
func BindTreeModel(tree *Tree, p *model.TreeProxy) (unbind func()) {
	tree.Root = p.Root()
	tree.ChildUIDs = p.Children
	tree.IsBranch = p.IsBranch

	l := binding.NewDataListener(tree.Refresh)
	p.AddListener(l) // fires immediately, rendering the current view
	return func() { p.RemoveListener(l) }
}

// BindListSelection keeps a [List]'s selection in sync with a
// [model.SelectionModel] that is keyed by stable item identity, so the selection
// survives a re-sort or re-filter of m: a tap updates the selection model, and
// when the model reorders or the selection changes the list's highlight is
// re-derived from the selected item's current row.
//
// The [List] widget can only paint a single highlighted row; for multi-selection
// have the list's updateItem call [model.SelectionModel.IsIndexSelected] to style
// every selected row itself. This function still tracks the full multi-selection
// in sel — it only reflects the most recent selected row into the built-in
// highlight. The returned function detaches the binding.
//
// Since: 2.8
func BindListSelection[T any, K comparable](list *List, m model.Model[T], sel *model.SelectionModel[T, K]) (unbind func()) {
	applying := false

	list.OnSelected = func(id ListItemID) {
		if applying { // ignore the echo from our own list.Select below
			return
		}
		sel.SelectIndex(id)
	}

	reflect := func() {
		idxs := sel.SelectedIndices()
		applying = true
		if len(idxs) == 0 {
			list.UnselectAll()
		} else {
			list.Select(idxs[len(idxs)-1]) // most recent still-visible row
		}
		applying = false
		list.Refresh() // let updateItem restyle rows for multi-selection
	}

	// React to both selection changes and model reorders (which move rows).
	ls := binding.NewDataListener(reflect)
	sel.AddListener(ls)
	lm := binding.NewDataListener(reflect)
	m.AddListener(lm)

	return func() {
		sel.RemoveListener(ls)
		m.RemoveListener(lm)
		list.OnSelected = nil
	}
}
