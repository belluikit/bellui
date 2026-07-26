package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with Layout interface.
var _ core.Layout = (*shrinkLayout)(nil)

// shrinkLayout reports a minimum size of zero while still filling all
// available space. This allows parent containers such as Split, VScroll, or
// AppTabs to shrink below the natural minimum size of their children.
type shrinkLayout struct{}

// NewShrinkLayout creates a layout that reports MinSize (0, 0) and positions
// every child at position (0, 0) with the full container size. This is useful
// when a resizable parent (Split, Scroll, AppTabs) must be able to shrink
// below the natural minimum size of its content.
//
// Since: 2.7
func NewShrinkLayout() core.Layout {
	return &shrinkLayout{}
}

func (s *shrinkLayout) MinSize(_ []core.CanvasObject) geo.Size {
	return geo.NewSize(0, 0)
}

func (s *shrinkLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	for _, o := range objects {
		o.Move(geo.NewPos(0, 0))
		o.Resize(size)
	}
}
