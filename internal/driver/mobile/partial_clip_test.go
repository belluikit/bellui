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

// partialClipPainter records the clip each object was painted under. Calls the
// walk is not expected to make hit the nil gpu.Painter and panic.
type partialClipPainter struct {
	gpu.Painter
	clip    string
	painted map[core.CanvasObject]string
}

func (p *partialClipPainter) SetTransform(geo.Matrix) {}
func (p *partialClipPainter) Paint(obj core.CanvasObject, _ geo.Position, _ geo.Size) {
	p.painted[obj] = p.clip
}
func (p *partialClipPainter) StartClipping(pos geo.Position, size geo.Size) {
	p.clip = fmt.Sprintf("%v,%v %vx%v", pos.X, pos.Y, size.Width, size.Height)
}
func (p *partialClipPainter) StopClipping() { p.clip = "none" }

// Test_mobileTreeWalker_PartialRepaintKeepsRegionClip pins #591: under a
// partial repaint, an object overlapping the dirty region is drawn under the
// region's clip. Drawn unclipped, a full-canvas background painted over the
// retained pixels of everything the walk skipped, so on an iPad only the
// region's content stayed on screen.
func Test_mobileTreeWalker_PartialRepaintKeepsRegionClip(t *testing.T) {
	bg := belluicanvas.NewRectangle(color.NRGBA{B: 0x80, A: 0xff})
	bg.Resize(geo.NewSize(300, 300))
	inside := belluicanvas.NewRectangle(color.White)
	inside.Move(geo.NewPos(20, 20))
	inside.Resize(geo.NewSize(50, 30))
	outside := belluicanvas.NewRectangle(color.White)
	outside.Move(geo.NewPos(200, 200))
	outside.Resize(geo.NewSize(50, 30))

	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(container.NewWithoutLayout(bg, inside, outside))
	c.Resize(geo.NewSize(300, 300))
	p := &partialClipPainter{clip: "none", painted: map[core.CanvasObject]string{}}
	c.SetPainter(p)

	walk := &mobileTreeWalker{
		c:              c,
		clips:          &internal.ClipStack{},
		size:           c.Size(),
		partialRepaint: true,
		drPos:          inside.Position(),
		drSize:         inside.Size(),
	}
	walk.paintClipped()

	region := fmt.Sprintf("%v,%v %vx%v", inside.Position().X, inside.Position().Y, inside.Size().Width, inside.Size().Height)
	assert.Equal(t, region, p.painted[bg], "the background overlapping the region is drawn under its clip")
	assert.Equal(t, region, p.painted[inside], "so is what the region holds")
	assert.NotContains(t, p.painted, outside, "what lies outside the region is not drawn")
	assert.Equal(t, "none", p.clip, "the clip is released after the walk")
}

// Without a partial repaint nothing is clipped.
func Test_mobileTreeWalker_FullRepaintIsUnclipped(t *testing.T) {
	bg := belluicanvas.NewRectangle(color.NRGBA{B: 0x80, A: 0xff})
	bg.Resize(geo.NewSize(300, 300))
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(container.NewWithoutLayout(bg))
	c.Resize(geo.NewSize(300, 300))
	p := &partialClipPainter{clip: "none", painted: map[core.CanvasObject]string{}}
	c.SetPainter(p)

	walk := &mobileTreeWalker{c: c, clips: &internal.ClipStack{}, size: c.Size()}
	walk.paintClipped()

	assert.Equal(t, "none", p.painted[bg])
}

// retentionPainter counts MarkCanvasFBORetentionUnreliable calls.
type retentionPainter struct {
	gpu.Painter
	marked int
}

func (p *retentionPainter) MarkCanvasFBORetentionUnreliable() { p.marked++ }

// Test_driver_RetentionMarkedOnlyAfterResume pins #591's second half: the
// surface coming up at launch leaves partial repaint on. Only a surface shown
// again after it went away (a background→resume, #342) marks the canvas FBO's
// retention unreliable.
func Test_driver_RetentionMarkedOnlyAfterResume(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	p := &retentionPainter{}
	c.SetPainter(p)
	d := &driver{}

	d.surfaceShown(c)
	assert.Zero(t, p.marked, "the first surface is no resume")

	d.surfaceWasHidden = true // the visible CrossOff
	d.surfaceShown(c)
	assert.Equal(t, 1, p.marked, "a resume marks retention unreliable")
}
