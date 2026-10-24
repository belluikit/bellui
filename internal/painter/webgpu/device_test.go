//go:build !gl

package webgpu

import (
	"bytes"
	"log"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// These exercise the device-independent translation from the neutral descriptors
// to wgpu descriptor structs. They need the binding (hence the build tag) but no
// GPU device, so they run under `go test -tags webgpu`.

// TestResolveScissor guards the empty-clip handling behind the pre-rebrand tracker #333 fix: an empty
// clip (in particular a clip-owner that does not intersect the partial-repaint
// dirty region) must resolve to a zero-area scissor that culls every draw, not a
// no-op that leaves the previous scissor — which let clipped content leak through.
func TestResolveScissor(t *testing.T) {
	const tw, th = 200, 100
	cases := []struct {
		name           string
		x, y, w, h     int
		hasTarget      bool
		wantOK         bool
		rx, ry, rw, rh int
	}{
		{name: "normal in-bounds", x: 10, y: 20, w: 50, h: 30, hasTarget: true, wantOK: true, rx: 10, ry: 20, rw: 50, rh: 30},
		{name: "clamped to target", x: 180, y: 90, w: 100, h: 100, hasTarget: true, wantOK: true, rx: 180, ry: 90, rw: 20, rh: 10},
		// The pre-rebrand tracker #333 case: a clip that starts below the target / dirty band clamps to
		// empty and must still apply a zero-area scissor (culls all draws), pinned
		// inside the target so WebGPU accepts it.
		{name: "empty below target pins to edge", x: 10, y: 120, w: 50, h: 30, hasTarget: true, wantOK: true, rx: 10, ry: th, rw: 0, rh: 0},
		{name: "zero size pins in place", x: 10, y: 20, w: 0, h: 0, hasTarget: true, wantOK: true, rx: 10, ry: 20, rw: 0, rh: 0},
		{name: "empty past right edge", x: 250, y: 10, w: 10, h: 10, hasTarget: true, wantOK: true, rx: tw, ry: 10, rw: 0, rh: 0},
		// No target to validate against: keep the historical no-op for empty input.
		{name: "empty without target is no-op", x: 10, y: 20, w: 0, h: 5, hasTarget: false, wantOK: false},
		{name: "non-empty without target passes through", x: 10, y: 20, w: 5, h: 5, hasTarget: false, wantOK: true, rx: 10, ry: 20, rw: 5, rh: 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rx, ry, rw, rh, ok := resolveScissor(c.x, c.y, c.w, c.h, tw, th, c.hasTarget)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if rw < 0 || rh < 0 {
				t.Fatalf("negative scissor size %dx%d would crash WebGPU", rw, rh)
			}
			if rx < 0 || ry < 0 || rx+rw > tw || ry+rh > th {
				t.Fatalf("scissor (%d,%d,%d,%d) escapes target %dx%d", rx, ry, rw, rh, tw, th)
			}
			if rx != c.rx || ry != c.ry || rw != c.rw || rh != c.rh {
				t.Errorf("got (%d,%d,%d,%d), want (%d,%d,%d,%d)", rx, ry, rw, rh, c.rx, c.ry, c.rw, c.rh)
			}
		})
	}
}

func TestWGPUVertexFormat(t *testing.T) {
	cases := map[VertexFormat]wgpu.VertexFormat{
		Float32:   wgpu.VertexFormatFloat32,
		Float32x2: wgpu.VertexFormatFloat32x2,
		Float32x3: wgpu.VertexFormatFloat32x3,
		Float32x4: wgpu.VertexFormatFloat32x4,
	}
	for in, want := range cases {
		if got := wgpuVertexFormat(in); got != want {
			t.Errorf("wgpuVertexFormat(%d) = %v, want %v", in, got, want)
		}
	}
}

