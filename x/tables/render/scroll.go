package render

import (
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
)

// ScrollContainer wraps a CellsWidget in a Bellui scroll container.
// It exposes the OnScrolled callback for viewport updates.
type ScrollContainer struct {
	Scroll *container.Scroll
	cells  *CellsWidget
}

// NewScrollContainer creates a scroll container wrapping the cells widget.
// The scroll handles both directions but the vertical scrollbar position
// is managed by the DataGrid layout.
func NewScrollContainer(cells *CellsWidget) *ScrollContainer {
	scroll := container.NewScroll(cells)
	sc := &ScrollContainer{
		Scroll: scroll,
		cells:  cells,
	}
	cells.resized = sc.keepLeadingOffset
	return sc
}

// keepLeadingOffset keeps the scroll's distance from the cells' leading edge
// when they change width. Right to left that edge is the right one, where
// column 0 is, so the cells grow and shrink at their left end, and the Scroll,
// which measures its offset from the left, must move by as much (#779). The
// grid's offset, ScrollOffsetX, is measured from the leading edge. The
// Scroll resizes the cells in its layout, in its own direction, and applies
// this offset right after.
func (sc *ScrollContainer) keepLeadingOffset() {
	if !lang.IsRTLDirection() || sc.cells.Grid == nil {
		return
	}
	far := sc.cells.Size().Width - sc.Scroll.Size().Width
	sc.Scroll.Offset.X = geo.Max(0, far-sc.cells.Grid.ScrollOffsetX())
}

// SetOnScrolled sets the callback invoked when the scroll position changes.
func (sc *ScrollContainer) SetOnScrolled(fn func(pos geo.Position)) {
	sc.Scroll.OnScrolled = fn
}

// ScrollTo scrolls to the given position and fires the OnScrolled callback.
// widget.Scroll only notifies OnScrolled for user-driven scrolling (wheel /
// bar drag), so a programmatic scroll must dispatch it here or the grid's
// cached offset and viewport desync from the real scroll position.
func (sc *ScrollContainer) ScrollTo(pos geo.Position) {
	if sc.Scroll.Offset == pos {
		return
	}
	sc.Scroll.Offset = pos
	if sc.Scroll.OnScrolled != nil {
		sc.Scroll.OnScrolled(pos)
	}
	sc.Scroll.Refresh()
}

// Offset returns the current scroll offset.
func (sc *ScrollContainer) Offset() geo.Position {
	return sc.Scroll.Offset
}
