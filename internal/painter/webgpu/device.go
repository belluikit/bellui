//go:build !gl

package webgpu

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/webgpu/wgpu"
)

// choosePresentMode returns the surface present mode. It defaults to Fifo (vsync,
// the only universally-supported mode and the right choice for normal apps), but
// honors BELLUI_WEBGPU_PRESENT=immediate|mailbox|fifo when that mode is advertised
// by the surface. The override exists to take vsync out of the loop when
// frame-time profiling (BELLUI_FRAME_DEBUG, pre-rebrand issue #9): under Fifo every paint
// blocks on the swapchain at the next refresh, which masks the actual draw-call
// cost the partial-repaint optimization changes.
func choosePresentMode(modes []wgpu.PresentMode) wgpu.PresentMode {
	want := os.Getenv("BELLUI_WEBGPU_PRESENT")
	if want == "" {
		return wgpu.PresentModeFifo
	}
	var target wgpu.PresentMode
	switch want {
	case "immediate":
		target = wgpu.PresentModeImmediate
	case "mailbox":
		target = wgpu.PresentModeMailbox
	case "fifo":
		return wgpu.PresentModeFifo
	default:
		return wgpu.PresentModeFifo
	}
	for _, m := range modes {
		if m == target {
			bellui.LogError(fmt.Sprintf("webgpu present mode: using %s (BELLUI_WEBGPU_PRESENT)", want), nil)
			return target
		}
	}
	bellui.LogError(fmt.Sprintf("webgpu present mode: %s not advertised %v, keeping fifo", want, modes), nil)
	return wgpu.PresentModeFifo
}

// logSurfaceFormats reports the surface's advertised formats (preferred first)
// and the one chooseSurfaceFormat selected, gated behind BELLUI_WEBGPU_DEBUG so it
// is silent in normal use. It is the diagnostic for pre-rebrand tracker #325's "watch for a surface
// advertising only sRGB" concern: a backend that offers no non-sRGB sibling
// makes chooseSurfaceFormat keep an sRGB format, which re-encodes authored
// colors and over-brightens the theme-dark background (pre-rebrand tracker #322). When that happens
// the logged line marks the chosen format SRGB-ONLY so the per-box sweep can
// spot it without a windowed capture.
func logSurfaceFormats(where string, formats []wgpu.TextureFormat, chosen wgpu.TextureFormat) {
	if os.Getenv("BELLUI_WEBGPU_DEBUG") == "" {
		return
	}
	srgbOnly := nonSRGBFormat(chosen) != chosen
	tag := ""
	if srgbOnly {
		tag = " [SRGB-ONLY: #322 over-bright risk]"
	}
	bellui.LogError(fmt.Sprintf("webgpu %s: advertised=%v chosen=%v%s", where, formats, chosen, tag), nil)
}

// surfaceConfig builds the surface configuration from the capabilities the
// surface advertises for the chosen adapter, or reports an error when those
// capabilities are degenerate.
//
// Empty capability lists mean the surface cannot present on this adapter at all
// (wgpu-native answers a surface it cannot serve with all-empty lists rather
// than an error). Indexing them regardless — the shape this code had until #20 —
// panics on AlphaModes[0]; and configuring a surface wgpu has already declared
// unusable trades that Go panic for an unrecoverable one, since wgpu-native
// routes every wgpuSurfaceConfigure failure through handle_error_fatal: a Rust
// panic that unwinds into cgo and kills the process (exception 0x20474343, the
// mingw foreign-exception code — not the access violation it resembles). There
// is no error to catch on the far side of Configure, so the only defence is to
// not call it with something already known to be unusable.
func surfaceConfig(caps wgpu.SurfaceCapabilities, width, height int, where string) (*wgpu.SurfaceConfiguration, error) {
	if len(caps.Formats) == 0 || len(caps.AlphaModes) == 0 {
		return nil, fmt.Errorf("%s: surface advertises no usable capabilities (formats=%d alpha modes=%d) — "+
			"it cannot present on this adapter", where, len(caps.Formats), len(caps.AlphaModes))
	}
	format := chooseSurfaceFormat(caps.Formats)
	logSurfaceFormats(where, caps.Formats, format)
	return &wgpu.SurfaceConfiguration{
		Usage:       wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc, // CopySrc enables Capture
		Format:      format,
		Width:       uint32(width),
		Height:      uint32(height),
		PresentMode: choosePresentMode(caps.PresentModes),
		AlphaMode:   caps.AlphaModes[0],
	}, nil
}

// capturePixelsSwapped reports whether a readback of a surface in this format
// arrives as BGRA and so needs its red and blue channels swapped.
func capturePixelsSwapped(format wgpu.TextureFormat) bool {
	return format == wgpu.TextureFormatBGRA8Unorm || format == wgpu.TextureFormatBGRA8UnormSrgb
}

// Device owns the WebGPU instance/adapter/device/queue/surface and the caches
// (shader modules, pipelines) shared by the painter. It is the WebGPU analogue
// of the GL painter's per-platform context plus its ProgramState set, built
// from the backend-neutral descriptors in layout.go and the WGSL in shaders/.
//
// This is the device-dependent half of WebGPU Phase 2 (pre-rebrand tracker #201): PSO management
// keyed by shader+blend+format, bind groups for uniforms+texture/sampler, a
// per-frame command encoder + render pass, and surface acquire/present. The
// draw-op geometry (Paint dispatch) is layered on top in painter.go.
type Device struct {
	instance *wgpu.Instance
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue
	surface  *wgpu.Surface
	config   *wgpu.SurfaceConfiguration

	modules   map[string]*wgpu.ShaderModule
	pipelines *PipelineCache[*wgpu.RenderPipeline]
	sampler   *wgpu.Sampler // shared linear clamp sampler for textured draws

	// depthStencils caches the depth+stencil attachment texture per pixel size,
	// reused across frames and passes. Every render pass attaches one (and every
	// pipeline carries a matching DepthStencilState) so stencil-based clipping
	// and path fill work (pre-rebrand tracker #202/#222). Depth is unused for now (compare Always,
	// no write) but the same attachment is the foundation for Scene3D depth.
	depthStencils map[[2]int]*depthStencil

	// per-frame state, valid between BeginFrame and EndFrame
	frameView *wgpu.TextureView
	frameTex  *wgpu.Texture
	encoder   *wgpu.CommandEncoder
	pass      *wgpu.RenderPassEncoder

	// GPU-time profiling (BELLUI_GPU_PROFILE, pre-rebrand tracker #336). See gpuprofile.go.
	gpuProfile bool   // per-frame GPU drain timing enabled
	gpuValid   bool   // gpuNanos holds a real measurement
	gpuNanos   uint64 // last measured GPU drain time (ns)

	// targets is the render-target stack. The root is the surface; pushing a
	// target (e.g. a RepaintBoundary FBO) ends the current pass and opens a new
	// one on the pushed texture, popping re-opens the parent with LoadOp=Load.
	// This is the WebGPU analogue of the GL painter's bind/unbind FBO stack and
	// the foundation for the FBO-based features (boundary, shadows, blur).
	targets []renderTarget

	// Per-frame vertex/uniform arena (pre-rebrand tracker #255). Every draw appends its vertex bytes
	// and its 256-padded uniform block to arena's CPU staging during Record, and
	// EndFrame flushes each staging slice to vtxArena / uniArena with a single
	// queue.WriteBuffer — collapsing the slot pool's ~2N WriteBuffer/frame to ~2.
	// Draws index the arenas: a byte offset on SetVertexBuffer for vertices, a
	// dynamic bind-group offset for uniforms. The GPU buffers persist across
	// frames (CopyDst so WriteBuffer refreshes them) and are pre-sized at
	// BeginFrame to the high-water mark; a frame larger than any before grows them
	// for the next frame and routes its overflowing draws through transient
	// per-draw buffers (the overflow path) so it still renders correctly.
	arena       arena
	vtxArena    *wgpu.Buffer
	vtxArenaCap int
	uniArena    *wgpu.Buffer
	uniArenaCap int
	arenaMaxVtx int // high-water marks (bytes) used to pre-size the arenas
	arenaMaxUni int
	// frameDemandVtx / frameDemandUni accumulate the bytes every draw this frame
	// would consume if it all packed into the arenas — counting the overflow
	// path's draws too, so EndFrame raises the high-water mark to the frame's full
	// demand and the next frame's pre-sized arenas fit it in one step.
	frameDemandVtx int
	frameDemandUni int

	// arenaBindUniform is the single dynamic-offset bind group over uniArena used
	// by every non-textured draw (SetBindGroup with the block's dynamic offset).
	// Rebuilt only when uniArena regrows. Textured draws need the texture in the
	// bind group, so they use texBinds: a per-frame-position cache (mirroring the
	// pre-rebrand tracker #245 slot pool's per-slot caching) of dynamic-offset bind groups over
	// uniArena, rebuilt when the position's texture view changes or uniArena
	// regrows. texBindInUse resets each frame; the slice persists for reuse.
	arenaBindUniform *wgpu.BindGroup
	texBinds         []*texBind
	texBindInUse     int

	// Explicit bind-group + pipeline layouts declaring the uniform binding with
	// hasDynamicOffset (the shader-reflected auto-layout does not), enabling the
	// arena's dynamic uniform offsets. bglUniform / plUniform serve non-textured
	// shaders; bglTextured / plTextured add the texture+sampler bindings for the
	// simple and blur shaders. Built once in NewDevice, released in Destroy.
	bglUniform, bglTextured *wgpu.BindGroupLayout
	plUniform, plTextured   *wgpu.PipelineLayout

	// frameBuffers / frameBinds hold the transient per-draw GPU resources created
	// by the overflow path (a frame larger than any before), released after the
	// frame is submitted so the recorded draws keep using them until the GPU is done.
	frameBuffers []*wgpu.Buffer
	frameBinds   []*wgpu.BindGroup
	// frameDepths holds dedicated depth-stencil attachments created mid-frame (the
	// Scene3D off-screen render, pre-rebrand tracker #231), released after the frame is submitted so
	// the recorded pass keeps them alive until the GPU is done.
	frameDepths []*depthStencil
	// frameTextures holds transient textures created mid-frame (the box-shadow and
	// backdrop-blur ping-pong / capture targets, pre-rebrand tracker #223), released after the frame is
	// submitted so the recorded draws keep sampling them until the GPU is done.
	frameTextures []*Texture

	// scissor is the active window-space clip (painter StartClipping). It is
	// tracked here so it can be re-applied when a content pass is reopened after a
	// render-target pop (pre-rebrand tracker #235) — a fresh RenderPassEncoder starts unclipped, so
	// without this an enclosing clip would be lost across a boundary/scene/shadow
	// off-screen pass. The off-screen passes themselves run unclipped (full).
	scissor scissorRect

	// stencilRef is the active stencil reference value (painter clip / fill ops).
	// Like scissor, it is render-pass state that a fresh RenderPassEncoder resets,
	// so it is tracked here and re-applied when a content pass is reopened after a
	// render-target pop (pre-rebrand tracker #305) — otherwise an enclosing rounded/path stencil clip
	// silently rejects everything once a boundary/Scene3D/shadow off-screen pass
	// has round-tripped (e.g. a Card's elevation shadow blanked its own content).
	stencilRef uint32

	// capture is a persistent copy of the last presented frame, kept so
	// Capture() can read back content after the swapchain texture is gone. The
	// per-frame copy is only maintained once Capture has been requested (pre-rebrand tracker #219).
	captureTex       *wgpu.Texture
	captureW         int
	captureH         int
	captureSwap      bool // surface format is BGRA: swap R/B on readback
	captureRequested bool // EndFrame maintains the capture copy only after this is set
}

