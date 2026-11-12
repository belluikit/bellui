//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/ycbcr/ycbcrtest"
)

const yuvBarW, yuvBarH = 8, 8

// A raster showing Y'CbCr colour bars draws them in the right colours, by the
// matrix it was given: BT.601 and BT.709 limited range, the done-when of
// #1107. The first frame creates the texture, the next ones convert into it in
// place, and a frame of another size replaces it.
func TestYCbCrRasterDrawsTheColourBars(t *testing.T) {
	w := len(ycbcrtest.BT601Limited) * yuvBarW
	o := newOffscreenPainterSized(t, w, yuvBarH)
	rast := canvas.NewRaster(nil)
	rast.Streaming = true
	rast.Resize(geo.NewSize(float32(w), yuvBarH))
	t.Cleanup(func() { cache.DeleteTexture(rast) })
	frame := geo.NewSize(float32(w), yuvBarH)
	draw := func() { o.drawRaster(rast, geo.Position{}, frame) }

	rast.SetYCbCr(ycbcrtest.Frame(ycbcrtest.BT601Limited, yuvBarW, yuvBarH), canvas.YCbCrBT601, false)
	ycbcrtest.AssertBars(t, ycbcrtest.BT601Limited, yuvBarW, o.frame(t, draw), 3)
	id, ok := cache.GetTexture(rast)
	require.True(t, ok)
	first := o.textures[uint32(id)]
	require.NotNil(t, first.yuv, "the frame was converted on the GPU, not through PixelImage")

	second := ycbcrtest.Reversed(ycbcrtest.BT709Limited)
	rast.SetYCbCr(ycbcrtest.Frame(second, yuvBarW, yuvBarH), canvas.YCbCrBT709, false)
	ycbcrtest.AssertBars(t, second, yuvBarW, o.frame(t, draw), 3)
	id, _ = cache.GetTexture(rast)
	assert.Same(t, first, o.textures[uint32(id)], "a same-size frame converts into the same texture")

	// A smaller frame: the texture is replaced, and the bars stretch to
	// fill the raster.
	half := ycbcrtest.Frame(ycbcrtest.BT709Limited, yuvBarW, yuvBarH).SubImage(image.Rect(0, 0, w/2, yuvBarH)).(*image.YCbCr)
	rast.SetYCbCr(half, canvas.YCbCrBT709, false)
	o.frame(t, draw) // drops the old texture
	got := o.frame(t, draw)
	id, _ = cache.GetTexture(rast)
	require.NotNil(t, o.textures[uint32(id)].yuv)
	assert.NotSame(t, first, o.textures[uint32(id)])
	c := got.RGBAAt(3*yuvBarW, yuvBarH/2) // bar 1 of the half, stretched ×2
	assert.InDelta(t, 191, int(c.R), 3, "%v", c)
	assert.InDelta(t, 191, int(c.G), 3, "%v", c)
	assert.InDelta(t, 0, int(c.B), 3, "%v", c)
}
