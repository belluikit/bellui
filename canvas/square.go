package canvas

import "image/color"

// NewSquare returns a new Rectangle instance that has a square aspect ratio.
//
// Since: 1.0
func NewSquare(color color.Color) *Rectangle {
	return &Rectangle{
		Aspect:    1,
		FillColor: color,
	}
}
