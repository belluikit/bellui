//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertCanvasSize(t *testing.T, w *safeWindow, size geo.Size) {
	if runtime.GOOS == "linux" {
		// TODO: find the root cause for these problems and solve them without additional repaint
		// fixes issues where the window does not have the correct size
		waitForCanvasSize(t, w, size, false)
	}
	assert.Equal(t, size, w.Canvas().Size())
}

func ensureCanvasSize(t *testing.T, w *safeWindow, size geo.Size) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// TODO: find the root cause for these problems and solve them without additional repaint
		// fixes issues where the window does not have the correct size
		waitForCanvasSize(t, w, size, true)
	}
	require.Equal(t, size, w.Canvas().Size())
}

// requireWebGPUEnv names the environment variable CI sets on its GPU legs
// (#449). There the windowed capture tests fail, rather than skip, when no
// window can be created, its canvas never reaches the asked size, or a capture
// comes back empty. An occluded window still skips: hosted macOS runners
// report one intermittently (#631).
const requireWebGPUEnv = "BELLUI_REQUIRE_WEBGPU"

// noWindowGPU skips the test because the environment cannot give it a real
// window to present and capture, or fails it when BELLUI_REQUIRE_WEBGPU=1
// says the environment promised one. Local runs keep skipping.
func noWindowGPU(t testing.TB, format string, args ...any) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	if os.Getenv(requireWebGPUEnv) == "1" {
		t.Fatalf("%s=1 but the windowed test cannot run: %s", requireWebGPUEnv, msg)
	}
	t.Skipf("%s (set %s=1 to make this a failure)", msg, requireWebGPUEnv)
}

// windowOrSkip stops the test through noWindowGPU when createWindow gave no
// real window.
func windowOrSkip(t testing.TB, w *safeWindow) {
	t.Helper()
	if w == nil || w.window == nil {
		noWindowGPU(t, "no window available (headless / no GPU surface)")
	}
}

// captureOrSkip captures w, retrying while Capture returns nil: macOS begins
// no frame for a window it reports occluded, as it does for a moment after
// showing one (#309, #631). After two seconds it skips, even under
// BELLUI_REQUIRE_WEBGPU, since that is the occlusion case. An empty image
// goes through noWindowGPU.
func captureOrSkip(t *testing.T, w *safeWindow) image.Image {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if img := w.Canvas().Capture(); img != nil {
			if img.Bounds().Empty() {
				noWindowGPU(t, "no capture available (headless / no GPU surface)")
			}
			return img
		}
		if time.Now().After(deadline) {
			t.Skip("Capture returned nil for 2s: the window began no frame (occluded, or no GPU surface)")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func repaintWindow(w *safeWindow) {
	runOnMain(func() {
		w.RunWithContext(func() {
			d.repaintWindow(w.window)
		})
	})

	time.Sleep(time.Millisecond * 150) // wait for the frames to be rendered... o
}

// closeWhenDone closes windows a test created directly (create, viewport.Show)
// when it ends, failed or not, children before parents. Left open, they stay
// on screen until the test binary exits (#589).
func closeWhenDone(t *testing.T, windows ...*window) {
	t.Helper()
	t.Cleanup(func() {
		runOnMain(func() {
			for _, w := range windows {
				if w != nil && w.viewport != nil {
					w.Close()
				}
			}
		})
	})
}

func waitForCanvasSize(t *testing.T, w *safeWindow, size geo.Size, resizeIfNecessary bool) {
	attempts := 0
	for {
		if w.Canvas().Size() == size {
			break
		}
		attempts++
		if !assert.Less(t, attempts, 100, "canvas did not get correct size in time") {
			break
		}
		if resizeIfNecessary && attempts%20 == 0 {
			// sometimes the resize does not seem to reach the actual window at all
			w.Resize(size)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
