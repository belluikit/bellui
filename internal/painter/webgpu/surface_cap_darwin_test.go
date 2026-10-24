//go:build !gl && darwin && !ios && !wasm

package webgpu

import (
	"os"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSurfaceBeyondDeviceLimitIsCapped configures a real Metal surface larger
// than the device's maxTextureDimension2D, as a window whose content asked for
// 52006 px did (#437). wgpuSurfaceConfigure rejects that size, and the rejection
// is fatal inside wgpu-native: before the cap this test aborted the binary
// instead of failing.
func TestSurfaceBeyondDeviceLimitIsCapped(t *testing.T) {
	// A Metal surface on an instance without the Metal backend makes
	// wgpuInstanceCreateSurface panic inside wgpu-native, which aborts the
	// test binary rather than returning an error NewDevice could report. So
	// an instance narrowed to other backends has no adapter for this test.
	if desc := chooseInstanceBackends(); desc != nil && desc.Backends != 0 && desc.Backends&instanceBackendMetal == 0 {
		noGPU(t, "no WebGPU adapter available: BELLUI_WEBGPU_BACKENDS=%q leaves out metal", os.Getenv("BELLUI_WEBGPU_BACKENDS"))
	}
	d, err := NewDevice(bareMetalLayerSurfaceDescriptor(), 1920, 52006)
	if err != nil {
		noGPU(t, "no WebGPU device available: %v", err)
	}
	defer d.Destroy()
	limit := d.maxSurfaceDim
	require.NotZero(t, limit, "the device reports no maxTextureDimension2D")
	t.Logf("device maxTextureDimension2D = %d", limit)

	assert.Equal(t, uint32(1920), d.config.Width)
	assert.Equal(t, limit, d.config.Height)

	// A resize past the limit in the other dimension is capped too.
	d.Resize(60000, 1080)
	assert.Equal(t, limit, d.config.Width)
	assert.Equal(t, uint32(1080), d.config.Height)

	// And a frame renders and presents on the capped surface.
	d.Resize(1920, int(limit)+1)
	require.NoError(t, d.BeginFrame(wgpu.Color{A: 1}))
	require.NoError(t, d.EndFrame())
}
