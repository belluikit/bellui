//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; headless Chromium grants one,
// but the fork's wgpu binding needs its JS support glue, which wasmbrowsertest
// does not load — TryWriteTexture panics with "wasm is not defined". The wasm
// WebGPU path is verified live via the settings-app build instead
// (docs/webgpu.md); the glue gap is tracked in the issue filed from #79.)

package webgpu

import (
	"image"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// TestTextTexturePremultiplyReadback guards the glyph-AA premultiply fix: text is
// rasterized to a straight-alpha *image.NRGBA, and the `simple` shader composites
// textures with premultiplied-over blending without multiplying rgb by the
// texture's coverage alpha. So CreateImageTexture MUST premultiply NRGBA on upload
// (like the GL painter's imgToTexture); uploading it raw makes anti-aliased edge
// pixels blow out to full intensity, collapsing the AA gradient into blocky text.
//
// The test uploads a uniform 50%-coverage white NRGBA texture via
// CreateImageTexture, draws it as a full-target quad through the real `simple`
// pipeline (BlendPremul) over a dark-cleared offscreen target, reads back the
// centre pixel, and asserts it is a mid grey — not blown out to white.
func TestTextTexturePremultiplyReadback(t *testing.T) {
	const dim = 64
	const bytesPerRow = dim * 4

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
	target, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "offscreen",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        format,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	defer target.Release()
	targetView, err := target.TryCreateView(nil)
	if err != nil {
		t.Fatalf("create target view: %v", err)
	}
	defer targetView.Release()

	// A uniform straight-alpha glyph-edge texture: white at 50% coverage.
	const cov = 128
	glyph := image.NewNRGBA(image.Rect(0, 0, dim, dim))
	for i := 0; i < len(glyph.Pix); i += 4 {
		glyph.Pix[i], glyph.Pix[i+1], glyph.Pix[i+2], glyph.Pix[i+3] = 255, 255, 255, cov
	}
	d := &Device{device: device, queue: queue}
	tex, err := d.CreateImageTexture(glyph)
	if err != nil {
		t.Fatalf("CreateImageTexture: %v", err)
	}
	defer tex.Release()

	sampler, err := device.TryCreateSampler(&wgpu.SamplerDescriptor{
		AddressModeU: wgpu.AddressModeClampToEdge, AddressModeV: wgpu.AddressModeClampToEdge,
		AddressModeW: wgpu.AddressModeClampToEdge,
		MagFilter:    wgpu.FilterModeLinear, MinFilter: wgpu.FilterModeLinear,
		MipmapFilter: wgpu.MipmapFilterModeNearest, LodMaxClamp: 1, MaxAnisotropy: 1,
	})
	if err != nil {
		t.Fatalf("sampler: %v", err)
	}
	defer sampler.Release()

	// Build the real `simple` (textured, premultiplied-over) pipeline.
	desc, _ := DescriptorFor("simple")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "simple.wgsl",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("simple"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer mod.Release()
	pipe, err := device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
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

	// Full-target quad: pos.xyz + uv per vertex (triangle strip), uv spanning 0..1.
	verts := []float32{
		-1, -1, 0, 0, 1,
		-1, 1, 0, 0, 0,
		1, -1, 0, 1, 1,
		1, 1, 0, 1, 0,
	}
	vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	defer vbuf.Release()

	u := SimpleUniforms{Alpha: 1, CornerRadius: 0, Size: [2]float32{dim, dim}, Inset: [4]float32{0, 0, 1, 1}}
	ubuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&u), Usage: wgpu.BufferUsageUniform})
	if err != nil {
		t.Fatalf("uniform buffer: %v", err)
	}
	defer ubuf.Release()

	layout := pipe.GetBindGroupLayout(0)
	defer layout.Release()
	bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: ubuf, Size: uint64(len(uniformBytes(&u)))},
			{Binding: 1, TextureView: tex.View()},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("bind group: %v", err)
	}
	defer bg.Release()

	readBuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Size: bytesPerRow * dim, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("readback buffer: %v", err)
	}
	defer readBuf.Release()

	enc, err := device.TryCreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	// Clear to a dark background (~#121216 in linear-byte terms).
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: targetView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 18.0 / 255, G: 18.0 / 255, B: 22.0 / 255, A: 1},
		}},
	})
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg, nil)
	pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	pass.Draw(4, 1, 0, 0)
	pass.End()
	pass.Release()

	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: target},
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
	r := data[off]
	// 50% white premultiplied over dark ≈ 128 + 0.5*18 ≈ 136. The pre-fix bug
	// (raw NRGBA upload) blows this out to ~255. Guard a clear mid-grey band.
	if r < 100 || r > 180 {
		t.Fatalf("centre pixel r=%d, want mid-grey (~136); >180 means non-premultiplied AA blow-out (the bug)", r)
	}
}
