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

// TestLoadOpRetainsAcrossSubmits is the decisive #238 probe: does LoadOp=Load on a
// render texture retain content written by a PREVIOUS, separately-submitted
// command buffer? Submit 1 clears the texture red and stores; submit 2 opens a
// LoadOp=Load pass that draws nothing and stores; the readback must still be red.
// If it comes back cleared/undefined, the persistent-canvas-FBO retain (the basis
// of the partialrepaint path) cannot work as written.
func TestLoadOpRetainsAcrossSubmits(t *testing.T) {
	const dim = 64
	const bytesPerRow = dim * 4

	instance := wgpu.CreateInstance(chooseInstanceBackends())
	defer instance.Release()
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Skipf("no WebGPU adapter: %v", err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("no WebGPU device: %v", err)
	}
	defer device.Release()
	queue := device.GetQueue()
	defer queue.Release()

	const format = wgpu.TextureFormatRGBA8Unorm
	tex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "retain",
		Size:          wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: wgpu.TextureDimension2D,
		Format: format, Usage: wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("texture: %v", err)
	}
	defer tex.Release()
	view, err := tex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	defer view.Release()

	// Submit 1: clear to red, store.
	enc1, _ := device.TryCreateCommandEncoder(nil)
	p1 := enc1.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 1, G: 0, B: 0, A: 1},
		}},
	})
	p1.End()
	p1.Release()
	cmd1, err := enc1.TryFinish(nil)
	if err != nil {
		t.Fatalf("finish1: %v", err)
	}
	queue.Submit(cmd1)
	cmd1.Release()
	enc1.Release()
	device.Poll(true, nil)

	// Submit 2: LoadOp=Load, draw nothing, store.
	enc2, _ := device.TryCreateCommandEncoder(nil)
	p2 := enc2.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpLoad, StoreOp: wgpu.StoreOpStore,
		}},
	})
	p2.End()
	p2.Release()

	readBuf, _ := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: bytesPerRow * dim, Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead})
	defer readBuf.Release()
	enc2.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: tex},
		&wgpu.TexelCopyBufferInfo{Buffer: readBuf, Layout: wgpu.TexelCopyBufferLayout{BytesPerRow: bytesPerRow, RowsPerImage: dim}},
		&wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
	)
	cmd2, err := enc2.TryFinish(nil)
	if err != nil {
		t.Fatalf("finish2: %v", err)
	}
	queue.Submit(cmd2)
	cmd2.Release()
	enc2.Release()

	var status wgpu.MapAsyncStatus
	if err := readBuf.TryMapAsync(wgpu.MapModeRead, 0, bytesPerRow*dim, func(s wgpu.MapAsyncStatus) { status = s }); err != nil {
		t.Fatalf("map: %v", err)
	}
	device.Poll(true, nil)
	if status != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("map status: %v", status)
	}
	data := readBuf.GetMappedRange(0, uint(bytesPerRow*dim))
	defer readBuf.Unmap()
	off := (dim/2)*bytesPerRow + (dim/2)*4
	r, g, b, a := data[off], data[off+1], data[off+2], data[off+3]
	t.Logf("after LoadOp=Load across submits: center=(%d,%d,%d,%d)", r, g, b, a)
	if r < 250 || g > 5 || b > 5 || a < 250 {
		t.Fatalf("LoadOp=Load did NOT retain across submits: center=(%d,%d,%d,%d), want red — #238 root cause", r, g, b, a)
	}
}