// scissorRect is the active scissor rectangle (pixels) plus whether one is set.
type scissorRect struct {
	x, y, w, h int
	active     bool
}

// depthStencilFormat is the combined depth+stencil attachment format used by
// every render pass and pipeline. Depth24PlusStencil8 is universally supported.
const depthStencilFormat = wgpu.TextureFormatDepth24PlusStencil8

// depthStencil bundles a depth+stencil attachment texture with its view.
type depthStencil struct {
	tex  *wgpu.Texture
	view *wgpu.TextureView
}

// Release frees the depth-stencil texture and its view.
func (ds *depthStencil) Release() {
	if ds.view != nil {
		ds.view.Release()
		ds.view = nil
	}
	if ds.tex != nil {
		destroyTexture(ds.tex)
		ds.tex = nil
	}
}

// Texture is a sampled GPU texture owned by the caller (the painter's texture
// cache), as opposed to the transient per-frame textures. Release it via the
// painter's Free / dirty-texture lifecycle.
type Texture struct {
	tex  *wgpu.Texture
	view *wgpu.TextureView
	// w and h are the allocated pixel dimensions, kept so a caller doing
	// in-place updates (streaming rasters, #34) can tell a same-size refresh
	// from one that needs a fresh allocation.
	w, h int
}

// View returns the texture view for use as a DrawCall.TexView.
func (t *Texture) View() *wgpu.TextureView { return t.view }

// Size reports the texture's allocated pixel dimensions.
func (t *Texture) Size() (w, h int) { return t.w, t.h }

// Release frees the texture and its view, including its GPU memory.
func (t *Texture) Release() {
	if t.view != nil {
		t.view.Release()
		t.view = nil
	}
	if t.tex != nil {
		destroyTexture(t.tex)
		t.tex = nil
	}
}

// destroyTexture and destroyBuffer free a resource's GPU memory and drop the
// handle. Release alone is not enough: in the browser it is a no-op, and the
// memory stays allocated until JavaScript's garbage collector reclaims the
// wrapper, which it does not do under GPU memory pressure. Per-frame shadow
// targets and buffers piled up until Android killed the tab (#385). Destroy is
// safe for anything already submitted: WebGPU frees it once that work is done.
// Call them only for resources no unsubmitted command still uses.
func destroyTexture(t *wgpu.Texture) {
	t.Destroy()
	t.Release()
}

func destroyBuffer(b *wgpu.Buffer) {
	b.Destroy()
	b.Release()
}

// submitFrame submits a frame's commands and then polls the device without
// waiting. The poll is what frees destroyed resources on native: destroying a
// texture or buffer leaves its views and bind groups on a list in wgpu-core
// that only a device poll drains, not a submit. Without it, every
// destroyTexture and destroyBuffer leaked native memory for the life of the
// window, about 10 MB a second in a scene of 100 shadowed cards (#452). In
// the browser the poll is a no-op.
func (d *Device) submitFrame(cmd *wgpu.CommandBuffer) {
	d.queue.Submit(cmd)
	d.device.Poll(false, nil)
}

// renderTarget is one entry of the Device's render-target stack: a color view,
// its pixel size, the clear value for its first pass, and whether that first
// (clearing) pass has already opened.
type renderTarget struct {
	view *wgpu.TextureView
	// tex is the texture backing view, kept so a feature that needs to read the
	// target back (backdrop blur's CopyActiveTargetRegion, pre-rebrand tracker #223) has a copy source.
	// nil targets simply cannot be copied from.
	tex     *wgpu.Texture
	w, h    int
	clear   wgpu.Color
	cleared bool
	// depthView, when set, is a dedicated depth-stencil attachment for this target
	// instead of the shared size-keyed one (ensureDepthStencil). Scene3D uses this
	// so its depth/stencil never aliases — and thus never clears — the canvas
	// pass's stencil clip when their pixel sizes coincide (pre-rebrand tracker #231).
	depthView *wgpu.TextureView
}

// DrawCall is one recorded draw: a pipeline (by key), an interleaved vertex
// buffer, the matching uniform block bytes, and an optional texture for the
// `simple`/`blur` shaders.
type DrawCall struct {
	Key         PipelineKey
	Vertices    []float32
	VertexCount int
	Uniform     []byte
	TexView     *wgpu.TextureView // nil unless the shader samples a texture
}

// NewDevice brings up WebGPU against the given surface (created from a native
// window handle by the platform surface bridge) at the initial pixel size.
// chooseInstanceBackends selects which wgpu-native backends the instance
// initializes. The default (a nil descriptor) initializes *all* backends and
// lets wgpu pick the best adapter — the right choice on real hardware.
//
// BELLUI_WEBGPU_BACKENDS overrides it with a '|'-separated list, e.g.
// "dx12", "vulkan|dx12", "primary". This is a proactive escape hatch for
// environments whose *virtual* GPU driver crashes during a backend's init:
// on the Windows-on-ARM Parallels/VM GPUs the all-backends path segfaults in
// GL init, while restricting to "dx12" enumerates the D3D12 adapter cleanly
// (pre-rebrand tracker #612 / pre-rebrand tracker #609). Because that failure is a hard segfault *inside*
// CreateInstance — not a returnable error — the backend set must be narrowed
// here, up-front; it cannot be recovered from after the call.
//
// Backend names: all, primary, secondary, vulkan, gl, metal, dx12 (d3d12
// accepted as an alias). Note wgpu encodes "All" as 0, so combining "all" with
// another backend is meaningless — pass "all" alone (or unset). DX11 was dropped
// by wgpu-native and is no longer selectable.
//
// The bit values mirror WGPUInstanceBackend_* in wgpu.h; the Go bindings only
// export the "All" (zero) member, so the individual flags are spelled out here.
const (
	instanceBackendVulkan  wgpu.InstanceBackend = 1 << 0
	instanceBackendGL      wgpu.InstanceBackend = 1 << 1
	instanceBackendMetal   wgpu.InstanceBackend = 1 << 2
	instanceBackendDX12    wgpu.InstanceBackend = 1 << 3
	instanceBackendBrowser wgpu.InstanceBackend = 1 << 5

	instanceBackendPrimary   = instanceBackendVulkan | instanceBackendMetal | instanceBackendDX12 | instanceBackendBrowser
	instanceBackendSecondary = instanceBackendGL
)

func chooseInstanceBackends() *wgpu.InstanceDescriptor {
	spec := strings.TrimSpace(os.Getenv("BELLUI_WEBGPU_BACKENDS"))
	if spec == "" {
		return nil // all backends (default, unchanged behavior)
	}
	var backends wgpu.InstanceBackend
	for _, name := range strings.Split(spec, "|") {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "", "all":
			// All == 0; contributes nothing to the mask.
		case "primary":
			backends |= instanceBackendPrimary
		case "secondary":
			backends |= instanceBackendSecondary
		case "vulkan":
			backends |= instanceBackendVulkan
		case "gl":
			backends |= instanceBackendGL
		case "metal":
			backends |= instanceBackendMetal
		case "dx12", "d3d12":
			backends |= instanceBackendDX12
		default:
			bellui.LogError(fmt.Sprintf("webgpu: ignoring unknown BELLUI_WEBGPU_BACKENDS entry %q", name), nil)
		}
	}
	if os.Getenv("BELLUI_WEBGPU_DEBUG") != "" {
		bellui.LogError(fmt.Sprintf("webgpu: BELLUI_WEBGPU_BACKENDS=%q -> backends=0x%x", spec, uint32(backends)), nil)
	}
	return &wgpu.InstanceDescriptor{Backends: backends}
}

