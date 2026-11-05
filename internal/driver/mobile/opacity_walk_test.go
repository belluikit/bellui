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

// opacityPainter records the group opacity each named object is painted at,
// and each boundary call. Calls the walk is not expected to make hit the nil
// gpu.Painter and panic.
type opacityPainter struct {
	gpu.Painter
	names   map[core.CanvasObject]string
	opacity float32
	calls   []string
}

func (p *opacityPainter) SetGroupOpacity(v float32)            { p.opacity = v }
func (p *opacityPainter) ClearGroupOpacity()                   { p.opacity = 1 }
func (p *opacityPainter) SetTransform(geo.Matrix)              {}
func (p *opacityPainter) StartClipping(geo.Position, geo.Size) {}
func (p *opacityPainter) StopClipping()                        {}
func (p *opacityPainter) Paint(o core.CanvasObject, _ geo.Position, _ geo.Size) {
	if n, ok := p.names[o]; ok {
		p.calls = append(p.calls, fmt.Sprintf("%s@%g", n, p.opacity))
	}
}

func (p *opacityPainter) EnsureBoundaryFBO(core.CanvasObject, int, int, geo.Size) gpu.BoundaryFBO {
	return boundaryClipFBO{}
}
func (p *opacityPainter) BindBoundaryFBO(gpu.BoundaryFBO) { p.calls = append(p.calls, "bind") }
func (p *opacityPainter) UnbindBoundaryFBO(int, int)      { p.calls = append(p.calls, "unbind") }
func (p *opacityPainter) BlitBoundaryTexture(gpu.BoundaryFBO, geo.Position, geo.Size) {
	p.calls = append(p.calls, fmt.Sprintf("blit@%g", p.opacity))
}
func (p *opacityPainter) FreeBoundaryFBO(core.CanvasObject)      {}
func (p *opacityPainter) MarkBoundaryDirty(core.CanvasObject)    {}
func (p *opacityPainter) IsBoundaryClean(core.CanvasObject) bool { return false }
func (p *opacityPainter) InvalidateAllBoundaries()               {}

func walkOpacity(t *testing.T, content core.CanvasObject, names map[core.CanvasObject]string) []string {
	t.Helper()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(content)
	c.Resize(geo.NewSize(200, 200))
	p := &opacityPainter{names: names, opacity: 1}
	c.SetPainter(p)
	walk := &mobileTreeWalker{c: c, clips: &internal.ClipStack{}, size: c.Size(), bp: p, hasBoundary: true}
	c.WalkTrees(walk.draw, walk.afterDraw)
	assert.Equal(t, float32(1), p.opacity, "the walk ends at full opacity")
	return p.calls
}

func rect() *belluicanvas.Rectangle {
	r := belluicanvas.NewRectangle(color.White)
	r.SetMinSize(geo.NewSize(10, 10))
	return r
}

// An Opacity's content is drawn at its opacity; nested ones multiply; a
// sibling after it is drawn at full opacity again.
func Test_mobileTreeWalker_GroupOpacityNests(t *testing.T) {
	a, b, c := rect(), rect(), rect()
	tree := container.NewVBox(
		container.NewOpacity(container.NewVBox(a, container.NewOpacity(b, 0.5)), 0.5),
		c,
	)
	calls := walkOpacity(t, tree, map[core.CanvasObject]string{a: "a", b: "b", c: "c"})
	assert.Equal(t, []string{"a@0.5", "b@0.25", "c@1"}, calls)
}

// At opacity 0 nothing in the group is drawn: the painters read a group
// opacity of 0 as unset.
func Test_mobileTreeWalker_GroupOpacityZeroDrawsNothing(t *testing.T) {
	a, b := rect(), rect()
	calls := walkOpacity(t, container.NewVBox(container.NewOpacity(a, 0), b),
		map[core.CanvasObject]string{a: "a", b: "b"})
	assert.Equal(t, []string{"b@1"}, calls)
}

// A RepaintBoundary inside a group caches its content at full opacity, and
// its texture is drawn at the group's opacity, so the opacity applies once
// and a change to it does not need the cache redrawn.
func Test_mobileTreeWalker_GroupOpacityOverABoundary(t *testing.T) {
	a := rect()
	tree := container.NewOpacity(container.NewRepaintBoundary(container.NewVBox(a, container.NewOpacity(rect(), 1))), 0.5)
	calls := walkOpacity(t, tree, map[core.CanvasObject]string{a: "a"})
	assert.Equal(t, []string{"bind", "a@1", "unbind", "blit@0.5"}, calls)
}
