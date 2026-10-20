//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// shadowKey is everything that decides a blurred shadow texture's pixels. Where
// the shadow is drawn (its position and offset) is not part of it, so every
// shadow of the same shape, size and elevation shares one texture. The key
// matches the WebGPU painter's (#434), plus the linear part of the world
// transform, which the GL shape draw applies inside the offscreen target.
type shadowKey struct {
	r, g, b, a    uint32
	corner, blur  float32
	width, height float32
	pixScale      float32
	opacity       float32    // the group opacity, which the shape draw applies
	linear        [4]float32 // A, B, C, D of a non-translation transform; zero otherwise
}

// shadowEntry is a cached blurred shadow and the frame it was last drawn in.
type shadowEntry struct {
	tex  Texture
	used uint64
}

// drawBoxShadow renders a canvas.BoxShadow: the shadow shape (a rounded rect in
// the shadow color, padded for the blur spread) is drawn into an offscreen
// FBO, separably blurred, then drawn behind the widget.
//
// The blurred texture is cached (shadowTexture), and a shadow the active
// scissor clips away entirely is skipped before anything is rendered: rendering
// every shadow afresh every frame cost an FBO, two textures and three passes
// per shadow, on screen or not — 79% of the CPU of a static 100-card screen
// (#444).
func (p *painter) drawBoxShadow(shadow *canvas.BoxShadow, pos geo.Position, frame geo.Size) {
	if shadow.BlurRadius <= 0 {
		return
	}

	// The shadow is a two-pass FBO blur. On targets without framebuffer
	// support (wasm) the blur programs are uninitialised and the FBO ops are
	// stubs, so attempting the draw would panic on a nil program state. Skip
	// it — the shadowed widget still renders, just without its shadow (pre-rebrand tracker #118).
	if !framebufferSupported {
		return
	}

	// Determine the shadow area size (widget size + blur padding).
	spread := shadow.BlurRadius * 2
	shadowW := shadow.Size().Width + spread*2
	shadowH := shadow.Size().Height + spread*2

	texW := int(p.textureScale(shadowW))
	texH := int(p.textureScale(shadowH))
	if texW <= 0 || texH <= 0 {
		return
	}

	shadowPos := geo.NewPos(pos.X+shadow.OffsetX-spread, pos.Y+shadow.OffsetY-spread)
	// GL's window space is y-up, like the scissor rect.
	if p.scissorCulls(int(p.textureScale(shadowPos.X)), int(p.textureScale(frame.Height-shadowPos.Y-shadowH)), texW, texH) {
		return
	}

	blurredTex := p.shadowTexture(shadow, spread, shadowW, shadowH, texW, texH, frame)

	// Draw the blurred texture as a quad behind the widget, inside the
	// active scissor like every other draw.
	points, _ := p.rectCoords(geo.NewSize(shadowW, shadowH), shadowPos, frame, canvas.ImageFillStretch, 0, 0)
	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(p.program, "vert", 3, 5, 0)
	p.UpdateVertexArray(p.program, "vertTexCoord", 2, 5, 3)
	p.SetUniform1f(p.program, "alpha", p.effectiveAlpha(1))
	p.SetUniform1f(p.program, "cornerRadius", 0)
	p.SetUniform2f(p.program, "size", 0, 0)
	p.SetUniform4f(p.program, "inset", 0, 0, 1, 1)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, blurredTex)
	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