func NewDevice(sd *wgpu.SurfaceDescriptor, width, height int) (d *Device, err error) {
	defer func() {
		if err != nil && d != nil {
			d.Destroy()
			d = nil
		}
	}()
	d = &Device{
		modules:       map[string]*wgpu.ShaderModule{},
		depthStencils: map[[2]int]*depthStencil{},
		// Seed the arena high-water marks so the first frame allocates usefully
		// sized buffers instead of routing every draw through the overflow path:
		// 64KB of vertices and 256 uniform blocks cover a typical UI's first frame.
		arenaMaxVtx: 64 * 1024,
		arenaMaxUni: 256 * uniformBlockStride,
	}

	d.instance = wgpu.CreateInstance(chooseInstanceBackends())
	d.surface = d.instance.CreateSurface(sd)

	d.adapter, err = d.instance.RequestAdapter(&wgpu.RequestAdapterOptions{CompatibleSurface: d.surface})
	if err != nil {
		return d, fmt.Errorf("request adapter: %w", err)
	}
	if os.Getenv("BELLUI_WEBGPU_DEBUG") != "" {
		info := d.adapter.GetInfo()
		bellui.LogError(fmt.Sprintf("webgpu adapter: name=%q backend=%v type=%v", info.Device, info.BackendType, info.AdapterType), nil)
	}

	d.device, err = d.adapter.RequestDevice(nil)
	if err != nil {
		return d, fmt.Errorf("request device: %w", err)
	}
	d.queue = d.device.GetQueue()
	d.initGPUProfiling() // BELLUI_GPU_PROFILE, pre-rebrand tracker #336

	caps := d.surface.GetCapabilities(d.adapter)
	d.config, err = surfaceConfig(caps, width, height, "surface configure")
	if err != nil {
		return d, err
	}
	d.captureSwap = capturePixelsSwapped(d.config.Format)
	d.surface.Configure(d.device, d.config)

	d.sampler, err = d.device.TryCreateSampler(&wgpu.SamplerDescriptor{
		Label:         "linear-clamp",
		AddressModeU:  wgpu.AddressModeClampToEdge,
		AddressModeV:  wgpu.AddressModeClampToEdge,
		AddressModeW:  wgpu.AddressModeClampToEdge,
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		LodMaxClamp:   1,
		MaxAnisotropy: 1, // zero value fails validation (Phase 0 finding)
	})
	if err != nil {
		return d, fmt.Errorf("create sampler: %w", err)
	}

	if err = d.buildLayouts(); err != nil {
		return d, err
	}

	d.pipelines = NewPipelineCache(d.buildPipeline)
	return d, nil
}

// Format returns the surface's texture format as a neutral uint32, for use in
// PipelineKey.Format.
func (d *Device) Format() uint32 { return uint32(d.config.Format) }

// chooseSurfaceFormat picks the surface texture format to configure from the
// formats the surface advertises (caps.Formats, preferred first).
//
// Bellui's GL and software backends blend in gamma (sRGB-encoded) space and
// present authored colors directly; the WebGPU painter likewise writes straight
// sRGB color values — for solid fills, sampled images and glyph atlases alike —
// and its shaders do no gamma encoding. Configuring the surface with an sRGB
// format (e.g. macOS Metal reports BGRA8UnormSrgb first) makes the GPU re-encode
// those writes a second time, brightening every color: theme-dark #121216
// reads back as #4b4b53 (pre-rebrand tracker #322). To match GL/software, prefer a NON-sRGB
// (linear-store) surface format: the non-sRGB sibling of the preferred format
// when the surface advertises it, otherwise the preferred format unchanged so a
// configure never fails on a backend that only offers sRGB. Staying within the
// preferred format's channel family (BGRA/RGBA) keeps the capture readback swap
// correct. (The JS/wasm backend already reports BGRA8Unorm first, so this is a
// no-op there.)
func chooseSurfaceFormat(formats []wgpu.TextureFormat) wgpu.TextureFormat {
	if len(formats) == 0 {
		return wgpu.TextureFormatBGRA8Unorm
	}
	preferred := formats[0]
	sibling := nonSRGBFormat(preferred)
	if sibling == preferred {
		return preferred // already non-sRGB
	}
	for _, f := range formats {
		if f == sibling {
			return sibling
		}
	}
	// No non-sRGB sibling advertised: forced to keep an sRGB format, which makes
	// the GPU re-encode every authored color a second time and over-brightens the
	// whole surface (pre-rebrand tracker #322, theme-dark #121216 → #4b4b53). No current backend hits
	// this (pre-rebrand tracker #327 sweep), but a future driver / headless config could regress here
	// and only show as washed-out colors. Warn once, unconditionally (not gated
	// behind BELLUI_WEBGPU_DEBUG), so the condition is visible in normal logs (pre-rebrand tracker #331).
	srgbOnlyWarnOnce.Do(func() {
		bellui.LogError(fmt.Sprintf("webgpu: surface advertises only sRGB formats %v; "+
			"keeping %v — authored colors will over-brighten (#322/#331)", formats, preferred), nil)
	})
	return preferred
}

// srgbOnlyWarnOnce guards the one-time sRGB-only surface warning in
// chooseSurfaceFormat so a multi-configure session logs it at most once.
var srgbOnlyWarnOnce sync.Once

// nonSRGBFormat maps an sRGB surface format to its bit-identical non-sRGB
// sibling, returning the format unchanged when it is already non-sRGB.
func nonSRGBFormat(f wgpu.TextureFormat) wgpu.TextureFormat {
	switch f {
	case wgpu.TextureFormatBGRA8UnormSrgb:
		return wgpu.TextureFormatBGRA8Unorm
	case wgpu.TextureFormatRGBA8UnormSrgb:
		return wgpu.TextureFormatRGBA8Unorm
	default:
		return f
	}
}

// Resize reconfigures the surface for a new pixel size. It is a no-op when the
// size is unchanged: the mobile paint loop calls SetOutputSize on every frame
// (driver.paintWindow), so reconfiguring unconditionally would re-run
// surface.Configure per frame — wasted work, and on iOS/Metal it makes wgpu
// re-read the CAMetalLayer view dimensions from the render goroutine, flooding
// the log with "Unable to get the current view dimensions on a non-main thread"
// (pre-rebrand tracker #262). Only an actual size change needs a reconfigure.
func (d *Device) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	if uint32(width) == d.config.Width && uint32(height) == d.config.Height {
		return
	}
	d.config.Width = uint32(width)
	d.config.Height = uint32(height)
	d.surface.Configure(d.device, d.config)
	// Drop cached depth-stencil attachments; they are recreated on demand at the
	// new size (and any boundary FBOs are reallocated on resize too).
	d.releaseDepthStencils()
}

// RecreateSurface rebuilds the presentation surface from a new native-window
// descriptor, keeping the existing device/adapter/queue/pipelines. Android
// destroys the ANativeWindow when the app is backgrounded and delivers a fresh
// handle on resume; without rebuilding the surface the painter keeps presenting
// to the destroyed one — a black canvas and a main-thread stall/ANR (pre-rebrand tracker #310).
func (d *Device) RecreateSurface(sd *wgpu.SurfaceDescriptor) {
	if sd == nil || d.instance == nil {
		return
	}
	d.endPass() // no frame should straddle the swap, but be safe
	if d.surface != nil {
		d.surface.Release()
	}
	d.surface = d.instance.CreateSurface(sd)
	// Re-derive format/alpha from the new surface's caps (same adapter, so
	// normally identical to before) and reconfigure at the current size.
	caps := d.surface.GetCapabilities(d.adapter)
	cfg, err := surfaceConfig(caps, int(d.config.Width), int(d.config.Height), "surface recreate")
	if err != nil {
		// The new native window cannot present on this adapter. Leave the surface
		// unconfigured rather than configuring one wgpu has declared unusable —
		// that is fatal inside wgpu-native (#20). Frames then fail to acquire and
		// are skipped until a later RecreateSurface brings a usable window.
		bellui.LogError("webgpu: keeping the surface unconfigured", err)
		return
	}
	d.config = cfg
	d.captureSwap = capturePixelsSwapped(cfg.Format)
	d.surface.Configure(d.device, d.config)
}

// ReleaseSurface drops this device's presentation surface without tearing down
// the device/adapter/queue/pipelines, so another window's surface can take over
// the shared Android ANativeWindow — two configured Vulkan swapchains on one
// native window can't both present (pre-rebrand tracker #753). The surface is rebuilt by
// RecreateSurface when this window is presented again. No-op if already
// surfaceless.
func (d *Device) ReleaseSurface() {
	if d.surface == nil {
		return
	}
	d.endPass() // no frame should straddle the release, but be safe
	d.surface.Release()
	d.surface = nil
}

// releaseDepthStencils frees and clears the depth-stencil attachment cache.
func (d *Device) releaseDepthStencils() {
	for k, ds := range d.depthStencils {
		ds.view.Release()
		destroyTexture(ds.tex)
		delete(d.depthStencils, k)
	}
}

// shaderModule lazily creates (and caches) the shader module for a shader name.
func (d *Device) shaderModule(name string) (*wgpu.ShaderModule, error) {
	if m, ok := d.modules[name]; ok {
		return m, nil
	}
	src := ShaderSource(name)
	if src == nil {
		return nil, fmt.Errorf("unknown shader %q", name)
	}
	m, err := d.device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      name + ".wgsl",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(src)},
	})
	if err != nil {
		return nil, fmt.Errorf("create shader module %q: %w", name, err)
	}
	d.modules[name] = m
	return m, nil
}

// buildPipeline creates a render pipeline for a key. It is the cache's builder.
func (d *Device) buildPipeline(key PipelineKey) (*wgpu.RenderPipeline, error) {
	desc, ok := DescriptorFor(key.Shader)
	if !ok {
		return nil, fmt.Errorf("no descriptor for shader %q", key.Shader)
	}
	module, err := d.shaderModule(key.Shader)
	if err != nil {
		return nil, err
	}

	// Use the explicit pipeline layout matching the shader's bind-group shape so
	// the uniform binding declares hasDynamicOffset (required by the arena's
	// dynamic uniform offsets); auto-layout would omit it.
	layout := d.plUniform
	if desc.Textured {
		layout = d.plTextured
	}

	return d.device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  key.Shader + ":" + key.Fragment,
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     module,
			EntryPoint: VertexEntry,
			Buffers:    []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)},
		},
		Fragment: &wgpu.FragmentState{
			Module:     module,
			EntryPoint: key.Fragment,
			Targets: []wgpu.ColorTargetState{{
				Format:    wgpu.TextureFormat(key.Format),
				Blend:     blendState(key.Blend),
				WriteMask: colorWriteMask(key.Stencil),
			}},
		},
		Primitive: wgpu.PrimitiveState{
			Topology:  wgpuTopology(desc.Topology),
			FrontFace: wgpu.FrontFaceCCW,
			CullMode:  wgpu.CullModeNone,
		},
		DepthStencil: depthStencilState(key.Stencil, key.Depth),
		Multisample:  wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
}

