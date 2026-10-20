package glfw

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiffImages pins the partial-repaint verifier's comparison (#496): what
// it counts, the box it reports, and that a size mismatch counts as entirely
// different rather than as equal.
func TestDiffImages(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 10, 10))
	b := image.NewRGBA(image.Rect(0, 0, 10, 10))
	n, _, _ := diffImages(a, b)
	assert.Zero(t, n)

	b.SetRGBA(2, 3, color.RGBA{R: 7, A: 255})
	b.SetRGBA(8, 5, color.RGBA{G: 1})
	n, box, maxDelta := diffImages(a, b)
	assert.Equal(t, 2, n)
	assert.Equal(t, image.Rect(2, 3, 9, 6), box)
	assert.Equal(t, uint8(255), maxDelta)

	n, box, _ = diffImages(a, image.NewRGBA(image.Rect(0, 0, 10, 11)))
	assert.Equal(t, 100, n)
	assert.Equal(t, a.Bounds(), box)

	// Not an *image.RGBA: converted, then compared.
	g := image.NewGray(image.Rect(0, 0, 10, 10))
	g.SetGray(1, 1, color.Gray{Y: 9})
	n, box, _ = diffImages(a, g)
	assert.Equal(t, 100, n, "a gray image is opaque, a's pixels are transparent")
	assert.Equal(t, a.Bounds(), box)
}
