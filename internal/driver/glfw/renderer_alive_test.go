//go:build !no_glfw && !ci && !mobile && !wasm

package glfw

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// statefulWidget's renderer counts the times it is destroyed, as a renderer
// holding a decoder or a stream would notice.
type statefulWidget struct {
	widget.BaseWidget

	destroyed int
}

func newStatefulWidget() *statefulWidget {
	w := &statefulWidget{}
	w.ExtendBaseWidget(w)
	return w
}

func (w *statefulWidget) CreateRenderer() core.WidgetRenderer {
	return &statefulRenderer{w: w, rect: canvas.NewRectangle(nil)}
}

type statefulRenderer struct {
	w    *statefulWidget
	rect *canvas.Rectangle
}

func (r *statefulRenderer) Destroy()                     { r.w.destroyed++ }
func (r *statefulRenderer) Layout(size geo.Size)         { r.rect.Resize(size) }
func (r *statefulRenderer) MinSize() geo.Size            { return geo.NewSize(10, 10) }
func (r *statefulRenderer) Objects() []core.CanvasObject { return []core.CanvasObject{r.rect} }
func (r *statefulRenderer) Refresh()                     {}

// TestPaintedWidgetKeepsItsRendererPastValidDuration is #1146's first
// done-when on desktop: a widget the paint walk reaches every frame keeps its
// renderer however long it is painted, although each Clean destroys what was
// last looked up more than ValidDuration before.
func TestPaintedWidgetKeepsItsRendererPastValidDuration(t *testing.T) {
	now := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	runOnMain(func() { cache.SetClockForTests(t, func() time.Time { return now }) })
	step := func() { // clock and Clean belong to the main goroutine
		runOnMain(func() { now = now.Add(cache.ValidDuration + 11*time.Second) })
	}

	w := createWindow("Renderer lifetime")
	defer w.Close()
	sw := newStatefulWidget()
	w.SetContent(container.NewVBox(widget.NewLabel("above"), sw))
	w.Resize(geo.NewSize(100, 100))
	runOnMain(func() { w.window.Show() })
	begun := func() uint64 {
		var n uint64
		runOnMain(func() {
			n = w.canvas.Painter().(interface{ FramesBegun() uint64 }).FramesBegun()
		})
		return n
	}
	// What drawSingleFrame does with a dirty window: paint it, then clean the
	// caches with the canvas refreshed.
	frame := func() {
		repaintWindow(w)
		runOnMain(func() { cache.Clean(true) })
	}

	var first core.WidgetRenderer
	for deadline := time.Now().Add(2 * time.Second); first == nil; {
		if time.Now().After(deadline) {
			t.Skip("the window began no frame for 2s (occluded, or no GPU surface)")
		}
		frame()
		runOnMain(func() { first, _ = cache.CachedRenderer(sw) })
	}
	require.NotNil(t, first)

	// Clean runs at most every 10 s; step further than that and than
	// ValidDuration, so that every frame's Clean expires what the last frame's
	// walk did not reach.
	began := begun()
	for i := 0; i < 5; i++ {
		step()
		frame()
	}
	if begun()-began < 5 {
		t.Skipf("the window began %d of 5 frames (occluded)", begun()-began)
	}

	var r core.WidgetRenderer
	var destroyed int
	runOnMain(func() { r, _ = cache.CachedRenderer(sw); destroyed = sw.destroyed })
	assert.Same(t, first, r, "the painted widget got a new renderer")
	assert.Zero(t, destroyed, "the painted widget's renderer was destroyed")

	// The control: without the paint, the second Clean destroys it.
	for i := 0; i < 2; i++ {
		step()
		runOnMain(func() { cache.Clean(true) })
	}
	runOnMain(func() { destroyed = sw.destroyed })
	assert.Equal(t, 1, destroyed, "a renderer no walk reached outlived ValidDuration")
}
