package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Declare conformity with Layout interface
var _ core.Layout = (*CustomPaddedLayout)(nil)

// CustomPaddedLayout is a layout similar to PaddedLayout, but uses
// custom values for padding on each side, rather than the theme padding value.
//
// Since: 1.0
type CustomPaddedLayout struct {
	TopPadding    float32
	BottomPadding float32
	LeftPadding   float32
	RightPadding  float32
}

// NewCustomPaddedLayout creates a new CustomPaddedLayout instance
// with the specified paddings.
//
// Since: 1.0
func NewCustomPaddedLayout(padTop, padBottom, padLeft, padRight float32) core.Layout {
	return CustomPaddedLayout{
		TopPadding:    padTop,
		BottomPadding: padBottom,
		LeftPadding:   padLeft,
		RightPadding:  padRight,
	}
}

// Layout is called to pack all child objects into a specified size.
// For CustomPaddedLayout this sets all children to the full size passed minus the given paddings all around.
func (c CustomPaddedLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	pos := geo.NewPos(c.LeftPadding, c.TopPadding)
	siz := geo.Size{
		Width:  size.Width - c.LeftPadding - c.RightPadding,
		Height: size.Height - c.TopPadding - c.BottomPadding,
	}
	for _, child := range objects {
		child.Resize(siz)
		child.Move(pos)
	}
}

// MinSize finds the smallest size that satisfies all the child objects.
// For CustomPaddedLayout this is determined simply as the MinSize of the largest child plus the given paddings all around.
func (c CustomPaddedLayout) MinSize(objects []core.CanvasObject) (min geo.Size) {
	for _, child := range objects {
		if !child.Visible() {
			continue
		}

		min = min.Max(child.MinSize())
	}
	min.Width += c.LeftPadding + c.RightPadding
	min.Height += c.TopPadding + c.BottomPadding
	return min
}
