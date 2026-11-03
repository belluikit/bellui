//go:build !wasm

package glfw

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/stretchr/testify/assert"
)

// surfacePainter is a WebGPU-like painter that draws nothing and can be told
// its surface is gone, as for a window macOS reports occluded: a paint then
// begins no frame, and its frame count stands still.
type surfacePainter struct {
	surface bool
	paints  int
	frames  uint64
}

var _ gpu.Painter = (*surfacePainter)(nil)

func (p *surfacePainter) FramesBegun() uint64 { return p.frames }

// SetFrameBufferScale is called once per paint, before the walk.
func (p *surfacePainter) SetFrameBufferScale(float32) {
	p.paints++
	if p.surface {
		p.frames++
	}
}

func (p *surfacePainter) Init()                                                  {}
func (p *surfacePainter) Capture(core.Canvas) image.Image                        { return nil }
func (p *surfacePainter) Clear()                                                 {}
func (p *surfacePainter) Free(core.CanvasObject)                                 {}
func (p *surfacePainter) Paint(core.CanvasObject, geo.Position, geo.Size)        {}
func (p *surfacePainter) SetOutputSize(int, int)                                 {}
func (p *surfacePainter) StartClipping(geo.Position, geo.Size)                   {}
func (p *surfacePainter) StopClipping()                                          {}
func (p *surfacePainter) StartRoundedClipping(geo.Position, geo.Size, float32)   {}
func (p *surfacePainter) StopRoundedClipping()                                   {}
func (p *surfacePainter) StartPathClipping(*canvas.Path, geo.Position, geo.Size) {}
func (p *surfacePainter) StopPathClipping()                                      {}
func (p *surfacePainter) SetGroupOpacity(float32)                                {}
func (p *surfacePainter) ClearGroupOpacity()                                     {}
func (p *surfacePainter) SetTransform(geo.Matrix)                                {}

// TestRepaintWindow_RetriesFrameThatDidNotBegin pins #864: a change made
// while the window could not paint used to be lost on screen. The draw loop
// took the canvas' dirty flag though the frame never began, so the window kept
// its old content once it could paint again, until something else changed.
func TestRepaintWindow_RetriesFrameThatDidNotBegin(t *testing.T) {
	p := &surfacePainter{}
	w := &window{canvas: newCanvas(), visible: true}
	w.canvas.SetPainter(p)
	d := &gLDriver{windows: []bellui.Window{w}}

	// What drawSingleFrame does with a dirty window: take the flag, paint.
	w.canvas.SetDirty()
	assert.True(t, w.canvas.CheckDirtyAndClear())
	d.repaintWindow(w)
	assert.Equal(t, 1, p.paints)
	assert.True(t, w.canvas.IsDirty(), "a change no frame showed is still to paint")
	now := time.Now()
	assert.True(t, w.waitingToRetryPaint(now), "the retry waits paintRetry")
	assert.False(t, w.needsPaint(now), "a waiting window does not hold the loop on the frame grid")
	assert.True(t, w.needsPaint(w.paintRetryAt), "it needs its frame once the retry is due")

	// The draw loop leaves a waiting window's flag alone. It has no viewport
	// here, so it would not paint it either way.
	d.drawSingleFrame()
	assert.True(t, w.canvas.IsDirty(), "the draw loop must not take the flag before the retry")

	p.surface = true
	assert.True(t, w.canvas.CheckDirtyAndClear())
	d.repaintWindow(w)
	assert.Equal(t, uint64(1), p.frames)
	assert.False(t, w.canvas.IsDirty(), "the frame that began showed the change")
	assert.True(t, w.paintRetryAt.IsZero(), "nothing is left to retry")
	assert.False(t, w.needsPaint(time.Now()))
}

// TestWithPaintRetry pins that an idle loop wakes for a paint retry due
// before its next tick, and is not held by one due after it.
func TestWithPaintRetry(t *testing.T) {
	now := time.Unix(1000, 0)
	idle := now.Add(idleTick)
	retry := now.Add(paintRetry)

	assert.Equal(t, retry, withPaintRetry(idle, retry), "a retry before the idle tick")
	assert.Equal(t, idle, withPaintRetry(idle, time.Time{}), "no retry waiting")
	frame := now.Add(time.Second / 60)
	assert.Equal(t, frame, withPaintRetry(frame, retry), "a frame due before the retry")
	assert.Less(t, paintRetry, idleTick, "a retry must not wait for the idle tick")
}
