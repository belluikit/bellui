//go:build !no_glfw && !ci && !mobile

// (!ci, like window_test.go: it captures from a real window.)

package glfw

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
)

// boundaryStateRecorder keeps one dirty flag per boundary, as the painters
// do, and records the boundary calls and the capture.
type boundaryStateRecorder struct {
	boundaryClipPainter
	states map[core.CanvasObject]*recordedBoundary
}

type recordedBoundary struct{ dirty bool }

func (b *recordedBoundary) SetDirty(dirty bool) { b.dirty = dirty }

func (p *boundaryStateRecorder) EnsureBoundaryFBO(obj core.CanvasObject, _, _ int, _ geo.Size) gpu.BoundaryFBO {
	if s, ok := p.states[obj]; ok {
		return s
	}
	s := &recordedBoundary{dirty: true}
	p.states[obj] = s
	return s
}

func (p *boundaryStateRecorder) MarkBoundaryDirty(obj core.CanvasObject) {
	if s, ok := p.states[obj]; ok {
		s.dirty = true
	}
}

func (p *boundaryStateRecorder) IsBoundaryClean(obj core.CanvasObject) bool {
	s, ok := p.states[obj]
	return ok && !s.dirty
}

func (p *boundaryStateRecorder) SetFrameBufferScale(float32) {}
func (p *boundaryStateRecorder) Free(core.CanvasObject)      {}

func (p *boundaryStateRecorder) CaptureBack(core.Canvas) image.Image {
	p.calls = append(p.calls, "capture")
	return image.NewRGBA(image.Rect(0, 0, 1, 1))
}

// A capture taken right after a change must draw it, RepaintBoundaries
// included. The GL capture painted without taking the refreshes off the
// queue, so a boundary whose content changed was still clean and was blitted
// from its old image: a scroll in a boundary came out one step behind in
// every capture, while the screen was current (#565).
func TestCapture_RendersRefreshedBoundary(t *testing.T) {
	w := createWindow("capture boundary")
	defer w.Close()
	content := canvas.NewRectangle(color.White)
	content.SetMinSize(geo.NewSize(50, 50))
	boundary := container.NewRepaintBoundary(content)
	w.SetContent(boundary)
	w.Resize(geo.NewSize(100, 100))

	var calls []string
	runOnMain(func() {
		c := w.canvas
		p := &boundaryStateRecorder{states: map[core.CanvasObject]*recordedBoundary{}}
		c.SetPainter(p)
		prepareFrame(w.window) // registers the objects with the canvas, as a frame does
		c.paint(c.Size())      // renders the boundary; it is clean from here
		p.calls = nil

		content.FillColor = color.Black
		c.Refresh(content) // what content.Refresh() does once content is on a canvas
		c.Capture()
		calls = p.calls
	})
	assert.Contains(t, calls, "bind", "the changed boundary must render before the capture reads the frame")
	assert.Equal(t, "capture", calls[len(calls)-1])
}
