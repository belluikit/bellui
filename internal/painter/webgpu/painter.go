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
	"fmt"
	"image"
	"image/color"
	"sync"

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
	// boundaryFrames holds the logical size of each RepaintBoundary target
	// pushed and not yet popped, innermost last. An unbind pops only what a
	// bind pushed, and the clip stamps are drawn in the innermost frame (#564).
	boundaryFrames []geo.Size

	// stencilMode is the depth-stencil mode injected into every draw's pipeline
	// key while a stencil clip is active (StencilNone otherwise). Set by the
	// Start*Clipping / drawPathFillStencil ops; see device.go depthStencilState.
	stencilMode StencilMode

	// roundClips holds the active rounded clips and path clips, innermost
	// last, so a Stop can erase what its Start stamped (#466).
	roundClips []roundClip
	// clipBases holds, for each bound RepaintBoundary target, how many rounded
	// clips were active when it was bound: its own stencil starts empty, so
	// only the clips above that index stamp into it.
	clipBases []int
	// stamp is the box a rounded clip stamps into the stencil and erases.
	stamp stencilBox

	// verts and uni are scratch for one draw's vertices and uniform block.
	// Record copies both into the frame's arena before it returns, so draws
	// reuse them instead of allocating (#447). A vertex builder's result is
	// valid only until the next builder runs.
	verts []float32
	uni   [uniformBlockStride]byte

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

	// batch holds the batched drawing's glyph atlas and text layouts
	// (batch.go, #475).
	batch batchState
	frame uint64

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
	p.roundClips = p.roundClips[:0] // a frame never inherits an unpaired clip
	p.clipBases = p.clipBases[:0]
	p.evictShadows()
	p.evictLayouts()
	p.evictBoundaries()
	p.resetFullAtlas()
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
	// Drop the Scene3D cache entry; the texture itself was released above via
	// the shared cache handle. Only a Scene3D has one, so other objects, about
	// 4,000 a frame when 2,000 labels change, skip hashing into the map (#543).
	if _, ok := obj.(*canvas.Scene3D); ok {
		delete(p.scenes, obj)
	}
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
	//
	// The edges are scaled, not the origin and the size: two clips sharing an
	// edge in dp then share it in pixels. Scaling y and h separately rounds
	// them separately, so a partial repaint's clear could end a row below the
	// clip it then redraws a ClipToBounds container under, leaving that row
	// cleared (#975).
	x := int(p.textureScale(pos.X))
	y := int(p.textureScale(pos.Y))
	w := int(p.textureScale(pos.X+size.Width)) - x
	h := int(p.textureScale(pos.Y+size.Height)) - y
	p.dev.SetScissor(x, y, w, h)
}

func (p *painter) StopClipping() {
	if p.dev != nil {
		p.dev.ClearScissor()
	}
}

// roundClip is an active rounded clip: the rect its stamp lies in, its corner
// radius and the transform it was stamped with, so StopRoundedClipping can
// erase it.
//
// Clips nest by depth (#1012), kept in the stencil's depth bits (#1015, see
// paint.StencilDepthMask): a clip at depth d stamps d only where its enclosing
// clip left d-1, and draws test for d. So an inner clip narrows the outer one,
// and Stop returns its area to d-1, which is the outer clip again. Outside
// every clip the depth bits stay zero (#466).
type roundClip struct {
	pos       geo.Position
	size      geo.Size
	radius    float32
	transform geo.Matrix
	depth     int
	skipped   bool // nothing was stamped: the rect lies outside the pass's clip
	// shared is a clip deeper than paint.MaxClipDepth: it stamps nothing and
	// its content is cut by the clip around it only.
	shared bool
	// path is a path clip's level: its stamp is where the path's winding was
	// counted, and it is erased over the whole target (#1015).
	path bool
}

