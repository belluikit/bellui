//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
)

// canvasFBOState holds the persistent full-canvas framebuffer object.
// Unlike the double-buffered default framebuffer, this FBO retains its
// content across frames, enabling dirty-region partial repaints.
type canvasFBOState struct {
	fbo          Framebuffer
	texture      Texture
	width        int // pixel dimensions
	height       int
	initialClear bool // true after first full clear with background color
}

// Verify painter implements gpu.DirtyRegionPainter.
var _ gpu.DirtyRegionPainter = (*painter)(nil)

// EnsureCanvasFBO creates or resizes the persistent canvas FBO.
func (p *painter) EnsureCanvasFBO(pixW, pixH int) {
	if p.canvasFBO != nil && p.canvasFBO.width == pixW && p.canvasFBO.height == pixH {
		return
	}

	// Flush pending draw calls before deleting old resources — queued
	// draws targeting the old FBO would be silently lost otherwise.
	if p.canvasFBO != nil {
		p.ctx.Flush()
		p.ctx.DeleteFramebuffer(p.canvasFBO.fbo)
		p.ctx.DeleteTexture(p.canvasFBO.texture)
	}

	fbo := p.ctx.CreateFramebuffer()
	tex := p.ctx.CreateTexture()
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, tex)
	p.ctx.TexImage2D(texture2D, 0, pixW, pixH, colorFormatRGBA, unsignedByte, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.BindFramebuffer(glFramebuffer, fbo)
	p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, tex, 0)
	p.checkFBOComplete()
	p.ctx.BindFramebuffer(glFramebuffer, defaultFramebuffer)
	p.logError()

	p.canvasFBO = &canvasFBOState{
		fbo:          fbo,
		texture:      tex,
		width:        pixW,
		height:       pixH,
		initialClear: false, // needs a full clear on first use
	}
}

// BindCanvasFBO binds the persistent canvas FBO as the render target.
// This is the frame-start bind, so the target stack is reset first — a frame
// aborted mid-paint must not leak stale targets into the next frame.
func (p *painter) BindCanvasFBO() {
	if p.canvasFBO == nil {
		return
	}
	p.targetStack = p.targetStack[:0]
	p.pushRenderTarget(p.canvasFBO.fbo, p.canvasFBO.width, p.canvasFBO.height, 0)
	p.logError()
}

// UnbindAndBlitCanvasFBO restores the default framebuffer and draws the
// canvas FBO texture as a full-screen quad.
func (p *painter) UnbindAndBlitCanvasFBO(pixW, pixH int) {
	if p.canvasFBO == nil {
		return
	}

	p.popRenderTarget(pixW, pixH)

	// Draw the FBO texture as a full-screen quad covering [-1,1] in NDC.
	// The FBO texture is in GL orientation (Y=0 at bottom), so we must flip
	// the V texture coordinates compared to normal canvas object rendering.
	points := []float32{
		// x,    y,   z, texU, texV
		-1, -1, 0, 0, 0, // bottom-left  → texture bottom-left
		-1, 1, 0, 0, 1, // top-left     → texture top-left
		1, -1, 0, 1, 0, // bottom-right → texture bottom-right
		1, 1, 0, 1, 1, // top-right    → texture top-right
	}
	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(p.program, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.program, "vertTexCoord", 2, 5, 3)
	p.SetUniform1f(p.program, "alpha", 1)
	p.SetUniform1f(p.program, "cornerRadius", 0)
	p.SetUniform2f(p.program, "size", 0, 0)
	p.SetUniform4f(p.program, "inset", 0, 0, 1, 1)
	p.SetTransform(geo.Identity())

	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, p.canvasFBO.texture)
	// Opaque copy — no blending. The FBO holds the complete frame, and the
	// back buffer still contains the frame from two swaps ago. Content is
	// drawn into the FBO with non-premultiplied blending, which leaves
	// alpha < 1 along anti-aliased edges; blending here would let the stale
	// back-buffer frame bleed through at exactly those border pixels
	// (visible as bright dashes on borders while scrolling).
	p.blendFunc(one, 0)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
}

// HasCanvasFBO returns true if the canvas FBO is allocated.
func (p *painter) HasCanvasFBO() bool {
	return p.canvasFBO != nil
}

// CanvasFBONeedsFullClear returns true if the FBO was just created/resized
// and has no valid previous content.
func (p *painter) CanvasFBONeedsFullClear() bool {
	return p.canvasFBO != nil && !p.canvasFBO.initialClear
}

// MarkCanvasFBOCleared records that the FBO has been fully cleared at least once.
func (p *painter) MarkCanvasFBOCleared() {
	if p.canvasFBO != nil {
		p.canvasFBO.initialClear = true
	}
}
