//go:build !gl

package webgpu

import (
	"image/color"
	"testing"

	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestOffscreenRenderReadback is a headless, permission-independent proof that
// the WebGPU pipeline actually rasterizes correct pixels: it builds the `path`
// pipeline (the simplest color shader) from the embedded WGSL + the neutral
// descriptors, draws a full-target red triangle into an offscreen texture, then
// copies it back and checks the center pixel is red. This exercises shader
// compilation, pipeline + auto bind-group layout, vertex/uniform plumbing
// (PathUniforms), the render pass, and submission — without a window/surface.
func TestOffscreenRenderReadback(t *testing.T) {
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

	// Build the path pipeline from the embedded shader + neutral descriptor.
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
			Targets: []wgpu.ColorTargetState{{Format: format, Blend: blendState(desc.DefaultBlend), WriteMask: wgpu.ColorWriteMaskAll}},
		},
		Primitive:   wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), CullMode: wgpu.CullModeNone},
		Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Release()

	// Full-target triangle (vert.xy + alpha), opaque.
	verts := []float32{
		-1, -1, 1,
		3, -1, 1,
		-1, 3, 1,
	}
	vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	defer vbuf.Release()

	u := PathUniforms{FillColor: [4]float32{1, 0, 0, 1}} // red
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
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg, nil)
	pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
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
	err = readBuf.TryMapAsync(wgpu.MapModeRead, 0, bytesPerRow*dim, func(s wgpu.MapAsyncStatus) { mapStatus = s })
	if err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if mapStatus != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", mapStatus)
	}
	data := readBuf.GetMappedRange(0, uint(bytesPerRow*dim))
	defer readBuf.Unmap()

	// Center pixel.
	off := (dim/2)*bytesPerRow + (dim/2)*4
	r, g, b, alpha := data[off], data[off+1], data[off+2], data[off+3]
	if r < 250 || g > 5 || b > 5 || alpha < 250 {
		t.Fatalf("center pixel = (%d,%d,%d,%d), want opaque red", r, g, b, alpha)
	}
}

// TestRippleRenderReadback drives the `ripple` shader + RippleUniforms +
// premultiplied blend through an offscreen pass: a full-target rounded-rect clip
// with the ripple centered and large enough to cover the center, so the center
// pixel must be the (opaque red) ripple color while a far corner stays outside
// the ripple radius and remains transparent. Proves the std140 RippleUniforms
// layout, the SDF clip/ripple maths, and the Premul blend all agree end-to-end.
func TestRippleRenderReadback(t *testing.T) {
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

	desc, _ := DescriptorFor("ripple")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "ripple.wgsl",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("ripple"))},
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

	// Full-target quad (vert.xy + normal.xy; vs adds them) — vert 0, normal = NDC.
	verts := []float32{
		0, 0, -1, -1,
		0, 0, 1, -1,
		0, 0, -1, 1,
		0, 0, 1, 1,
	}
	vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	defer vbuf.Release()

	// Rect clip = whole target, ripple centered with radius covering the center
	// but not the corners. Identity inv_transform.
	u := RippleUniforms{
		FrameSize:    [2]float32{dim, dim},
		Radius:       0,
		EdgeSoftness: 1,
		RectCoords:   [4]float32{0, dim, 0, dim},
		RectSizeHalf: [2]float32{dim / 2, dim / 2},
		RippleCenter: [2]float32{dim / 2, dim / 2},
		RippleRadius: dim / 2,
		RippleColor:  [4]float32{1, 0, 0, 1},
		InvTransform: Mat3ToStd140(geo.Identity().ToGL()),
	}
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
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg, nil)
	pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	pass.Draw(4, 1, 0, 0)
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
	err = readBuf.TryMapAsync(wgpu.MapModeRead, 0, bytesPerRow*dim, func(s wgpu.MapAsyncStatus) { mapStatus = s })
	if err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if mapStatus != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", mapStatus)
	}
	data := readBuf.GetMappedRange(0, uint(bytesPerRow*dim))
	defer readBuf.Unmap()

	// Center pixel is inside the ripple → opaque red.
	cOff := (dim/2)*bytesPerRow + (dim/2)*4
	if r, g, b, a := data[cOff], data[cOff+1], data[cOff+2], data[cOff+3]; r < 250 || g > 5 || b > 5 || a < 250 {
		t.Fatalf("center pixel = (%d,%d,%d,%d), want opaque red ripple", r, g, b, a)
	}
	// Top-left corner is outside the ripple radius → untouched (transparent).
	if a := data[0+3]; a > 5 {
		t.Fatalf("corner alpha = %d, want transparent (outside ripple radius)", a)
	}
}

