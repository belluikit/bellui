package widget

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// shapeEntryChrome finishes the background box and border of an entry whose
// refresh styled the border as an outline (#494). The outlined style keeps
// that. The filled style draws it the MD3 way instead: the box's bottom
// corners are square, and the border becomes the active indicator, a line
// along the bottom edge in the outline's colour. The line is one input border
// thick, or two when thick (focused, or showing an error).
//
// It returns the indicator's thickness, zero for the outlined style, which
// layoutEntryChrome needs.
func shapeEntryChrome(box, border *canvas.Rectangle, filled, thick bool, radius, borderSize float32) float32 {
	if !filled {
		box.CornerRadius = radius
		box.TopLeftCornerRadius, box.TopRightCornerRadius = 0, 0
		border.CornerRadius = radius
		border.FillColor = color.Transparent
		return 0
	}

	box.CornerRadius = 0
	box.TopLeftCornerRadius, box.TopRightCornerRadius = radius, radius
	border.CornerRadius = 0
	border.FillColor = border.StrokeColor
	border.StrokeWidth = 0
	if thick {
		return borderSize * 2
	}
	return borderSize
}

// layoutEntryChrome places an entry's box and border for its size. The box is
// inset by one input border on every side; an outlined border strokes around
// it, and a filled one (indicator > 0) covers the box's bottom edge.
func layoutEntryChrome(box, border *canvas.Rectangle, size geo.Size, borderSize, indicator float32) {
	box.Resize(size.Subtract(geo.NewSquareSize(borderSize * 2)))
	box.Move(geo.NewSquareOffsetPos(borderSize))

	if indicator > 0 {
		border.Resize(geo.NewSize(size.Width-borderSize*2, indicator))
		border.Move(geo.NewPos(borderSize, size.Height-borderSize-indicator))
		return
	}
	// 0.5 is removed so on low DPI it rounds down on the trailing edge
	border.Resize(geo.NewSize(size.Width-borderSize-.5, size.Height-borderSize-.5))
	border.Move(geo.NewSquareOffsetPos(borderSize / 2))
}
