//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import "github.com/belluikit/bellui/geo"

// pushRenderTarget binds an FBO as the current render target and records it
// on the target stack so temporary offscreen work can restore it.
// logical is the target's size in canvas units, zero for the full canvas.
func (p *painter) pushRenderTarget(fbo Framebuffer, w, h int, logical geo.Size) {
	p.targetStack = append(p.targetStack, renderTarget{fbo: fbo, w: w, h: h, logical: logical})
	p.ctx.BindFramebuffer(glFramebuffer, fbo)
	p.ctx.Viewport(0, 0, w, h)
}

// popRenderTarget removes the current render target and rebinds the previous
// one. When the stack empties, the default framebuffer is bound with the given
// fallback viewport.
func (p *painter) popRenderTarget(fallbackW, fallbackH int) {
	if n := len(p.targetStack); n > 0 {
		p.targetStack = p.targetStack[:n-1]
	}
	p.rebindRenderTarget(fallbackW, fallbackH)
}

// rebindRenderTarget restores the binding and viewport of the current render
// target after temporary FBO work (shadow shape, blur ping-pong, Scene3D).
// With an empty stack the default framebuffer is bound; callers pass the
// screen viewport as fallback (0, 0 keeps the caller's own viewport restore
// in charge).
func (p *painter) rebindRenderTarget(fallbackW, fallbackH int) {
	if n := len(p.targetStack); n > 0 {
		t := p.targetStack[n-1]
		p.ctx.BindFramebuffer(glFramebuffer, t.fbo)
		p.ctx.Viewport(0, 0, t.w, t.h)
		return
	}
	p.ctx.BindFramebuffer(glFramebuffer, defaultFramebuffer)
	if fallbackW > 0 && fallbackH > 0 {
		p.ctx.Viewport(0, 0, fallbackW, fallbackH)
	}
}

// attachStencil gives the bound framebuffer an 8-bit stencil buffer of w×h
// pixels and returns it. Rounded and path clips and stencil path fills draw
// into the stencil buffer, and an FBO has none unless it is given one: without
// it the stencil test always passes and nothing clips (#990).
func (p *painter) attachStencil(w, h int) Renderbuffer {
	rb := p.ctx.CreateRenderbuffer()
	p.ctx.BindRenderbuffer(renderbuffer, rb)
	p.ctx.RenderbufferStorage(renderbuffer, stencilIndex8, w, h)
	p.ctx.FramebufferRenderbuffer(glFramebuffer, stencilAttachment, renderbuffer, rb)
	return rb
}
