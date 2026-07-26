package render

import (
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
)

// ScrollContainer wraps a CellsWidget in a Fyne scroll container.
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
	return &ScrollContainer{
		Scroll: scroll,
		cells:  cells,
	}
}

// SetOnScrolled sets the callback invoked when the scroll position changes.
func (sc *ScrollContainer) SetOnScrolled(fn func(pos geo.Position)) {
	sc.Scroll.OnScrolled = fn
}

// ScrollTo scrolls to the given position.
func (sc *ScrollContainer) ScrollTo(pos geo.Position) {
	sc.Scroll.Offset = pos
	sc.Scroll.Refresh()
}

// Offset returns the current scroll offset.
func (sc *ScrollContainer) Offset() geo.Position {
	return sc.Scroll.Offset
}
