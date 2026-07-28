//go:build !gl

package webgpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func streamPixels(w, h int, c color.RGBA) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = c.R, c.G, c.B, c.A
	}
	return pix
}

// rasterizeRaster used to call Generator unconditionally, so a raster driven
// purely by SetPixels — which has no Generator by design — segfaulted on its
// first paint (#34).
func TestRasterizeRaster_NilGenerator(t *testing.T) {
	p := &painter{pixScale: 1}
	rast := canvas.NewRaster(nil)
	rast.Streaming = true
	rast.Resize(geo.NewSize(8, 8))

	var img image.Image
	require.NotPanics(t, func() { img = p.rasterizeRaster(rast) })
	assert.Nil(t, img, "no pixels yet: nothing to upload")

	red := color.RGBA{R: 255, A: 255}
	rast.SetPixels(streamPixels(8, 8, red), 8, 8)

	require.NotPanics(t, func() { img = p.rasterizeRaster(rast) })
	require.NotNil(t, img, "the streamed frame is the texture source")
	assert.Equal(t, red, color.RGBAModel.Convert(img.At(4, 4)))
}

// Before any texture exists there is nothing to update in place; getTexture
// creates one from the current pixels on the same paint. This must not touch
// the device (which is nil before the first successful Init on mobile).
func TestUpdateStreamingTexture_NoCachedTexture(t *testing.T) {
	p := &painter{pixScale: 1}
	rast := canvas.NewRaster(nil)
	rast.Streaming = true
	rast.Resize(geo.NewSize(4, 4))
	rast.SetPixels(streamPixels(4, 4, color.RGBA{G: 255, A: 255}), 4, 4)

	require.NotPanics(t, func() { p.updateStreamingTexture(rast) })

	// A cache handle with no matching painter-local texture is the same no-op.
	cache.SetTexture(rast, cache.TextureType(7), nil)
	t.Cleanup(func() { cache.DeleteTexture(rast) })
	require.NotPanics(t, func() { p.updateStreamingTexture(rast) })
}

func TestUpdateStreamingTexture_GeneratorOnlyRasterIsSafe(t *testing.T) {
	p := &painter{pixScale: 1}
	rast := canvas.NewRaster(nil) // streaming, but neither pixels nor generator
	rast.Streaming = true
	rast.Resize(geo.NewSize(4, 4))

	assert.Nil(t, p.streamingGeneratorPixels(rast))

	rast.Generator = func(w, h int) image.Image { return nil }
	assert.Nil(t, p.streamingGeneratorPixels(rast), "a generator may legitimately return nil")

	rast.Generator = func(w, h int) image.Image { return image.NewNRGBA(image.Rect(0, 0, w, h)) }
	rgba := p.streamingGeneratorPixels(rast)
	require.NotNil(t, rgba)
	assert.Equal(t, image.Rect(0, 0, 4, 4), rgba.Rect, "non-RGBA output is converted, not rejected")
}

func TestValidateTextureWrite(t *testing.T) {
	fourByFour := &Texture{w: 4, h: 4}
	pix := streamPixels(4, 4, color.RGBA{A: 255})

	assert.EqualError(t, validateTextureWrite(nil, pix, 4, 4), "no texture to write")
	assert.EqualError(t, validateTextureWrite(fourByFour, pix, 0, 4), "invalid pixel size 0x4")
	assert.EqualError(t, validateTextureWrite(fourByFour, pix, 8, 8),
		"size mismatch: texture is 4x4, pixels are 8x8")
	assert.EqualError(t, validateTextureWrite(fourByFour, pix[:len(pix)-1], 4, 4),
		"short pixel buffer: have 63 bytes, need 64")
	// Everything coherent — only the (untestable without a GPU) handle is missing.
	assert.EqualError(t, validateTextureWrite(fourByFour, pix, 4, 4), "texture has no GPU handle")
}

func TestWriteTexturePixels_NoDevice(t *testing.T) {
	var d *Device
	err := d.WriteTexturePixels(&Texture{w: 1, h: 1}, streamPixels(1, 1, color.RGBA{}), 1, 1)
	assert.Error(t, err, "a nil device must report, not panic")
}
