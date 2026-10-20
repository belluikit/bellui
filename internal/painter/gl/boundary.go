//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
)

// BoundaryFBOState tracks the off-screen framebuffer for a RepaintBoundary.
// It is exposed so the paint walk (in the glfw/mobile packages) can pass it
// through the boundary stack without type assertions on every frame. It
// implements the backend-neutral gpu.BoundaryFBO handle.
type BoundaryFBOState struct {
	fbo         Framebuffer
	texture     Texture
	width       int      // pixel dimensions
	height      int      // pixel dimensions
	LogicalSize geo.Size // logical size for coordinate adjustment
	Dirty       bool     // true if the subtree needs re-rendering into the FBO
}

// SetDirty implements gpu.BoundaryFBO.
func (s *BoundaryFBOState) SetDirty(dirty bool) {
	s.Dirty = dirty
}

// Verify painter implements gpu.BoundaryPainter and the state implements
// gpu.BoundaryFBO.
var (
	_ gpu.BoundaryPainter = (*painter)(nil)
	_ gpu.BoundaryFBO     = (*BoundaryFBOState)(nil)
)

// EnsureBoundaryFBO creates or resizes the FBO for a RepaintBoundary object.
func (p *painter) EnsureBoundaryFBO(obj core.CanvasObject, pixW, pixH int, logicalSize geo.Size) gpu.BoundaryFBO {
	if p.boundaries == nil {
		p.boundaries = make(map[core.CanvasObject]*BoundaryFBOState)
	}

	state, exists := p.boundaries[obj]
	if exists && state.width == pixW && state.height == pixH {
		return state
	}

	// Free old resources if resizing.
	if exists {
		p.ctx.DeleteFramebuffer(state.fbo)
		p.ctx.DeleteTexture(state.texture)
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
	p.rebindRenderTarget(0, 0) // EnsureBoundaryFBO runs mid-frame; restore the active target
	p.logError()

	state = &BoundaryFBOState{
		fbo:         fbo,
		texture:     tex,
		width:       pixW,
		height:      pixH,
		LogicalSize: logicalSize,
		Dirty:       true, // newly created, must render
	}
	p.boundaries[obj] = state
	return state
}

// BindBoundaryFBO redirects rendering to the boundary's FBO.
func (p *painter) BindBoundaryFBO(s gpu.BoundaryFBO) {
	state := s.(*BoundaryFBOState)
	p.pushRenderTarget(state.fbo, state.width, state.height, state.LogicalSize.Height)
	p.ctx.ClearColor(0, 0, 0, 0)
	p.ctx.Clear(bitColorBuffer | bitDepthBuffer)
	p.logError()
}

// UnbindBoundaryFBO restores the enclosing render target (the canvas FBO, a
// parent boundary FBO, or the default framebuffer with the given viewport).
func (p *painter) UnbindBoundaryFBO(canvasPixW, canvasPixH int) {
	p.popRenderTarget(canvasPixW, canvasPixH)
	p.logError()
}

// BlitBoundaryTexture draws the boundary's cached FBO texture as a textured
// quad at the given canvas position.
func (p *painter) BlitBoundaryTexture(s gpu.BoundaryFBO, pos geo.Position, frame geo.Size) {
	state := s.(*BoundaryFBOState)
	points, _ := p.rectCoords(state.LogicalSize, pos, frame, canvas.ImageFillStretch, 0, 0)
	// The FBO texture is in GL orientation (Y=0 at bottom), but rectCoords
	// produces V coordinates for canvas-space textures (V=0 at top). Flip
	// the V coordinates so the blit is right-side up.
	points[4], points[9] = points[9], points[4]     // swap V of vertex 0 and 1
	points[14], points[19] = points[19], points[14] // swap V of vertex 2 and 3
	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(p.program, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.program, "vertTexCoord", 2, 5, 3)
	p.SetUniform1f(p.program, "alpha", 1)
	p.SetUniform1f(p.program, "cornerRadius", 0)
	p.SetUniform2f(p.program, "size", 0, 0)
	p.SetUniform4f(p.program, "inset", 0, 0, 1, 1)

	// Reset transform — the boundary quad is drawn in canvas space.
	p.SetTransform(geo.Identity())

	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, state.texture)
	p.ctx.BlendFunc(srcAlpha, oneMinusSrcAlpha)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

// FreeBoundaryFBO releases the FBO and texture for a RepaintBoundary.
func (p *painter) FreeBoundaryFBO(obj core.CanvasObject) {
	if p.boundaries == nil {
		return
	}
	state, exists := p.boundaries[obj]
	if !exists {
		return
	}
	p.ctx.DeleteFramebuffer(state.fbo)
	p.ctx.DeleteTexture(state.texture)
	delete(p.boundaries, obj)
}

// MarkBoundaryDirty marks a RepaintBoundary's FBO as needing re-render.
func (p *painter) MarkBoundaryDirty(obj core.CanvasObject) {
	if p.boundaries == nil {
		return
	}
	if state, exists := p.boundaries[obj]; exists {
		state.Dirty = true
	}
}

// IsBoundaryClean returns true if the boundary's FBO exists and is up-to-date.
func (p *painter) IsBoundaryClean(obj core.CanvasObject) bool {
	if p.boundaries == nil {
		return false
	}
	state, exists := p.boundaries[obj]
	return exists && !state.Dirty
}

// InvalidateAllBoundaries marks all cached boundary FBOs as dirty.
// Called when canvas scale changes.
func (p *painter) InvalidateAllBoundaries() {
	for _, state := range p.boundaries {
		state.Dirty = true
	}
}
