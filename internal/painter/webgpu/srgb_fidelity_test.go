//go:build !gl

package webgpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// These tests guard #322 end-to-end on a real GPU adapter (Metal / Vulkan /
// D3D12 in the webgpu_tests CI job). They render a known theme-dark color
// through the painter's solid-color path (the `path` pipeline, the same one
// fragColor feeds) into an offscreen target and read the pixel back.
//
// The mechanism behind #322: the painter writes straight sRGB-encoded color
// values and does NO gamma encoding in its shaders. If the render target is an
// sRGB format, the GPU treats those writes as linear and re-encodes them to
// sRGB on store, so a dark color comes back far brighter than authored. The fix
// (chooseSurfaceFormat) configures the surface with a NON-sRGB format so the
// values pass through unchanged, matching the GL/software backends.
//
// The existing readback/baseline tests render into a hardcoded RGBA8Unorm target
// and so never exercise the format-selection or the sRGB re-encode at all. These
// two tests close that gap:
//
//   - TestSurfaceFormatColorFidelity is the regression guard: it renders through
//     the format chooseSurfaceFormat picks for an sRGB-preferred surface and
//     asserts the dark color survives. If chooseSurfaceFormat ever regresses to
//     returning the sRGB format, this renders into an sRGB target and fails.
//   - TestSrgbTargetOverBrightensCharacterization proves the guard has teeth: it
//     renders the SAME color into an explicit sRGB target and asserts it comes
//     back conspicuously brighter, so the fidelity test's tolerance genuinely
//     distinguishes the fixed path from the bug.
//
// The RGBA family is used (RGBA8Unorm / RGBA8UnormSrgb) so the read-back bytes
// are in image-native order with no channel swap; the sRGB re-encode mechanism
// is identical across the BGRA/RGBA families. Real-surface, per-backend BGRA
// verification is the manual Phase 2 sweep (#325 task 1).

// darkColor is the theme-dark sample from #322 (#121216); on an sRGB target it
// was observed to read back as #4b4b53.
var darkColor = color.NRGBA{R: 0x12, G: 0x12, B: 0x16, A: 0xff}

func TestSurfaceFormatColorFidelity(t *testing.T) {
	// The Metal-style caps list: the sRGB variant is advertised first, the
	// non-sRGB sibling second — exactly the case #322 fixes.
	caps := []wgpu.TextureFormat{wgpu.TextureFormatRGBA8UnormSrgb, wgpu.TextureFormatRGBA8Unorm}
	format := chooseSurfaceFormat(caps)
	if format != wgpu.TextureFormatRGBA8Unorm {
		t.Fatalf("chooseSurfaceFormat picked %v, want the non-sRGB sibling RGBA8Unorm (#322)", format)
	}

	got := renderSolidCenterPixel(t, format, darkColor)
	t.Logf("dark #%02x%02x%02x through %v read back as #%02x%02x%02x", darkColor.R, darkColor.G, darkColor.B, format, got.R, got.G, got.B)

	// Solid-fill interior: GPU AA/rounding noise is sub-LSB, so a tight tolerance
	// is safe and keeps the assertion well clear of the ~#4b sRGB-encoded value.
	const tol = 4
	if !channelsClose(got, darkColor, tol) {
		t.Fatalf("dark color rendered through the surface format read back as #%02x%02x%02x, want ~#%02x%02x%02x (±%d). "+
			"A brighter value (~#4b4b53) means the surface format regressed to sRGB — see #322.",
			got.R, got.G, got.B, darkColor.R, darkColor.G, darkColor.B, tol)
	}
}

func TestSrgbTargetOverBrightensCharacterization(t *testing.T) {
	// Render the identical color into an explicit sRGB target — the bug's
	// condition. The GPU re-encodes the dark write, so every channel comes back
	// far brighter; this is what chooseSurfaceFormat avoids.
	got := renderSolidCenterPixel(t, wgpu.TextureFormatRGBA8UnormSrgb, darkColor)
	t.Logf("dark #%02x%02x%02x through an sRGB target read back as #%02x%02x%02x (over-bright = the #322 mechanism)", darkColor.R, darkColor.G, darkColor.B, got.R, got.G, got.B)

	// Authored channels are 0x12/0x12/0x16; the sRGB re-encode lifts them to
	// roughly 0x4b/0x4b/0x53. Require a wide, unambiguous gap so this test fails
	// only if sRGB re-encoding stopped happening (which would also invalidate the
	// fidelity guard above). Use the green channel for the floor (identical in
	// either channel order) and require it well above the authored value + the
	// fidelity tolerance.
	const minBright = 0x30 // 48 — comfortably above authored 0x12 and the ±4 guard
	if got.G < minBright {
		t.Fatalf("dark color on an sRGB target read back as #%02x%02x%02x; expected a conspicuously brighter "+
			"value (~#4b4b53) from sRGB re-encoding. If this is dark, the over-bright mechanism #322 guards is "+
			"gone and TestSurfaceFormatColorFidelity no longer proves anything.", got.R, got.G, got.B)
	}
}