// runStencilReadback renders a center-quarter stencil mask then a full-target
// red fill restricted to that mask, into an offscreen target, and returns the
// center and top-left-corner RGBA bytes. It exercises the depth-stencil
// attachment, the depthStencilState / colorWriteMask pipeline helpers and
// pass.SetStencilReference — the machinery behind #222 stencil clipping and path
// fill — headless. maskMode stamps the mask (color off); fillMode draws color
// only where the stencil matches ref.
func runStencilReadback(t *testing.T, maskMode, fillMode StencilMode, ref uint32) (center, corner [4]byte) {
	t.Helper()
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
	colorTex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "offscreen",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: format, Usage: wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("color texture: %v", err)
	}
	defer colorTex.Release()
	colorView, err := colorTex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("color view: %v", err)
	}
	defer colorView.Release()

	depthTex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "depth-stencil",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: depthStencilFormat, Usage: wgpu.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("depth texture: %v", err)
	}
	defer depthTex.Release()
	depthView, err := depthTex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("depth view: %v", err)
	}
	defer depthView.Release()

	desc, _ := DescriptorFor("path")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "path.wgsl", WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("path"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer mod.Release()

	buildPipe := func(mode StencilMode) *wgpu.RenderPipeline {
		pipe, err := device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
			Vertex: wgpu.VertexState{Module: mod, EntryPoint: VertexEntry, Buffers: []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)}},
			Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: FragmentEntry, Targets: []wgpu.ColorTargetState{{
				Format: format, Blend: blendState(BlendOver), WriteMask: colorWriteMask(mode),
			}}},
			Primitive:    wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), FrontFace: wgpu.FrontFaceCCW, CullMode: wgpu.CullModeNone},
			DepthStencil: depthStencilState(mode, false),
			Multisample:  wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		})
		if err != nil {
			t.Fatalf("pipeline (mode %d): %v", mode, err)
		}
		return pipe
	}
	maskPipe := buildPipe(maskMode)
	defer maskPipe.Release()
	fillPipe := buildPipe(fillMode)
	defer fillPipe.Release()

	// Mask = centered quarter quad (CCW, two triangles); fill = full-target tri.
	maskVerts := []float32{
		-0.5, -0.5, 1, 0.5, -0.5, 1, 0.5, 0.5, 1,
		-0.5, -0.5, 1, 0.5, 0.5, 1, -0.5, 0.5, 1,
	}
	fillVerts := []float32{-1, -1, 1, 3, -1, 1, -1, 3, 1}
	maskVB, _ := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(maskVerts), Usage: wgpu.BufferUsageVertex})
	defer maskVB.Release()
	fillVB, _ := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(fillVerts), Usage: wgpu.BufferUsageVertex})
	defer fillVB.Release()

	maskU := PathUniforms{}
	fillU := PathUniforms{FillColor: [4]float32{1, 0, 0, 1}}
	maskUB, _ := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&maskU), Usage: wgpu.BufferUsageUniform})
	defer maskUB.Release()
	fillUB, _ := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&fillU), Usage: wgpu.BufferUsageUniform})
	defer fillUB.Release()

	bind := func(pipe *wgpu.RenderPipeline, ub *wgpu.Buffer, sz int) *wgpu.BindGroup {
		layout := pipe.GetBindGroupLayout(0)
		defer layout.Release()
		bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: ub, Size: uint64(sz)}}})
		if err != nil {
			t.Fatalf("bind group: %v", err)
		}
		return bg
	}
	maskBG := bind(maskPipe, maskUB, len(uniformBytes(&maskU)))
	defer maskBG.Release()
	fillBG := bind(fillPipe, fillUB, len(uniformBytes(&fillU)))
	defer fillBG.Release()

	readBuf, _ := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: bytesPerRow * dim, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead})
	defer readBuf.Release()

	enc, _ := device.TryCreateCommandEncoder(nil)
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{View: colorView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{}}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View:        depthView,
			DepthLoadOp: wgpu.LoadOpClear, DepthStoreOp: wgpu.StoreOpStore, DepthClearValue: 1,
			StencilLoadOp: wgpu.LoadOpClear, StencilStoreOp: wgpu.StoreOpStore, StencilClearValue: 0,
		},
	})
	pass.SetStencilReference(ref)
	pass.SetPipeline(maskPipe)
	pass.SetBindGroup(0, maskBG, nil)
	pass.SetVertexBuffer(0, maskVB, 0, wgpu.WholeSize)
	pass.Draw(6, 1, 0, 0)
	pass.SetPipeline(fillPipe)
	pass.SetBindGroup(0, fillBG, nil)
	pass.SetVertexBuffer(0, fillVB, 0, wgpu.WholeSize)
	pass.Draw(3, 1, 0, 0)
	pass.End()
	pass.Release()

	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: colorTex},
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

	cOff := (dim/2)*bytesPerRow + (dim/2)*4
	copy(center[:], data[cOff:cOff+4])
	copy(corner[:], data[0:4])
	return center, corner
}

