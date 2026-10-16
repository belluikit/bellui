//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; headless Chromium grants one,
// but the fork's wgpu binding needs its JS support glue, which wasmbrowsertest
// does not load — TryWriteTexture panics with "wasm is not defined". The wasm
// WebGPU path is verified live via the settings-app build instead
// (docs/webgpu.md); the glue gap is tracked in the issue filed from #79.)

package webgpu

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// baselineGolden is the committed reference render; baselineDim its pixel size.
const (
	baselineDim    = 96
	baselineGolden = "testdata/baseline_scene.png"
)

// TestVisualBaseline renders a fixed multi-primitive scene through the WebGPU
// pipeline offscreen and compares the whole image, within a small tolerance for
// GPU anti-aliasing differences, against a committed golden render. It is a
// per-platform visual baseline (pre-rebrand tracker #204): a shader/pipeline/blend regression on any
// backend (Metal, Vulkan, D3D12) changes solid-fill interiors far beyond the
// tolerance, so it is caught. Edge anti-aliasing differs slightly between GPUs,
// which the per-pixel + total-fraction tolerance absorbs.
//
// Regenerate the golden (e.g. after an intentional rendering change) with:
//
//	BELLUI_WEBGPU_UPDATE_BASELINE=1 go test -tags ... -run TestVisualBaseline ./internal/painter/webgpu/
func TestVisualBaseline(t *testing.T) {
	got := renderBaselineScene(t)

	if os.Getenv("BELLUI_WEBGPU_UPDATE_BASELINE") != "" {
		writePNG(t, baselineGolden, got)
		t.Logf("updated golden %s", baselineGolden)
		return
	}

	want := readPNG(t, baselineGolden)
	if want.Bounds() != got.Bounds() {
		t.Fatalf("size mismatch: golden %v, got %v", want.Bounds(), got.Bounds())
	}

	// Per-channel tolerance and the max fraction of pixels allowed to exceed it.
	const chTol = 8
	const maxBadFrac = 0.05
	bounds := got.Bounds()
	total := bounds.Dx() * bounds.Dy()
	bad := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !pixelsClose(got.RGBAAt(x, y), want.RGBAAt(x, y), chTol) {
				bad++
			}
		}
	}
	frac := float64(bad) / float64(total)
	if frac > maxBadFrac {
		out := writeFailed(t, got)
		t.Fatalf("visual baseline diverged: %d/%d pixels (%.2f%%) exceed ±%d, limit %.0f%%; wrote actual to %s",
			bad, total, frac*100, chTol, maxBadFrac*100, out)
	}
}

func pixelsClose(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol && d(a.A, b.A) <= tol
}

