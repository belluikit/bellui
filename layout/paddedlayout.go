package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Declare conformity with Layout interface
var _ core.Layout = (*paddedLayout)(nil)

type paddedLayout struct{}

// Layout is called to pack all child objects into a specified size.
// For PaddedLayout this sets all children to the full size passed minus padding all around.
func (l paddedLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	padding := theme.Padding()
	pos := geo.NewSquareOffsetPos(padding)
	siz := geo.NewSize(size.Width-2*padding, size.Height-2*padding)
	for _, child := range objects {
		child.Resize(siz)
		child.Move(pos)
	}
}

// MinSize finds the smallest size that satisfies all the child objects.
// For PaddedLayout this is determined simply as the MinSize of the largest child plus padding all around.
func (l paddedLayout) MinSize(objects []core.CanvasObject) (min geo.Size) {
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		min = min.Max(child.MinSize())
	}
	min = min.Add(geo.NewSquareSize(2 * theme.Padding()))
	return min
}

// NewPaddedLayout creates a new PaddedLayout instance
//
// Since: 1.4
func NewPaddedLayout() core.Layout {
	return paddedLayout{}
}