// TestScene3DRenderReadback drives the Scene3D pipeline (depth-enabled scene3d
// shader + Scene3DUniforms + a depth-stencil attachment) through an offscreen
// pass, exercising the device-independent geometry/uniform code (buildScene3DVerts
// + sceneUniforms, incl. the GL→WebGPU depth remap) end-to-end on real hardware.
//
// Two overlapping triangles both cover the screen center: a RED one near the
// camera, emitted FIRST, and a BLUE one far away, emitted SECOND. With correct
// hardware z-buffering the later blue fragment fails the depth test, so the center
// is RED. The red triangle also sits just past the near plane (GL NDC z<0), so a
// red center additionally proves the depth remap kept it inside the clip volume —
// without the remap it would be clipped and the blue (drawn last) would show.
func TestScene3DRenderReadback(t *testing.T) {
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
	colorTex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "offscreen",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: format, Usage: wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("color texture: %v", err)
	}
	defer colorTex.Release()
	colorView, err := colorTex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("color view: %v", err)
	}
	defer colorView.Release()

	depthTex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "depth-stencil",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: depthStencilFormat, Usage: wgpu.TextureUsageRenderAttachment,
	})
	if err != nil {
		t.Fatalf("depth texture: %v", err)
	}
	defer depthTex.Release()
	depthView, err := depthTex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("depth view: %v", err)
	}
	defer depthView.Release()

	desc, _ := DescriptorFor("scene3d")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "scene3d.wgsl", WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("scene3d"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer mod.Release()
	pipe, err := device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Vertex: wgpu.VertexState{Module: mod, EntryPoint: VertexEntry, Buffers: []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)}},
		Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: FragmentEntry, Targets: []wgpu.ColorTargetState{{
			Format: format, Blend: blendState(BlendReplace), WriteMask: wgpu.ColorWriteMaskAll,
		}}},
		Primitive:    wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), FrontFace: wgpu.FrontFaceCCW, CullMode: wgpu.CullModeNone},
		DepthStencil: depthStencilState(StencilNone, true),
		Multisample:  wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Release()

	// Camera at the origin looking down -Z. The red triangle (z=-0.6) is just
	// beyond the near plane (0.5); the blue one (z=-5) is far. Both project large
	// enough to cover the center. Ambient 1 → unlit color (pure red / blue).
	scene := &canvas.Scene3D{
		Camera: canvas.Camera3D{
			Position: canvas.Vec3{X: 0, Y: 0, Z: 0},
			Target:   canvas.Vec3{X: 0, Y: 0, Z: -1},
			Up:       canvas.Vec3{X: 0, Y: 1, Z: 0},
			FOV:      90, Near: 0.5, Far: 1000,
		},
		Lights: []canvas.Light3D{{Direction: canvas.Vec3{X: 0, Y: 0, Z: -1}, Color: color.White, Ambient: 1}},
		Meshes: []canvas.Mesh3D{{
			Vertices: []canvas.Vec3{
				{X: -2, Y: -2, Z: -0.6}, {X: 2, Y: -2, Z: -0.6}, {X: 0, Y: 3, Z: -0.6}, // near
				{X: -2, Y: -2, Z: -5}, {X: 2, Y: -2, Z: -5}, {X: 0, Y: 3, Z: -5}, // far
			},
			Faces: []canvas.Face{{A: 0, B: 1, C: 2}, {A: 3, B: 4, C: 5}}, // near first, far second
			FaceColors: []color.Color{
				color.NRGBA{R: 255, G: 0, B: 0, A: 255},
				color.NRGBA{R: 0, G: 0, B: 255, A: 255},
			},
		}},
	}
	verts := buildScene3DVerts(scene, scene.Camera.Position, false)
	if len(verts) != 60 {
		t.Fatalf("verts = %d floats, want 60 (2 faces)", len(verts))
	}
	vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
	if err != nil {
		t.Fatalf("vertex buffer: %v", err)
	}
	defer vbuf.Release()

	u := sceneUniforms(scene, dim, dim)
	ubuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&u), Usage: wgpu.BufferUsageUniform})
	if err != nil {
		t.Fatalf("uniform buffer: %v", err)
	}
	defer ubuf.Release()

	layout := pipe.GetBindGroupLayout(0)
	defer layout.Release()
	bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: layout, Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: ubuf, Size: uint64(len(uniformBytes(&u)))}},
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
		ColorAttachments: []wgpu.RenderPassColorAttachment{{View: colorView, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{}}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View:        depthView,
			DepthLoadOp: wgpu.LoadOpClear, DepthStoreOp: wgpu.StoreOpStore, DepthClearValue: 1,
			StencilLoadOp: wgpu.LoadOpClear, StencilStoreOp: wgpu.StoreOpStore, StencilClearValue: 0,
		},
	})
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg, nil)
	pass.SetVertexBuffer(0, vbuf, 0, wgpu.WholeSize)
	pass.Draw(uint32(len(verts)/10), 1, 0, 0)
	pass.End()
	pass.Release()

	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: colorTex},
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

	// Center: the near (red) triangle wins the depth test over the later far
	// (blue) one, and survived the near-plane clip — opaque red.
	cOff := (dim/2)*bytesPerRow + (dim/2)*4
	if r, g, b, a := data[cOff], data[cOff+1], data[cOff+2], data[cOff+3]; r < 250 || g > 5 || b > 100 || a < 250 {
		t.Fatalf("center = (%d,%d,%d,%d), want opaque red (depth ordering + remap)", r, g, b, a)
	}
}

