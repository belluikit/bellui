package widget

import (
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// SortDirection is the order a column is sorted in by a [TableSorter].
//
// Since: 2.8
type SortDirection int

const (
	// SortNone means the column does not contribute to the sort order.
	SortNone SortDirection = iota
	// SortAscending sorts the column from smallest to largest.
	SortAscending
	// SortDescending sorts the column from largest to smallest.
	SortDescending
)

// SortableColumn describes one sortable column for [BindTableSort]: a header
// Title and a Less comparator that orders two row items by this column in
// ascending order (return true when a should sort before b). For descending
// order the [TableSorter] inverts the result, so callers write Less once.
//
// For user-visible text use [model.StringLess] for locale-aware ordering, e.g.
//
//	{Title: "Name", Less: func(a, b person) bool { return model.StringLess(a.Name, b.Name) }}
//
// A nil Less makes the column inert (its header is shown but clicking it does
// nothing), which is convenient for display-only columns.
//
// Since: 2.8
type SortableColumn[T any] struct {
	Title string
	Less  func(a, b T) bool
}

// ColumnSort reports one active sort key: the column index and its direction.
//
// Since: 2.8
type ColumnSort struct {
	Column    int
	Direction SortDirection
}

// tableSortKey is the internal, ordered representation of one active sort key.
type tableSortKey struct {
	col int
	dir SortDirection
}

// TableSorter wires a [Table]'s header row to a [model.SortProxy], turning a
// header click into a change of the proxy's comparator. It is returned by
// [BindTableSort].
//
// Clicking a column header cycles that column through ascending → descending →
// unsorted and makes it the primary sort key; columns clicked earlier stay
// active as stable tie-breakers, so clicking column A then column B sorts by B
// then A (the proxy sorts stably). The composed comparator is recomputed and
// pushed to the proxy on every change, which re-sorts the rows and refreshes the
// header arrows.
//
// A TableSorter mutates widget state; call its methods on the main goroutine
// (use [bellui.Do] from a background goroutine), the same rule the widgets follow.
//
// Since: 2.8
type TableSorter[T any] struct {
	table *Table
	proxy *model.SortProxy[T]
	cols  []SortableColumn[T]
	keys  []tableSortKey // primary key first
}

// BindTableSort adds header-click sorting to t, driving proxy's comparator from
// the descriptors in cols. It enables t.ShowHeaderRow and installs CreateHeader
// and UpdateHeader that render each column's Title with an ascending/descending
// arrow and a click target; it does not touch t.Length/CreateCell/UpdateCell,
// so it composes with [BindTableModel] over the same proxy:
//
//	proxy := model.NewSortProxy[person](src)
//	widget.BindTableModel(table, proxy, 2, createCell, updateCell)
//	sorter := widget.BindTableSort(table, proxy, []widget.SortableColumn[person]{
//		{Title: "Name", Less: func(a, b person) bool { return a.Name < b.Name }},
//		{Title: "Age", Less: func(a, b person) bool { return a.Age < b.Age }},
//	})
//
// The returned [TableSorter] also drives the sort programmatically
// ([TableSorter.SortByColumn], [TableSorter.Clear]) and reports the current
// state ([TableSorter.Sorts], [TableSorter.DirectionOf]).
//
// Since: 2.8
func BindTableSort[T any](t *Table, proxy *model.SortProxy[T], cols []SortableColumn[T]) *TableSorter[T] {
	s := &TableSorter[T]{table: t, proxy: proxy, cols: cols}

	t.ShowHeaderRow = true
	t.CreateHeader = func() core.CanvasObject { return newTableSortHeader() }
	t.UpdateHeader = func(id TableCellID, o core.CanvasObject) {
		h, ok := o.(*tableSortHeader)
		if !ok {
			return
		}
		if id.Row < 0 && id.Col >= 0 && id.Col < len(s.cols) {
			col := id.Col
			h.onTapped = func() { s.SortByColumn(col) }
			h.set(s.cols[col].Title, s.DirectionOf(col))
			return
		}

		// Row-number header (ShowHeaderColumn) or the corner: no sort affordance.
		h.onTapped = nil
		if id.Col < 0 && id.Row >= 0 {
			h.set(strconv.Itoa(id.Row+1), SortNone)
		} else {
			h.set("", SortNone)
		}
	}

	return s
}

// SortByColumn advances the sort state of the given column as if its header were
// clicked: the column cycles ascending → descending → unsorted and becomes the
// primary key. Out-of-range columns and columns with a nil Less are ignored.
func (s *TableSorter[T]) SortByColumn(col int) {
	if col < 0 || col >= len(s.cols) || s.cols[col].Less == nil {
		return
	}

	next := nextSortDirection(s.DirectionOf(col))
	keys := make([]tableSortKey, 0, len(s.keys)+1)
	if next != SortNone {
		keys = append(keys, tableSortKey{col: col, dir: next})
	}
	for _, k := range s.keys {
		if k.col != col {
			keys = append(keys, k)
		}
	}
	s.keys = keys
	s.apply()
}

// Clear removes all active sort keys, restoring the proxy's source order.
func (s *TableSorter[T]) Clear() {
	if len(s.keys) == 0 {
		return
	}
	s.keys = nil
	s.apply()
}

// DirectionOf reports the current [SortDirection] for a column, or [SortNone] if
// the column is not part of the active sort.
func (s *TableSorter[T]) DirectionOf(col int) SortDirection {
	for _, k := range s.keys {
		if k.col == col {
			return k.dir
		}
	}
	return SortNone
}

// Sorts returns the active sort keys in priority order (primary first). The most
// recently clicked column is primary; earlier clicks act as stable tie-breakers.
func (s *TableSorter[T]) Sorts() []ColumnSort {
	out := make([]ColumnSort, len(s.keys))
	for i, k := range s.keys {
		out[i] = ColumnSort{Column: k.col, Direction: k.dir}
	}
	return out
}

// Unbind detaches the sorter from the table by clearing the header callbacks it
// installed. It leaves the proxy comparator untouched.
func (s *TableSorter[T]) Unbind() {
	s.table.CreateHeader = nil
	s.table.UpdateHeader = nil
}

// apply composes the active keys into a single comparator and pushes it to the
// proxy (nil when there is no active key, restoring source order). Setting the
// comparator re-sorts and notifies, which refreshes the bound table — including
// re-running UpdateHeader so the arrows track the new state.
func (s *TableSorter[T]) apply() {
	if len(s.keys) == 0 {
		s.proxy.SetLess(nil)
		return
	}

	keys := append([]tableSortKey(nil), s.keys...)
	cols := s.cols
	s.proxy.SetLess(func(a, b T) bool {
		for _, k := range keys {
			less := cols[k.col].Less
			if less == nil {
				continue
			}
			ab := less(a, b)
			ba := less(b, a)
			if !ab && !ba {
				continue // equal on this key, fall through to the next
			}
			if k.dir == SortDescending {
				return ba
			}
			return ab
		}
		return false
	})
}

// nextSortDirection is the header-click cycle: none → asc → desc → none.
func nextSortDirection(d SortDirection) SortDirection {
	switch d {
	case SortNone:
		return SortAscending
	case SortAscending:
		return SortDescending
	default:
		return SortNone
	}
}

// tableSortHeader is the tappable header cell installed by BindTableSort. It is a
// Label showing the column title plus a sort arrow; being Tappable it receives
// the header click (data cells are deliberately not Tappable so the Table keeps
// cell input, but nothing consumes header taps, so the arrow widget can).
type tableSortHeader struct {
	BaseWidget

	label    *Label
	onTapped func()
}

func newTableSortHeader() *tableSortHeader {
	h := &tableSortHeader{label: NewLabel("")}
	h.label.TextStyle.Bold = true
	h.label.Alignment = bellui.TextAlignCenter
	h.ExtendBaseWidget(h)
	return h
}

// set updates the shown title and appends an arrow for the sort direction.
func (h *tableSortHeader) set(title string, dir SortDirection) {
	switch dir {
	case SortAscending:
		title += " ↑" // up arrow
	case SortDescending:
		title += " ↓" // down arrow
	}
	h.label.SetText(title)
}

// Tapped implements event.Tappable, forwarding to the column's sort handler.
func (h *tableSortHeader) Tapped(*event.PointEvent) {
	if h.onTapped != nil {
		h.onTapped()
	}
}

// Cursor shows a pointer over sortable headers as a click affordance.
func (h *tableSortHeader) Cursor() desktop.Cursor {
	if h.onTapped != nil {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

func (h *tableSortHeader) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	return NewSimpleRenderer(h.label)
}
