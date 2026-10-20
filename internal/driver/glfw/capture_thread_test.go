//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image"
	"image/color"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/gpu"
)

// threadProbe wraps the window's painter and counts the paints that run off
// the main goroutine.
type threadProbe struct {
	gpu.Painter
	paints, offMain atomic.Int32
}

func (p *threadProbe) Paint(obj core.CanvasObject, pos geo.Position, size geo.Size) {
	p.paints.Add(1)
	if !async.IsMainGoroutine() {
		p.offMain.Add(1)
	}
	p.Painter.Paint(obj, pos, size)
}

// glThreadProbe keeps the GL painter's back-buffer read, so Capture takes the
// GL path.
type glThreadProbe struct{ *threadProbe }

func (p glThreadProbe) CaptureBack(c core.Canvas) image.Image {
	return p.Painter.(interface{ CaptureBack(core.Canvas) image.Image }).CaptureBack(c)
}

// webgpuThreadProbe keeps the WebGPU painter's capture copy and present, so
// Capture takes the WebGPU path.
type webgpuThreadProbe struct{ *threadProbe }

func (p webgpuThreadProbe) EnableCapture() { p.Painter.(interface{ EnableCapture() }).EnableCapture() }
func (p webgpuThreadProbe) Present()       { p.Painter.(interface{ Present() }).Present() }

// TestCanvas_CaptureFromGoroutinePaintsOnMain: a Capture called from a
// goroutine paints on the main thread, which owns the window's GL context and
// paints the window every frame (#562). It paints on the caller's thread
// otherwise, where the GL calls could find no context current or race the
// render loop.
func TestCanvas_CaptureFromGoroutinePaintsOnMain(t *testing.T) {
	w := createWindow("Capture thread")
	defer w.Close()
	w.SetPadded(false)
	w.SetContent(canvas.NewRectangle(color.NRGBA{R: 200, A: 255}))
	w.Resize(geo.NewSize(100, 100))
	repaintWindow(w)

	probe := &threadProbe{}
	runOnMain(func() {
		probe.Painter = w.canvas.Painter()
		switch probe.Painter.(type) {
		case interface{ EnableCapture() }:
			w.canvas.SetPainter(webgpuThreadProbe{probe})
		case interface{ CaptureBack(core.Canvas) image.Image }:
			w.canvas.SetPainter(glThreadProbe{probe})
		default:
			w.canvas.SetPainter(probe)
		}
	})
	defer runOnMain(func() { w.canvas.SetPainter(probe.Painter) })

	require.False(t, async.IsMainGoroutine(), "the test runs off the main goroutine")
	img := w.canvas.Capture()

	require.NotNil(t, img)
	assert.NotZero(t, probe.paints.Load(), "Capture paints a fresh frame")
	assert.Zero(t, probe.offMain.Load(), "no paint ran off the main goroutine")
}
