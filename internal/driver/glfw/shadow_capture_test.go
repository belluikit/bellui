//go:build !no_glfw && !ci && !mobile

// (!ci, like window_test.go: it captures from a real window.)

package glfw

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
)

// A box shadow's edge falls off as the box blurred by a Gaussian of sigma
// BlurRadius/2, with either painter (#442): built with -tags gl this checks
// the GL painter, which no CI leg draws with. Both painters left the shadow a
// hard-edged box. The WebGPU painter's own test,
// TestBoxShadowEdgeProfile, checks the profile more closely.
func TestCapture_BoxShadowEdgeProfile(t *testing.T) {
	const radius = 12
	bg := canvas.NewRectangle(color.White)
	bg.Resize(geo.NewSize(300, 200))
	shadow := canvas.NewBoxShadow(color.NRGBA{A: 255}, radius)
	shadow.Resize(geo.NewSize(120, 80))
	shadow.Move(geo.NewPos(60, 60))

	w := createWindow("shadow edge")
	defer w.Close()
	w.SetPadded(false)
	w.SetContent(container.NewWithoutLayout(bg, shadow))
	runOnMain(func() { w.window.Show() })
	w.Resize(geo.NewSize(300, 200))
	ensureCanvasSizeOrSkip(t, w, geo.NewSize(300, 200))
	repaintWindow(w)
	repaintWindow(w)

	img := captureOrSkip(t, w)
	scale := float64(img.Bounds().Dx()) / float64(w.Canvas().Size().Width)
	y := int(100 * scale)
	// Opaque black over white leaves red at 1 - coverage.
	coverage := func(x int) float64 {
		return 1 - float64(color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).R)/255
	}
	require.Greater(t, coverage(int(120*scale)), 0.95, "the capture must show the shadow")

	// In device pixels: the box spans x 60..180 and y 60..140.
	sigma := radius / 2 * scale
	inside := func(c, lo, hi float64) float64 {
		return 0.5 * (math.Erf((hi-c)/(sigma*math.Sqrt2)) - math.Erf((lo-c)/(sigma*math.Sqrt2)))
	}
	edge := 180 * scale
	worst := 0.0
	for x := int(edge - 3*sigma); x < int(edge+3*sigma); x++ {
		want := inside(float64(x)+0.5, 60*scale, edge) * inside(float64(y)+0.5, 60*scale, 140*scale)
		worst = max(worst, math.Abs(coverage(x)-want))
	}
	assert.Less(t, worst, 0.04, "the edge falls off as a Gaussian of sigma radius/2")
}

// A coloured box shadow is drawn in its colour by either painter: GL blended
// the premultiplied shadow as if it were not, multiplying its colour by its
// alpha twice, so half-transparent red over white came out (191, 127, 127)
// where WebGPU drew (255, 127, 127) (#442).
func TestCapture_BoxShadowColor(t *testing.T) {
	bg := canvas.NewRectangle(color.White)
	bg.Resize(geo.NewSize(300, 200))
	shadow := canvas.NewBoxShadow(color.NRGBA{R: 255, A: 128}, 12)
	shadow.Resize(geo.NewSize(120, 80))
	shadow.Move(geo.NewPos(60, 60))

	w := createWindow("shadow colour")
	defer w.Close()
	w.SetPadded(false)
	w.SetContent(container.NewWithoutLayout(bg, shadow))
	runOnMain(func() { w.window.Show() })
	w.Resize(geo.NewSize(300, 200))
	ensureCanvasSizeOrSkip(t, w, geo.NewSize(300, 200))
	repaintWindow(w)
	repaintWindow(w)

	img := captureOrSkip(t, w)
	scale := float64(img.Bounds().Dx()) / float64(w.Canvas().Size().Width)
	middle := color.NRGBAModel.Convert(img.At(int(120*scale), int(100*scale))).(color.NRGBA)
	assert.InDelta(t, 255, int(middle.R), 1, "red stays red")
	assert.InDelta(t, 127, int(middle.G), 2, "half covers the white")
}
