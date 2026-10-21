//go:build !gl

package webgpu

import (
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
)

// Box shadows and backdrop blur for WebGPU (pre-rebrand tracker #223), porting the GL painter's
// drawBoxShadow / drawBackdropBlur / blurTexture. Both reuse the Device's
// render-target stack (pre-rebrand tracker #218) and the ported `blur` WGSL (separable 9-tap,
// fs_main_h / fs_main_v) — the GL FBO ping-pong becomes off-screen render
// targets, the GL CopyTexSubImage2D framebuffer grab becomes CopyActiveTargetRegion.
//
// The off-screen shape / blur passes are pushed with PushOffscreenTarget so they
// get a dedicated depth-stencil attachment — never the shared size-keyed one —
// so their clearing passes can't wipe an enclosing canvas stencil clip when the
// off-screen pixel size happens to match the canvas (the pre-rebrand tracker #231 hazard). The blur
// content is premultiplied (the shape is rendered into a transparent target, and
// captured content is already composited), so it composites via the simple
// shader's premultiplied blend, matching the GL backend.

// shadowKey is everything that decides a blurred shadow texture's pixels. Where
// the shadow is drawn (its position and offset) is not part of it, so every
// shadow of the same shape, size and elevation shares one texture.
type shadowKey struct {
	r, g, b, a    uint32
	corner, blur  float32
	width, height float32
	pixScale      float32
	opacity       float32 // the group opacity, which the shape draw applies
}

// shadowEntry is a cached blurred shadow and the frame it was last drawn in.
type shadowEntry struct {
	tex  *Texture
	used uint64
}

// drawBoxShadow renders a canvas.BoxShadow: the shadow shape (a rounded rect in
// the shadow color, padded for the blur spread) is drawn into an off-screen
// target, separably blurred, then drawn behind the widget. Ports gl.drawBoxShadow.
//
// The blurred texture is cached (shadowTexture), and a shadow the open pass
// clips away entirely is skipped before anything is rendered: rendering every
// shadow afresh every frame cost three off-screen passes and two textures per
// shadow, on screen or not, and past ~680 shadows exhausted Metal's command
// buffers (#434).
func (p *painter) drawBoxShadow(shadow *canvas.BoxShadow, pos geo.Position, frame geo.Size) {
	if shadow.BlurRadius <= 0 {
		return
	}

	// Shadow area = widget size + blur spread padding on each side.
	spread := shadow.BlurRadius * 2
	shadowW := shadow.Size().Width + spread*2
	shadowH := shadow.Size().Height + spread*2

	texW := int(p.textureScale(shadowW))
	texH := int(p.textureScale(shadowH))
	if texW <= 0 || texH <= 0 {
		return
	}

	shadowPos := geo.NewPos(pos.X+shadow.OffsetX-spread, pos.Y+shadow.OffsetY-spread)
	if p.dev.Culls(int(p.textureScale(shadowPos.X)), int(p.textureScale(shadowPos.Y)), texW, texH) {
		return
	}

	// The off-screen passes draw axis-aligned with no clip; save and neutralise
	// the world transform + stencil clip, restored before returning.
	prevTransform, prevStencil := p.transform, p.stencilMode
	p.transform = geo.Identity()
	p.stencilMode = StencilNone
	defer func() {
		p.transform = prevTransform
		p.stencilMode = prevStencil
	}()

	blurred := p.shadowTexture(shadow, spread, shadowW, shadowH, texW, texH)
	if blurred == nil {
		return
	}

	// Draw the blurred texture as a quad behind the widget (premultiplied content).
	p.blitTextureAlpha(blurred.View(), geo.NewSize(shadowW, shadowH), shadowPos, frame, p.effectiveAlpha(1))
}

