//go:build !no_glfw && !ci && !mobile

package glfw

import (
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