func TestBlendStateMapping(t *testing.T) {
	if blendState(BlendReplace) != nil {
		t.Error("BlendReplace should map to nil (no blend)")
	}

	over := blendState(BlendOver)
	if over == nil {
		t.Fatal("BlendOver should not be nil")
	}
	if over.Color.SrcFactor != wgpu.BlendFactorSrcAlpha || over.Color.DstFactor != wgpu.BlendFactorOneMinusSrcAlpha {
		t.Errorf("BlendOver color = %+v, want SrcAlpha/OneMinusSrcAlpha", over.Color)
	}

	premul := blendState(BlendPremul)
	if premul == nil {
		t.Fatal("BlendPremul should not be nil")
	}
	if premul.Color.SrcFactor != wgpu.BlendFactorOne || premul.Color.DstFactor != wgpu.BlendFactorOneMinusSrcAlpha {
		t.Errorf("BlendPremul color = %+v, want One/OneMinusSrcAlpha", premul.Color)
	}
}

func TestVertexBufferLayoutTranslation(t *testing.T) {
	d, ok := DescriptorFor("scene3d")
	if !ok {
		t.Fatal("missing scene3d descriptor")
	}
	l := vertexBufferLayout(d.Vertex)
	if l.ArrayStride != 40 {
		t.Errorf("scene3d stride = %d, want 40", l.ArrayStride)
	}
	if len(l.Attributes) != 3 {
		t.Fatalf("scene3d attrs = %d, want 3", len(l.Attributes))
	}
	if l.Attributes[2].Format != wgpu.VertexFormatFloat32x4 || l.Attributes[2].Offset != 24 {
		t.Errorf("scene3d color attr = %+v, want Float32x4 @24", l.Attributes[2])
	}
}

// TestUniformBytes confirms the raw-bytes view has the struct's std140 size.
func TestUniformBytes(t *testing.T) {
	var u RoundRectangleUniforms
	if got := len(uniformBytes(&u)); got != 144 {
		t.Errorf("uniformBytes(RoundRectangleUniforms) len = %d, want 144", got)
	}
}

// TestResizeSkipsUnchangedSize guards the pre-rebrand tracker #262 fix: the mobile paint loop calls
// SetOutputSize (→ Resize) every frame, so Resize must not reconfigure the
// surface when the pixel size is unchanged (or non-positive). A reconfigure
// re-reads the CAMetalLayer view dimensions off the render goroutine on iOS,
// flooding the log. Here the surface is nil, so any Configure call would panic —
// the test passes only because the unchanged/invalid sizes early-return.
func TestResizeSkipsUnchangedSize(t *testing.T) {
	d := &Device{config: &wgpu.SurfaceConfiguration{Width: 200, Height: 100}}

	d.Resize(200, 100) // unchanged: must not touch the (nil) surface
	d.Resize(0, 0)     // non-positive: must not touch the surface
	d.Resize(-1, 50)   // non-positive: must not touch the surface

	if d.config.Width != 200 || d.config.Height != 100 {
		t.Fatalf("config size = %dx%d, want 200x100 (no-op Resize must not mutate)", d.config.Width, d.config.Height)
	}
}

// TestChooseSurfaceFormat guards the pre-rebrand tracker #322 fix: the surface must be configured
// with a NON-sRGB format so the painter's straight sRGB color writes are not
// re-encoded (over-brightened) by the GPU, keeping parity with the GL/software
// backends. macOS Metal reports the sRGB BGRA variant first, so the chooser must
// drop to its non-sRGB sibling when the surface advertises it.
func TestChooseSurfaceFormat(t *testing.T) {
	cases := []struct {
		name    string
		formats []wgpu.TextureFormat
		want    wgpu.TextureFormat
	}{
		{
			name:    "metal: sRGB BGRA preferred, non-sRGB sibling available",
			formats: []wgpu.TextureFormat{wgpu.TextureFormatBGRA8UnormSrgb, wgpu.TextureFormatBGRA8Unorm},
			want:    wgpu.TextureFormatBGRA8Unorm,
		},
		{
			name:    "wasm: non-sRGB BGRA already preferred",
			formats: []wgpu.TextureFormat{wgpu.TextureFormatBGRA8Unorm, wgpu.TextureFormatRGBA8Unorm},
			want:    wgpu.TextureFormatBGRA8Unorm,
		},
		{
			name:    "sRGB RGBA preferred, non-sRGB sibling available",
			formats: []wgpu.TextureFormat{wgpu.TextureFormatRGBA8UnormSrgb, wgpu.TextureFormatRGBA8Unorm},
			want:    wgpu.TextureFormatRGBA8Unorm,
		},
		{
			name:    "sibling not advertised: keep preferred rather than fail configure",
			formats: []wgpu.TextureFormat{wgpu.TextureFormatBGRA8UnormSrgb, wgpu.TextureFormatRGBA16Float},
			want:    wgpu.TextureFormatBGRA8UnormSrgb,
		},
		{
			name:    "empty caps: safe default",
			formats: nil,
			want:    wgpu.TextureFormatBGRA8Unorm,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := chooseSurfaceFormat(c.formats); got != c.want {
				t.Fatalf("chooseSurfaceFormat(%v) = %v, want %v", c.formats, got, c.want)
			}
		})
	}
}

