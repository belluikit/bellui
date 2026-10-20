//go:build (!windows || !ci) && !wasm

package mobile

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
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
func (p *boundaryClipPainter) UnbindBoundaryFBO(int, int)      { p.calls = append(p.calls, "unbind") }

func (p *boundaryClipPainter) BlitBoundaryTexture(gpu.BoundaryFBO, geo.Position, geo.Size) {
	p.calls = append(p.calls, "blit")
}
func (p *boundaryClipPainter) FreeBoundaryFBO(core.CanvasObject)      {}
func (p *boundaryClipPainter) MarkBoundaryDirty(core.CanvasObject)    {}
func (p *boundaryClipPainter) IsBoundaryClean(core.CanvasObject) bool { return false }
func (p *boundaryClipPainter) InvalidateAllBoundaries()               {}

// The mobile walk's half of the glfw TestPaint_BoundaryGetsItsOwnClips: a
// dirty RepaintBoundary renders with a clip stack of its own, the scissor
// stops before the bind, and the outer clip is back for the blit (#559).
func Test_mobileTreeWalker_BoundaryGetsItsOwnClips(t *testing.T) {
	tall := belluicanvas.NewRectangle(color.Black)
	tall.SetMinSize(geo.NewSize(200, 300))
	boundary := container.NewRepaintBoundary(container.NewVScroll(tall))
	outer := container.NewVScroll(container.NewGridWrap(geo.NewSize(200, 80), boundary))
	outer.Move(geo.NewPos(10, 20))
	outer.Resize(geo.NewSize(200, 100))

	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(container.NewWithoutLayout(outer))
	c.Resize(geo.NewSize(300, 300))
	p := &boundaryClipPainter{}
	c.SetPainter(p)

	walk := &mobileTreeWalker{
		c:           c,
		clips:       &internal.ClipStack{},
		size:        c.Size(),
		bp:          p,
		hasBoundary: true,
	}
	c.WalkTrees(walk.draw, walk.afterDraw)

	assert.Equal(t, []string{
		"clip 10,20 200x100", // the outer scroll
		"noclip", "bind",     // the boundary starts unclipped
		"clip 1,1 200x80", // the inner scroll, boundary-local, past the texture's 2 px (1 pt at scale 2) margin (#570)
		"noclip",
		"unbind", "clip 10,20 200x100", "blit", // the outer clip applies to the blit
		"noclip",
	}, p.calls)
}

// boundaryStateRecorder keeps one dirty flag per boundary, as the painters
// do, records what it paints and where it blits.
type boundaryStateRecorder struct {
	boundaryClipPainter
	states  map[core.CanvasObject]*recordedBoundary
	painted map[core.CanvasObject]bool
}

type recordedBoundary struct{ dirty bool }

func (b *recordedBoundary) SetDirty(dirty bool) { b.dirty = dirty }

func (p *boundaryStateRecorder) Paint(obj core.CanvasObject, _ geo.Position, _ geo.Size) {
	p.painted[obj] = true
}

func (p *boundaryStateRecorder) EnsureBoundaryFBO(obj core.CanvasObject, _, _ int, _ geo.Size) gpu.BoundaryFBO {
	if s, ok := p.states[obj]; ok {
		return s
	}
	s := &recordedBoundary{dirty: true}
	p.states[obj] = s
	return s
}

func (p *boundaryStateRecorder) BlitBoundaryTexture(_ gpu.BoundaryFBO, pos geo.Position, _ geo.Size) {
	p.calls = append(p.calls, fmt.Sprintf("blit %v,%v", pos.X, pos.Y))
}

func (p *boundaryStateRecorder) IsBoundaryClean(obj core.CanvasObject) bool {
	s, ok := p.states[obj]
	return ok && !s.dirty
}

func newRecordingWalk(t *testing.T, content core.CanvasObject) (*canvas, *boundaryStateRecorder, *mobileTreeWalker) {
	t.Helper()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(content)
	c.Resize(geo.NewSize(300, 300))
	p := &boundaryStateRecorder{states: map[core.CanvasObject]*recordedBoundary{}, painted: map[core.CanvasObject]bool{}}
	c.SetPainter(p)
	walk := &mobileTreeWalker{c: c, clips: &internal.ClipStack{}, size: c.Size(), bp: p, hasBoundary: true}
	return c, p, walk
}

// The mobile walk's half of the glfw TestPaint_BoundaryRerendersOnSubPixelMove
// (#570): blitted at its position snapped to a device pixel, less the margin,
// and rendered again only when it moves by a fraction of a pixel.
func Test_mobileTreeWalker_BoundaryRerendersOnSubPixelMove(t *testing.T) {
	content := belluicanvas.NewRectangle(color.White)
	content.SetMinSize(geo.NewSize(50, 50))
	boundary := container.NewRepaintBoundary(content)
	boundary.Resize(geo.NewSize(50, 50))
	boundary.Move(geo.NewPos(10, 10))
	c, p, walk := newRecordingWalk(t, container.NewWithoutLayout(boundary))
	pxPerUnit := c.Scale()

	walk.paint()
	origin := 10 - 2/pxPerUnit // the 2 px margin
	assert.Equal(t, []string{"noclip", "bind", "unbind", "noclip", fmt.Sprintf("blit %v,%v", origin, origin)}, p.calls)

	p.calls = nil
	boundary.Move(geo.NewPos(10+3/pxPerUnit, 10)) // 3 whole pixels
	walk.paint()
	assert.Equal(t, []string{fmt.Sprintf("blit %v,%v", origin+3/pxPerUnit, origin)}, p.calls, "a whole-pixel move reuses the texture")

	p.calls = nil
	boundary.Move(geo.NewPos(10+3.5/pxPerUnit, 10))
	walk.paint()
	assert.Equal(t, []string{"noclip", "bind", "unbind", "noclip", fmt.Sprintf("blit %v,%v", origin+3/pxPerUnit, origin)}, p.calls,
		"a sub-pixel move renders it again")
}