// runSceneStencilIsolation reproduces the #231 hazard at the GPU level on a
// single color target shared by three passes — modelling a full-canvas Scene3D
// nested in an active stencil clip:
//
//	A (canvas): stamp a centered-quarter stencil mask (ref 1) into the SHARED
//	            depth-stencil, color off.
//	B (scene):  a same-size off-screen-style pass that clears its depth-stencil
//	            and draws full-target blue with hardware depth — using either a
//	            DEDICATED depth (dedicated=true, the fix) or the SHARED one
//	            (dedicated=false, the bug).
//	C (canvas): reopen the SHARED depth (Load) and draw full-target red only
//	            where stencil == 1.
//
// With a dedicated depth, B never touches the shared stencil, so C's red lands in
// the masked center (corner stays blue). With the shared depth, B's stencil clear
// wipes the mask, so C matches nowhere and the center stays blue. Returns the
// center and corner RGBA.
func runSceneStencilIsolation(t *testing.T, dedicated bool) (center, corner [4]byte) {
	t.Helper()
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
	mkTex := func(label string, f wgpu.TextureFormat, usage wgpu.TextureUsage) (*wgpu.Texture, *wgpu.TextureView) {
		tex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
			Label: label, Size: wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D, Format: f, Usage: usage,
		})
		if err != nil {
			t.Fatalf("%s texture: %v", label, err)
		}
		view, err := tex.TryCreateView(nil)
		if err != nil {
			t.Fatalf("%s view: %v", label, err)
		}
		return tex, view
	}

	colorTex, colorView := mkTex("color", format, wgpu.TextureUsageRenderAttachment|wgpu.TextureUsageCopySrc)
	defer colorTex.Release()
	defer colorView.Release()
	sharedTex, sharedView := mkTex("shared-depth", depthStencilFormat, wgpu.TextureUsageRenderAttachment)
	defer sharedTex.Release()
	defer sharedView.Release()

	sceneView := sharedView // bug: scene aliases the canvas depth-stencil
	if dedicated {
		sceneTex, sv := mkTex("scene-depth", depthStencilFormat, wgpu.TextureUsageRenderAttachment)
		defer sceneTex.Release()
		defer sv.Release()
		sceneView = sv
	}

	desc, _ := DescriptorFor("path")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "path.wgsl", WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("path"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer mod.Release()

	buildPipe := func(mode StencilMode, depth bool) *wgpu.RenderPipeline {
		pipe, err := device.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
			Vertex:       wgpu.VertexState{Module: mod, EntryPoint: VertexEntry, Buffers: []wgpu.VertexBufferLayout{vertexBufferLayout(desc.Vertex)}},
			Fragment:     &wgpu.FragmentState{Module: mod, EntryPoint: FragmentEntry, Targets: []wgpu.ColorTargetState{{Format: format, Blend: blendState(BlendOver), WriteMask: colorWriteMask(mode)}}},
			Primitive:    wgpu.PrimitiveState{Topology: wgpuTopology(desc.Topology), FrontFace: wgpu.FrontFaceCCW, CullMode: wgpu.CullModeNone},
			DepthStencil: depthStencilState(mode, depth),
			Multisample:  wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		})
		if err != nil {
			t.Fatalf("pipeline (mode %d depth %v): %v", mode, depth, err)
		}
		return pipe
	}
	maskPipe := buildPipe(StencilWriteReplace, false) // A: stamp mask, color off
	scenePipe := buildPipe(StencilNone, true)         // B: depth-tested scene draw
	fillPipe := buildPipe(StencilTestEqual, false)    // C: fill where stencil == 1
	defer maskPipe.Release()
	defer scenePipe.Release()
	defer fillPipe.Release()

	maskVerts := []float32{
		-0.5, -0.5, 1, 0.5, -0.5, 1, 0.5, 0.5, 1,
		-0.5, -0.5, 1, 0.5, 0.5, 1, -0.5, 0.5, 1,
	}
	fullVerts := []float32{-1, -1, 1, 3, -1, 1, -1, 3, 1}
	mkVB := func(v []float32) *wgpu.Buffer {
		b, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(v), Usage: wgpu.BufferUsageVertex})
		if err != nil {
			t.Fatalf("vertex buffer: %v", err)
		}
		return b
	}
	maskVB := mkVB(maskVerts)
	fullVB := mkVB(fullVerts)
	defer maskVB.Release()
	defer fullVB.Release()

	mkUB := func(u PathUniforms) *wgpu.Buffer {
		b, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&u), Usage: wgpu.BufferUsageUniform})
		if err != nil {
			t.Fatalf("uniform buffer: %v", err)
		}
		return b
	}
	maskUB := mkUB(PathUniforms{})
	blueUB := mkUB(PathUniforms{FillColor: [4]float32{0, 0, 1, 1}})
	redUB := mkUB(PathUniforms{FillColor: [4]float32{1, 0, 0, 1}})
	defer maskUB.Release()
	defer blueUB.Release()
	defer redUB.Release()

	uSize := len(uniformBytes(&PathUniforms{}))
	mkBG := func(pipe *wgpu.RenderPipeline, ub *wgpu.Buffer) *wgpu.BindGroup {
		layout := pipe.GetBindGroupLayout(0)
		defer layout.Release()
		bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: layout, Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: ub, Size: uint64(uSize)}}})
		if err != nil {
			t.Fatalf("bind group: %v", err)
		}
		return bg
	}
	maskBG := mkBG(maskPipe, maskUB)
	sceneBG := mkBG(scenePipe, blueUB)
	fillBG := mkBG(fillPipe, redUB)
	defer maskBG.Release()
	defer sceneBG.Release()
	defer fillBG.Release()

	readBuf, _ := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: bytesPerRow * dim, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead})
	defer readBuf.Release()

	depthAtt := func(view *wgpu.TextureView, load wgpu.LoadOp) *wgpu.RenderPassDepthStencilAttachment {
		return &wgpu.RenderPassDepthStencilAttachment{
			View:        view,
			DepthLoadOp: load, DepthStoreOp: wgpu.StoreOpStore, DepthClearValue: 1,
			StencilLoadOp: load, StencilStoreOp: wgpu.StoreOpStore, StencilClearValue: 0,
		}
	}
	colorAtt := func(load wgpu.LoadOp) []wgpu.RenderPassColorAttachment {
		return []wgpu.RenderPassColorAttachment{{View: colorView, LoadOp: load, StoreOp: wgpu.StoreOpStore, ClearValue: wgpu.Color{}}}
	}

	enc, _ := device.TryCreateCommandEncoder(nil)

	// Pass A — canvas: stamp the clip mask into the shared depth-stencil.
	passA := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: colorAtt(wgpu.LoadOpClear), DepthStencilAttachment: depthAtt(sharedView, wgpu.LoadOpClear)})
	passA.SetStencilReference(1)
	passA.SetPipeline(maskPipe)
	passA.SetBindGroup(0, maskBG, nil)
	passA.SetVertexBuffer(0, maskVB, 0, wgpu.WholeSize)
	passA.Draw(6, 1, 0, 0)
	passA.End()
	passA.Release()

	// Pass B — scene: clear its depth-stencil and draw full-target blue.
	passB := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: colorAtt(wgpu.LoadOpLoad), DepthStencilAttachment: depthAtt(sceneView, wgpu.LoadOpClear)})
	passB.SetPipeline(scenePipe)
	passB.SetBindGroup(0, sceneBG, nil)
	passB.SetVertexBuffer(0, fullVB, 0, wgpu.WholeSize)
	passB.Draw(3, 1, 0, 0)
	passB.End()
	passB.Release()

	// Pass C — canvas: reopen the shared depth-stencil and fill red where masked.
	passC := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: colorAtt(wgpu.LoadOpLoad), DepthStencilAttachment: depthAtt(sharedView, wgpu.LoadOpLoad)})
	passC.SetStencilReference(1)
	passC.SetPipeline(fillPipe)
	passC.SetBindGroup(0, fillBG, nil)
	passC.SetVertexBuffer(0, fullVB, 0, wgpu.WholeSize)
	passC.Draw(3, 1, 0, 0)
	passC.End()
	passC.Release()

	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: colorTex},
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

	cOff := (dim/2)*bytesPerRow + (dim/2)*4
	copy(center[:], data[cOff:cOff+4])
	copy(corner[:], data[0:4])
	return center, corner
}