// depthStencilState builds the pipeline depth-stencil state for a stencil mode
// and an optional hardware depth test. The stencil face state and write mask
// vary with mode (mirroring the GL painter's StencilFunc/StencilOp calls); depth
// is inert (compare Always, no write) unless depth is true, when it becomes a
// real z-buffer — write enabled, compare Less — for Scene3D (pre-rebrand tracker #224). The same
// Depth24PlusStencil8 attachment serves both (pre-rebrand tracker #222 stencil + pre-rebrand tracker #224 depth).
func depthStencilState(mode StencilMode, depth bool) *wgpu.DepthStencilState {
	keep := wgpu.StencilFaceState{
		Compare:     wgpu.CompareFunctionAlways,
		FailOp:      wgpu.StencilOperationKeep,
		DepthFailOp: wgpu.StencilOperationKeep,
		PassOp:      wgpu.StencilOperationKeep,
	}
	front, back := keep, keep
	var writeMask uint32
	switch mode {
	case StencilWriteReplace:
		front.PassOp = wgpu.StencilOperationReplace
		back = front
		writeMask = 0xFF
	case StencilWriteIncrDecr:
		front.PassOp = wgpu.StencilOperationIncrementWrap
		back.PassOp = wgpu.StencilOperationDecrementWrap
		writeMask = 0xFF
	case StencilWriteInvert:
		front.PassOp = wgpu.StencilOperationInvert
		back = front
		writeMask = 0xFF
	case StencilTestEqual:
		front.Compare = wgpu.CompareFunctionEqual
		back = front
	case StencilTestNotEqual:
		front.Compare = wgpu.CompareFunctionNotEqual
		back = front
	}
	depthCompare := wgpu.CompareFunctionAlways
	if depth {
		// Nearer fragments (smaller clip-space z after the [0,1] depth remap in
		// computeScene3DMVP) win; depth is cleared to 1 (far) at the pass load op.
		depthCompare = wgpu.CompareFunctionLess
	}
	return &wgpu.DepthStencilState{
		Format:            depthStencilFormat,
		DepthWriteEnabled: optionalBool(depth),
		DepthCompare:      depthCompare,
		StencilFront:      front,
		StencilBack:       back,
		StencilReadMask:   0xFF,
		StencilWriteMask:  writeMask,
	}
}

// optionalBool converts a Go bool to the tri-state the depth/stencil descriptor
// takes (the third state, Undefined, means "leave it to the pipeline default" and
// is never what we want here).
func optionalBool(b bool) wgpu.OptionalBool {
	if b {
		return wgpu.OptionalBoolTrue
	}
	return wgpu.OptionalBoolFalse
}

// colorWriteMask masks color off for the stencil-write modes (which only stamp
// the stencil) and writes all channels otherwise.
func colorWriteMask(mode StencilMode) wgpu.ColorWriteMask {
	switch mode {
	case StencilWriteReplace, StencilWriteIncrDecr, StencilWriteInvert:
		return wgpu.ColorWriteMaskNone
	}
	return wgpu.ColorWriteMaskAll
}

// Pipeline returns the cached/built pipeline for the key.
func (d *Device) Pipeline(key PipelineKey) (*wgpu.RenderPipeline, error) {
	return d.pipelines.Get(key)
}

// vertexBufferLayout translates a neutral VertexLayout into a wgpu layout.
func vertexBufferLayout(l VertexLayout) wgpu.VertexBufferLayout {
	attrs := make([]wgpu.VertexAttribute, len(l.Attrs))
	for i, a := range l.Attrs {
		attrs[i] = wgpu.VertexAttribute{
			Format:         wgpuVertexFormat(a.Format),
			Offset:         uint64(a.Offset),
			ShaderLocation: uint32(a.Location),
		}
	}
	return wgpu.VertexBufferLayout{
		ArrayStride: uint64(l.Stride),
		StepMode:    wgpu.VertexStepModeVertex,
		Attributes:  attrs,
	}
}

func wgpuTopology(t Topology) wgpu.PrimitiveTopology {
	if t == TriangleStrip {
		return wgpu.PrimitiveTopologyTriangleStrip
	}
	return wgpu.PrimitiveTopologyTriangleList
}

func wgpuVertexFormat(f VertexFormat) wgpu.VertexFormat {
	switch f {
	case Float32:
		return wgpu.VertexFormatFloat32
	case Float32x2:
		return wgpu.VertexFormatFloat32x2
	case Float32x3:
		return wgpu.VertexFormatFloat32x3
	case Float32x4:
		return wgpu.VertexFormatFloat32x4
	}
	// An unsupported format would otherwise panic the whole app while building a
	// pipeline. Log it and fall back to a valid format so rendering degrades to a
	// (possibly wrong) mesh rather than crashing.
	bellui.LogError(fmt.Sprintf("webgpu: invalid vertex format %d, falling back to Float32x4", f), nil)
	return wgpu.VertexFormatFloat32x4
}

// blendState maps a neutral BlendMode to a wgpu color-target blend state.
// nil means no blending (BlendReplace).
func blendState(mode BlendMode) *wgpu.BlendState {
	switch mode {
	case BlendOver:
		// straight (non-premultiplied) alpha over
		return &wgpu.BlendState{
			Color: wgpu.BlendComponent{
				SrcFactor: wgpu.BlendFactorSrcAlpha,
				DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
				Operation: wgpu.BlendOperationAdd,
			},
			Alpha: wgpu.BlendComponent{
				SrcFactor: wgpu.BlendFactorOne,
				DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
				Operation: wgpu.BlendOperationAdd,
			},
		}
	case BlendPremul:
		// premultiplied alpha over (shader already multiplied rgb by alpha)
		return &wgpu.BlendState{
			Color: wgpu.BlendComponent{
				SrcFactor: wgpu.BlendFactorOne,
				DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
				Operation: wgpu.BlendOperationAdd,
			},
			Alpha: wgpu.BlendComponent{
				SrcFactor: wgpu.BlendFactorOne,
				DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
				Operation: wgpu.BlendOperationAdd,
			},
		}
	default: // BlendReplace
		return nil
	}
}

// uniformBytes reinterprets a uniform struct as raw bytes for queue.WriteBuffer.
// T must be one of the std140 uniform structs in uniforms.go.
func uniformBytes[T any](u *T) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(u)), unsafe.Sizeof(*u))
}

// texBind caches a textured draw's dynamic-offset bind group (uniArena + a
// texture view + the shared sampler) for one position within a frame, mirroring
// the pre-rebrand tracker #245 slot pool's per-slot bind-group caching but with the uniform buffer
// now the shared arena addressed via dynamic offset. Rebuilt when the position's
// texture view changes or uniArena regrows; reused across frames otherwise.
type texBind struct {
	bind *wgpu.BindGroup
	view *wgpu.TextureView
}

// roundUpBuffer rounds a requested buffer size up to a multiple of
// uniformBlockStride (256) so the arenas grow in coarse steps (reused across
// frames) and the uniform arena's capacity is always a whole number of blocks.
func roundUpBuffer(n int) int {
	if n < uniformBlockStride {
		return uniformBlockStride
	}
	return (n + uniformBlockStride - 1) &^ (uniformBlockStride - 1)
}

// buildLayouts creates the explicit bind-group and pipeline layouts that declare
// the uniform binding with hasDynamicOffset, which the arena's dynamic uniform
// offsets require (shader auto-layout via GetBindGroupLayout does not set it).
// One layout pair serves the non-textured shaders, another adds the texture and
// sampler bindings for the simple and blur shaders.
func (d *Device) buildLayouts() error {
	// binding 0: a uniform buffer addressed by dynamic offset, visible to both
	// stages (line/scene3d read it in the vertex stage, the rest in fragment; an
	// unused stage in the visibility mask is harmless).
	uniformEntry := wgpu.BindGroupLayoutEntry{
		Binding:    0,
		Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
		Buffer: wgpu.BufferBindingLayout{
			Type:             wgpu.BufferBindingTypeUniform,
			HasDynamicOffset: true,
		},
	}
	var err error
	if d.bglUniform, err = d.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label:   "uniform-dyn",
		Entries: []wgpu.BindGroupLayoutEntry{uniformEntry},
	}); err != nil {
		return fmt.Errorf("uniform bind-group layout: %w", err)
	}
	if d.bglTextured, err = d.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "uniform-dyn+tex",
		Entries: []wgpu.BindGroupLayoutEntry{
			uniformEntry,
			{Binding: 1, Visibility: wgpu.ShaderStageFragment, Texture: wgpu.TextureBindingLayout{
				SampleType:    wgpu.TextureSampleTypeFloat,
				ViewDimension: wgpu.TextureViewDimension2D,
			}},
			{Binding: 2, Visibility: wgpu.ShaderStageFragment, Sampler: wgpu.SamplerBindingLayout{
				Type: wgpu.SamplerBindingTypeFiltering,
			}},
		},
	}); err != nil {
		return fmt.Errorf("textured bind-group layout: %w", err)
	}
	if d.plUniform, err = d.device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "pl-uniform",
		BindGroupLayouts: []*wgpu.BindGroupLayout{d.bglUniform},
	}); err != nil {
		return fmt.Errorf("uniform pipeline layout: %w", err)
	}
	if d.plTextured, err = d.device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "pl-textured",
		BindGroupLayouts: []*wgpu.BindGroupLayout{d.bglTextured},
	}); err != nil {
		return fmt.Errorf("textured pipeline layout: %w", err)
	}
	return nil
}