// shadowTexture returns the blurred shadow texture for shadow, from the cache or
// rendered into it. The texture stays owned by the cache. Returns nil when it
// cannot be rendered.
func (p *painter) shadowTexture(shadow *canvas.BoxShadow, spread, shadowW, shadowH float32, texW, texH int) *Texture {
	key := p.shadowKeyFor(shadow)
	if e, ok := p.shadows[key]; ok {
		e.used = p.frame
		return e.tex
	}

	shapeTex, err := p.dev.CreateRenderTexture(texW, texH)
	if err != nil {
		return nil
	}

	// Render the solid rounded rect into the (transparent) shape target.
	p.dev.PushOffscreenTarget(shapeTex, texW, texH, wgpu.Color{})
	shadowRect := &canvas.Rectangle{FillColor: shadow.Color, CornerRadius: shadow.CornerRadius}
	shadowRect.Resize(shadow.Size())
	fboFrame := geo.NewSize(shadowW, shadowH)
	p.drawRectangle(shadowRect, geo.NewPos(spread, spread), fboFrame)
	p.dev.PopTarget()

	// Separably blur the shape. The texture may come back smaller than the
	// shape's; it is drawn stretched to the shadow's size.
	blurred := p.blurTexture(shapeTex.View(), texW, texH, paint.BlurSigma(shadow.BlurRadius)*p.pixScale)
	p.dev.ReleaseTextureAfterFrame(shapeTex)
	if blurred == nil {
		return nil
	}
	if p.shadows == nil {
		p.shadows = map[shadowKey]*shadowEntry{}
	}
	p.shadows[key] = &shadowEntry{tex: blurred, used: p.frame}
	return blurred
}

// shadowKeyFor returns the cache key of shadow's blurred texture as this
// painter would render it now.
func (p *painter) shadowKeyFor(shadow *canvas.BoxShadow) shadowKey {
	var r, g, b, a uint32
	if shadow.Color != nil {
		r, g, b, a = shadow.Color.RGBA()
	}
	return shadowKey{
		r: r, g: g, b: b, a: a,
		corner: shadow.CornerRadius, blur: shadow.BlurRadius,
		width: shadow.Size().Width, height: shadow.Size().Height,
		pixScale: p.pixScale, opacity: p.groupOpacity,
	}
}

// evictShadows releases the cached shadows that the previous frame did not
// draw, at the start of a frame. A shadow drawn every frame is kept; one that
// changed shape (an elevation animation's intermediate steps), scrolled out
// of view or went away is dropped a frame later, so the cache holds about one
// frame's distinct shadows. Release waits for the frame's submit, as the
// previous frame's commands may still sample the texture.
func (p *painter) evictShadows() {
	for k, e := range p.shadows {
		if e.used+1 < p.frame {
			p.dev.ReleaseTextureAfterFrame(e.tex)
			delete(p.shadows, k)
		}
	}
}

// drawBackdropBlur renders a canvas.BackdropBlur: the already-rendered content
// under the widget is copied off the active render target, separably blurred, and
// drawn back (frosted glass). Ports gl.drawBackdropBlur. GPU-readback-heavy, like
// the GL path.
func (p *painter) drawBackdropBlur(blur *canvas.BackdropBlur, pos geo.Position, frame geo.Size) {
	if blur.BlurRadius <= 0 {
		return
	}

	// Screen-space capture rectangle. The WebGPU render target is top-down, so no
	// Y flip is needed (unlike the GL framebuffer grab).
	x := int(p.textureScale(pos.X))
	y := int(p.textureScale(pos.Y))
	w := int(p.textureScale(blur.Size().Width))
	h := int(p.textureScale(blur.Size().Height))
	if w <= 0 || h <= 0 {
		return
	}

	captured, capW, capH, err := p.dev.CopyActiveTargetRegion(x, y, w, h)
	if err != nil {
		return
	}

	blurred := p.blurTexture(captured.View(), capW, capH, paint.BlurSigma(blur.BlurRadius)*p.pixScale)
	p.dev.ReleaseTextureAfterFrame(captured)
	if blurred == nil {
		return
	}
	p.dev.ReleaseTextureAfterFrame(blurred)

	// Draw the blurred backdrop back over the widget's rectangle.
	prevTransform := p.transform
	p.transform = geo.Identity()
	p.blitTextureAlpha(blurred.View(), blur.Size(), pos, frame, p.effectiveAlpha(1))
	p.transform = prevTransform
}

