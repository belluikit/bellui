package render

import "github.com/belluikit/bellui/event"

// RowScroller is what a grid whose rows a touchpad gesture can swipe adds to
// GridAccess (#1483). The widgets that draw the rows, CellsWidget and
// PinnedCellsWidget, are the innermost scrollables under the pointer, so they
// are offered each scroll before the body's Scroll is: they pass it to the
// grid first, and to the Scroll only when the grid did not take it. A row
// swipe keeps every scroll of its gesture that way, even one with a little
// vertical drift that the Scroll could have moved by.
type RowScroller interface {
	// RowScrolled offers the grid a scroll over its rows, and reports
	// whether the grid took it.
	RowScrolled(ev *event.ScrollEvent) bool
}

// rowScrolled offers ev to grid when it is a RowScroller.
func rowScrolled(grid GridAccess, ev *event.ScrollEvent) bool {
	rs, ok := grid.(RowScroller)
	return ok && rs.RowScrolled(ev)
}

// forwardScroll hands a scroll the grid did not take to the grid's own
// Scrolled, which scrolls the body. A path that offers a scroll only to the
// innermost scrollable, rather than along the chain, still scrolls the grid:
// the accessibility walker keys a cell's scroll target on its nearest
// scrollable ancestor, which these widgets now are (#82).
func forwardScroll(grid GridAccess, ev *event.ScrollEvent) {
	if s, ok := grid.(event.Scrollable); ok {
		s.Scrolled(ev)
	}
}

// TryScrolled offers a scroll over the rows to the grid (RowScroller), and
// reports whether it took it. A scroll it leaves goes on to the body's
// Scroll, as it did before the grid's rows could be swiped.
func (c *CellsWidget) TryScrolled(ev *event.ScrollEvent) bool {
	return rowScrolled(c.Grid, ev)
}

// Scrolled is TryScrolled for a caller without propagation: a scroll the grid
// does not take scrolls the grid's body.
func (c *CellsWidget) Scrolled(ev *event.ScrollEvent) {
	if !c.TryScrolled(ev) {
		forwardScroll(c.Grid, ev)
	}
}

// TryScrolled offers a scroll over the pinned rows to the grid, as
// CellsWidget.TryScrolled does.
func (w *PinnedCellsWidget) TryScrolled(ev *event.ScrollEvent) bool {
	return rowScrolled(w.Grid, ev)
}

// Scrolled is TryScrolled for a caller without propagation.
func (w *PinnedCellsWidget) Scrolled(ev *event.ScrollEvent) {
	if !w.TryScrolled(ev) {
		forwardScroll(w.Grid, ev)
	}
}
