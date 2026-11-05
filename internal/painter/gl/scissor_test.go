//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !wasm

package gl

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// scissorContext records the scissor rects set; the rest of the context
// comes from the embedded nil interface and panics if called.
type scissorContext struct {
	recordingContext
	scissors [][4]int32
}

func (s *scissorContext) Scissor(x, y, w, h int32) {
	s.scissors = append(s.scissors, [4]int32{x, y, w, h})
}
func (*scissorContext) Enable(uint32)    {}
func (*scissorContext) GetError() uint32 { return 0 }

// GL's scissor is y-up, so StartClipping flips the rect against the height of
// the bound target: the canvas, or a boundary FBO, whose rects the paint walk
// hands over boundary-local. Flipping against the canvas there put the clip
// above the FBO, and the boundary painted nothing (#559).
func TestStartClippingFlipsAgainstTarget(t *testing.T) {
	ctx := &scissorContext{}
	c := test.NewCanvas()
	c.Resize(geo.NewSize(600, 400))
	p := &painter{ctx: ctx, canvas: c, pixScale: 2}

	clip := func() {
		p.StartClipping(geo.NewPos(10, 20), geo.NewSize(100, 50))
	}
	p.pushRenderTarget(1, 1200, 800, geo.Size{}) // the canvas FBO
	clip()
	p.pushRenderTarget(2, 300, 160, geo.NewSize(150, 80)) // a boundary 80 high
	clip()
	p.popRenderTarget(0, 0)
	clip()

	assert.Equal(t, [][4]int32{
		{20, 2 * (400 - 20 - 50), 200, 100},
		{20, 2 * (80 - 20 - 50), 200, 100},
		{20, 2 * (400 - 20 - 50), 200, 100},
	}, ctx.scissors)
}

// A partial repaint clears the dirty region under one scissor and redraws a
// ClipToBounds container under the container's clip within it. Rects sharing
// an edge in dp must share it in pixels, or a row is cleared and never redrawn
// (#975). At 2.625 the region (0, 11.4 high) and the clip (1.7, 9.7 high) share
// their bottom edge, which, scaling y and h separately, landed a row apart.
func TestStartClippingSharedEdgesMatch(t *testing.T) {
	ctx := &scissorContext{}
	c := test.NewCanvas()
	c.Resize(geo.NewSize(40, 40))
	p := &painter{ctx: ctx, canvas: c, pixScale: 2.625}

	p.StartClipping(geo.NewPos(0, 0), geo.NewSize(40, 11.4))
	p.StartClipping(geo.NewPos(0, 1.7), geo.NewSize(40, 9.7))

	region, clip := ctx.scissors[0], ctx.scissors[1]
	assert.Equal(t, region[1], clip[1], "bottom edges (GL y-up: the scissor's y)")
	assert.Equal(t, region[0]+region[2], clip[0]+clip[2], "right edges")
	// The clip's top edge is its own: 1.7 dp down, rounded once.
	assert.Equal(t, int32(105-4), clip[1]+clip[3], "the clip's top edge")
}
