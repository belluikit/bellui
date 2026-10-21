//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	paint := func() { repaintWindow(w) } // paints whether or not the canvas is dirty
	paint()
	require.Equal(t, 2, cached(), "both boundaries are cached once painted")

	runOnMain(func() { box.Remove(gone) })
	// Eviction counts the frames a painter began, and a window macOS reports
	// occluded, as it does for a moment after showing one, begins none: so
	// paint until the boundary goes, not a fixed number of times (#615).
	start, paints := time.Now(), 0
	for deadline := start.Add(5 * time.Second); cached() != 1 && time.Now().Before(deadline); paints++ {
		paint()
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
