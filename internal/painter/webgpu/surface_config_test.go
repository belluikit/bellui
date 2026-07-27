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
			cfg, err := surfaceConfig(c.caps, 640, 480, "test")
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
	cfg, err := surfaceConfig(caps, 800, 600, "test")
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