// ensureArenas pre-sizes the GPU vertex/uniform arenas to the high-water mark
// before a frame, growing (recreating) a buffer when the mark exceeds its
// capacity. Releasing the old buffer is safe even though the previous frame's
// submitted commands reference it: wgpu refcounts keep the native buffer alive
// until the GPU is done. When uniArena is recreated every cached bind group over
// it is invalidated, so arenaBindUniform and the texBinds are released and the
// uniform bind group is rebuilt over the new buffer.
func (d *Device) ensureArenas() error {
	if d.vtxArenaCap < d.arenaMaxVtx {
		if d.vtxArena != nil {
			destroyBuffer(d.vtxArena)
		}
		size := roundUpBuffer(d.arenaMaxVtx)
		buf, err := d.device.TryCreateBuffer(&wgpu.BufferDescriptor{
			Size:  uint64(size),
			Usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst,
		})
		if err != nil {
			return err
		}
		d.vtxArena, d.vtxArenaCap = buf, size
	}

	if d.uniArenaCap < d.arenaMaxUni {
		if d.uniArena != nil {
			destroyBuffer(d.uniArena)
		}
		size := roundUpBuffer(d.arenaMaxUni)
		buf, err := d.device.TryCreateBuffer(&wgpu.BufferDescriptor{
			Size:  uint64(size),
			Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
		})
		if err != nil {
			return err
		}
		d.uniArena, d.uniArenaCap = buf, size
		d.invalidateUniformBinds()
	}

	if d.arenaBindUniform == nil && d.uniArena != nil {
		bg, err := d.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
			Layout:  d.bglUniform,
			Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: d.uniArena, Offset: 0, Size: uniformBlockStride}},
		})
		if err != nil {
			return err
		}
		d.arenaBindUniform = bg
	}
	return nil
}

// invalidateUniformBinds releases every bind group built over uniArena, so they
// are rebuilt over the new buffer after a regrow.
func (d *Device) invalidateUniformBinds() {
	if d.arenaBindUniform != nil {
		d.arenaBindUniform.Release()
		d.arenaBindUniform = nil
	}
	for _, tb := range d.texBinds {
		if tb.bind != nil {
			tb.bind.Release()
		}
	}
	d.texBinds = d.texBinds[:0]
}

// nextTexBind returns the dynamic-offset bind group for a textured draw at the
// current frame position, building it over uniArena + view + sampler and caching
// it so an unchanged position/texture reuses it across frames.
func (d *Device) nextTexBind(view *wgpu.TextureView) (*wgpu.BindGroup, error) {
	i := d.texBindInUse
	d.texBindInUse++
	if i < len(d.texBinds) {
		tb := d.texBinds[i]
		if tb.bind != nil && tb.view == view {
			return tb.bind, nil
		}
		if tb.bind != nil {
			tb.bind.Release()
		}
	} else {
		d.texBinds = append(d.texBinds, &texBind{})
	}
	bg, err := d.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: d.bglTextured,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: d.uniArena, Offset: 0, Size: uniformBlockStride},
			{Binding: 1, TextureView: view},
			{Binding: 2, Sampler: d.sampler},
		},
	})
	if err != nil {
		return nil, err
	}
	d.texBinds[i].bind = bg
	d.texBinds[i].view = view
	return bg, nil
}

// Record draws dc into the active render pass. The common path packs the draw's
// vertices and uniform block into the per-frame arenas (flushed with one
// WriteBuffer each in EndFrame) and indexes them via a vertex byte offset and a
// uniform dynamic offset; a draw that would overflow the arena's current
// capacity falls back to transient per-draw buffers (and the arena grows for the
// next frame). Must be called between BeginFrame and EndFrame.
func (d *Device) Record(dc DrawCall) error {
	if d.pass == nil {
		return fmt.Errorf("Record called outside a frame")
	}
	if dc.VertexCount == 0 || len(dc.Vertices) == 0 {
		return nil
	}
	if len(dc.Uniform) > uniformBlockStride {
		return fmt.Errorf("uniform block %d bytes exceeds arena stride %d", len(dc.Uniform), uniformBlockStride)
	}
	pipe, err := d.Pipeline(dc.Key)
	if err != nil {
		return err
	}
	vbytes := wgpu.ToBytes(dc.Vertices)

	// Track this frame's total arena demand (both paths) so EndFrame can raise the
	// high-water mark and the next frame's arenas are pre-sized to fit it.
	d.frameDemandVtx += len(vbytes)
	d.frameDemandUni += uniformBlockStride

	// Arena path: the draw fits the current capacity. Append to staging, record
	// the draw against the arena buffers at the returned offsets.
	if d.vtxArena != nil && d.uniArena != nil &&
		d.arena.vtxLen()+len(vbytes) <= d.vtxArenaCap &&
		d.arena.uniLen()+uniformBlockStride <= d.uniArenaCap {
		vtxOff := d.arena.addVertices(vbytes)
		uniOff := d.arena.addUniform(dc.Uniform)

		var bind *wgpu.BindGroup
		if dc.TexView != nil {
			if bind, err = d.nextTexBind(dc.TexView); err != nil {
				return err
			}
		} else {
			bind = d.arenaBindUniform
		}

		d.pass.SetPipeline(pipe)
		d.pass.SetBindGroup(0, bind, []uint32{uint32(uniOff)})
		d.pass.SetVertexBuffer(0, d.vtxArena, uint64(vtxOff), uint64(len(vbytes)))
		d.pass.Draw(uint32(dc.VertexCount), 1, 0, 0)
		return nil
	}

	return d.recordOverflow(dc, pipe, vbytes)
}

// recordOverflow renders a draw that does not fit the current arena capacity
// (a frame larger than any before) via transient per-draw buffers and a
// transient dynamic-offset bind group, all released after the frame is
// submitted. The arena grows for the next frame from the high-water mark raised
// in Record, so this path is only hit on the warmup / growth frame.
func (d *Device) recordOverflow(dc DrawCall, pipe *wgpu.RenderPipeline, vbytes []byte) error {
	vbuf, err := d.device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: vbytes, Usage: wgpu.BufferUsageVertex})
	if err != nil {
		return err
	}
	d.frameBuffers = append(d.frameBuffers, vbuf)

	ublock := make([]byte, uniformBlockStride)
	copy(ublock, dc.Uniform)
	ubuf, err := d.device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: ublock, Usage: wgpu.BufferUsageUniform})
	if err != nil {
		return err
	}
	d.frameBuffers = append(d.frameBuffers, ubuf)

	layout := d.bglUniform
	entries := []wgpu.BindGroupEntry{{Binding: 0, Buffer: ubuf, Offset: 0, Size: uniformBlockStride}}
	if dc.TexView != nil {
		layout = d.bglTextured
		entries = append(
			entries,
			wgpu.BindGroupEntry{Binding: 1, TextureView: dc.TexView},
			wgpu.BindGroupEntry{Binding: 2, Sampler: d.sampler},
		)
	}
	bind, err := d.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: entries})
	if err != nil {
		return err
	}
	d.frameBinds = append(d.frameBinds, bind)

	d.pass.SetPipeline(pipe)
	d.pass.SetBindGroup(0, bind, []uint32{0})
	d.pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	d.pass.Draw(uint32(dc.VertexCount), 1, 0, 0)
	return nil
}

// CreateImageTexture creates a persistent sampled RGBA8 texture from a Go image.
// Ownership transfers to the caller (the painter's texture cache), which must
// Release it. The image's raw 8-bit RGBA bytes are uploaded as-is.
func (d *Device) CreateImageTexture(img image.Image) (*Texture, error) {
	// Upload 8-bit RGBA bytes, matching the GL painter's imgToTexture. RGBA is
	// already premultiplied and uploaded as-is. NRGBA (straight alpha, e.g. text/AA
	// glyphs) MUST be premultiplied first: the `simple` shader multiplies rgb only by
	// the opacity uniform — NOT by the texture's coverage alpha — and the blend is
	// premultiplied-over, so uploading non-premultiplied NRGBA blows anti-aliased edge
	// pixels (e.g. white at 50% coverage) up to full intensity, collapsing the AA
	// gradient → blocky text. draw.Draw NRGBA→RGBA premultiplies, exactly as GL does.
	var pix []uint8
	var stride, w, h int
	switch i := img.(type) {
	case *image.NRGBA:
		rgba := image.NewRGBA(image.Rect(0, 0, i.Rect.Dx(), i.Rect.Dy()))
		draw.Draw(rgba, rgba.Rect, i, i.Rect.Min, draw.Src)
		pix, stride, w, h = rgba.Pix, rgba.Stride, rgba.Rect.Dx(), rgba.Rect.Dy()
	case *image.RGBA:
		pix, stride, w, h = i.Pix, i.Stride, i.Rect.Dx(), i.Rect.Dy()
	default:
		b := img.Bounds()
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Rect, img, b.Min, draw.Over)
		pix, stride, w, h = rgba.Pix, rgba.Stride, rgba.Rect.Dx(), rgba.Rect.Dy()
	}
	if w == 0 || h == 0 || len(pix) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	extent := wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}
	tex, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "image",
		Size:          extent,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, err
	}
	d.queue.WriteTexture(tex.AsImageCopy(), pix, &wgpu.TexelCopyBufferLayout{
		Offset:       0,
		BytesPerRow:  uint32(stride),
		RowsPerImage: uint32(h),
	}, &extent)
	view, err := tex.TryCreateView(nil)
	if err != nil {
		tex.Release()
		return nil, err
	}
	return &Texture{tex: tex, view: view, w: w, h: h}, nil
}

// WriteTexturePixels re-uploads premultiplied 8-bit RGBA pixels into an existing
// texture, the WebGPU counterpart of GL's TexSubImage2D. The dimensions must
// match the texture's allocation — a size change needs a fresh texture, which
// the caller detects via Texture.Size. Used by streaming rasters (#34) so video
// frames update in place instead of reallocating a texture every frame.
func (d *Device) WriteTexturePixels(t *Texture, pix []byte, w, h int) error {
	if err := validateTextureWrite(t, pix, w, h); err != nil {
		return err
	}
	if d == nil || d.queue == nil {
		return fmt.Errorf("no device queue to write with")
	}
	extent := wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}
	d.queue.WriteTexture(t.tex.AsImageCopy(), pix[:w*h*4], &wgpu.TexelCopyBufferLayout{
		Offset:       0,
		BytesPerRow:  uint32(w * 4),
		RowsPerImage: uint32(h),
	}, &extent)
	return nil
}

