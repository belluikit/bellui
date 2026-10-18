//go:build !gl

// Package webgpu provides a WebGPU implementation of the backend-neutral
// gpu.Painter (WebGPU migration, pre-rebrand tracker #198). The shader ports and the
// device-independent pipeline/uniform/layout foundation compile on every
// platform; the GPU-facing painter here pulls in the cgo wgpu-native binding and
// is now the default backend (Phase 5, pre-rebrand tracker #204). The legacy OpenGL painter is the
// fallback behind the `gl` build tag.
//
// Default build uses this painter; for the GL fallback build with: go build -tags gl ...
package webgpu

import (
	"image"
	"image/color"

	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/gpu"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/theme"
)

// Verify painter implements the backend-neutral painter interfaces.
var (
	_ gpu.Painter            = (*painter)(nil)
	_ gpu.BoundaryPainter    = (*painter)(nil)
	_ gpu.DirtyRegionPainter = (*painter)(nil)
)

// painter is the WebGPU renderer for a single canvas. It mirrors the GL
// painter's role: it holds per-canvas paint state (scale, group opacity, world
// transform) and delegates GPU work to a shared Device.
type painter struct {
	canvas          core.Canvas
	contextProvider driver.WithContext

	dev *Device
	sd  *wgpu.SurfaceDescriptor // provided by the driver before Init

	texScale     float32
	pixScale     float32
	groupOpacity float32
	transform    geo.Matrix
	outW, outH   int
	framing      bool // true between Clear (frame begin) and Present

	// stencilMode is the depth-stencil mode injected into every draw's pipeline
	// key while a stencil clip is active (StencilNone otherwise). Set by the
	// Start*Clipping / drawPathFillStencil ops; see device.go depthStencilState.
	stencilMode StencilMode

	// textures maps cache handles (stored in internal/cache) to GPU textures.
	textures  map[uint32]*Texture
	nextTexID uint32

	// boundaries maps RepaintBoundary objects to their cached off-screen target.
	boundaries map[core.CanvasObject]*boundaryState

	// scenes maps Scene3D objects to their cached rendered color texture + the
	// pixel size it was rendered at (so a resize re-renders). See draw_scene3d.go.
	scenes map[core.CanvasObject]*sceneState

	// shadows caches blurred box-shadow textures by everything that decides
	// their pixels, so an unchanged shadow is blitted, not re-rendered, and
	// equal shadows share one texture. frame counts begun frames, for eviction.
	// See draw_shadow.go (#434).
	shadows map[shadowKey]*shadowEntry
	frame   uint64

	// canvasFBO is the persistent full-canvas render target for dirty-region
	// culling (gpu.DirtyRegionPainter); nil when not allocated. See canvas_fbo.go.
	canvasFBO *canvasFBOState

	// fboRetentionUnreliable forces full repaint for the session once the canvas
	// FBO is known not to retain across frames (after an Android resume, pre-rebrand tracker #342).
	fboRetentionUnreliable bool
}

// NewPainter creates a WebGPU renderer for the provided canvas. It matches the
// gl.NewPainter signature so the driver can swap backends at the call site. The
// driver must supply a surface descriptor (via SetSurfaceDescriptor, using the
// platform bridge) before Init brings the device up.
func NewPainter(c core.Canvas, ctx driver.WithContext) gpu.Painter {
	p := &painter{canvas: c, contextProvider: ctx, groupOpacity: 1, transform: geo.Identity()}
	p.SetFrameBufferScale(1.0)
	return p
}

// SetSurfaceDescriptor supplies the native surface descriptor used by Init.
// The driver reaches it via a type assertion on the gpu.Painter returned by
// NewPainter. See surface_*.go for the platform bridges that build the
// descriptor from a window handle.
func (p *painter) SetSurfaceDescriptor(sd *wgpu.SurfaceDescriptor) { p.sd = sd }

