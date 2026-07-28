package canvas_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// solidPixels builds w*h premultiplied RGBA bytes of a single colour.
func solidPixels(w, h int, c color.RGBA) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = c.R, c.G, c.B, c.A
	}
	return pix
}

// A raster driven purely by SetPixels has no Generator by design (#34) — asking
// it for its content must yield the streamed pixels, not a nil dereference.
func TestRaster_PixelImage_StreamedPixelsWithoutGenerator(t *testing.T) {
	r := canvas.NewRaster(nil)
	r.Streaming = true

	assert.Nil(t, r.PixelImage(10, 10), "no generator and no pixels yet: no content")

	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	r.SetPixels(solidPixels(4, 3, green), 4, 3)

	img := r.PixelImage(10, 10)
	require.NotNil(t, img)
	assert.Equal(t, image.Rect(0, 0, 4, 3), img.Bounds(), "streamed dimensions win over the requested size")
	assert.Equal(t, green, color.RGBAModel.Convert(img.At(2, 1)))
}

// Streamed pixels take precedence over a Generator, so the fyplayer workaround
// of supplying a Generator purely to dodge the crash still shows live frames.
func TestRaster_PixelImage_StreamedPixelsBeatGenerator(t *testing.T) {
	generated := image.NewRGBA(image.Rect(0, 0, 2, 2))
	r := canvas.NewRaster(func(w, h int) image.Image { return generated })

	red := color.RGBA{R: 255, A: 255}
	r.SetPixels(solidPixels(2, 2, red), 2, 2)

	img := r.PixelImage(2, 2)
	require.NotNil(t, img)
	assert.Equal(t, red, color.RGBAModel.Convert(img.At(0, 0)))
}

func TestRaster_PixelImage_FallsBackToGenerator(t *testing.T) {
	var gotW, gotH int
	want := image.NewRGBA(image.Rect(0, 0, 1, 1))
	r := canvas.NewRaster(func(w, h int) image.Image {
		gotW, gotH = w, h
		return want
	})

	assert.Equal(t, image.Image(want), r.PixelImage(7, 9))
	assert.Equal(t, 7, gotW)
	assert.Equal(t, 9, gotH)
}

// A short or zero-sized buffer must be ignored rather than read out of bounds.
func TestRaster_PixelImage_IgnoresInconsistentPixelData(t *testing.T) {
	r := canvas.NewRaster(nil)

	r.SetPixels(make([]byte, 4*4*4-1), 4, 4) // one byte short of 4x4 RGBA
	assert.Nil(t, r.PixelImage(4, 4), "a short buffer must not be read")

	r.SetPixels(solidPixels(2, 2, color.RGBA{A: 255}), 0, 0)
	assert.Nil(t, r.PixelImage(4, 4), "zero dimensions carry no content")
}

// The returned image must not alias the caller's buffer: a video player reuses
// one frame buffer, so the painter's copy has to be independent.
func TestRaster_PixelImage_CopiesPixels(t *testing.T) {
	pix := solidPixels(2, 2, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	r := canvas.NewRaster(nil)
	r.SetPixels(pix, 2, 2)

	img := r.PixelImage(2, 2)
	require.NotNil(t, img)

	for i := range pix { // caller reuses the buffer for the next frame
		pix[i] = 0
	}
	assert.Equal(t, color.RGBA{R: 10, G: 20, B: 30, A: 255}, color.RGBAModel.Convert(img.At(0, 0)))
}
