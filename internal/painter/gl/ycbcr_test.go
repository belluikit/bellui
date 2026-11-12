//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !android && !ios && !mobile && !wasm && !no_glfw

package gl

import (
	"image"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/ycbcr/ycbcrtest"
)

// The GL painter draws a raster's Y'CbCr colour bars in the right colours,
// converted on the processor (#1107). The first frame creates the texture; the
// second goes through the streaming update, which once only knew RGBA and a
// Generator, so a Y'CbCr raster froze on its first frame.
func TestYCbCrRasterDrawsTheColourBars(t *testing.T) {
	const barW, h = 8, 8
	o := newOffscreenPainter(t)
	w := len(ycbcrtest.BT601Limited) * barW
	rast := canvas.NewRaster(nil)
	rast.Streaming = true
	rast.Resize(geo.NewSize(float32(w), h))
	t.Cleanup(func() { cache.DeleteTexture(rast) })
	at := image.Rect(10, 10, 10+w, 10+h)

	rast.SetYCbCr(ycbcrtest.Frame(ycbcrtest.BT601Limited, barW, h), canvas.YCbCrBT601, false)
	ycbcrtest.AssertBars(t, ycbcrtest.BT601Limited, barW, o.paint(rast).SubImage(at), 2)

	second := ycbcrtest.Reversed(ycbcrtest.BT709Limited)
	rast.SetYCbCr(ycbcrtest.Frame(second, barW, h), canvas.YCbCrBT709, false)
	ycbcrtest.AssertBars(t, second, barW, o.paint(rast).SubImage(at), 2)
}