// RecreateSurface rebuilds the presentation surface from a new native-window
// descriptor (e.g. Android delivers a fresh ANativeWindow on resume, pre-rebrand tracker #310).
// Before the device is up it just records the descriptor for Init to use.
func (p *painter) RecreateSurface(sd *wgpu.SurfaceDescriptor) {
	p.sd = sd
	if p.dev != nil {
		// Drop any frame still holding a texture from the old surface before it is
		// released and rebuilt (same SurfaceOutput hazard as SetOutputSize, pre-rebrand tracker #337).
		if p.framing && p.dev.HasOpenFrame() {
			p.dev.DiscardFrame()
			p.framing = false
		}
		p.dev.RecreateSurface(sd)
	}
}

// ReleaseSurface drops the presentation surface while keeping the device, so
// another window can own the shared native window (Android's single
// ANativeWindow hosts one configured surface at a time, pre-rebrand tracker #753). A later
// RecreateSurface rebinds this painter to the native window. A dangling
// un-presented frame is discarded first (same SurfaceOutput hazard as
// SetOutputSize, pre-rebrand tracker #337). No-op before the device is up.
func (p *painter) ReleaseSurface() {
	if p.dev == nil {
		return
	}
	if p.framing && p.dev.HasOpenFrame() {
		p.dev.DiscardFrame()
		p.framing = false
	}
	p.dev.ReleaseSurface()
}

// Init brings up the WebGPU device against the configured surface. It is a
// no-op until both a surface descriptor and an output size are known.
func (p *painter) Init() {
	if p.dev != nil || p.sd == nil || p.outW == 0 || p.outH == 0 {
		return
	}
	dev, err := NewDevice(p.sd, p.outW, p.outH)
	if err != nil {
		// Surfacing this to the driver's backend-select is a wiring follow-up;
		// for now a failed Init leaves dev nil and paint calls are skipped — but
		// log it, since otherwise a device-bring-up failure renders a silent
		// blank window.
		bellui.LogError("WebGPU device init failed", err)
		return
	}
	p.dev = dev
}

func (p *painter) SetFrameBufferScale(scale float32) {
	p.texScale = scale
	p.pixScale = p.canvas.Scale() * p.texScale
}

func (p *painter) SetOutputSize(width, height int) {
	p.outW, p.outH = width, height
	if p.dev != nil {
		// A frame begun by paint() but never presented (the driver skips Present for
		// a non-visible window) still holds the surface texture; reconfiguring the
		// surface with that SurfaceOutput live is a fatal WebGPU validation error, so
		// discard the dangling frame before resizing (pre-rebrand tracker #337).
		if p.framing && p.dev.HasOpenFrame() {
			p.dev.DiscardFrame()
			p.framing = false
		}
		p.dev.Resize(width, height)
	} else {
		p.Init()
	}
}

// Clear begins a new frame, clearing the surface to the theme background. In
// WebGPU the clear is the render pass LoadOp, so the frame's render pass opens
// here and is submitted+presented by Present.
//
// When a persistent canvas FBO is bound (dirty-region path, canvas_fbo.go) the
// frame has already begun and the FBO is the active target; this call then
// clears the FBO — fully, or just the scissored dirty region — to the
// background instead of beginning a new frame.
func (p *painter) Clear() {
	if p.dev == nil {
		return
	}
	if p.framing && p.canvasFBO != nil && p.canvasFBO.active {
		p.clearActiveToBackground()
		return
	}
	if !p.framing {
		p.beginFrame()
	}
}

// beginFrame opens the frame's root render pass on the surface, clearing it to
// the theme background.
func (p *painter) beginFrame() {
	bg := backgroundColor()
	clear := wgpu.Color{R: float64(bg[0]), G: float64(bg[1]), B: float64(bg[2]), A: float64(bg[3])}
	if err := p.dev.BeginFrame(clear); err != nil {
		return
	}
	p.framing = true
	p.frame++
	p.evictShadows()
}

// backgroundColor returns the theme background as straight RGBA in 0..1.
func backgroundColor() [4]float32 {
	r, g, b, a := theme.Color(theme.ColorNameBackground).RGBA()
	return [4]float32{
		float32(r) / 0xffff,
		float32(g) / 0xffff,
		float32(b) / 0xffff,
		float32(a) / 0xffff,
	}
}