func channelsClose(a color.RGBA, b color.NRGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// renderSolidCenterPixel draws a single full-target opaque quad of the given
// color through the `path` pipeline into an offscreen texture of the requested
// format, then returns the read-back center pixel. The color is fed in exactly
// as the painter's fragColor would emit it for an opaque color (straight RGBA in
// 0..1). Skips if no GPU adapter/device is available.
func renderSolidCenterPixel(t *testing.T, format wgpu.TextureFormat, c color.NRGBA) color.RGBA {
	t.Helper()
	const dim = 32

	instance := wgpu.CreateInstance(chooseInstanceBackends())
	t.Cleanup(instance.Release)
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Skipf("no WebGPU adapter available: %v", err)
	}
	t.Cleanup(adapter.Release)
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("no WebGPU device available: %v", err)
	}
	t.Cleanup(device.Release)
	queue := device.GetQueue()
	t.Cleanup(queue.Release)

	tex, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "srgb-fidelity",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        format,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("create texture: %v", err)
	}
	t.Cleanup(tex.Release)
	view, err := tex.CreateView(nil)
	if err != nil {
		t.Fatalf("create view: %v", err)
	}
	t.Cleanup(view.Release)

	desc, _ := DescriptorFor("path")
	mod, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:          "path.wgsl",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{Code: string(ShaderSource("path"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	t.Cleanup(mod.Release)
	pipe, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Vertex: wgpu.VertexState{
			Module: mod, EntryPoint: VertexEntry,
			Buffers: []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)},
		},
		Fragment: &wgpu.FragmentState{
			Module: mod, EntryPoint: FragmentEntry,
			Targets: []wgpu.ColorTargetState{{Format: format, Blend: blendState(desc.DefaultBlend), WriteMask: wgpu.ColorWriteMaskAll}},
		},
		Primitive:   wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), CullMode: wgpu.CullModeNone},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	t.Cleanup(pipe.Release)

	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 0, G: 0, B: 0, A: 1},
		}},
	})

	// Straight RGBA in 0..1, as the painter's fragColor emits for an opaque color.
	fill := [4]float32{float32(c.R) / 0xff, float32(c.G) / 0xff, float32(c.B) / 0xff, float32(c.A) / 0xff}
	verts := quadVerts(-1, -1, 1, 1)
	vbuf, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	u := PathUniforms{FillColor: fill}
	ubuf, err := device.CreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&u), Usage: wgpu.BufferUsageUniform})
	if err != nil {
		t.Fatalf("uniform buffer: %v", err)
	}
	layout := pipe.GetBindGroupLayout(0)
	bg, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout:  layout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: ubuf, Size: uint64(len(uniformBytes(&u)))}},
	})
	if err != nil {
		t.Fatalf("bind group: %v", err)
	}
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg, nil)
	pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	pass.Draw(6, 1, 0, 0)
	pass.End()
	pass.Release()

	const bytesPerRow = dim * 4
	alignedRow := ((bytesPerRow + 255) / 256) * 256
	readBuf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  uint64(alignedRow * dim),
		Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("readback buffer: %v", err)
	}
	t.Cleanup(readBuf.Release)
	enc.CopyTextureToBuffer(
		&wgpu.ImageCopyTexture{Texture: tex},
		&wgpu.ImageCopyBuffer{Buffer: readBuf, Layout: wgpu.TextureDataLayout{BytesPerRow: uint32(alignedRow), RowsPerImage: dim}},
		&wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
	)
	cmd, err := enc.Finish(nil)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	queue.Submit(cmd)
	cmd.Release()
	enc.Release()
	vbuf.Release()
	ubuf.Release()
	bg.Release()
	layout.Release()

	var status wgpu.BufferMapAsyncStatus
	if err := readBuf.MapAsync(wgpu.MapModeRead, 0, uint64(alignedRow*dim), func(s wgpu.BufferMapAsyncStatus) { status = s }); err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if status != wgpu.BufferMapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", status)
	}
	data := readBuf.GetMappedRange(0, uint(alignedRow*dim))
	defer readBuf.Unmap()

	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		copy(img.Pix[y*img.Stride:y*img.Stride+dim*4], data[y*alignedRow:y*alignedRow+dim*4])
	}
	return img.RGBAAt(dim/2, dim/2)
}
