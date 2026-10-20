//go:build !no_glfw && !mobile

package glfw

import (
	"fmt"
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
