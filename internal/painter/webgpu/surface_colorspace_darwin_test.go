//go:build !gl && darwin && !ios && !wasm

package webgpu

import (
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The window's Metal layer is declared sRGB before wgpu-native sees it, and
// surface configuration and a presented frame leave it so: macOS colour-matches
// Bellui's sRGB output to a wide-gamut display rather than showing it
// oversaturated (#373).
//
// Measured on wgpu-native as pinned at the time (2026-10-26): an untagged
// layer has no colour space before configuration and kCGColorSpaceSRGB after,
// so wgpu-native tags it too. Bellui tags it itself so it does not depend on
// that; the untagged case is only logged.
func TestMetalLayerIsSRGBAfterConfigure(t *testing.T) {
	if instanceBackendsOverride != 0 {
		noGPU(t, "no WebGPU adapter available: %s=1", noAdapterEnv)
	}
	for _, tagged := range []bool{false, true} {
		desc := bareMetalLayer(tagged)
		before := metalLayerColorSpace(desc.MetalLayer.Layer)
		d, err := NewDevice(desc, 64, 64)
		if err != nil {
			noGPU(t, "no WebGPU device available: %v", err)
		}
		require.NoError(t, d.BeginFrame(wgpu.Color{A: 1}))
		require.NoError(t, d.EndFrame())
		after := metalLayerColorSpace(desc.MetalLayer.Layer)
		t.Logf("tagged=%v: layer colour space %q before configure, %q after a frame", tagged, before, after)
		if tagged {
			assert.Equal(t, "kCGColorSpaceSRGB", before)
			assert.Equal(t, "kCGColorSpaceSRGB", after)
		}
		d.Destroy()
	}
}
