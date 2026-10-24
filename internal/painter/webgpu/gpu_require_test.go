//go:build !gl && !wasm

package webgpu

import (
	"fmt"
	"os"
	"testing"
)

// requireWebGPUEnv names the environment variable a CI leg sets when it is
// expected to have a WebGPU adapter (#449). There, a test that cannot get an
// adapter or a device fails instead of skipping, because a skipped GPU test
// prints the same "ok" as one that ran.
const requireWebGPUEnv = "BELLUI_REQUIRE_WEBGPU"

// noGPU skips the test because no WebGPU adapter or device is available, or
// fails it when BELLUI_REQUIRE_WEBGPU=1 says the environment promised one.
// Local runs without a GPU keep skipping.
func noGPU(tb testing.TB, format string, args ...any) {
	tb.Helper()
	msg := fmt.Sprintf(format, args...)
	if os.Getenv(requireWebGPUEnv) == "1" {
		tb.Fatalf("%s=1 but the GPU test cannot run: %s", requireWebGPUEnv, msg)
	}
	tb.Skipf("%s (set %s=1 to make this a failure)", msg, requireWebGPUEnv)
}