// validateTextureWrite reports whether pix can be uploaded into t as w×h
// premultiplied RGBA. Kept separate from the GPU call so the guards can be
// exercised without a device.
func validateTextureWrite(t *Texture, pix []byte, w, h int) error {
	if t == nil {
		return fmt.Errorf("no texture to write")
	}
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid pixel size %dx%d", w, h)
	}
	if t.w != w || t.h != h {
		return fmt.Errorf("size mismatch: texture is %dx%d, pixels are %dx%d", t.w, t.h, w, h)
	}
	if need := w * h * 4; len(pix) < need {
		return fmt.Errorf("short pixel buffer: have %d bytes, need %d", len(pix), need)
	}
	if t.tex == nil {
		return fmt.Errorf("texture has no GPU handle")
	}
	return nil
}

// CreateRenderTexture creates an off-screen color target (e.g. a RepaintBoundary
// FBO) in the surface format — so the cached render pipelines, keyed by the
// surface format, work unchanged when rendering into it — usable both as a
// render attachment and a sampled texture for the later blit.
func (d *Device) CreateRenderTexture(w, h int) (*Texture, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid render texture size %dx%d", w, h)
	}
	tex, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "render-target",
		Size:          wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        d.config.Format,
		// CopySrc lets a render target double as a backdrop-blur copy source (pre-rebrand tracker #223).
		Usage: wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		return nil, err
	}
	view, err := tex.TryCreateView(nil)
	if err != nil {
		tex.Release()
		return nil, err
	}
	return &Texture{tex: tex, view: view, w: w, h: h}, nil
}

// ReleaseTextureAfterFrame defers a transient texture's release until the frame
// is submitted, so draws that sample it stay valid until the GPU is done with
// them. Used by the box-shadow / backdrop-blur ping-pong + capture targets (pre-rebrand tracker #223).
func (d *Device) ReleaseTextureAfterFrame(t *Texture) {
	if t != nil {
		d.frameTextures = append(d.frameTextures, t)
	}
}

// CopyActiveTargetRegion copies a pixel rectangle of the current render target
// into a fresh sampled texture, for backdrop blur (pre-rebrand tracker #223): the already-rendered
// content under a widget is captured, blurred, and drawn back. The active pass is
// ended for the copy (a GPU copy cannot run inside a render pass) and reopened
// with LoadOp=Load afterwards so its contents are preserved. The region is
// clamped to the target and the clamped width/height returned (so the caller can
// size the blur to match); the returned texture is owned by the caller (release
// it, e.g. via ReleaseTextureAfterFrame).
func (d *Device) CopyActiveTargetRegion(x, y, w, h int) (*Texture, int, int, error) {
	if len(d.targets) == 0 {
		return nil, 0, 0, fmt.Errorf("no active render target")
	}
	t := d.targets[len(d.targets)-1]
	if t.tex == nil {
		return nil, 0, 0, fmt.Errorf("active render target has no copy source")
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > t.w {
		w = t.w - x
	}
	if y+h > t.h {
		h = t.h - y
	}
	if w <= 0 || h <= 0 {
		return nil, 0, 0, fmt.Errorf("capture region outside target")
	}
	extent := wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}
	dst, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "backdrop-capture",
		Size:          extent,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        d.config.Format,
		Usage:         wgpu.TextureUsageCopyDst | wgpu.TextureUsageTextureBinding,
	})
	if err != nil {
		return nil, 0, 0, err
	}
	view, err := dst.TryCreateView(nil)
	if err != nil {
		dst.Release()
		return nil, 0, 0, err
	}

	d.endPass()
	d.encoder.CopyTextureToTexture(
		&wgpu.TexelCopyTextureInfo{Texture: t.tex, Origin: wgpu.Origin3D{X: uint32(x), Y: uint32(y)}},
		&wgpu.TexelCopyTextureInfo{Texture: dst},
		&extent,
	)
	d.beginTopPass()

	return &Texture{tex: dst, view: view, w: w, h: h}, w, h, nil
}

// PushTarget / PopTarget expose the render-target stack to the painter's
// RepaintBoundary implementation (boundary.go). The pushed *Texture is kept on
// the stack entry so a nested backdrop blur can read it back (pre-rebrand tracker #223).
func (d *Device) PushTarget(t *Texture, w, h int) { d.pushTarget(t.View(), t.tex, w, h) }
func (d *Device) PopTarget()                      { d.popTarget() }

// PushOffscreenTarget pushes an off-screen render target whose first pass clears
// the color to the given value and gets a fresh DEDICATED depth-stencil
// attachment (cleared to far / 0). The dedicated attachment — rather than the
// shared size-keyed one — keeps the off-screen pass's depth/stencil from
// aliasing, and thus clearing, the canvas pass's stencil clip when the pixel
// sizes coincide (pre-rebrand tracker #231). It is released after the frame is submitted
// (frameDepths). Falls back to the shared attachment if the dedicated one cannot
// be created. Used by Scene3D (z-buffering, pre-rebrand tracker #231) and by the box-shadow / blur
// off-screen passes (pre-rebrand tracker #223), which need the same clip isolation.
func (d *Device) PushOffscreenTarget(t *Texture, w, h int, clear wgpu.Color) {
	d.endPass()
	rt := renderTarget{view: t.View(), tex: t.tex, w: w, h: h, clear: clear}
	if ds := d.newDepthStencil(w, h); ds != nil {
		rt.depthView = ds.view
		d.frameDepths = append(d.frameDepths, ds)
	}
	d.targets = append(d.targets, rt)
	d.beginTopPass()
}

// PushTargetLoad pushes a render target whose first pass preserves (LoadOp=Load)
// the texture's existing contents instead of clearing them. Used for the
// persistent canvas FBO (canvas_fbo.go), so a dirty-region partial repaint keeps
// the previous frame's pixels outside the redrawn region.
func (d *Device) PushTargetLoad(t *Texture, w, h int) {
	d.endPass()
	// cleared:true makes beginTopPass open with LoadOpLoad on the first pass.
	d.targets = append(d.targets, renderTarget{view: t.View(), tex: t.tex, w: w, h: h, cleared: true})
	d.beginTopPass()
}

// SetScissor restricts subsequent draws to the given pixel rectangle. The rect is
// remembered so it can be restored after a render-target pop (pre-rebrand tracker #235). The rect is
// clamped to the active render target's bounds: WebGPU rejects a scissor that
// extends past the attachment (a validation error that can escalate to a lost
// device), and dirty-region / clip rects are padded and can spill a few pixels
// past the canvas edge.
func (d *Device) SetScissor(x, y, w, h int) {
	tw, th, hasTarget := d.activeTargetSize()
	rx, ry, rw, rh, ok := resolveScissor(x, y, w, h, tw, th, hasTarget)
	if !ok {
		return
	}
	d.scissor = scissorRect{x: rx, y: ry, w: rw, h: rh, active: true}
	if d.pass != nil {
		d.pass.SetScissorRect(uint32(rx), uint32(ry), uint32(rw), uint32(rh))
	}
}

// resolveScissor computes the scissor rect to apply for a requested clip within a
// hasTarget-sized (tw×th) render target. ok is false only when the clip is empty
// and there is no target to validate against — the historical no-op. Crucially, an
// empty intersection with a known target yields a zero-area rect pinned inside the
// target so it culls every draw, instead of leaving the previous scissor active:
// that "leave prior" behavior let fully-clipped content (e.g. list rows below a
// scrolled viewport, whose clip-owner does not intersect the partial-repaint dirty
// region) leak through (pre-rebrand tracker #333).
func resolveScissor(x, y, w, h, tw, th int, hasTarget bool) (rx, ry, rw, rh int, ok bool) {
	if hasTarget {
		x, y, w, h = clampRect(x, y, w, h, tw, th)
	}
	if w <= 0 || h <= 0 {
		if !hasTarget {
			return 0, 0, 0, 0, false
		}
		w, h = 0, 0
		if x < 0 {
			x = 0
		} else if x > tw {
			x = tw
		}
		if y < 0 {
			y = 0
		} else if y > th {
			y = th
		}
	}
	return x, y, w, h, true
}

// activeTargetSize returns the current render target's pixel size.
func (d *Device) activeTargetSize() (w, h int, ok bool) {
	if len(d.targets) == 0 {
		return 0, 0, false
	}
	t := d.targets[len(d.targets)-1]
	return t.w, t.h, true
}

// clampRect clips rect (x,y,w,h) to the [0,maxW]×[0,maxH] box, returning a
// possibly-empty rect (w<=0 or h<=0) when it falls entirely outside.
func clampRect(x, y, w, h, maxW, maxH int) (int, int, int, int) {
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > maxW {
		w = maxW - x
	}
	if y+h > maxH {
		h = maxH - y
	}
	return x, y, w, h
}

// ClearScissor resets the scissor rectangle to the full active render target.
func (d *Device) ClearScissor() {
	d.scissor.active = false
	if d.pass == nil {
		return
	}
	t := d.targets[len(d.targets)-1]
	d.pass.SetScissorRect(0, 0, uint32(t.w), uint32(t.h))
}

// BeginFrame acquires the next surface texture and opens a clearing render pass
// on it as the root render target.
// acquireAction is the recovery BeginFrame takes after a failed surface acquire.
type acquireAction int

const (
	acquireSkip     acquireAction = iota // skip this frame, retry next (Timeout / non-status error)
	acquireReconfig                      // reconfigure the surface and re-acquire once (Outdated)
	acquireFatal                         // unrecoverable this frame — log then skip (Lost/DeviceLost/OutOfMemory)
)

// SurfaceStatusError carries a non-success surface-acquire status as an error.
// The bindings report the status in the returned SurfaceTexture rather than as an
// error, so acquireSurfaceTexture converts it here — keeping the classification
// below a pure function over an error, as the recovery path and its tests expect.
type SurfaceStatusError struct {
	Status wgpu.SurfaceGetCurrentTextureStatus
}

func (e *SurfaceStatusError) Error() string {
	return "surface acquire status: " + e.Status.String()
}

