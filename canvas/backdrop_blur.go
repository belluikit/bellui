package canvas

import "github.com/belluikit/bellui/geo"

// BackdropBlur is a canvas object that captures the content behind it,
// applies a Gaussian blur, and renders the blurred result as a background.
//
// WARNING: This uses GPU readback (ReadPixels) which is expensive.
// Use sparingly — one or two BackdropBlur objects per window is fine,
// but many will impact frame rate.
//
// The blurred content is cached and only re-captured when Refresh() is called.
type BackdropBlur struct {
	baseObject

	// BlurRadius is the blur radius in logical pixels, as for BoxShadow: the
	// content behind is blurred by a Gaussian of sigma BlurRadius/2.
	BlurRadius float32
}

// NewBackdropBlur creates a backdrop blur canvas object.
func NewBackdropBlur(radius float32) *BackdropBlur {
	return &BackdropBlur{BlurRadius: radius}
}

// Refresh marks this backdrop blur for re-capture on the next frame.
func (b *BackdropBlur) Refresh() {
	Refresh(b)
}

// Hide will set this backdrop blur to not be visible.
func (b *BackdropBlur) Hide() {
	b.baseObject.Hide()

	repaint(b)
}

// Move the backdrop blur to a new position, relative to its parent.
func (b *BackdropBlur) Move(pos geo.Position) {
	if b.Position() == pos {
		return
	}

	b.baseObject.Move(pos)

	repaint(b)
}

// Resize sets a new size for the backdrop blur.
func (b *BackdropBlur) Resize(s geo.Size) {
	if s == b.Size() {
		return
	}

	b.baseObject.Resize(s)

	repaint(b)
}

// MinSize returns the minimum size — backdrop blur has no inherent size.
func (b *BackdropBlur) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}
