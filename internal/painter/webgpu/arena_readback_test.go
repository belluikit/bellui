//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; headless Chromium grants one,
// but the fork's wgpu binding needs its JS support glue, which wasmbrowsertest
// does not load — TryWriteTexture panics with "wasm is not defined". The wasm
// WebGPU path is verified live via the settings-app build instead
// (docs/webgpu.md); the glue gap is tracked in the issue filed from #79.)

package webgpu

import (
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// TestArenaDynamicOffsetReadback is a headless, cross-GPU proof of the per-frame
// arena mechanism (pre-rebrand tracker #255): an explicit pipeline layout that declares the uniform
// binding with hasDynamicOffset, a uniform "arena" holding two 256-byte blocks
// (red then green) and a vertex "arena" holding two identical full-target
// triangles, drawn by indexing the *second* of each via a vertex byte offset and
// a 256-byte dynamic uniform offset. The center pixel must be GREEN: if the
// dynamic offset were ignored the draw would read block 0 and come out red, and
// if the vertex offset were wrong the triangle would not cover the center. This
// runs on whatever adapter the CI leg provides (Metal / Vulkan / D3D12), unlike
// the live-window arena path which needs a surface.
func TestArenaDynamicOffsetReadback(t *testing.T) {
	const dim = 64
	const bytesPerRow = dim * 4 // 256, already 256-aligned

	instance := wgpu.CreateInstance(chooseInstanceBackends())
	defer instance.Release()
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Skipf("no WebGPU adapter available: %v", err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("no WebGPU device available: %v", err)
	}
	defer device.Release()
	queue := device.GetQueue()
	defer queue.Release()

	const format = wgpu.TextureFormatRGBA8Unorm
	tex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "offscreen",
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
	defer tex.Release()
	view, err := tex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("create view: %v", err)
	}
	defer view.Release()

	// Explicit bind-group + pipeline layout with the uniform binding marked
	// hasDynamicOffset — the same shape buildLayouts() builds for non-textured
	// shaders, and the prerequisite for the arena's dynamic uniform offsets.
	bgl, err := device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
			Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform, HasDynamicOffset: true},
		}},
	})
	if err != nil {
		t.Fatalf("bind-group layout: %v", err)
	}
	defer bgl.Release()
	pl, err := device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{bgl},
	})
	if err != nil {
		t.Fatalf("pipeline layout: %v", err)
	}
	defer pl.Release()

	desc, _ := DescriptorFor("path")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "path.wgsl",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("path"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer mod.Release()
	pipe, err := device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Layout: pl,
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
	defer pipe.Release()

	// Vertex arena: two identical full-target triangles back to back, packed via
	// the arena's appendVertices offsets.
	var va arena
	tri := []float32{-1, -1, 1, 3, -1, 1, -1, 3, 1}
	va.addVertices(wgpu.ToBytes(tri))
	tri1Off := va.addVertices(wgpu.ToBytes(tri))
	vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: va.vtx, Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	defer vbuf.Release()

	// Uniform arena: block 0 red, block 1 green, each padded to the 256 stride.
	var ua arena
	red := PathUniforms{FillColor: [4]float32{1, 0, 0, 1}}
	green := PathUniforms{FillColor: [4]float32{0, 1, 0, 1}}
	ua.addUniform(uniformBytes(&red))
	greenOff := ua.addUniform(uniformBytes(&green))
	if greenOff != uniformBlockStride {
		t.Fatalf("green dynamic offset = %d, want %d", greenOff, uniformBlockStride)
	}
	ubuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: ua.uni, Usage: wgpu.BufferUsageUniform})
	if err != nil {
		t.Fatalf("uniform buffer: %v", err)
	}
	defer ubuf.Release()

	bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout:  bgl,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: ubuf, Offset: 0, Size: uniformBlockStride}},
	})
	if err != nil {
		t.Fatalf("bind group: %v", err)
	}
	defer bg.Release()

	readBuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Size:  bytesPerRow * dim,
		Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("readback buffer: %v", err)
	}
	defer readBuf.Release()

	enc, err := device.TryCreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	pass.SetPipeline(pipe)
	// Index the SECOND triangle (vertex byte offset) and the GREEN block (dynamic
	// uniform offset) — exactly what Record does for an arena draw.
	pass.SetBindGroup(0, bg, []uint32{uint32(greenOff)})
	pass.SetVertexBuffer(0, vbuf, uint64(tri1Off), uint64(len(wgpu.ToBytes(tri))))
	pass.Draw(3, 1, 0, 0)
	pass.End()
	pass.Release()

	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: tex},
		&wgpu.TexelCopyBufferInfo{Buffer: readBuf, Layout: wgpu.TexelCopyBufferLayout{BytesPerRow: bytesPerRow, RowsPerImage: dim}},
		&wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
	)
	cmd, err := enc.TryFinish(nil)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	queue.Submit(cmd)
	cmd.Release()
	enc.Release()

	var mapStatus wgpu.MapAsyncStatus
	if err = readBuf.TryMapAsync(wgpu.MapModeRead, 0, bytesPerRow*dim, func(s wgpu.MapAsyncStatus) { mapStatus = s }); err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if mapStatus != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", mapStatus)
	}
	data := readBuf.GetMappedRange(0, uint(bytesPerRow*dim))
	defer readBuf.Unmap()

	off := (dim/2)*bytesPerRow + (dim/2)*4
	r, g, b, alpha := data[off], data[off+1], data[off+2], data[off+3]
	if g < 250 || r > 5 || b > 5 || alpha < 250 {
		t.Fatalf("center pixel = (%d,%d,%d,%d), want opaque green (dynamic offset selected block 1)", r, g, b, alpha)
	}
}