// TestChooseSurfaceFormat_SRGBOnlyWarns guards pre-rebrand tracker #331: when the surface advertises
// only sRGB formats (no non-sRGB sibling), chooseSurfaceFormat keeps the sRGB
// format (over-bright risk) and must emit a one-time unconditional warning — not
// gated behind BELLUI_WEBGPU_DEBUG — so a silent color regression is visible.
func TestChooseSurfaceFormat_SRGBOnlyWarns(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })

	// Reset the once so this test observes the warning regardless of order.
	srgbOnlyWarnOnce = sync.Once{}

	srgbOnly := []wgpu.TextureFormat{wgpu.TextureFormatBGRA8UnormSrgb, wgpu.TextureFormatRGBA16Float}
	if got := chooseSurfaceFormat(srgbOnly); got != wgpu.TextureFormatBGRA8UnormSrgb {
		t.Fatalf("got %v, want sRGB preferred kept", got)
	}
	// Second call must not log again (one-time).
	_ = chooseSurfaceFormat(srgbOnly)

	out := buf.String()
	if !strings.Contains(out, "only sRGB") || !strings.Contains(out, "#331") {
		t.Fatalf("expected one-time sRGB-only warning, got: %q", out)
	}
	if strings.Count(out, "only sRGB") != 1 {
		t.Fatalf("warning should fire exactly once, got %d: %q", strings.Count(out, "only sRGB"), out)
	}

	// A surface that offers a non-sRGB sibling must NOT warn.
	buf.Reset()
	srgbOnlyWarnOnce = sync.Once{}
	_ = chooseSurfaceFormat([]wgpu.TextureFormat{wgpu.TextureFormatBGRA8UnormSrgb, wgpu.TextureFormatBGRA8Unorm})
	if strings.Contains(buf.String(), "only sRGB") {
		t.Fatalf("should not warn when a non-sRGB sibling is available, got: %q", buf.String())
	}
}