// GPUFrameNanos returns the most recent frame's measured GPU execution time in
// nanoseconds (BELLUI_GPU_PROFILE, pre-rebrand tracker #336), and whether a measurement is available.
// The driver's frame profiler reads it via a type assertion.
func (p *painter) GPUFrameNanos() (uint64, bool) {
	if p.dev == nil {
		return 0, false
	}
	return p.dev.GPUFrameNanos()
}

// Present submits the frame's commands and presents the surface. The driver
// calls this (via a type assertion) where it would call SwapBuffers for GL.
func (p *painter) Present() {
	if p.dev == nil || !p.framing {
		return
	}
	_ = p.dev.EndFrame()
	p.framing = false
}

// AbandonFrame discards the frame in progress without presenting it. The
// mobile driver calls this instead of Present when the native window died
// while the frame was being recorded (Android destroys the ANativeWindow
// mid-recreation, #283): presenting into the dead window is what wedges the
// GPU channel, and the open frame's SurfaceOutput must not be left dangling
// (the same hazard SetOutputSize and RecreateSurface guard against, #337).
func (p *painter) AbandonFrame() {
	if p.dev == nil || !p.framing {
		return
	}
	if p.dev.HasOpenFrame() {
		p.dev.DiscardFrame()
	}
	p.framing = false
}

func (p *painter) Paint(obj core.CanvasObject, pos geo.Position, frame geo.Size) {
	if p.dev == nil || !p.framing || !obj.Visible() {
		return
	}
	p.drawObject(obj, pos, frame)
}

func (p *painter) Free(obj core.CanvasObject) {
	if id, ok := cache.GetTexture(obj); ok {
		p.releaseTexture(uint32(id))
		cache.DeleteTexture(obj)
	}
	// Drop any Scene3D cache entry (no-op for non-scene objects); the texture
	// itself was released above via the shared cache handle.
	delete(p.scenes, obj)
}

// EnableCapture arms the device's per-frame capture copy without reading a
// frame back. The driver calls it before forcing the fresh pre-capture repaint
// (pre-rebrand tracker #473), so that repaint's EndFrame copies the presented surface — overlays
// included — for the Capture that follows.
func (p *painter) EnableCapture() {
	if p.dev != nil {
		p.dev.EnableCapture()
	}
}

func (p *painter) Capture(c core.Canvas) image.Image {
	if p.dev != nil {
		// Maintain the per-frame capture copy from now on (pre-rebrand tracker #219); the copy is
		// skipped until the first capture request to save per-frame GPU cost.
		p.dev.EnableCapture()
		if img := p.dev.CaptureImage(); img != nil {
			return img
		}
		// First capture before the per-frame copy is populated: read the
		// persistent canvas FBO directly (it retains the last frame's content,
		// minus overlays, and is CopySrc-capable).
		if p.canvasFBO != nil && p.canvasFBO.tex != nil {
			if img := p.dev.CaptureTexture(p.canvasFBO.tex.tex, p.canvasFBO.width, p.canvasFBO.height); img != nil {
				return img
			}
		}
	}
	return image.NewRGBA(image.Rect(0, 0, p.outW, p.outH))
}

func (p *painter) StartClipping(pos geo.Position, size geo.Size) {
	if p.dev == nil {
		return
	}
	// WebGPU framebuffer origin is top-left (y-down), so no Y flip is needed
	// (unlike the GL painter's bottom-left scissor).
	x := int(p.textureScale(pos.X))
	y := int(p.textureScale(pos.Y))
	w := int(p.textureScale(size.Width))
	h := int(p.textureScale(size.Height))
	p.dev.SetScissor(x, y, w, h)
}

func (p *painter) StopClipping() {
	if p.dev != nil {
		p.dev.ClearScissor()
	}
}

