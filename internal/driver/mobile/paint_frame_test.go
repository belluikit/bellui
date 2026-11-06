//go:build !wasm

package mobile

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	bcanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
)

// framePainter is a painter whose frame opens only when it has a surface, as
// the WebGPU painter's does, and that counts the draws the walk makes.
type framePainter struct {
	surface bool
	draws   int
}

var (
	_ gpu.Painter            = (*framePainter)(nil)
	_ gpu.DirtyRegionPainter = (*framePainter)(nil)
	_ gpu.FramePainter       = (*framePainter)(nil)
)

func (p *framePainter) FrameOpen() bool { return p.surface }

func (p *framePainter) Paint(core.CanvasObject, geo.Position, geo.Size) { p.draws++ }

func (p *framePainter) Init()                                                   {}
func (p *framePainter) Capture(core.Canvas) image.Image                         { return nil }
func (p *framePainter) Clear()                                                  {}
func (p *framePainter) Free(core.CanvasObject)                                  {}
func (p *framePainter) SetFrameBufferScale(float32)                             {}
func (p *framePainter) SetOutputSize(int, int)                                  {}
func (p *framePainter) StartClipping(geo.Position, geo.Size)                    {}
func (p *framePainter) StopClipping()                                           {}
func (p *framePainter) StartRoundedClipping(geo.Position, geo.Size, float32)    {}
func (p *framePainter) StopRoundedClipping()                                    {}
func (p *framePainter) StartPathClipping(*bcanvas.Path, geo.Position, geo.Size) {}
func (p *framePainter) StopPathClipping()                                       {}
func (p *framePainter) SetGroupOpacity(float32)                                 {}
func (p *framePainter) ClearGroupOpacity()                                      {}
func (p *framePainter) SetTransform(geo.Matrix)                                 {}
func (p *framePainter) EnsureCanvasFBO(int, int)                                {}
func (p *framePainter) BindCanvasFBO()                                          {}
func (p *framePainter) UnbindAndBlitCanvasFBO(int, int)                         {}
func (p *framePainter) HasCanvasFBO() bool                                      { return true }
func (p *framePainter) CanvasFBONeedsFullClear() bool                           { return true }
func (p *framePainter) MarkCanvasFBOCleared()                                   {}

// TestPaintWindow_DoesNotWalkWithoutFrame pins #1019 on mobile: a paint whose
// frame did not begin walked the window all the same, and every draw in it
// was refused.
func TestPaintWindow_DoesNotWalkWithoutFrame(t *testing.T) {
	w := d.CreateWindow("No frame")
	defer w.Close()
	w.SetContent(bcanvas.NewRectangle(nil))
	c := w.Canvas().(*canvas)
	p := &framePainter{}
	c.SetPainter(p)
	size := geo.NewSize(100, 100)

	d.paintWindow(w, size)
	assert.Zero(t, p.draws, "no draw is made without a frame")

	p.surface = true
	d.paintWindow(w, size)
	assert.NotZero(t, p.draws, "the walk draws once the frame is open")
}
