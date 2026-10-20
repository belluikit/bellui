//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordingContext records framebuffer binds and viewport calls. All other
// context methods come from the embedded nil interface and panic if called —
// the render-target stack must touch nothing else.
type recordingContext struct {
	context
	binds     []Framebuffer
	viewports [][2]int
}

func (r *recordingContext) BindFramebuffer(_ uint32, fb Framebuffer) {
	r.binds = append(r.binds, fb)
}

func (r *recordingContext) Viewport(_, _, width, height int) {
	r.viewports = append(r.viewports, [2]int{width, height})
}

// The render-target stack must restore the enclosing target (canvas FBO,
// nested boundary FBO) after temporary offscreen work — never assume the
// default framebuffer. Regression test: drawBoxShadow/blurTexture/Scene3D
// used to bind framebuffer 0 mid-frame, silently discarding every draw call
// after the first drop shadow (empty MD3 cards, vanished overlay menus).
func TestRenderTargetStack(t *testing.T) {
	ctx := &recordingContext{}
	p := &painter{ctx: ctx}

	canvasFBO, boundaryFBO := Framebuffer(7), Framebuffer(9)

	p.pushRenderTarget(canvasFBO, 100, 200, 0)
	assert.Equal(t, []Framebuffer{canvasFBO}, ctx.binds)
	assert.Equal(t, [][2]int{{100, 200}}, ctx.viewports)

	// Nested boundary FBO.
	p.pushRenderTarget(boundaryFBO, 30, 40, 40)
	assert.Equal(t, boundaryFBO, ctx.binds[len(ctx.binds)-1])

	// Offscreen work (shadow blur) restores the boundary, not framebuffer 0.
	p.rebindRenderTarget(0, 0)
	assert.Equal(t, boundaryFBO, ctx.binds[len(ctx.binds)-1])
	assert.Equal(t, [2]int{30, 40}, ctx.viewports[len(ctx.viewports)-1])

	// Leaving the boundary restores the canvas FBO with its viewport.
	p.popRenderTarget(999, 999)
	assert.Equal(t, canvasFBO, ctx.binds[len(ctx.binds)-1])
	assert.Equal(t, [2]int{100, 200}, ctx.viewports[len(ctx.viewports)-1])

	// Popping the last target falls back to the default framebuffer and the
	// caller-provided viewport.
	p.popRenderTarget(640, 480)
	assert.Equal(t, Framebuffer(0), ctx.binds[len(ctx.binds)-1])
	assert.Equal(t, [2]int{640, 480}, ctx.viewports[len(ctx.viewports)-1])

	// Empty stack + zero fallback: bind default framebuffer but leave the
	// viewport alone (the caller restores it explicitly).
	nViewports := len(ctx.viewports)
	p.rebindRenderTarget(0, 0)
	assert.Equal(t, Framebuffer(0), ctx.binds[len(ctx.binds)-1])
	assert.Len(t, ctx.viewports, nViewports, "zero fallback must not set a 0x0 viewport")
}