// StartRoundedClipping stencils a rounded-rect mask (ref=1) and restricts
// subsequent draws to it. Mirrors the GL painter's stencil rounded clip; since
// WebGPU cannot clear the stencil mid-pass, the buffer is first zeroed with a
// full-target quad (clearStencil) to emulate GL's glClear(STENCIL).
func (p *painter) StartRoundedClipping(pos geo.Position, size geo.Size, cornerRadius float32) {
	if p.dev == nil {
		return
	}
	p.clearStencil()
	// Stamp the rounded-rect shape into the stencil as 1 (the round_rectangle
	// shader discards outside the shape, so only the interior is marked), color
	// writes masked off via the StencilWriteReplace pipeline.
	//
	// The shader keeps every fragment with any anti-aliasing coverage
	// (alpha > 0), so a stamp at the exact geometry would mark one
	// edge-softness band BEYOND the shape — clipped content then pokes past
	// the visibly drawn arc by a couple of hidpi pixels (pre-rebrand tracker #469). Inset the
	// stamp by 1.5 softness (concentric radius): its coverage then ends half
	// a band INSIDE the geometric boundary — the midpoint of the drawn
	// edge's own anti-aliasing, which is where the eye reads the outline.
	p.dev.SetStencilRef(1)
	p.stencilMode = StencilWriteReplace
	r := canvas.NewRectangle(color.White)
	inset := float32(edgeSoftness) * 1.5
	r.Resize(size.Subtract(geo.NewSquareSize(inset * 2)))
	r.Move(pos.Add(geo.NewSquareOffsetPos(inset)))
	r.CornerRadius = geo.Max(0, cornerRadius-inset)
	p.drawRectangle(r, r.Position(), p.canvas.Size())
	// Restrict subsequent draws to where the stencil == 1.
	p.stencilMode = StencilTestEqual
}

// StopRoundedClipping disables stencil clipping (GL: Disable(stencilTest)).
func (p *painter) StopRoundedClipping() {
	if p.dev != nil {
		p.stencilMode = StencilNone
	}
}

// StartPathClipping stencils an arbitrary path's fill (non-zero winding) and
// restricts subsequent draws to it, mirroring the GL painter.
func (p *painter) StartPathClipping(path *canvas.Path, pos geo.Position, size geo.Size) {
	if p.dev == nil || path == nil {
		return
	}
	elements := path.Elements()
	if len(elements) == 0 {
		return
	}
	subpaths := paint.FlattenPath(elements, p.pixScale)
	if len(subpaths) == 0 {
		return
	}
	fanTris := paint.FanTriangulate(subpaths)
	if len(fanTris) < 6 {
		return
	}
	p.clearStencil()
	// Winding pass: increment front / decrement back faces into the stencil,
	// color masked off.
	p.dev.SetStencilRef(0)
	p.stencilMode = StencilWriteIncrDecr
	verts := p.pathFanToNDC(fanTris, pos, p.canvas.Size())
	u := PathUniforms{}
	p.record("path", verts, len(verts)/3, uniformBytes(&u), nil)
	// Restrict subsequent draws to where the winding count is non-zero.
	p.stencilMode = StencilTestNotEqual
}

// StopPathClipping disables stencil path clipping.
func (p *painter) StopPathClipping() {
	if p.dev != nil {
		p.stencilMode = StencilNone
	}
}

// clearStencil zeroes the whole stencil buffer by drawing a full-target quad in
// StencilWriteReplace mode with reference 0 (color masked off). WebGPU can only
// clear the stencil at a pass's load op, so each clip/fill op that needs a clean
// slate calls this first — the equivalent of the GL painter's glClear(STENCIL).
func (p *painter) clearStencil() {
	prev := p.stencilMode
	p.dev.SetStencilRef(0)
	p.stencilMode = StencilWriteReplace
	// Full-target triangle in NDC (covers the whole clip space); vert.xy + alpha.
	verts := []float32{-1, -1, 1, 3, -1, 1, -1, 3, 1}
	u := PathUniforms{}
	p.record("path", verts, 3, uniformBytes(&u), nil)
	p.stencilMode = prev
}

func (p *painter) SetGroupOpacity(opacity float32) { p.groupOpacity = opacity }
func (p *painter) ClearGroupOpacity()              { p.groupOpacity = 1 }
func (p *painter) SetTransform(m geo.Matrix)       { p.transform = m }

// effectiveAlpha applies the active group opacity (MD3 disabled state).
func (p *painter) effectiveAlpha(alpha float32) float32 {
	if p.groupOpacity > 0 && p.groupOpacity < 1 {
		return alpha * p.groupOpacity
	}
	return alpha
}
