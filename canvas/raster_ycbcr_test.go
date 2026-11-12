package canvas_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// solidYCbCr builds a 4:2:0 frame of one colour.
func solidYCbCr(w, h int, y, cb, cr uint8) *image.YCbCr {
	img := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for i := range img.Y {
		img.Y[i] = y
	}
	for i := range img.Cb {
		img.Cb[i], img.Cr[i] = cb, cr
	}
	return img
}

// A frame set by SetYCbCr is what the raster shows, converted by the matrix
// and range it was given: BT.709 limited-range 75% red here (the colour bar),
// which BT.601 would darken.
func TestRaster_PixelImage_ConvertsTheYCbCrFrameByItsOwnMatrix(t *testing.T) {
	r := canvas.NewRaster(nil)
	r.Streaming = true
	r.SetYCbCr(solidYCbCr(6, 4, 51, 109, 212), canvas.YCbCrBT709, false)

	img := r.PixelImage(10, 10)
	require.NotNil(t, img)
	assert.Equal(t, image.Rect(0, 0, 6, 4), img.Bounds(), "the frame's own size wins over the requested size")
	c := color.RGBAModel.Convert(img.At(3, 2)).(color.RGBA)
	assert.InDelta(t, 191, int(c.R), 2, "%v", c)
	assert.InDelta(t, 0, int(c.G), 2, "%v", c)
	assert.InDelta(t, 0, int(c.B), 2, "%v", c)
	assert.Equal(t, uint8(0xff), c.A)

	r.SetYCbCr(solidYCbCr(6, 4, 51, 109, 212), canvas.YCbCrBT601, false)
	c = color.RGBAModel.Convert(r.PixelImage(10, 10).At(3, 2)).(color.RGBA)
	assert.Less(t, int(c.R), 185, "the matrix is applied, not assumed: %v", c)
}

// Each setter replaces what the other set, so a raster never shows a stale
// frame of the other kind.
func TestRaster_SetPixelsAndSetYCbCrReplaceEachOther(t *testing.T) {
	r := canvas.NewRaster(nil)
	r.Streaming = true
	green := color.RGBA{G: 255, A: 255}

	r.SetYCbCr(solidYCbCr(4, 4, 51, 109, 212), canvas.YCbCrBT709, false)
	r.SetPixels(solidPixels(2, 2, green), 2, 2)
	assert.Nil(t, r.StreamYCbCr)
	assert.Equal(t, green, color.RGBAModel.Convert(r.PixelImage(4, 4).At(1, 1)))

	r.SetYCbCr(solidYCbCr(4, 4, 51, 109, 212), canvas.YCbCrBT709, false)
	assert.Nil(t, r.StreamPix)
	assert.Equal(t, image.Rect(0, 0, 4, 4), r.PixelImage(2, 2).Bounds())
}