// shadowTexture returns the blurred shadow texture for shadow, from the cache or
// rendered into it. The texture stays owned by the cache. The render target
// and viewport for frame are bound again on return.
func (p *painter) shadowTexture(shadow *canvas.BoxShadow, spread, shadowW, shadowH float32, texW, texH int, frame geo.Size) Texture {
	key := p.shadowKeyFor(shadow)
	if e, ok := p.shadows[key]; ok {
		e.used = p.frame
		return e.tex
	}

	// Create FBO for shadow shape — defer cleanup so resources are freed
	// even if a panic occurs during blur or draw operations.
	shapeFBO := p.ctx.CreateFramebuffer()
	defer p.ctx.DeleteFramebuffer(shapeFBO)
	shapeTex := p.ctx.CreateTexture()
	defer p.ctx.DeleteTexture(shapeTex)
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, shapeTex)
	p.ctx.TexImage2D(texture2D, 0, texW, texH, colorFormatRGBA, unsignedByte, nil)
	p.ctx.TexParameteri(texture2D, textureMinFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureMagFilter, textureFilterToGL[0])
	p.ctx.TexParameteri(texture2D, textureWrapS, clampToEdge)
	p.ctx.TexParameteri(texture2D, textureWrapT, clampToEdge)
	p.ctx.BindFramebuffer(glFramebuffer, shapeFBO)
	p.ctx.FramebufferTexture2D(glFramebuffer, colorAttachment0, shapeTex, 0)
	p.checkFBOComplete()

	// An active window-space scissor clip would clip this offscreen FBO
	// render — suspend it for the shape + blur passes.
	restoreScissor := p.suspendScissor()
	defer restoreScissor()

	// Render a filled rounded rectangle into the FBO
	p.ctx.Viewport(0, 0, texW, texH)
	p.ctx.ClearColor(0, 0, 0, 0)
	p.ctx.Clear(bitColorBuffer)

	// Draw a solid rounded rect centered in the FBO (with padding for blur spread)
	shadowRect := &canvas.Rectangle{
		FillColor:    shadow.Color,
		CornerRadius: shadow.CornerRadius,
	}
	shadowRect.Resize(shadow.Size())
	p.drawRectangle(shadowRect, geo.NewPos(spread, spread), geo.NewSize(shadowW, shadowH))

	// Restore the active render target (canvas/boundary FBO or screen).
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.rebindRenderTarget(int(frameWidthScaled), int(frameHeightScaled))

	blurRadius := shadow.BlurRadius * p.pixScale / float32(texW) * 2 // normalize to texture space
	blurredTex := p.blurTexture(shapeTex, texW, texH, blurRadius)

	// Restore the render target again — blurTexture leaves the blur-pass
	// viewport behind when no FBO target is on the stack (overlay pass).
	p.rebindRenderTarget(int(frameWidthScaled), int(frameHeightScaled))

	if p.shadows == nil {
		p.shadows = map[shadowKey]*shadowEntry{}
	}
	p.shadows[key] = &shadowEntry{tex: blurredTex, used: p.frame}
	return blurredTex
}

// shadowKeyFor returns the cache key of shadow's blurred texture as this
// painter would render it now.
func (p *painter) shadowKeyFor(shadow *canvas.BoxShadow) shadowKey {
	var r, g, b, a uint32
	if shadow.Color != nil {
		r, g, b, a = shadow.Color.RGBA()
	}
	var linear [4]float32
	if p.hasTransform() {
		m := p.currentTransform
		linear = [4]float32{m.A, m.B, m.C, m.D}
	}
	return shadowKey{
		r: r, g: g, b: b, a: a,
		corner: shadow.CornerRadius, blur: shadow.BlurRadius,
		width: shadow.Size().Width, height: shadow.Size().Height,
		pixScale: p.pixScale, opacity: p.groupOpacity,
		linear: linear,
	}
}

// evictShadows deletes the cached shadows that the previous frame did not
// draw, at the start of a frame. A shadow drawn every frame is kept; one that
// changed shape (an elevation animation's intermediate steps), scrolled out
// of view or went away is dropped a frame later, so the cache holds about one
// frame's distinct shadows. GL keeps a deleted texture alive for commands
// already issued, so deleting needs no wait.
func (p *painter) evictShadows() {
	for k, e := range p.shadows {
		if e.used+1 < p.frame {
			p.ctx.DeleteTexture(e.tex)
			delete(p.shadows, k)
		}
	}
}

// ReleaseCaches deletes the painter's own cached textures (the box shadows).
// The driver calls it when the window closes, with the painter's context
// current: the textures belong to that context, and nothing else frees them.
func (p *painter) ReleaseCaches() {
	if len(p.shadows) == 0 {
		return // also a painter never initialised: it has no context to call
	}
	for k, e := range p.shadows {
		p.ctx.DeleteTexture(e.tex)
		delete(p.shadows, k)
	}
	p.logError()
}

// CachedShadows returns how many blurred shadow textures the painter holds.
func (p *painter) CachedShadows() int {
	return len(p.shadows)
}

// scissorCulls reports whether a draw covering the given rect, in the bound
// target's y-up pixel space, lies wholly outside the active scissor, so it
// would put no pixel on screen. The rect is widened by a pixel on every side
// to absorb the rounding rectCoords applies. With no scissor active nothing is
// culled; an empty scissor culls everything.
func (p *painter) scissorCulls(x, y, w, h int) bool {
	if !p.scissorActive {
		return false
	}
	s := p.scissor
	return s[2] <= 0 || s[3] <= 0 ||
		x+w+1 <= int(s[0]) || x-1 >= int(s[0]+s[2]) ||
		y+h+1 <= int(s[1]) || y-1 >= int(s[1]+s[3])
}