// blurTexture runs a separable Gaussian blur of sigma texels on a w×h texture
// view via the blur shader and ping-pong off-screen targets, returning the
// blurred texture (owned by the caller). As paint.PlanBlur says, a wide blur
// first halves the texture, so the result can be smaller than w×h: draw it
// stretched. Mirrors gl.blurTexture. Returns nil on allocation failure.
func (p *painter) blurTexture(src *wgpu.TextureView, w, h int, sigma float32) *Texture {
	halvings, step := paint.PlanBlur(sigma)
	var halved *Texture
	for range halvings {
		if w == 1 && h == 1 {
			break
		}
		w, h = paint.HalveTexels(w), paint.HalveTexels(h)
		next, err := p.dev.CreateRenderTexture(w, h)
		if err != nil {
			if halved != nil {
				halved.Release()
			}
			return nil
		}
		// A blur of no width samples each target texel's centre, between
		// four source texels: linear filtering averages them.
		u := BlurUniforms{Resolution: [2]float32{float32(w), float32(h)}}
		p.dev.PushOffscreenTarget(next, w, h, wgpu.Color{})
		p.recordBlur(src, uniformBytes(&u), BlurFragmentEntryH)
		p.dev.PopTarget()
		if halved != nil {
			p.dev.ReleaseTextureAfterFrame(halved)
		}
		halved, src = next, next.View()
	}
	if halved != nil {
		defer p.dev.ReleaseTextureAfterFrame(halved)
	}

	texA, err := p.dev.CreateRenderTexture(w, h)
	if err != nil {
		return nil
	}
	texB, err := p.dev.CreateRenderTexture(w, h)
	if err != nil {
		texA.Release()
		return nil
	}
	u := BlurUniforms{Resolution: [2]float32{float32(w), float32(h)}, BlurRadius: step}

	// Horizontal then vertical, paint.BlurPasses times: src → texA → texB →
	// texA → texB.
	for range paint.BlurPasses {
		p.dev.PushOffscreenTarget(texA, w, h, wgpu.Color{})
		p.recordBlur(src, uniformBytes(&u), BlurFragmentEntryH)
		p.dev.PopTarget()

		p.dev.PushOffscreenTarget(texB, w, h, wgpu.Color{})
		p.recordBlur(texA.View(), uniformBytes(&u), BlurFragmentEntryV)
		p.dev.PopTarget()
		src = texB.View()
	}

	p.dev.ReleaseTextureAfterFrame(texA) // intermediate; texB is returned
	return texB
}

// recordBlur draws a full-screen quad through the blur shader's given fragment
// entry (BlurFragmentEntryH / V), sampling view, with premultiplied blend and no
// stencil (it must ignore any enclosing clip). The quad maps NDC top to texcoord
// v=0 so the top-down render-to-texture stays consistent across the ping-pong.
func (p *painter) recordBlur(view *wgpu.TextureView, uniform []byte, fragment string) {
	staged, ok := p.stage(uniform)
	if !ok {
		return
	}
	_ = p.dev.Record(DrawCall{
		Key: PipelineKey{
			Shader:   "blur",
			Fragment: fragment,
			Blend:    BlendPremul,
			Stencil:  StencilNone,
			Format:   p.dev.Format(),
		},
		Vertices:    fullScreenTexQuad,
		VertexCount: 4,
		Uniform:     staged,
		TexView:     view,
	})
}

// blitTextureAlpha draws a texture view as a stretched quad at pos via the simple
// shader at the given alpha (no corner radius). Like blitTexture but with an
// explicit alpha so shadows/backdrop honour the active group opacity.
func (p *painter) blitTextureAlpha(view *wgpu.TextureView, logical geo.Size, pos geo.Position, frame geo.Size, alpha float32) {
	points, insets := p.rectCoords(logical, pos, frame, canvas.ImageFillStretch, 0, 0)
	u := SimpleUniforms{Alpha: alpha, Inset: insets}
	p.record("simple", points, 4, uniformBytes(&u), view)
}
