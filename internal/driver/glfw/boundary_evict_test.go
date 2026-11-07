//go:build !no_glfw && !ci && !mobile && !wasm

package glfw

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// TestRepaintBoundaryRemovedIsFreed pins #569 on both painters: a boundary
// removed from the window is freed once no paint walk has reached it for
// cache.ValidDuration and a few frames, and a boundary still on screen is
// kept. Before, both painters kept every boundary's target until the painter
// went away.
func TestRepaintBoundaryRemovedIsFreed(t *testing.T) {
	var saved time.Duration
	runOnMain(func() { saved, cache.ValidDuration = cache.ValidDuration, 50*time.Millisecond })
	defer runOnMain(func() { cache.ValidDuration = saved })

	w := createWindow("Boundary eviction")
	defer w.Close()
	w.SetPadded(false)
	kept := container.NewRepaintBoundary(canvas.NewRectangle(color.NRGBA{R: 200, A: 255}))
	gone := container.NewRepaintBoundary(canvas.NewRectangle(color.NRGBA{G: 200, A: 255}))
	box := container.NewVBox(kept, gone)
	w.SetContent(box)
	w.Resize(geo.NewSize(100, 100))
	runOnMain(func() { w.window.Show() })

	cached := func() int {
		var n int
		runOnMain(func() {
			n = w.canvas.Painter().(interface{ CachedBoundaries() int }).CachedBoundaries()
		})
		return n
	}
	begun := func() uint64 {
		var n uint64
		runOnMain(func() {
			n = w.canvas.Painter().(interface{ FramesBegun() uint64 }).FramesBegun()
		})
		return n
	}
	paint := func() { repaintWindow(w) } // paints whether or not the canvas is dirty

	// Eviction counts the frames a painter began, and a window macOS reports
	// occluded, as it does for a moment after showing one, begins none: so
	// paint until a frame begins, not a fixed number of times (#615). A window
	// that stays occluded tells nothing about eviction, and the test skips, as
	// captureOrSkip does (#1063).
	for deadline := time.Now().Add(2 * time.Second); cached() != 2; {
		if time.Now().After(deadline) {
			t.Skip("the window began no frame for 2s (occluded, or no GPU surface)")
		}
		paint()
	}

	runOnMain(func() { box.Remove(gone) })
	start, paints, began := time.Now(), 0, begun()
	for deadline := start.Add(5 * time.Second); cached() != 1 && time.Now().Before(deadline); paints++ {
		paint()
	}
	if cached() != 1 && begun()-began <= keepFrames {
		t.Skipf("the window began %d frames in %d paints, too few to evict (occluded)", begun()-began, paints)
	}

	if !assert.Equal(t, 1, cached(), "the removed boundary is freed, the one on screen kept") {
		// Few frames since it was reached: a walk still reaches it. Many, or
		// as many as before: the window began no frames (#615).
		runOnMain(func() {
			since, held := w.canvas.Painter().(interface {
				BoundaryFramesSinceUse(core.CanvasObject) (uint64, bool)
			}).BoundaryFramesSinceUse(gone)
			t.Logf("after %d paints in %v: removed boundary held=%v, reached %d frames ago, window visible=%v",
				paints, time.Since(start).Round(time.Millisecond), held, since, w.visible)
		})
	}
}

// keepFrames mirrors boundaryKeepFrames, which both painters keep: a boundary
// no walk reaches is evicted only once more frames than this have begun.
const keepFrames = 3
