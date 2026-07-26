//go:build !gl

package webgpu

import (
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// Box shadows and backdrop blur for WebGPU (#223), porting the GL painter's
// drawBoxShadow / drawBackdropBlur / blurTexture. Both reuse the Device's
// render-target stack (#218) and the ported `blur` WGSL (separable 9-tap,
// fs_main_h / fs_main_v) — the GL FBO ping-pong becomes off-screen render
// targets, the GL CopyTexSubImage2D framebuffer grab becomes CopyActiveTargetRegion.
//
// The off-screen shape / blur passes are pushed with PushOffscreenTarget so they
// get a dedicated depth-stencil attachment — never the shared size-keyed one —
// so their clearing passes can't wipe an enclosing canvas stencil clip when the
// off-screen pixel size happens to match the canvas (the #231 hazard). The blur
// content is premultiplied (the shape is rendered into a transparent target, and
// captured content is already composited), so it composites via the simple
// shader's premultiplied blend, matching the GL backend.

// drawBoxShadow renders a canvas.BoxShadow: the shadow shape (a rounded rect in
// the shadow color, padded for the blur spread) is drawn into an off-screen
// target, separably blurred, then drawn behind the widget. Ports gl.drawBoxShadow.
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

	shapeTex, err := p.dev.CreateRenderTexture(texW, texH)
	if err != nil {
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

	// Render the solid rounded rect into the (transparent) shape target.
	p.dev.PushOffscreenTarget(shapeTex, texW, texH, wgpu.Color{})
	shadowRect := &canvas.Rectangle{FillColor: shadow.Color, CornerRadius: shadow.CornerRadius}
	shadowRect.Resize(shadow.Size())
	fboFrame := geo.NewSize(shadowW, shadowH)
	p.drawRectangle(shadowRect, geo.NewPos(spread, spread), fboFrame)
	p.dev.PopTarget()

	// Separably blur the shape (radius normalised to texture space, as in GL).
	blurRadius := shadow.BlurRadius * p.pixScale / float32(texW) * 2
	blurred := p.blurTexture(shapeTex.View(), texW, texH, blurRadius)
	p.dev.ReleaseTextureAfterFrame(shapeTex)
	if blurred == nil {
		return
	}
	p.dev.ReleaseTextureAfterFrame(blurred)

	// Draw the blurred texture as a quad behind the widget (premultiplied content).
	shadowPos := geo.NewPos(pos.X+shadow.OffsetX-spread, pos.Y+shadow.OffsetY-spread)
	p.blitTextureAlpha(blurred.View(), geo.NewSize(shadowW, shadowH), shadowPos, frame, p.effectiveAlpha(1))
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

	// Blur radius in captured-texture pixels (matches the GL backdrop path).
	blurred := p.blurTexture(captured.View(), capW, capH, blur.BlurRadius*p.pixScale)
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

// blurTexture runs a two-pass separable Gaussian blur on a texture view via the
// blur shader and ping-pong off-screen targets, returning the blurred texture
// (owned by the caller). The radius is passed to the shader unchanged (resolution
// = w×h), mirroring gl.blurTexture's convention. Returns nil on allocation failure.
func (p *painter) blurTexture(src *wgpu.TextureView, w, h int, radius float32) *Texture {
	texA, err := p.dev.CreateRenderTexture(w, h)
	if err != nil {
		return nil
	}
	texB, err := p.dev.CreateRenderTexture(w, h)
	if err != nil {
		texA.Release()
		return nil
	}
	u := BlurUniforms{Resolution: [2]float32{float32(w), float32(h)}, BlurRadius: radius}

	// Pass 1: horizontal blur, src → texA.
	p.dev.PushOffscreenTarget(texA, w, h, wgpu.Color{})
	p.recordBlur(src, uniformBytes(&u), BlurFragmentEntryH)
	p.dev.PopTarget()

	// Pass 2: vertical blur, texA → texB.
	p.dev.PushOffscreenTarget(texB, w, h, wgpu.Color{})
	p.recordBlur(texA.View(), uniformBytes(&u), BlurFragmentEntryV)
	p.dev.PopTarget()

	p.dev.ReleaseTextureAfterFrame(texA) // intermediate; texB is returned
	return texB
}

// recordBlur draws a full-screen quad through the blur shader's given fragment
// entry (BlurFragmentEntryH / V), sampling view, with premultiplied blend and no
// stencil (it must ignore any enclosing clip). The quad maps NDC top to texcoord
// v=0 so the top-down render-to-texture stays consistent across the ping-pong.
func (p *painter) recordBlur(view *wgpu.TextureView, uniform []byte, fragment string) {
	points := []float32{
		-1, -1, 0, 0, 1,
		-1, 1, 0, 0, 0,
		1, -1, 0, 1, 1,
		1, 1, 0, 1, 0,
	}
	_ = p.dev.Record(DrawCall{
		Key: PipelineKey{
			Shader:   "blur",
			Fragment: fragment,
			Blend:    BlendPremul,
			Stencil:  StencilNone,
			Format:   p.dev.Format(),
		},
		Vertices:    points,
		VertexCount: 4,
		Uniform:     uniform,
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
