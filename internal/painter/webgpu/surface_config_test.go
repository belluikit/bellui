//go:build !gl

package webgpu

import (
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// TestSurfaceConfigRejectsDegenerateCaps pins the #20 guard: a surface that
// advertises no formats or no alpha modes cannot present on the adapter, and
// building a configuration from it must report that instead of indexing the
// empty lists. The caller then skips wgpuSurfaceConfigure, whose failures are
// fatal inside wgpu-native (a Rust panic across cgo) and so cannot be recovered
// after the fact.
func TestSurfaceConfigRejectsDegenerateCaps(t *testing.T) {
	cases := []struct {
		name string
		caps wgpu.SurfaceCapabilities
	}{
		{name: "wholly empty", caps: wgpu.SurfaceCapabilities{}},
		{name: "no formats", caps: wgpu.SurfaceCapabilities{
			AlphaModes:   []wgpu.CompositeAlphaMode{wgpu.CompositeAlphaModeOpaque},
			PresentModes: []wgpu.PresentMode{wgpu.PresentModeFifo},
		}},
		{name: "no alpha modes", caps: wgpu.SurfaceCapabilities{
			Formats:      []wgpu.TextureFormat{wgpu.TextureFormatBGRA8Unorm},
			PresentModes: []wgpu.PresentMode{wgpu.PresentModeFifo},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := surfaceConfig(c.caps, 640, 480, 0, "test")
			if err == nil {
				t.Fatalf("surfaceConfig accepted degenerate capabilities, returning %+v", cfg)
			}
			if cfg != nil {
				t.Errorf("config = %+v on error, want nil", cfg)
			}
		})
	}
}

// TestSurfaceConfigFromCaps covers the normal path: the configuration carries
// the size, the capture-enabling usage and the picked format/alpha mode, and
// captureSwap tracks the format's channel order.
func TestSurfaceConfigFromCaps(t *testing.T) {
	caps := wgpu.SurfaceCapabilities{
		// Preferred first, as the surface reports it: an sRGB BGRA whose non-sRGB
		// sibling is advertised, so chooseSurfaceFormat picks the linear one (#322).
		Formats: []wgpu.TextureFormat{
			wgpu.TextureFormatBGRA8UnormSrgb,
			wgpu.TextureFormatBGRA8Unorm,
		},
		AlphaModes:   []wgpu.CompositeAlphaMode{wgpu.CompositeAlphaModeOpaque, wgpu.CompositeAlphaModePremultiplied},
		PresentModes: []wgpu.PresentMode{wgpu.PresentModeFifo},
	}
	cfg, err := surfaceConfig(caps, 800, 600, 0, "test")
	if err != nil {
		t.Fatalf("surfaceConfig: %v", err)
	}
	if cfg.Width != 800 || cfg.Height != 600 {
		t.Errorf("size = %dx%d, want 800x600", cfg.Width, cfg.Height)
	}
	if cfg.Format != wgpu.TextureFormatBGRA8Unorm {
		t.Errorf("format = %v, want the non-sRGB sibling %v", cfg.Format, wgpu.TextureFormatBGRA8Unorm)
	}
	if cfg.AlphaMode != wgpu.CompositeAlphaModeOpaque {
		t.Errorf("alpha mode = %v, want the preferred %v", cfg.AlphaMode, wgpu.CompositeAlphaModeOpaque)
	}
	if cfg.Usage&wgpu.TextureUsageCopySrc == 0 {
		t.Error("usage lacks CopySrc, which Capture needs")
	}
	if cfg.Usage&wgpu.TextureUsageRenderAttachment == 0 {
		t.Error("usage lacks RenderAttachment")
	}
	if !capturePixelsSwapped(cfg.Format) {
		t.Errorf("capturePixelsSwapped(%v) = false, want true for a BGRA surface", cfg.Format)
	}
	if capturePixelsSwapped(wgpu.TextureFormatRGBA8Unorm) {
		t.Error("capturePixelsSwapped reported a swap for an RGBA surface")
	}
}

// TestCapSurfaceSize pins the #437 cap: each dimension is capped at the device
// limit on its own, and a zero limit (none reported) leaves the size alone.
func TestCapSurfaceSize(t *testing.T) {
	for _, c := range []struct {
		w, h, wantW, wantH int
		maxDim             uint32
	}{
		{1920, 52006, 1920, 8192, 8192},
		{60000, 1080, 8192, 1080, 8192},
		{9000, 9000, 8192, 8192, 8192},
		{8192, 8192, 8192, 8192, 8192},
		{1920, 52006, 1920, 52006, 0},
	} {
		w, h := capSurfaceSize(c.w, c.h, c.maxDim)
		if w != c.wantW || h != c.wantH {
			t.Errorf("capSurfaceSize(%d, %d, %d) = %dx%d, want %dx%d", c.w, c.h, c.maxDim, w, h, c.wantW, c.wantH)
		}
	}

	caps := wgpu.SurfaceCapabilities{
		Formats:    []wgpu.TextureFormat{wgpu.TextureFormatBGRA8Unorm},
		AlphaModes: []wgpu.CompositeAlphaMode{wgpu.CompositeAlphaModeOpaque},
	}
	cfg, err := surfaceConfig(caps, 1920, 52006, 8192, "test")
	if err != nil {
		t.Fatalf("surfaceConfig: %v", err)
	}
	if cfg.Width != 1920 || cfg.Height != 8192 {
		t.Errorf("surfaceConfig size = %dx%d, want 1920x8192", cfg.Width, cfg.Height)
	}
}

// TestResizeCapsToDeviceLimit: a resize past the limit lands on the capped size,
// so one already configured at that size does not reconfigure. The surface is
// nil, so a Configure call would panic.
func TestResizeCapsToDeviceLimit(t *testing.T) {
	d := &Device{config: &wgpu.SurfaceConfiguration{Width: 1920, Height: 8192}, maxSurfaceDim: 8192}
	d.Resize(1920, 52006)
	if d.config.Width != 1920 || d.config.Height != 8192 {
		t.Fatalf("config size = %dx%d, want 1920x8192", d.config.Width, d.config.Height)
	}
}
