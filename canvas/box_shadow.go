package canvas

import (
	"image/color"
)

// BoxShadow is a canvas object that renders a blurred shadow behind a widget.
// When the GL painter supports FBO blur, this produces a soft Gaussian-blurred
// shadow. On software painters or when FBO is unavailable, it falls back to a
// solid colored rectangle with reduced opacity.
type BoxShadow struct {
	baseObject

	// Color is the shadow base color (typically black with some alpha).
	Color color.Color

	// BlurRadius controls the Gaussian blur sigma in logical pixels.
	// Larger values produce softer, more spread-out shadows.
	BlurRadius float32

	// OffsetX and OffsetY shift the shadow relative to the widget.
	OffsetX, OffsetY float32

	// CornerRadius matches the parent widget's corner rounding.
	CornerRadius float32
}

// NewBoxShadow creates a new box shadow with the given color and blur radius.
func NewBoxShadow(c color.Color, blurRadius float32) *BoxShadow {
	return &BoxShadow{Color: c, BlurRadius: blurRadius}
}

// Refresh causes this shadow to be redrawn.
func (s *BoxShadow) Refresh() {
	Refresh(s)
}
