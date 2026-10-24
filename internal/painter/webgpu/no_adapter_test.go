//go:build !gl && !wasm

package webgpu

import (
	"os"
	"runtime"

	"github.com/belluikit/webgpu/wgpu"
)

// noAdapterEnv names the test-only switch that simulates a machine with no
// usable WebGPU adapter: BELLUI_WEBGPU_TEST_NO_ADAPTER=1 go test ./internal/painter/webgpu/
// The GPU tests then take their no-adapter path. Since #731,
// BELLUI_WEBGPU_BACKENDS always keeps the native backend, so a backend list
// can no longer do this.
const noAdapterEnv = "BELLUI_WEBGPU_TEST_NO_ADAPTER"

func init() {
	if os.Getenv(noAdapterEnv) == "1" {
		instanceBackendsOverride = foreignInstanceBackend(runtime.GOOS)
	}
}

// foreignInstanceBackend is a backend goos has no driver for, so an instance
// limited to it finds no adapter. A surface needs the native backend, so a
// test that creates one must skip under the switch rather than call
// CreateSurface (see TestSurfaceBeyondDeviceLimitIsCapped).
func foreignInstanceBackend(goos string) wgpu.InstanceBackend {
	if goos == "darwin" || goos == "ios" {
		return instanceBackendDX12
	}
	return instanceBackendMetal
}
