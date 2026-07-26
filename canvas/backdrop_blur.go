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

	// BlurRadius controls the Gaussian blur strength in logical pixels.
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

// MinSize returns the minimum size — backdrop blur has no inherent size.
func (b *BackdropBlur) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}
