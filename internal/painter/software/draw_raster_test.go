package software_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"

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

// A raster driven purely by SetPixels carries no Generator (#34). Painting one
// used to dereference nil; it must now draw the streamed frame.
func TestPainter_paintStreamingRasterWithoutGenerator(t *testing.T) {
	obj := canvas.NewRaster(nil)
	obj.Streaming = true

	c := test.NewCanvas()
	c.SetContent(obj)
	c.Resize(geo.NewSize(10, 10))
	p := software.NewPainter()

	// No pixels supplied yet: nothing to draw, but no crash either.
	require.NotPanics(t, func() { p.Paint(c) })

	blue := color.RGBA{B: 255, A: 255}
	obj.SetPixels(streamPixels(10, 10, blue), 10, 10)

	img := p.Paint(c)
	require.NotNil(t, img)
	assert.Equal(t, blue, color.RGBAModel.Convert(img.At(5, 5)))
}

// The software painter re-reads the raster's pixels every paint, so successive
// frames must actually change what is drawn (the "frozen video" half of #34).
func TestPainter_paintStreamingRasterFollowsFrames(t *testing.T) {
	obj := canvas.NewRaster(nil)
	obj.Streaming = true

	c := test.NewCanvas()
	c.SetContent(obj)
	c.Resize(geo.NewSize(10, 10))
	p := software.NewPainter()

	red := color.RGBA{R: 255, A: 255}
	obj.SetPixels(streamPixels(10, 10, red), 10, 10)
	assert.Equal(t, red, color.RGBAModel.Convert(p.Paint(c).At(5, 5)))

	green := color.RGBA{G: 255, A: 255}
	obj.SetPixels(streamPixels(10, 10, green), 10, 10)
	assert.Equal(t, green, color.RGBAModel.Convert(p.Paint(c).At(5, 5)), "second frame must replace the first")
}

// A raster with neither pixels nor Generator is simply empty.
func TestPainter_paintEmptyRaster(t *testing.T) {
	c := test.NewCanvas()
	c.SetContent(canvas.NewRaster(nil))
	c.Resize(geo.NewSize(10, 10))

	var img image.Image
	require.NotPanics(t, func() { img = software.NewPainter().Paint(c) })
	assert.NotNil(t, img)
}

// Raster.Translucency fades the raster, exactly as it does a canvas.Image. The
// GL painter always honoured it; the software and WebGPU painters passed a
// hard-coded opaque alpha (#38).
func TestPainter_paintRasterTranslucency(t *testing.T) {
	render := func(translucency float64) color.RGBA {
		obj := canvas.NewRaster(nil)
		obj.Streaming = true
		obj.Translucency = translucency
		obj.SetPixels(streamPixels(10, 10, color.RGBA{R: 255, A: 255}), 10, 10)

		c := test.NewCanvas()
		c.SetContent(obj)
		c.Resize(geo.NewSize(10, 10))
		return color.RGBAModel.Convert(software.NewPainter().Paint(c).At(5, 5)).(color.RGBA)
	}

	opaque := render(0)
	require.Equal(t, uint8(255), opaque.A, "an opaque raster must stay opaque")

	faded := render(0.5)
	assert.Less(t, faded.A, opaque.A, "Translucency 0.5 must fade the raster")

	clear := render(1)
	assert.Less(t, clear.A, faded.A, "Translucency 1.0 must fade it further still")
}