// TestChooseInstanceBackends covers the BELLUI_WEBGPU_BACKENDS parsing that lets a
// virtual-GPU environment skip a crashing backend (e.g. the WoA GL segfault) by
// pinning to DX12 up-front (pre-rebrand tracker #612), on each platform: a list
// that leaves out the native backend gets it added, with a warning, since
// without it the window cannot open (#731). Pure, so it runs in normal CI with
// no GPU.
func TestChooseInstanceBackends(t *testing.T) {
	const (
		vk    = instanceBackendVulkan
		gl    = instanceBackendGL
		metal = instanceBackendMetal
		dx12  = instanceBackendDX12
	)
	cases := []struct {
		goos  string
		spec  string
		want  wgpu.InstanceBackend
		nil_  bool   // expect a nil descriptor (all backends)
		added string // the native backend the warning must name; "" = no warning
	}{
		// Unset and "all" are untouched everywhere.
		{goos: "darwin", spec: "", nil_: true},
		{goos: "windows", spec: "", nil_: true},
		{goos: "linux", spec: "", nil_: true},
		{goos: "darwin", spec: "all", want: 0},
		{goos: "windows", spec: "all", want: 0},
		{goos: "linux", spec: "all", want: 0},
		{goos: "linux", spec: "bogus", want: 0}, // nothing parsed: All

		// Native already listed: unchanged, no warning.
		{goos: "windows", spec: "dx12", want: dx12},
		{goos: "windows", spec: "d3d12", want: dx12},
		{goos: "windows", spec: " DX12 ", want: dx12},
		{goos: "windows", spec: "vulkan|dx12", want: vk | dx12},
		{goos: "windows", spec: "bogus|dx12", want: dx12},
		{goos: "darwin", spec: "metal", want: metal},
		{goos: "linux", spec: "vulkan", want: vk},
		{goos: "linux", spec: "gl|vulkan", want: gl | vk},
		{goos: "darwin", spec: "primary", want: instanceBackendPrimary},
		{goos: "windows", spec: "primary", want: instanceBackendPrimary},
		{goos: "linux", spec: "primary", want: instanceBackendPrimary},

		// Native left out: added, and the warning names it.
		{goos: "darwin", spec: "gl", want: gl | metal, added: "metal"},
		{goos: "darwin", spec: "vulkan", want: vk | metal, added: "metal"},
		{goos: "darwin", spec: "dx12", want: dx12 | metal, added: "metal"},
		{goos: "ios", spec: "gl", want: gl | metal, added: "metal"},
		{goos: "windows", spec: "gl", want: gl | dx12, added: "dx12"},
		{goos: "windows", spec: "vulkan", want: vk | dx12, added: "dx12"},
		{goos: "windows", spec: "metal", want: metal | dx12, added: "dx12"},
		{goos: "linux", spec: "gl", want: gl | vk, added: "vulkan"},
		{goos: "linux", spec: "dx12", want: dx12 | vk, added: "vulkan"},
		{goos: "linux", spec: "metal", want: metal | vk, added: "vulkan"},
		{goos: "linux", spec: "secondary", want: gl | vk, added: "vulkan"},
		{goos: "android", spec: "gl", want: gl | vk, added: "vulkan"},
		{goos: "freebsd", spec: "gl", want: gl | vk, added: "vulkan"},

		// The web has no native backend to insist on.
		{goos: "js", spec: "gl", want: gl},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.spec, func(t *testing.T) {
			var logs []string
			got := instanceBackendsFor(tc.spec, tc.goos, func(msg string) { logs = append(logs, msg) })
			if tc.nil_ {
				if got != nil {
					t.Fatalf("expected nil descriptor, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected descriptor, got nil")
			}
			if got.Backends != tc.want {
				t.Fatalf("backends = 0x%x, want 0x%x", uint32(got.Backends), uint32(tc.want))
			}
			var warned []string
			for _, l := range logs {
				if strings.Contains(l, "leaves out") {
					warned = append(warned, l)
				}
			}
			if tc.added == "" {
				if len(warned) != 0 {
					t.Fatalf("unexpected warning: %q", warned)
				}
				return
			}
			if len(warned) != 1 || !strings.Contains(warned[0], "leaves out "+tc.added+",") {
				t.Fatalf("want one warning naming %s, got %q", tc.added, warned)
			}
		})
	}
}

// TestChooseInstanceBackendsEnv covers the wrapper: it reads
// BELLUI_WEBGPU_BACKENDS for this platform, and the test-only override wins.
func TestChooseInstanceBackendsEnv(t *testing.T) {
	saved := instanceBackendsOverride
	t.Cleanup(func() { instanceBackendsOverride = saved })
	instanceBackendsOverride = 0 // as in an app, even under BELLUI_WEBGPU_TEST_NO_ADAPTER

	native := nativeInstanceBackend(runtime.GOOS)
	t.Setenv("BELLUI_WEBGPU_BACKENDS", "gl")
	if got := chooseInstanceBackends(); got == nil || got.Backends != instanceBackendGL|native {
		t.Fatalf("gl on %s: got %+v, want 0x%x", runtime.GOOS, got, uint32(instanceBackendGL|native))
	}

	instanceBackendsOverride = foreignInstanceBackend(runtime.GOOS)
	if got := chooseInstanceBackends(); got == nil || got.Backends != instanceBackendsOverride {
		t.Fatalf("override: got %+v, want 0x%x", got, uint32(instanceBackendsOverride))
	}
	if got := chooseInstanceBackends().Backends; got&native != 0 {
		t.Fatalf("override 0x%x includes the native backend 0x%x, so it would find an adapter", uint32(got), uint32(native))
	}
}
