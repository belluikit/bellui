//go:build !gl

package webgpu

import (
	"github.com/belluikit/webgpu/wgpu"
)

// Persistent full-canvas FBO for dirty-region culling (pre-rebrand tracker #202), implementing
// gpu.DirtyRegionPainter. It mirrors internal/painter/gl/canvas_fbo.go but maps
// onto the Device's render-target stack: content is rendered into a persistent
// off-screen texture (kept across frames so partial repaints can retain pixels
// outside the dirty region), then blitted to the surface as an opaque full-screen
// copy before overlays are drawn.
//
// WebGPU has no mid-pass clear, so the per-frame clear (full, or just the
// scissored dirty region) is performed by painting the background color over the
// FBO; the active scissor restricts it to the region. The FBO pass itself opens
// with LoadOp=Load (PushTargetLoad) to preserve last frame's content. Unlike GL,
// render-to-texture is top-down, so the blit needs no V-flip.

// canvasFBOState is the persistent canvas render target. Pixel dimensions; the
// texture is recreated on resize via EnsureCanvasFBO.
type canvasFBOState struct {
	tex          *Texture
	width        int
	height       int
	initialClear bool // true after the first full clear; drives CanvasFBONeedsFullClear
	active       bool // true while pushed as the current render target this frame
}

// EnsureCanvasFBO creates or resizes the persistent canvas FBO.
func (p *painter) EnsureCanvasFBO(pixW, pixH int) {
	if p.dev == nil || pixW <= 0 || pixH <= 0 {
		return
	}
	if p.canvasFBO != nil && p.canvasFBO.width == pixW && p.canvasFBO.height == pixH {
		return
	}
	if p.canvasFBO != nil && p.canvasFBO.tex != nil {
		// The old target may already be used by this frame's commands.
		p.dev.ReleaseTextureAfterFrame(p.canvasFBO.tex)
	}
	tex, err := p.dev.CreateRenderTexture(pixW, pixH)
	if err != nil {
		p.canvasFBO = nil
		return
	}
	p.canvasFBO = &canvasFBOState{tex: tex, width: pixW, height: pixH}
}

// MarkCanvasFBORetentionUnreliable disables dirty-region partial repaint for this
// painter: every subsequent frame does a full clear + redraw of the whole canvas.
// After a mobile background→resume the persistent off-screen FBO stops retaining
// its contents across frames on tiled GPUs (LoadOp=Load loads undefined even for a
// freshly allocated texture), so any frame that redraws only a dirty region loses
// all the retained pixels and static UI flickers/vanishes (pre-rebrand tracker #342). Falling back to
// full repaint — which never relies on retention and is already the safe default —
// keeps rendering correct at the cost of the optimization for the rest of the
// session.
func (p *painter) MarkCanvasFBORetentionUnreliable() {
	p.fboRetentionUnreliable = true
}

// BindCanvasFBO begins the frame (if not already begun) and redirects subsequent
// content draws into the persistent canvas FBO, opened with LoadOp=Load so the
// previous frame's pixels are retained for dirty-region partial repaints.
func (p *painter) BindCanvasFBO() {
	if p.dev == nil || p.canvasFBO == nil || p.canvasFBO.tex == nil {
		return
	}
	if !p.framing {
		p.beginFrame()
	}
	if !p.framing { // BeginFrame failed (e.g. lost surface) — fall back to surface draws.
		// Nothing this frame reaches the FBO, but the driver's partial-repaint
		// bookkeeping moves on as if it had: the next frame must clear and
		// redraw it all, or what this one missed stays missing (#595).
		p.canvasFBO.initialClear = false
		return
	}
	p.dev.PushTargetLoad(p.canvasFBO.tex, p.canvasFBO.width, p.canvasFBO.height)
	p.canvasFBO.active = true
}

// clearActiveToBackground clears the active canvas FBO to the theme background.
// Drawn as a full-screen background quad with Replace blend (an opaque overwrite,
// mirroring glClear); the current scissor, if any, restricts it to the dirty
// region so the rest of the retained FBO is preserved.
func (p *painter) clearActiveToBackground() {
	bg := backgroundColor()
	u := PathUniforms{FillColor: bg}
	p.recordWithBlend("path", fullScreenPathQuad, 6, uniformBytes(&u), nil, BlendReplace)
}

// fullScreenPathQuad is a full-screen quad in NDC (path layout: vert.xy +
// coverage alpha = 1).
var fullScreenPathQuad = []float32{
	-1, -1, 1,
	1, -1, 1,
	-1, 1, 1,
	1, -1, 1,
	1, 1, 1,
	-1, 1, 1,
}

// UnbindAndBlitCanvasFBO pops back to the surface and blits the canvas FBO over
// it as an opaque full-screen copy (Replace blend), so anti-aliased edges in the
// FBO don't let the stale swapchain buffer bleed through. Overlays are drawn onto
// the surface afterwards by the driver.
func (p *painter) UnbindAndBlitCanvasFBO(pixW, pixH int) {
	if p.dev == nil || p.canvasFBO == nil || p.canvasFBO.tex == nil || !p.canvasFBO.active {
		return
	}
	p.dev.PopTarget()
	p.canvasFBO.active = false
	p.blitFullScreen(p.canvasFBO.tex.View(), BlendReplace)
}

// blitFullScreen draws a texture view across the whole render target via the
// simple shader (full opacity, no corner radius). No V-flip: render-to-texture is
// top-down, matching the surface origin.
func (p *painter) blitFullScreen(view *wgpu.TextureView, blend BlendMode) {
	u := SimpleUniforms{Alpha: 1, Inset: [4]float32{0, 0, 1, 1}}
	p.recordWithBlend("simple", fullScreenTexQuad, 4, uniformBytes(&u), view, blend)
}

// fullScreenTexQuad is a full-screen textured quad. NDC top maps to v 0, since
// render-to-texture is top-down.
var fullScreenTexQuad = []float32{
	// x,  y, z, u, v
	-1, -1, 0, 0, 1,
	-1, 1, 0, 0, 0,
	1, -1, 0, 1, 1,
	1, 1, 0, 1, 0,
}

// HasCanvasFBO reports whether the persistent canvas FBO is allocated.
func (p *painter) HasCanvasFBO() bool {
	return p.canvasFBO != nil && p.canvasFBO.tex != nil
}

// CanvasFBONeedsFullClear reports that the FBO must be cleared fully this frame:
// either it was just created/resized (no valid previous content), or FBO
// retention has been marked unreliable after a resume so partial repaint is
// disabled for the rest of the session (pre-rebrand tracker #342).
func (p *painter) CanvasFBONeedsFullClear() bool {
	return p.fboRetentionUnreliable || (p.canvasFBO != nil && !p.canvasFBO.initialClear)
}

// MarkCanvasFBOCleared records that the FBO has been fully cleared at least once.
// A frame that could not be opened cleared nothing, so the mark is ignored then.
func (p *painter) MarkCanvasFBOCleared() {
	if p.canvasFBO != nil && p.canvasFBO.active {
		p.canvasFBO.initialClear = true
	}
}
