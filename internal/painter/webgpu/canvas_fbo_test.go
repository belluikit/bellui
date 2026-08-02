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

// TestCanvasFBORetainAndScissor is a headless proof of the two GPU behaviours the
// persistent canvas FBO (canvas_fbo.go) depends on for dirty-region partial
// repaints: a render pass with LoadOp=Load preserves the texture's prior content,
// and a scissor restricts a full-screen "clear" quad to just the dirty region.
//
// It clears an offscreen texture to red (pass 1), then in a second LoadOp=Load
// pass scissors a sub-rectangle and paints it green with a full-screen quad
// through the path pipeline (Replace blend, exactly as clearActiveToBackground
// does). Afterwards the scissored region must be green while everything outside
// it stays red — i.e. the retained pixels survived and only the region changed.
func TestCanvasFBORetainAndScissor(t *testing.T) {
	const dim = 64
	const bytesPerRow = dim * 4

	// Scissor region: [16,16) .. [48,48). Center (32,32) is inside; (0,0) outside.
	const rx, ry, rw, rh = 16, 16, 32, 32

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
		Label:         "canvas-fbo",
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

	// path pipeline with Replace blend, matching clearActiveToBackground's quad.
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
		Vertex: wgpu.VertexState{
			Module: mod, EntryPoint: VertexEntry,
			Buffers: []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)},
		},
		Fragment: &wgpu.FragmentState{
			Module: mod, EntryPoint: FragmentEntry,
			Targets: []wgpu.ColorTargetState{{Format: format, Blend: blendState(BlendReplace), WriteMask: wgpu.ColorWriteMaskAll}},
		},
		Primitive:   wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), CullMode: wgpu.CullModeNone},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Release()

	// Full-screen quad (path layout: vert.xy + coverage alpha), green.
	verts := []float32{
		-1, -1, 1,
		1, -1, 1,
		-1, 1, 1,
		1, -1, 1,
		1, 1, 1,
		-1, 1, 1,
	}
	vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	defer vbuf.Release()

	u := PathUniforms{FillColor: [4]float32{0, 1, 0, 1}} // green
	ubuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&u), Usage: wgpu.BufferUsageUniform})
	if err != nil {
		t.Fatalf("uniform buffer: %v", err)
	}
	defer ubuf.Release()
	layout := pipe.GetBindGroupLayout(0)
	defer layout.Release()
	bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout:  layout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: ubuf, Size: uint64(len(uniformBytes(&u)))}},
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

	// Pass 1: clear the whole texture to red (the "retained" previous frame).
	p1 := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 1, G: 0, B: 0, A: 1},
		}},
	})
	p1.End()
	p1.Release()

	// Pass 2: LoadOp=Load (retain red), scissor the region, paint it green.
	p2 := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpLoad, StoreOp: wgpu.StoreOpStore,
		}},
	})
	p2.SetPipeline(pipe)
	p2.SetBindGroup(0, bg, nil)
	p2.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	p2.SetScissorRect(rx, ry, rw, rh)
	p2.Draw(6, 1, 0, 0)
	p2.End()
	p2.Release()

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
	if err := readBuf.TryMapAsync(wgpu.MapModeRead, 0, bytesPerRow*dim, func(s wgpu.MapAsyncStatus) { mapStatus = s }); err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if mapStatus != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", mapStatus)
	}
	data := readBuf.GetMappedRange(0, uint(bytesPerRow*dim))
	defer readBuf.Unmap()

	// Inside the scissored region → repainted green.
	in := (ry+rh/2)*bytesPerRow + (rx+rw/2)*4
	if r, g, b := data[in], data[in+1], data[in+2]; r > 5 || g < 250 || b > 5 {
		t.Fatalf("region pixel = (%d,%d,%d), want green (scissored clear)", r, g, b)
	}
	// Outside the region → retained red from pass 1 (LoadOp=Load preserved it).
	if r, g, b := data[0], data[1], data[2]; r < 250 || g > 5 || b > 5 {
		t.Fatalf("corner pixel = (%d,%d,%d), want retained red (LoadOp=Load)", r, g, b)
	}
}