// acquireSurfaceTexture wraps TryGetCurrentTexture so a non-success status (and a
// success status with no texture) surfaces as a *SurfaceStatusError.
func (d *Device) acquireSurfaceTexture() (*wgpu.Texture, error) {
	st, err := d.surface.TryGetCurrentTexture()
	if err != nil {
		return nil, err
	}
	// SuccessSuboptimal still hands back a usable texture — render it and let the
	// next Outdated (or a resize) drive the reconfigure.
	if tex, ok := st.Get(); ok && tex != nil {
		return tex, nil
	}
	return nil, &SurfaceStatusError{Status: st.Status}
}

// classifyAcquire maps a surface-acquire error to the recovery action. A typed
// *SurfaceStatusError carries the concrete status; anything else (e.g. a
// validation error, or a nil-but-unexpected case) degrades to a plain skip. Pure
// and side-effect free so the branch table is unit-testable without a GPU (pre-rebrand tracker #385).
func classifyAcquire(err error) acquireAction {
	var se *SurfaceStatusError
	if errors.As(err, &se) {
		switch se.Status {
		case wgpu.SurfaceGetCurrentTextureStatusOutdated:
			return acquireReconfig
		case wgpu.SurfaceGetCurrentTextureStatusLost,
			wgpu.SurfaceGetCurrentTextureStatusError:
			// Lost/Error are the unrecoverable cases; wgpu-native folded the old
			// DeviceLost and OutOfMemory statuses into Error.
			return acquireFatal
		}
	}
	return acquireSkip
}

func (d *Device) BeginFrame(clear wgpu.Color) error {
	// Reset the per-frame arena staging and the textured bind-group position
	// cursor, then pre-size the GPU arenas to the high-water mark so this frame's
	// draws pack into them (a frame larger than any before still renders via the
	// overflow path and grows the arenas for next frame).
	d.arena.reset()
	d.texBindInUse = 0
	d.frameDemandVtx = 0
	d.frameDemandUni = 0
	if err := d.ensureArenas(); err != nil {
		return err
	}
	st, err := d.acquireSurfaceTexture()
	if err != nil {
		switch classifyAcquire(err) {
		case acquireReconfig:
			// Outdated: the swapchain no longer matches the window (a resize the
			// surface hasn't been reconfigured for, or a suboptimal present). Reconfigure
			// at the current size and re-acquire once so the frame renders instead of
			// dropping until some other event reconfigures (pre-rebrand tracker #385). Mirrors Resize's
			// reconfigure + depth-stencil drop; BeginFrame below re-sizes the render
			// target from the freshly acquired texture, so no size bookkeeping is needed.
			d.surface.Configure(d.device, d.config)
			d.releaseDepthStencils()
			st, err = d.acquireSurfaceTexture()
			if err != nil {
				return err // retry failed — skip this frame, try again next
			}
		case acquireFatal:
			bellui.LogError("webgpu: unrecoverable surface acquire, skipping frame", err)
			return err
		default: // acquireSkip: Timeout (transient) or a non-status error
			return err
		}
	}
	d.frameTex = st
	view, err := st.TryCreateView(nil)
	if err != nil {
		// frameTex was acquired; drop it so its SurfaceOutput can't block a later
		// surface reconfigure (pre-rebrand tracker #337).
		d.DiscardFrame()
		return err
	}
	d.frameView = view

	d.encoder, err = d.device.TryCreateCommandEncoder(nil)
	if err != nil {
		d.DiscardFrame()
		return err
	}
	d.targets = d.targets[:0]
	// Size the root render target from the ACQUIRED surface texture, not the
	// cached config size. On the web the surface's current texture follows the
	// live canvas backing size and can lead d.config by a frame during a
	// continuous resize; sizing the depth-stencil (in beginTopPass) from the
	// cached config then desyncs it from the color view by one frame, so every
	// mid-resize pass is invalid (depthStencil size != color size) and the frame
	// is dropped — the resize flicker in pre-rebrand tracker #346. Deriving the target extent (hence
	// both attachments) from one read of the acquired texture keeps them in step.
	// Native swapchains reallocate color+size together so this matches today's
	// behavior there; it only changes the transient web-resize case. Falls back to
	// the config size if the texture reports nothing.
	tw, th := int(st.GetWidth()), int(st.GetHeight())
	if tw <= 0 || th <= 0 {
		tw, th = int(d.config.Width), int(d.config.Height)
	}
	d.targets = append(d.targets, renderTarget{view: view, tex: st, w: tw, h: th, clear: clear})
	d.stencilRef = 0 // fresh pass starts with no clip reference (pre-rebrand tracker #305)
	d.beginTopPass()
	return nil
}

// beginTopPass opens a render pass on the top render target: clearing on its
// first open, loading (preserving) on subsequent re-opens. A depth-stencil
// attachment (sized to the target) is attached with the same load op, so the
// stencil is zeroed at the first open and preserved across pop-and-reopen.
func (d *Device) beginTopPass() {
	t := &d.targets[len(d.targets)-1]
	load := wgpu.LoadOpLoad
	if !t.cleared {
		load = wgpu.LoadOpClear
		t.cleared = true
	}
	desc := &wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       t.view,
			LoadOp:     load,
			StoreOp:    wgpu.StoreOpStore,
			ClearValue: t.clear,
		}},
	}
	dv := t.depthView
	if dv == nil {
		dv = d.ensureDepthStencil(t.w, t.h)
	}
	if dv != nil {
		desc.DepthStencilAttachment = &wgpu.RenderPassDepthStencilAttachment{
			View:              dv,
			DepthLoadOp:       load,
			DepthStoreOp:      wgpu.StoreOpStore,
			DepthClearValue:   1,
			StencilLoadOp:     load,
			StencilStoreOp:    wgpu.StoreOpStore,
			StencilClearValue: 0,
		}
	}
	d.pass = d.encoder.BeginRenderPass(desc)
}

// ensureDepthStencil returns a depth+stencil attachment view sized w×h, creating
// and caching it on first use. Cached across frames and shared between passes of
// the same size (they never overlap in time). Returns nil on failure (the pass
// then runs without depth/stencil, disabling clipping rather than crashing).
func (d *Device) ensureDepthStencil(w, h int) *wgpu.TextureView {
	if w <= 0 || h <= 0 {
		return nil
	}
	key := [2]int{w, h}
	if ds, ok := d.depthStencils[key]; ok {
		return ds.view
	}
	ds := d.newDepthStencil(w, h)
	if ds == nil {
		return nil
	}
	d.depthStencils[key] = ds
	return ds.view
}

// newDepthStencil creates a standalone depth+stencil attachment sized w×h, owned
// by the caller (not the shared size-keyed cache). Used for the Scene3D
// off-screen render's dedicated attachment (pre-rebrand tracker #231). Returns nil on failure.
func (d *Device) newDepthStencil(w, h int) *depthStencil {
	if w <= 0 || h <= 0 {
		return nil
	}
	tex, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "depth-stencil",
		Size:          wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        depthStencilFormat,
		Usage:         wgpu.TextureUsageRenderAttachment,
	})
	if err != nil {
		return nil
	}
	view, err := tex.TryCreateView(nil)
	if err != nil {
		tex.Release()
		return nil
	}
	return &depthStencil{tex: tex, view: view}
}

// SetStencilRef sets the active render pass's stencil reference value (used by
// the painter's clip / fill stencil ops). No-op outside a frame.
func (d *Device) SetStencilRef(ref uint32) {
	d.stencilRef = ref
	if d.pass != nil {
		d.pass.SetStencilReference(ref)
	}
}

// endPass ends and releases the active render pass, if any.
func (d *Device) endPass() {
	if d.pass != nil {
		d.pass.End()
		d.pass.Release()
		d.pass = nil
	}
}

// HasOpenFrame reports whether BeginFrame acquired a surface texture that has not
// yet been presented or discarded.
func (d *Device) HasOpenFrame() bool {
	return d.frameTex != nil
}

// DiscardFrame abandons an open frame without presenting it: it ends the render
// pass, drops the unsubmitted command encoder, and releases the acquired surface
// texture (the SurfaceOutput) and its view. A frame is left open when paint()
// begins one but the driver skips Present because the window is not visible; the
// held SurfaceOutput then makes the next surface reconfigure (Resize → Configure)
// or RecreateSurface a fatal WebGPU validation error ("SurfaceOutput must be
// dropped before a new Surface is made"). Callers invoke this before reconfiguring.
func (d *Device) DiscardFrame() {
	d.endPass()
	if d.encoder != nil {
		d.encoder.Release()
		d.encoder = nil
	}
	if d.frameView != nil {
		d.frameView.Release()
		d.frameView = nil
	}
	if d.frameTex != nil {
		d.frameTex.Release()
		d.frameTex = nil
	}
	d.targets = d.targets[:0]
}

// pushTarget redirects subsequent draws into the given color view (cleared to
// transparent), e.g. a RepaintBoundary FBO. Pair with popTarget.
func (d *Device) pushTarget(view *wgpu.TextureView, tex *wgpu.Texture, w, h int) {
	d.endPass()
	d.targets = append(d.targets, renderTarget{view: view, tex: tex, w: w, h: h, clear: wgpu.Color{}})
	d.beginTopPass()
}

// popTarget restores the enclosing render target (re-opened with LoadOp=Load so
// its prior contents are preserved). The off-screen pass that was active ran
// unclipped (a fresh pass starts with the full scissor); re-apply the saved
// window-space clip to the reopened parent so an enclosing StartClipping survives
// the boundary/scene/shadow render-target round-trip (pre-rebrand tracker #235).
func (d *Device) popTarget() {
	d.endPass()
	if len(d.targets) > 1 {
		d.targets = d.targets[:len(d.targets)-1]
	}
	d.beginTopPass()
	if d.pass != nil {
		if d.scissor.active {
			d.pass.SetScissorRect(uint32(d.scissor.x), uint32(d.scissor.y), uint32(d.scissor.w), uint32(d.scissor.h))
		}
		// Restore the stencil reference: the off-screen pass left it at the new
		// pass's default (0), but an enclosing rounded/path clip set it (pre-rebrand tracker #305).
		d.pass.SetStencilReference(d.stencilRef)
	}
}