// TestSceneDedicatedDepthPreservesStencilClip is the #231 regression: a scene
// pass with its own dedicated depth-stencil leaves an enclosing stencil clip
// intact (center fills red), whereas reusing the shared canvas depth-stencil
// wipes the clip (center stays the scene's blue). The second assertion confirms
// the test genuinely discriminates the hazard the dedicated attachment avoids.
func TestSceneDedicatedDepthPreservesStencilClip(t *testing.T) {
	center, corner := runSceneStencilIsolation(t, true)
	if center[0] < 250 || center[1] > 5 || center[2] > 5 || center[3] < 250 {
		t.Errorf("dedicated: center = %v, want opaque red (clip preserved)", center)
	}
	if corner[2] < 250 || corner[0] > 5 {
		t.Errorf("dedicated: corner = %v, want blue (scene pass, outside clip)", corner)
	}

	bugCenter, _ := runSceneStencilIsolation(t, false)
	if bugCenter[2] < 250 || bugCenter[0] > 5 {
		t.Errorf("shared: center = %v, want blue (clip wiped) — test no longer discriminates the bug", bugCenter)
	}
}

// TestStencilClipReplaceReadback validates the rounded-clip stencil family
// (StencilWriteReplace mask + StencilTestEqual fill, ref=1): only the masked
// center region receives the red fill; the corner stays clear.
func TestStencilClipReplaceReadback(t *testing.T) {
	center, corner := runStencilReadback(t, StencilWriteReplace, StencilTestEqual, 1)
	if center[0] < 250 || center[1] > 5 || center[2] > 5 || center[3] < 250 {
		t.Fatalf("center = %v, want opaque red inside stencil mask", center)
	}
	if corner[3] > 5 {
		t.Fatalf("corner alpha = %d, want transparent (outside stencil mask)", corner[3])
	}
}

// TestStencilFillWindingReadback validates the path-fill stencil family
// (StencilWriteIncrDecr winding mask + StencilTestNotEqual fill, ref=0): the
// wound center region is filled red, the corner (winding 0) stays clear.
func TestStencilFillWindingReadback(t *testing.T) {
	center, corner := runStencilReadback(t, StencilWriteIncrDecr, StencilTestNotEqual, 0)
	if center[0] < 250 || center[1] > 5 || center[2] > 5 || center[3] < 250 {
		t.Fatalf("center = %v, want opaque red inside wound region", center)
	}
	if corner[3] > 5 {
		t.Fatalf("corner alpha = %d, want transparent (winding 0)", corner[3])
	}
}