// renderBaselineScene draws the fixed scene into an offscreen texture via the
// `path` pipeline and returns the read-back image. Skips if no adapter is found.
func renderBaselineScene(t *testing.T) *image.RGBA {
	t.Helper()
	const dim = baselineDim
	const bytesPerRow = dim * 4 // 384, 256-aligned multiple not required for read here; we align below

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

	const format = wgpu.TextureFormatRGBA8Unorm
	tex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "baseline",
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
	view, err := tex.TryCreateView(nil)
	if err != nil {
		t.Fatalf("create view: %v", err)
	}
	t.Cleanup(view.Release)

	desc, _ := DescriptorFor("path")
	mod, err := device.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label:      "path.wgsl",
		WGSLSource: &wgpu.ShaderSourceWGSL{Code: string(ShaderSource("path"))},
	})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	t.Cleanup(mod.Release)
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
	t.Cleanup(pipe.Release)

	enc, err := device.TryCreateCommandEncoder(nil)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 0.1, G: 0.1, B: 0.1, A: 1}, // dark gray background
		}},
	})

	// Four opaque colored quadrants plus a white center quad drawn last, all in
	// NDC (clip +y is up; render-to-texture is top-down so +y is the image top).
	quads := []struct {
		x0, y0, x1, y1 float32
		col            [4]float32
	}{
		{-1, 0, 0, 1, [4]float32{1, 0, 0, 1}},          // top-left  red
		{0, 0, 1, 1, [4]float32{0, 1, 0, 1}},           // top-right green
		{-1, -1, 0, 0, [4]float32{0, 0, 1, 1}},         // bot-left  blue
		{0, -1, 1, 0, [4]float32{1, 1, 0, 1}},          // bot-right yellow
		{-0.4, -0.4, 0.4, 0.4, [4]float32{1, 1, 1, 1}}, // center  white
	}
	// Keep buffers/bind groups alive until the pass is submitted.
	var keep []interface{ Release() }
	for _, q := range quads {
		verts := quadVerts(q.x0, q.y0, q.x1, q.y1)
		vbuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: wgpu.ToBytes(verts), Usage: wgpu.BufferUsageVertex})
		if err != nil {
			t.Fatalf("vertex buffer: %v", err)
		}
		u := PathUniforms{FillColor: q.col}
		ubuf, err := device.TryCreateBufferInit(&wgpu.BufferInitDescriptor{Contents: uniformBytes(&u), Usage: wgpu.BufferUsageUniform})
		if err != nil {
			t.Fatalf("uniform buffer: %v", err)
		}
		layout := pipe.GetBindGroupLayout(0)
		bg, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
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
		keep = append(keep, vbuf, ubuf, bg, layout)
	}
	pass.End()
	pass.Release()

	// CopyTextureToBuffer requires bytesPerRow a multiple of 256.
	alignedRow := ((bytesPerRow + 255) / 256) * 256
	readBuf, err := device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Size:  uint64(alignedRow * dim),
		Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
	})
	if err != nil {
		t.Fatalf("readback buffer: %v", err)
	}
	t.Cleanup(readBuf.Release)
	enc.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: tex},
		&wgpu.TexelCopyBufferInfo{Buffer: readBuf, Layout: wgpu.TexelCopyBufferLayout{BytesPerRow: uint32(alignedRow), RowsPerImage: dim}},
		&wgpu.Extent3D{Width: dim, Height: dim, DepthOrArrayLayers: 1},
	)
	cmd, err := enc.TryFinish(nil)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	queue.Submit(cmd)
	cmd.Release()
	enc.Release()
	for _, r := range keep {
		r.Release()
	}

	var status wgpu.MapAsyncStatus
	if err := readBuf.TryMapAsync(wgpu.MapModeRead, 0, uint64(alignedRow*dim), func(s wgpu.MapAsyncStatus) { status = s }); err != nil {
		t.Fatalf("map async: %v", err)
	}
	device.Poll(true, nil)
	if status != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("buffer map status: %v", status)
	}
	data := readBuf.GetMappedRange(0, uint(alignedRow*dim))
	defer readBuf.Unmap()

	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		copy(img.Pix[y*img.Stride:y*img.Stride+dim*4], data[y*alignedRow:y*alignedRow+dim*4])
	}
	return img
}

// quadVerts builds two triangles (6 vertices of vec2 vert + f32 alpha=1) for the
// axis-aligned NDC rectangle (x0,y0)-(x1,y1).
func quadVerts(x0, y0, x1, y1 float32) []float32 {
	return []float32{
		x0, y0, 1, x1, y0, 1, x0, y1, 1,
		x1, y0, 1, x1, y1, 1, x0, y1, 1,
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readPNG(t *testing.T, path string) *image.RGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden %s (regenerate with BELLUI_WEBGPU_UPDATE_BASELINE=1): %v", path, err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	rgba := image.NewRGBA(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}
	return rgba
}

func writeFailed(t *testing.T, img image.Image) string {
	t.Helper()
	out := filepath.Join("testdata", "failed", "baseline_scene.png")
	writePNG(t, out, img)
	return out
}
