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

// These benchmarks isolate the per-draw allocation tradeoff the draw-slot pool
// addresses (#204): the old Record allocated a fresh vertex buffer, uniform
// buffer and bind group on every draw and released them after each frame;
// the pool keeps them and refreshes contents via queue.WriteBuffer. Run with:
//
//	CGO_LDFLAGS='' CGO_CFLAGS='' GOWORK=off \
//	  go test -run x -bench BenchmarkDrawBuffer -benchmem ./internal/painter/webgpu/
//
// They need a real WebGPU adapter (Metal/Vulkan/D3D12) and skip without one.
//
// BenchmarkWriteBufferPerDraw vs BenchmarkWriteBufferArena isolate the #255
// arena win on top of the pool: the pool still issues two WriteBuffer cgo calls
// per draw (vertex + uniform), so a frame with N draws makes ~2N calls; the
// arena accumulates every draw into two staging slices and flushes each with a
// single WriteBuffer, so the same frame makes ~2 calls. CAVEAT: these run
// without a per-frame Submit, so wgpu-native's staging belt never recycles and
// the absolute per-call nanoseconds are inflated — the meaningful signal is the
// call-count collapse (N writes -> 1) and the lower allocation/CPU per frame,
// not the absolute time. A true frame-level benchmark needs a window/surface (so
// BeginFrame can acquire a swapchain texture and Submit recycles the belt), which
// is not available headlessly; the live arena path is verified on a real window.

func benchRawDevice(b *testing.B) (*wgpu.Device, *wgpu.Queue) {
	b.Helper()
	instance := wgpu.CreateInstance(chooseInstanceBackends())
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		instance.Release()
		b.Skipf("no WebGPU adapter available: %v", err)
	}
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		adapter.Release()
		instance.Release()
		b.Skipf("no WebGPU device available: %v", err)
	}
	queue := device.GetQueue()
	b.Cleanup(func() {
		queue.Release()
		device.Release()
		adapter.Release()
		instance.Release()
	})
	return device, queue
}

// Representative per-draw payload: a textured quad (4 verts × 6 floats) and a
// typical std140 uniform block.
var (
	benchVerts   = make([]float32, 24)
	benchUniform = make([]byte, 96)
)

// BenchmarkDrawBufferAllocPerDraw measures the old behaviour: create + release a
// vertex and a uniform buffer for every draw.
func BenchmarkDrawBufferAllocPerDraw(b *testing.B) {
	device, _ := benchRawDevice(b)
	vb := wgpu.ToBytes(benchVerts)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: vb, Usage: wgpu.BufferUsageVertex})
		if err != nil {
			b.Fatal(err)
		}
		ubuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: benchUniform, Usage: wgpu.BufferUsageUniform})
		if err != nil {
			vbuf.Release()
			b.Fatal(err)
		}
		vbuf.Release()
		ubuf.Release()
	}
}

// BenchmarkBindGroupCreatePerDraw measures the other half of the old per-draw
// cost the pool removes: fetching the pipeline bind-group layout and building a
// fresh bind group every draw. In steady state the pool caches both, so this
// entire cost goes to ~zero for an unchanged draw.
func BenchmarkBindGroupCreatePerDraw(b *testing.B) {
	device, _ := benchRawDevice(b)
	bgl, err := device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
			Buffer:     wgpu.BufferBindingLayout{Type: wgpu.BufferBindingTypeUniform},
		}},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer bgl.Release()
	ubuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: 256, Usage: wgpu.BufferUsageUniform})
	if err != nil {
		b.Fatal(err)
	}
	defer ubuf.Release()
	entries := []wgpu.BindGroupEntry{{Binding: 0, Buffer: ubuf, Offset: 0, Size: 256}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: bgl, Entries: entries})
		if err != nil {
			b.Fatal(err)
		}
		bg.Release()
	}
}

// BenchmarkDrawBufferReuse measures the pooled behaviour: write into persistent
// CopyDst buffers, the steady state after the first frame warms the pool.
func BenchmarkDrawBufferReuse(b *testing.B) {
	device, queue := benchRawDevice(b)
	vb := wgpu.ToBytes(benchVerts)
	vbuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(roundUpBuffer(len(vb))), Usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst})
	if err != nil {
		b.Fatal(err)
	}
	defer vbuf.Release()
	ubuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(roundUpBuffer(len(benchUniform))), Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst})
	if err != nil {
		b.Fatal(err)
	}
	defer ubuf.Release()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.WriteBuffer(vbuf, 0, vb)
		queue.WriteBuffer(ubuf, 0, benchUniform)
	}
}

// drawsPerFrame is the number of draws each frame-level benchmark iteration
// simulates, so the per-draw vs per-frame WriteBuffer call counts differ.
const drawsPerFrame = 200

// BenchmarkWriteBufferPerDraw is the slot pool's behaviour: two WriteBuffer
// calls per draw, so 2*drawsPerFrame cgo calls per simulated frame.
func BenchmarkWriteBufferPerDraw(b *testing.B) {
	device, queue := benchRawDevice(b)
	vb := wgpu.ToBytes(benchVerts)
	vbuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(roundUpBuffer(len(vb))), Usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst})
	if err != nil {
		b.Fatal(err)
	}
	defer vbuf.Release()
	ubuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(roundUpBuffer(uniformBlockStride)), Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst})
	if err != nil {
		b.Fatal(err)
	}
	defer ubuf.Release()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for d := 0; d < drawsPerFrame; d++ {
			queue.WriteBuffer(vbuf, 0, vb)
			queue.WriteBuffer(ubuf, 0, benchUniform)
		}
	}
}

// BenchmarkWriteBufferArena is the #255 arena's behaviour: every draw's data is
// accumulated into two CPU staging slices (cheap appends) and each whole arena
// is flushed with a single WriteBuffer per frame — two cgo calls per frame
// regardless of draw count.
func BenchmarkWriteBufferArena(b *testing.B) {
	device, queue := benchRawDevice(b)
	vb := wgpu.ToBytes(benchVerts)
	vArena := make([]byte, 0, drawsPerFrame*len(vb))
	uArena := make([]byte, 0, drawsPerFrame*uniformBlockStride)
	vbuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(roundUpBuffer(drawsPerFrame * len(vb))), Usage: wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst})
	if err != nil {
		b.Fatal(err)
	}
	defer vbuf.Release()
	ubuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{Size: uint64(roundUpBuffer(drawsPerFrame * uniformBlockStride)), Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst})
	if err != nil {
		b.Fatal(err)
	}
	defer ubuf.Release()
	pad := make([]byte, uniformBlockStride-len(benchUniform))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vArena = vArena[:0]
		uArena = uArena[:0]
		for d := 0; d < drawsPerFrame; d++ {
			vArena = append(vArena, vb...)
			uArena = append(uArena, benchUniform...)
			uArena = append(uArena, pad...)
		}
		queue.WriteBuffer(vbuf, 0, vArena)
		queue.WriteBuffer(ubuf, 0, uArena)
	}
}