// Pass returns the active render pass (valid between BeginFrame and EndFrame).
func (d *Device) Pass() *wgpu.RenderPassEncoder { return d.pass }

// EndFrame closes the render pass, copies the frame into the capture texture
// (so Capture can read it back later), submits the command buffer and presents.
func (d *Device) EndFrame() error {
	d.endPass()

	// Copy the just-rendered surface texture into the persistent capture texture
	// before Present (the swapchain texture is invalid afterwards) — but only once
	// Capture has been requested, so frames cost nothing extra until then (pre-rebrand tracker #219).
	if d.captureRequested && d.ensureCaptureTex() {
		d.encoder.CopyTextureToTexture(
			&wgpu.TexelCopyTextureInfo{Texture: d.frameTex},
			&wgpu.TexelCopyTextureInfo{Texture: d.captureTex},
			&wgpu.Extent3D{Width: uint32(d.captureW), Height: uint32(d.captureH), DepthOrArrayLayers: 1},
		)
	}

	// Flush both arenas with a single WriteBuffer each (pre-rebrand tracker #255): every draw this
	// frame accumulated its vertices and uniform block into the staging slices,
	// and the recorded passes reference the GPU arenas at the matching offsets.
	// WriteBuffer is queue-ordered before the Submit below, so the draws read the
	// freshly written data. This collapses the slot pool's ~2N WriteBuffer/frame
	// (one vertex + one uniform per draw) to ~2.
	if n := d.arena.vtxLen(); n > 0 {
		d.queue.WriteBuffer(d.vtxArena, 0, d.arena.vtx[:n])
	}
	if n := d.arena.uniLen(); n > 0 {
		d.queue.WriteBuffer(d.uniArena, 0, d.arena.uni[:n])
	}
	// Raise the high-water marks to this frame's full demand so the next frame's
	// ensureArenas pre-sizes the arenas to fit it (any overflow self-heals in one
	// frame).
	d.arenaMaxVtx = max(d.arenaMaxVtx, d.frameDemandVtx)
	d.arenaMaxUni = max(d.arenaMaxUni, d.frameDemandUni)

	cmd, err := d.encoder.TryFinish(nil)
	if err != nil {
		return err
	}
	d.submitFrame(cmd)
	cmd.Release()
	d.encoder.Release()
	d.encoder = nil

	d.surface.Present()

	d.gpuDrainAfterPresent() // blocking GPU drain timing, pre-rebrand tracker #336

	// The arena buffers and the cached bind groups persist across frames and are
	// reused, so only the genuinely transient mid-frame resources are released
	// here: the off-screen depth/texture targets, and the overflow path's per-draw
	// buffers and bind groups. The recorded commands referencing them were just
	// submitted; wgpu refcounts keep the native resources alive until the GPU is
	// done, so releasing the Go handles now is safe.
	for _, buf := range d.frameBuffers {
		destroyBuffer(buf)
	}
	d.frameBuffers = d.frameBuffers[:0]
	for _, bind := range d.frameBinds {
		bind.Release()
	}
	d.frameBinds = d.frameBinds[:0]
	for _, ds := range d.frameDepths {
		ds.Release()
	}
	d.frameDepths = d.frameDepths[:0]
	for _, tex := range d.frameTextures {
		tex.Release()
	}
	d.frameTextures = d.frameTextures[:0]

	if d.frameView != nil {
		d.frameView.Release()
		d.frameView = nil
	}
	// Release the presented surface texture, don't just drop the handle.
	// Present hands the image back to the compositor, but the texture acquired
	// by BeginFrame still holds a reference to the swapchain's back buffer, and
	// an outstanding back-buffer reference is exactly what DXGI's ResizeBuffers
	// refuses: DXGI_ERROR_INVALID_CALL, which wgpu reports as "window is in
	// use" and wgpu-native turns into a fatal panic, aborting the process on
	// the first window resize (#20). Mirrors wgpu-native's own present sequence
	// (release the view, then the texture) and what DiscardFrame already does
	// on the abandon path; the wgpu refcount keeps the native image alive until
	// the GPU is finished with it.
	if d.frameTex != nil {
		d.frameTex.Release()
		d.frameTex = nil
	}
	return nil
}

// ensureCaptureTex (re)creates the capture texture to match the surface size.
// Returns false if it could not be created (capture then no-ops).
func (d *Device) ensureCaptureTex() bool {
	w, h := int(d.config.Width), int(d.config.Height)
	if w <= 0 || h <= 0 {
		return false
	}
	if d.captureTex != nil && d.captureW == w && d.captureH == h {
		return true
	}
	if d.captureTex != nil {
		destroyTexture(d.captureTex)
		d.captureTex = nil
	}
	tex, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "capture",
		Size:          wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        d.config.Format,
		Usage:         wgpu.TextureUsageCopyDst | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		return false
	}
	d.captureTex, d.captureW, d.captureH = tex, w, h
	return true
}

// EnableCapture requests that EndFrame maintain the per-frame capture copy from
// now on (pre-rebrand tracker #219). Until this is called, the copy is skipped to save the per-frame
// GPU cost.
func (d *Device) EnableCapture() { d.captureRequested = true }

// CaptureImage reads the last presented frame back into an *image.RGBA, or nil if
// no frame has been captured yet (the per-frame copy starts after EnableCapture).
// Channels are normalized to RGBA even when the surface format is BGRA.
func (d *Device) CaptureImage() *image.RGBA {
	if d.captureTex == nil {
		return nil
	}
	return d.captureTextureToImage(d.captureTex, d.captureW, d.captureH)
}

// CaptureTexture reads back an arbitrary texture (surface format, CopySrc) into
// an *image.RGBA. Used as the first-capture fallback that reads the persistent
// canvas FBO directly, before the per-frame copy has been populated (pre-rebrand tracker #219).
func (d *Device) CaptureTexture(tex *wgpu.Texture, w, h int) *image.RGBA {
	if tex == nil || w <= 0 || h <= 0 {
		return nil
	}
	return d.captureTextureToImage(tex, w, h)
}

// captureTextureToImage copies a texture into a mappable buffer and decodes it to
// an *image.RGBA (256-aligned rows, BGRA→RGBA swap when the surface is BGRA).
func (d *Device) captureTextureToImage(srcTex *wgpu.Texture, w, h int) *image.RGBA {
	// CopyTextureToBuffer requires bytesPerRow to be a multiple of 256.
	bytesPerRow := ((w*4 + 255) / 256) * 256
	buf, err := d.device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Size:  uint64(bytesPerRow * h),
		Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
	})
	if err != nil {
		return nil
	}
	defer buf.Release()

	enc, err := d.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return nil
	}
	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: srcTex},
		&wgpu.TexelCopyBufferInfo{Buffer: buf, Layout: wgpu.TexelCopyBufferLayout{BytesPerRow: uint32(bytesPerRow), RowsPerImage: uint32(h)}},
		&wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
	)
	cmd, err := enc.TryFinish(nil)
	if err != nil {
		enc.Release()
		return nil
	}
	d.queue.Submit(cmd)
	cmd.Release()
	enc.Release()

	var status wgpu.MapAsyncStatus
	if err := buf.TryMapAsync(wgpu.MapModeRead, 0, uint64(bytesPerRow*h), func(s wgpu.MapAsyncStatus) { status = s }); err != nil {
		return nil
	}
	d.device.Poll(true, nil)
	if status != wgpu.MapAsyncStatusSuccess {
		return nil
	}
	data := buf.GetMappedRange(0, uint(bytesPerRow*h))
	defer buf.Unmap()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		src := data[y*bytesPerRow : y*bytesPerRow+w*4]
		dst := img.Pix[y*img.Stride : y*img.Stride+w*4]
		copy(dst, src)
		if d.captureSwap {
			for x := 0; x < w*4; x += 4 {
				dst[x], dst[x+2] = dst[x+2], dst[x] // BGRA -> RGBA
			}
		}
	}
	return img
}

// Destroy releases every GPU resource. Safe to call on a partially-built Device.
func (d *Device) Destroy() {
	if d.pipelines != nil {
		d.pipelines.Release(func(p *wgpu.RenderPipeline) { p.Release() })
	}
	for name, m := range d.modules {
		m.Release()
		delete(d.modules, name)
	}
	if d.sampler != nil {
		d.sampler.Release()
		d.sampler = nil
	}
	// Per-frame arenas, their cached bind groups and any not-yet-released
	// overflow resources.
	d.invalidateUniformBinds()
	if d.vtxArena != nil {
		destroyBuffer(d.vtxArena)
		d.vtxArena = nil
	}
	if d.uniArena != nil {
		destroyBuffer(d.uniArena)
		d.uniArena = nil
	}
	for _, buf := range d.frameBuffers {
		destroyBuffer(buf)
	}
	d.frameBuffers = nil
	for _, bind := range d.frameBinds {
		bind.Release()
	}
	d.frameBinds = nil
	for _, pl := range []*wgpu.PipelineLayout{d.plUniform, d.plTextured} {
		if pl != nil {
			pl.Release()
		}
	}
	d.plUniform, d.plTextured = nil, nil
	for _, bgl := range []*wgpu.BindGroupLayout{d.bglUniform, d.bglTextured} {
		if bgl != nil {
			bgl.Release()
		}
	}
	d.bglUniform, d.bglTextured = nil, nil
	d.releaseDepthStencils()
	if d.captureTex != nil {
		destroyTexture(d.captureTex)
		d.captureTex = nil
	}
	d.config = nil
	if d.queue != nil {
		d.queue.Release()
		d.queue = nil
	}
	if d.device != nil {
		d.device.Release()
		d.device = nil
	}
	if d.adapter != nil {
		d.adapter.Release()
		d.adapter = nil
	}
	if d.surface != nil {
		d.surface.Release()
		d.surface = nil
	}
	if d.instance != nil {
		d.instance.Release()
		d.instance = nil
	}
}
