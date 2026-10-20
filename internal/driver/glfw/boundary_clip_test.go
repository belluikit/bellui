//go:build !no_glfw && !mobile

package glfw

import (
	"fmt"
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

// boundaryClipPainter records the clip and boundary calls of a paint walk.
// Calls the walk is not expected to make hit the nil gpu.Painter and panic.
type boundaryClipPainter struct {
	gpu.Painter
	calls []string
}

type boundaryClipFBO struct{}

func (boundaryClipFBO) SetDirty(bool) {}

func (p *boundaryClipPainter) Clear()                                          {}
func (p *boundaryClipPainter) SetOutputSize(int, int)                          {}
func (p *boundaryClipPainter) SetTransform(geo.Matrix)                         {}
func (p *boundaryClipPainter) Paint(core.CanvasObject, geo.Position, geo.Size) {}
func (p *boundaryClipPainter) StartClipping(pos geo.Position, size geo.Size) {
	p.calls = append(p.calls, fmt.Sprintf("clip %v,%v %vx%v", pos.X, pos.Y, size.Width, size.Height))
}
func (p *boundaryClipPainter) StopClipping() { p.calls = append(p.calls, "noclip") }

func (p *boundaryClipPainter) EnsureBoundaryFBO(core.CanvasObject, int, int, geo.Size) gpu.BoundaryFBO {
	return boundaryClipFBO{}
}

func (p *boundaryClipPainter) BindBoundaryFBO(gpu.BoundaryFBO) { p.calls = append(p.calls, "bind") }

func (p *boundaryClipPainter) UnbindBoundaryFBO(int, int) { p.calls = append(p.calls, "unbind") }

func (p *boundaryClipPainter) BlitBoundaryTexture(gpu.BoundaryFBO, geo.Position, geo.Size) {
	p.calls = append(p.calls, "blit")
}
func (p *boundaryClipPainter) FreeBoundaryFBO(core.CanvasObject)      {}
func (p *boundaryClipPainter) MarkBoundaryDirty(core.CanvasObject)    {}
func (p *boundaryClipPainter) IsBoundaryClean(core.CanvasObject) bool { return false }
func (p *boundaryClipPainter) InvalidateAllBoundaries()               {}

// A dirty RepaintBoundary renders its subtree in boundary-local space, so the
// clips outside it must not reach that render: the scissor stops before the
// bind (GL scissors the bind's clear), the inner scroll's clip is the
// boundary-local rect unintersected with the outer one, and the outer clip is
// back for the blit. Before, the inner clip came out empty and the outer
// scissor clipped the FBO, so a scroll in a boundary painted nothing (#559).
func TestPaint_BoundaryGetsItsOwnClips(t *testing.T) {
	tall := canvas.NewRectangle(color.Black)
	tall.SetMinSize(geo.NewSize(200, 300))
	boundary := container.NewRepaintBoundary(container.NewVScroll(tall))
	outer := container.NewVScroll(container.NewGridWrap(geo.NewSize(200, 80), boundary))
	outer.Move(geo.NewPos(10, 20))
	outer.Resize(geo.NewSize(200, 100))

	c := newCanvas()
	c.SetPadded(false)
	c.SetContent(container.NewWithoutLayout(outer))
	c.Resize(geo.NewSize(300, 300))
	p := &boundaryClipPainter{}
	c.SetPainter(p)

	c.paint(c.Size())

	assert.Equal(t, []string{
		"clip 10,20 200x100", // the outer scroll
		"noclip", "bind",     // the boundary starts unclipped
		"clip 0,0 200x80", // the inner scroll, boundary-local
		"noclip",
		"unbind", "clip 10,20 200x100", "blit", // the outer clip applies to the blit
		"noclip",
	}, p.calls)
}

// boundaryPaintRecorder records which objects the walk paints.
type boundaryPaintRecorder struct {
	boundaryClipPainter
	painted map[core.CanvasObject]bool
}

func (p *boundaryPaintRecorder) Paint(obj core.CanvasObject, _ geo.Position, _ geo.Size) {
	p.painted[obj] = true
}

// A boundary renders its whole subtree into its texture, which is then kept
// while it is clean. Culling that render by a clip outside the boundary left
// out what was off screen at the time, and it stayed out once scrolled into
// view: boundary-wrapped cards below a scroll's viewport never appeared
// (#564). A clip inside the boundary still culls.
func TestPaint_BoundaryRendersContentOffClip(t *testing.T) {
	marker := canvas.NewRectangle(color.White)
	marker.SetMinSize(geo.NewSize(50, 50))
	clipped := container.NewStack(marker)
	clipped.ClipToBounds = true // a clip-owner, which the walk culls off its clip
	boundary := container.NewRepaintBoundary(clipped)

	hidden := canvas.NewRectangle(color.White) // below the inner scroll's view
	hidden.Move(geo.NewPos(0, 400))
	hidden.Resize(geo.NewSize(50, 50))
	innerClip := container.NewWithoutLayout(hidden)
	innerClip.ClipToBounds = true
	innerClip.Resize(geo.NewSize(50, 50))
	hiddenOwner := container.NewStack(canvas.NewRectangle(color.Black))
	hiddenOwner.ClipToBounds = true
	hiddenOwner.Move(geo.NewPos(0, 400))
	hiddenOwner.Resize(geo.NewSize(50, 50))
	innerClip.Add(hiddenOwner)
	inner := container.NewRepaintBoundary(innerClip)

	spacer := canvas.NewRectangle(color.Black)
	spacer.SetMinSize(geo.NewSize(50, 400)) // pushes the boundary below the view
	outer := container.NewVScroll(container.NewVBox(spacer, boundary, inner))
	outer.Resize(geo.NewSize(200, 100))

	c := newCanvas()
	c.SetPadded(false)
	c.SetContent(container.NewWithoutLayout(outer))
	c.Resize(geo.NewSize(300, 300))
	p := &boundaryPaintRecorder{painted: map[core.CanvasObject]bool{}}
	c.SetPainter(p)

	c.paint(c.Size())

	assert.True(t, p.painted[marker], "content off the outer clip must render into its boundary")
	assert.False(t, p.painted[hiddenOwner.Objects[0]], "a clip inside the boundary still culls")
}

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
