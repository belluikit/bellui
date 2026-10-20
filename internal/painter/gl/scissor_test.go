//go:build gl || (!darwin && !linux && !windows && !wasm)

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
	p.pushRenderTarget(1, 1200, 800, 0) // the canvas FBO
	clip()
	p.pushRenderTarget(2, 300, 160, 80) // a boundary 80 high
	clip()
	p.popRenderTarget(0, 0)
	clip()

	assert.Equal(t, [][4]int32{
		{20, 2 * (400 - 20 - 50), 200, 100},
		{20, 2 * (80 - 20 - 50), 200, 100},
		{20, 2 * (400 - 20 - 50), 200, 100},
	}, ctx.scissors)
}
