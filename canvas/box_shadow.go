package canvas

import (
	"image/color"

	"github.com/belluikit/bellui/geo"
)

// BoxShadow is a canvas object that renders a blurred shadow behind a widget:
// its rounded rect, in Color, blurred by a Gaussian. The GPU painters
// approximate the Gaussian with a 9-tap blur; the software painter, which
// draws headless captures and test images, computes it exactly, within a few
// levels of the GPU result (hack/shadowprobe). The GL painter draws nothing on
// a target without framebuffers (wasm), and no painter draws a shadow whose
// BlurRadius is 0.
type BoxShadow struct {
	baseObject

	// Color is the shadow base color (typically black with some alpha).
	Color color.Color

	// BlurRadius is the blur radius in logical pixels, as in CSS box-shadow:
	// the shadow is blurred by a Gaussian of sigma BlurRadius/2, so its edge
	// fades over about BlurRadius either side. Larger values produce softer,
	// more spread-out shadows.
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

// Hide will set this shadow to not be visible.
func (s *BoxShadow) Hide() {
	s.baseObject.Hide()

	repaint(s)
}

// Move the shadow to a new position, relative to its parent.
func (s *BoxShadow) Move(pos geo.Position) {
	if s.Position() == pos {
		return
	}

	s.baseObject.Move(pos)

	repaint(s)
}

// Resize sets a new size for the shadow.
func (s *BoxShadow) Resize(size geo.Size) {
	if size == s.Size() {
		return
	}

	s.baseObject.Resize(size)

	repaint(s)
}