// StartRoundedClipping stencils a rounded-rect mask and restricts subsequent
// draws to it, inside any rounded clip already active.
//
// WebGPU cannot clear the stencil mid-pass, and clearing the whole target with
// a full-target quad for every clip cost a grid of cards one full-screen fill
// per card per frame, and on Apple GPUs tens of MB (#466). So the depth bits
// are kept zero outside active clips: StopRoundedClipping erases the clip's
// rect again, and a clip that finds them known to be zero just stamps. When
// they are not known to be zero (a pass that loaded its stencil), the whole
// target's depth bits are cleared first. Path ops write only the winding bits,
// so they leave the depth bits as they were (#1015).
func (p *painter) StartRoundedClipping(pos geo.Position, size geo.Size, cornerRadius float32) {
	if p.dev == nil {
		return
	}
	clip := roundClip{pos: pos, size: size, radius: cornerRadius, transform: p.transform}
	top := p.topRoundClip()
	if top != nil && top.skipped {
		// Inside a skipped clip nothing is drawn, so nothing is stamped either.
		clip.skipped = true
		p.roundClips = append(p.roundClips, clip)
		return
	}
	zero := top == nil && p.stencilMode == StencilNone && p.dev.StencilZero()
	if zero && !p.hasTransform() && p.dev.Culls(
		int(p.textureScale(pos.X)), int(p.textureScale(pos.Y)),
		int(p.textureScale(size.Width)), int(p.textureScale(size.Height)),
	) {
		// The clip lies outside everything the pass can draw, and the stencil is
		// zero: testing for paint.RejectClipCode rejects every draw in the
		// subtree without stamping.
		clip.skipped = true
		p.roundClips = append(p.roundClips, clip)
		p.dev.SetStencilRef(paint.RejectClipCode)
		p.stencilMode = StencilTestEqual
		return
	}
	clip.depth = 1
	if top != nil {
		clip.depth = top.depth + 1
	}
	if clip.depth > paint.MaxClipDepth {
		warnClipDepth()
		clip.depth = top.depth
		clip.shared = true
		p.roundClips = append(p.roundClips, clip)
		return
	}
	if top == nil && !p.dev.StencilZero() {
		p.clearStencilBits(StencilClearDepth)
	}
	p.roundClips = append(p.roundClips, clip)
	p.dev.SetStencilZero(false)
	p.stampRoundClip(&clip)
	p.applyRoundClip()
}

// warnClipDepth reports, once, a clip nested deeper than the stencil holds.
var warnClipDepth = sync.OnceFunc(func() {
	bellui.LogError(fmt.Sprintf("webgpu: clips nest deeper than %d; deeper ones are cut by the clip around them only", paint.MaxClipDepth), nil)
})

// stampRoundClip draws clip's rounded rect into the stencil, raising it from
// the enclosing clip's depth to clip's, color writes masked off.
func (p *painter) stampRoundClip(clip *roundClip) {
	// The round_rectangle shader discards outside the shape, so only the
	// interior is marked. It keeps every fragment with any anti-aliasing
	// coverage (alpha > 0), so a stamp at the exact geometry would mark one
	// edge-softness band BEYOND the shape — clipped content then pokes past
	// the visibly drawn arc by a couple of hidpi pixels (pre-rebrand tracker #469). Inset the
	// stamp by 1.5 softness (concentric radius): its coverage then ends half
	// a band INSIDE the geometric boundary — the midpoint of the drawn
	// edge's own anti-aliasing, which is where the eye reads the outline.
	prev := p.transform
	p.transform = clip.transform
	p.dev.SetStencilRef(paint.ClipDepthCode(clip.depth))
	p.stencilMode = stampMode(clip.depth)
	inset := float32(edgeSoftness) * 1.5
	p.stamp = stencilBox{pos: clip.pos.Add(geo.NewSquareOffsetPos(inset)), size: clip.size.Subtract(geo.NewSquareSize(inset * 2))}
	r := geo.Max(0, clip.radius-inset)
	p.drawOblong(&p.stamp, stampFill, nil, 0, r, r, r, r, 0, p.stamp.pos, p.targetFrame())
	p.transform = prev
}

// stampMode is the stencil mode that stamps and erases rounded-clip depth d.
func stampMode(d int) StencilMode {
	return StencilStampBit0 + StencilMode(paint.ClipStampBit(d))
}

// topRoundClip returns the innermost rounded clip active in the bound target,
// or nil.
func (p *painter) topRoundClip() *roundClip {
	if n := len(p.roundClips); n > p.roundClipBase() {
		return &p.roundClips[n-1]
	}
	return nil
}

// roundClipBase is the index of the first rounded clip that stamps into the
// bound target.
func (p *painter) roundClipBase() int {
	if n := len(p.clipBases); n > 0 {
		return p.clipBases[n-1]
	}
	return 0
}

