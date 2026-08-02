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

// TestBlurSpreadReadback drives the `blur` shader (horizontal pass, fs_main_h) +
// BlurUniforms + premultiplied blend through an offscreen pass, the core of the
// box-shadow / backdrop-blur port (#223). The source is a hard vertical edge:
// the left half opaque red, the right half transparent. After a horizontal blur
// the edge softens — a pixel just right of center (originally transparent) gains
// alpha as red bleeds across, while the far corners keep their original extremes.
// This proves the std140 BlurUniforms layout, the separable-kernel sampling, and
// the Premul blend all agree end-to-end on real hardware.
func TestBlurSpreadReadback(t *testing.T) {
	const dim = 64
	const bytesPerRow = dim * 4 // 256, already aligned

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

	// Source texture: left half opaque red (premultiplied), right half transparent.
	srcTex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "blur-src",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: format, Usage: wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("src texture: %v", err)
	}
	defer srcTex.Release()
	pix := make([]byte, bytesPerRow*dim)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim/2; x++ {
			o := y*bytesPerRow + x*4
			pix[o], pix[o+3] = 255, 255 // opaque red
		}
	}
	queue.WriteTexture(srcTex.AsImageCopy(), pix,
		&wgpu.TexelCopyBufferLayout{BytesPerRow: bytesPerRow, RowsPerImage: dim},
		&wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1})
	srcView, err := srcTex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("src view: %v", err)
	}
	defer srcView.Release()

	sampler, err := device.TryCreateSampler(&wgpu.SamplerDescriptor{
		AddressModeU: wgpu.AddressModeClampToEdge, AddressModeV: wgpu.AddressModeClampToEdge,
		AddressModeW: wgpu.AddressModeClampToEdge, MagFilter: wgpu.FilterModeLinear,
		MinFilter: wgpu.FilterModeLinear, MipmapFilter: wgpu.MipmapFilterModeNearest,
		LodMaxClamp: 1, MaxAnisotropy: 1,
	})
	if err != nil {
		t.Fatalf("sampler: %v", err)
	}
	defer sampler.Release()

	dstTex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "blur-dst",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: format, Usage: wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("dst texture: %v", err)
	}
	defer dstTex.Release()
	dstView, err := dstTex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("dst view: %v", err)
	}
	defer dstView.Release()

	desc, _ := DescriptorFor("blur")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "blur.wgsl", WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("blur"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer mod.Release()
	pipe, err := device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Vertex: wgpu.VertexState{Module: mod, EntryPoint: VertexEntry, Buffers: []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)}},
		Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: BlurFragmentEntryH, Targets: []wgpu.ColorTargetState{{
			Format: format, Blend: blendState(BlendPremul), WriteMask: wgpu.ColorWriteMaskAll,
		}}},
		Primitive:   wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), CullMode: wgpu.CullModeNone},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Release()

	// Full-screen quad (vert.xyz + uv); NDC top maps to v=0 (top-down texture).
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

	u := BlurUniforms{Resolution: [2]float32{dim, dim}, BlurRadius: 8}
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
			{Binding: 1, TextureView: srcView},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		t.Fatalf("bind group: %v", err)
	}
	defer bg.Release()

	readBuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: bytesPerRow * dim, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead})
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
			View: dstView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{},
		}},
	})
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg, nil)
	pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	pass.Draw(4, 1, 0, 0)
	pass.End()
	pass.Release()

	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: dstTex},
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

	var status wgpu.MapAsyncStatus
	if err := readBuf.TryMapAsync(wgpu.MapModeRead, 0, bytesPerRow*dim, func(s wgpu.MapAsyncStatus) { status = s }); err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if status != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", status)
	}
	data := readBuf.GetMappedRange(0, uint(bytesPerRow*dim))
	defer readBuf.Unmap()

	row := (dim / 2) * bytesPerRow // sample a middle row, away from clamp edges
	alphaAt := func(x int) byte { return data[row+x*4+3] }

	// Far left stays (near) opaque, far right stays (near) transparent.
	if a := alphaAt(2); a < 200 {
		t.Errorf("far-left alpha = %d, want ~opaque (>200)", a)
	}
	if a := alphaAt(dim - 3); a > 30 {
		t.Errorf("far-right alpha = %d, want ~transparent (<30)", a)
	}
	// Just right of the edge was transparent (alpha 0); the blur bled red into it.
	if a := alphaAt(dim/2 + 3); a < 20 {
		t.Errorf("just-right-of-edge alpha = %d, want softened (>20, red bled across)", a)
	}
	// Red channel tracks alpha (premultiplied), so it must be non-zero there too.
	if r := data[row+(dim/2+3)*4]; r < 20 {
		t.Errorf("just-right-of-edge red = %d, want non-zero (premultiplied bleed)", r)
	}
}
