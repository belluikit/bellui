//go:build !gl && !wasm

package webgpu

import (
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/belluikit/webgpu/wgpu"
)

// PROBE #803 (not for merge): the GPU cost of D2-A's final pass, which converts
// a gamma-encoded float image to linear (sign-preserving) into a float target,
// at a full-screen size. Headless; time is a blocking drain, so the per-pass
// cost is the difference between 21 and 1 passes per submission.
const finalPassWGSL = `
@group(0) @binding(0) var src: texture_2d<f32>;
@vertex fn vs(@builtin(vertex_index) i: u32) -> @builtin(position) vec4f {
	var p = array(vec2f(-1.0, -3.0), vec2f(-1.0, 1.0), vec2f(3.0, 1.0));
	return vec4f(p[i], 0.0, 1.0);
}
fn eotf(c: f32) -> f32 {
	let a = abs(c);
	var l = a / 12.92;
	if (a > 0.04045) { l = pow((a + 0.055) / 1.055, 2.4); }
	return sign(c) * l;
}
@fragment fn fs(@builtin(position) pos: vec4f) -> @location(0) vec4f {
	let c = textureLoad(src, vec2i(pos.xy), 0);
	return vec4f(eotf(c.r), eotf(c.g), eotf(c.b), c.a);
}`

func TestZZFinalPassCost(t *testing.T) {
	w, h := 2120, 3000
	if v := os.Getenv("PROBE_SIZE"); v != "" {
		w, _ = strconv.Atoi(v[:4])
		h, _ = strconv.Atoi(v[5:])
	}
	inst := wgpu.CreateInstance(chooseInstanceBackends())
	ad, err := inst.RequestAdapter(&wgpu.RequestAdapterOptions{PowerPreference: wgpu.PowerPreferenceHighPerformance})
	if err != nil {
		t.Skip(err)
	}
	info := ad.GetInfo()
	dev, err := ad.RequestDevice(nil)
	if err != nil {
		t.Skip(err)
	}
	q := dev.GetQueue()
	tex := func(f wgpu.TextureFormat, usage wgpu.TextureUsage) *wgpu.TextureView {
		tx, err := dev.TryCreateTexture(&wgpu.TextureDescriptor{
			Size: wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}, MipLevelCount: 1, SampleCount: 1,
			Dimension: wgpu.TextureDimension2D, Format: f, Usage: usage,
		})
		if err != nil {
			t.Fatal(err)
		}
		v, err := tx.TryCreateView(nil)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	src := tex(wgpu.TextureFormatRGBA16Float, wgpu.TextureUsageTextureBinding|wgpu.TextureUsageRenderAttachment)
	mod, err := dev.TryCreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSLSource: &wgpu.ShaderSourceWGSL{Code: finalPassWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	bgl, err := dev.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{Entries: []wgpu.BindGroupLayoutEntry{{
		Binding: 0, Visibility: wgpu.ShaderStageFragment,
		Texture: wgpu.TextureBindingLayout{SampleType: wgpu.TextureSampleTypeUnfilterableFloat, ViewDimension: wgpu.TextureViewDimension2D},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	pl, err := dev.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{BindGroupLayouts: []*wgpu.BindGroupLayout{bgl}})
	if err != nil {
		t.Fatal(err)
	}
	bg, err := dev.TryCreateBindGroup(&wgpu.BindGroupDescriptor{Layout: bgl, Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: src}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []wgpu.TextureFormat{wgpu.TextureFormatRGBA16Float, wgpu.TextureFormatRGBA8Unorm} {
		dst := tex(f, wgpu.TextureUsageRenderAttachment)
		pipe, err := dev.TryCreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
			Layout: pl,
			Vertex: wgpu.VertexState{Module: mod, EntryPoint: "vs"},
			Fragment: &wgpu.FragmentState{Module: mod, EntryPoint: "fs", Targets: []wgpu.ColorTargetState{{
				Format: f, WriteMask: wgpu.ColorWriteMaskAll,
			}}},
			Primitive:   wgpu.PrimitiveState{Topology: wgpu.PrimitiveTopologyTriangleList, FrontFace: wgpu.FrontFaceCCW, CullMode: wgpu.CullModeNone},
			Multisample: wgpu.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		})
		if err != nil {
			t.Fatal(err)
		}
		run := func(passes int) time.Duration {
			enc, err := dev.TryCreateCommandEncoder(nil)
			if err != nil {
				t.Fatal(err)
			}
			for range passes {
				p := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{
					View: dst, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore,
				}}})
				p.SetPipeline(pipe)
				p.SetBindGroup(0, bg, nil)
				p.Draw(3, 1, 0, 0)
				p.End()
			}
			cmd, err := enc.TryFinish(nil)
			if err != nil {
				t.Fatal(err)
			}
			t0 := time.Now()
			q.Submit(cmd)
			dev.Poll(true, nil)
			return time.Since(t0)
		}
		for range 5 {
			run(21)
		}
		var per []float64
		for range 15 {
			one, many := run(1), run(21)
			per = append(per, float64(many-one)/20/float64(time.Microsecond))
		}
		sort.Float64s(per)
		t.Logf("PROBE803 %s (%v) %dx%d float source -> %v target: final pass median %.0f us (min %.0f, max %.0f)",
			info.Device, info.BackendType, w, h, f, per[len(per)/2], per[0], per[len(per)-1])
	}
}