// applyRoundClip sets the stencil test for the innermost active rounded clip
// in the bound target, or turns it off when there is none.
func (p *painter) applyRoundClip() {
	top := p.topRoundClip()
	switch {
	case top == nil:
		p.stencilMode = StencilNone
	case top.skipped:
		p.dev.SetStencilRef(paint.RejectClipCode)
		p.stencilMode = StencilTestEqual
	default:
		p.dev.SetStencilRef(paint.ClipDepthCode(top.depth))
		p.stencilMode = StencilTestEqual
	}
}

// clipRef is the stencil reference, and the winding mode, of a path op inside
// the innermost rounded clip of the bound target: its winding is counted only
// there (#1015). Outside every clip, it is counted everywhere.
func (p *painter) clipRef(incrDecr bool) (uint32, StencilMode) {
	top := p.topRoundClip()
	switch {
	case top == nil && incrDecr:
		return 0, StencilWriteIncrDecr
	case top == nil:
		return 0, StencilWriteInvert
	}
	ref := paint.ClipDepthCode(top.depth)
	if top.skipped {
		ref = paint.RejectClipCode
	}
	if incrDecr {
		return ref, StencilWriteIncrDecrInClip
	}
	return ref, StencilWriteInvertInClip
}

// targetFrame returns the logical size of the render target draws land in: a
// bound RepaintBoundary's, or the canvas's. Clip stamps take no frame from
// the walk, and stamped against the canvas inside a boundary they came out
// scaled by the boundary's share of it (#564).
func (p *painter) targetFrame() geo.Size {
	if n := len(p.boundaryFrames); n > 0 {
		return p.boundaryFrames[n-1]
	}
	return p.canvas.Size()
}

// StopRoundedClipping ends the innermost rounded clip: it erases the clip's
// stamp back to the enclosing clip's depth and restores that clip's test, or
// turns the test off when no clip encloses it, leaving the depth bits zero.
func (p *painter) StopRoundedClipping() {
	if p.dev == nil {
		return
	}
	if len(p.roundClips) == p.roundClipBase() {
		p.stencilMode = StencilNone
		return
	}
	clip := p.roundClips[len(p.roundClips)-1]
	p.roundClips = p.roundClips[:len(p.roundClips)-1]
	p.popClip(clip)
}

// popClip erases the stamp of clip, just taken off roundClips, and restores
// the enclosing clip's test.
func (p *painter) popClip(clip roundClip) {
	stamped := !clip.skipped && !clip.shared
	switch {
	case stamped && clip.path:
		// The stamp's area is wherever the depth bits hold the clip's depth.
		p.clearStencilBitsWith(paint.ClipDepthCode(clip.depth-1), stampMode(clip.depth))
	case stamped:
		prev := p.transform
		p.transform = clip.transform
		p.fillStencilRect(clip.pos, clip.size, paint.ClipDepthCode(clip.depth-1), stampMode(clip.depth))
		p.transform = prev
	}
	p.applyRoundClip()
	// Start left the depth bits zero outside the stamp (they were zero, or
	// cleared), and the erase covers the stamp, so they are zero again unless
	// a clip still encloses this one.
	if stamped && p.topRoundClip() == nil {
		p.dev.SetStencilZero(true)
	}
}

// fillStencilRect draws the rect (pos, size) into the stencil in mode, with
// reference ref, color writes masked off. It uses the current transform, as
// the stamp does.
func (p *painter) fillStencilRect(pos geo.Position, size geo.Size, ref uint32, mode StencilMode) {
	prev := p.stencilMode
	p.dev.SetStencilRef(ref)
	p.stencilMode = mode
	p.stamp = stencilBox{pos: pos, size: size}
	p.drawOblong(&p.stamp, stampFill, nil, 0, 0, 0, 0, 0, 0, pos, p.targetFrame())
	p.stencilMode = prev
}

// stencilBox is a rectangle drawn only into the stencil. It is not a
// canvas.Rectangle, whose Move and Resize would look up a canvas for it on
// every clip (#447).
type stencilBox struct {
	pos  geo.Position
	size geo.Size
}

func (b *stencilBox) Position() geo.Position { return b.pos }
func (b *stencilBox) Size() geo.Size         { return b.size }

// stampFill is the stencil stamps' colour, converted to an interface once.
var stampFill color.Color = color.White

// StartPathClipping stencils an arbitrary path's fill (non-zero winding) and
// restricts subsequent draws to it, inside the clips around it, mirroring the
// GL painter. The path's area becomes a clip level of its own in the depth
// bits, so what is drawn inside it, rounded clips and paths included, is cut
// by it (#1015).
func (p *painter) StartPathClipping(path *canvas.Path, pos geo.Position, size geo.Size) {
	if p.dev == nil {
		return
	}
	// Every Start pushes a level, so its Stop pops its own.
	clip := roundClip{path: true}
	top := p.topRoundClip()
	if top != nil {
		clip.depth = top.depth
	}
	var fanTris []float32
	if path != nil {
		if subpaths := paint.FlattenPath(path.Elements(), p.pixScale); len(subpaths) > 0 {
			fanTris = paint.FanTriangulate(subpaths)
		}
	}
	switch {
	case len(fanTris) < 6:
		// Nothing to clip to: the content is drawn as without the clip.
		clip.shared = true
		p.roundClips = append(p.roundClips, clip)
		return
	case top != nil && top.skipped:
		clip.skipped = true
		p.roundClips = append(p.roundClips, clip)
		return
	}
	p.clearStencilBits(StencilClearWinding)
	// Winding pass: increment front / decrement back faces into the winding
	// bits, inside the innermost clip, color masked off.
	ref, mode := p.clipRef(true)
	p.dev.SetStencilRef(ref)
	p.stencilMode = mode
	verts := p.pathFanToNDC(fanTris, pos, p.targetFrame())
	u := PathUniforms{}
	p.record("path", verts, len(verts)/3, uniformBytes(&u), nil)

	clip.depth++
	if clip.depth > paint.MaxClipDepth {
		// Too deep for a level: the content is tested against the winding,
		// so it is cut by the path and the clips around it, but what is
		// nested inside it is not.
		warnClipDepth()
		clip.depth--
		clip.shared = true
		p.roundClips = append(p.roundClips, clip)
		p.dev.SetStencilRef(0)
		p.stencilMode = StencilTestNotEqual
		return
	}
	if top == nil && !p.dev.StencilZero() {
		p.clearStencilBits(StencilClearDepth)
	}
	p.dev.SetStencilZero(false)
	p.clearStencilBitsWith(paint.ClipDepthCode(clip.depth), StencilStampWinding)
	p.roundClips = append(p.roundClips, clip)
	p.applyRoundClip()
}

// StopPathClipping ends a path clip: its level is erased, and the clips
// around it apply again.
func (p *painter) StopPathClipping() {
	if p.dev == nil || len(p.roundClips) == p.roundClipBase() {
		return
	}
	clip := p.roundClips[len(p.roundClips)-1]
	p.roundClips = p.roundClips[:len(p.roundClips)-1]
	p.popClip(clip)
}

// clearStencilBits zeroes the bits mode writes, StencilClearWinding or
// StencilClearDepth, over the whole target, by drawing a full-target quad with
// reference 0 (color masked off). WebGPU can only clear the stencil at a pass's
// load op, so this is the equivalent of the GL painter's glClear(STENCIL) under
// a stencil write mask.
func (p *painter) clearStencilBits(mode StencilMode) {
	p.clearStencilBitsWith(0, mode)
}

// clearStencilBitsWith draws a full-target quad into the stencil in mode with
// reference ref, color masked off.
func (p *painter) clearStencilBitsWith(ref uint32, mode StencilMode) {
	prev := p.stencilMode
	p.dev.SetStencilRef(ref)
	p.stencilMode = mode
	u := PathUniforms{}
	p.record("path", fullTargetTriangle, 3, uniformBytes(&u), nil)
	p.stencilMode = prev
}

// fullTargetTriangle covers the whole clip space in NDC; vert.xy + alpha.
var fullTargetTriangle = []float32{-1, -1, 1, 3, -1, 1, -1, 3, 1}

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

// FramesBegun counts the frames the painter has begun. A frame for a window
// the system reports occluded never begins on WebGPU, so Canvas.Capture can
// tell a frame it painted from one it could not (#631).
func (p *painter) FramesBegun() uint64 { return p.frame }

// FrameOpen reports whether a frame is open to draw into: begun, and not yet
// presented or abandoned. The drivers do not walk a window without one (#1019).
func (p *painter) FrameOpen() bool { return p.dev != nil && p.framing }
