package widget

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

var globalProgressInfRenderer core.WidgetRenderer

func BenchmarkProgressbarInf(b *testing.B) {
	var renderer core.WidgetRenderer
	widget := &ProgressBarInfinite{}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		renderer = widget.CreateRenderer()
	}

	// Avoid having the value optimized out by the compiler.
	globalProgressInfRenderer = renderer
}

func TestProgressBarInfinite_Creation(t *testing.T) {
	bar := NewProgressBarInfinite()

	// ticker should be disabled until the widget is shown
	assert.False(t, bar.Running())

	win := test.NewWindow(bar)
	defer win.Close()
	win.Show()

	// ticker should start automatically once the renderer is created
	assert.True(t, bar.Running())
}

func TestProgressBarInfinite_Destroy(t *testing.T) {
	bar := NewProgressBarInfinite()
	win := test.NewWindow(bar)
	defer win.Close()
	win.Show()

	assert.True(t, cache.IsRendered(bar))
	assert.True(t, bar.Running())

	// check that it stopped
	cache.DestroyRenderer(bar)
	assert.False(t, bar.Running())

	// and that the cache was removed
	assert.False(t, cache.IsRendered(bar))
}

func TestProgressBarInfinite_Reshown(t *testing.T) {
	bar := NewProgressBarInfinite()
	win := test.NewWindow(bar)
	defer win.Close()
	win.Show()

	assert.True(t, bar.Running())
	bar.Hide()
	assert.False(t, bar.Running())

	// make sure it restarts when re-shown
	bar.Show()
	// Show() starts a goroutine, so pause for it to initialize
	time.Sleep(10 * time.Millisecond)
	assert.True(t, bar.Running())
	bar.Hide()
	assert.False(t, bar.Running())
}

func TestInfiniteProgressRenderer_Layout(t *testing.T) {
	bar := NewProgressBarInfinite()
	width := float32(100.0)
	bar.Resize(geo.NewSize(width, 10))

	render := test.TempWidgetRenderer(t, bar).(*infProgressRenderer)

	render.updateBar(0.0)
	// start at the smallest size
	assert.InEpsilon(t, width*minProgressBarInfiniteWidthRatio, render.bar.Size().Width, 0.0001)

	// make sure the inner progress bar grows in size
	// call updateBar() enough times to grow the inner bar
	maxWidth := width * maxProgressBarInfiniteWidthRatio
	render.updateBar(0.5)
	assert.Equal(t, maxWidth, render.bar.Size().Width)

	render.updateBar(1.0)
	// ends at the smallest size again
	assert.Equal(t, width*minProgressBarInfiniteWidthRatio, render.bar.Size().Width)

	bar.Hide()
}

// TestProgressBarInfinite_Animates pins #497: a shown bar starts its
// animation, a hidden one stops it, and a re-shown one starts it again. The
// Running flag alone said yes while nothing moved. The test driver runs no
// animations, so this checks the renderer's animation state.
func TestProgressBarInfinite_Animates(t *testing.T) {
	test.NewTempApp(t)
	bar := NewProgressBarInfinite()
	win := test.NewWindow(bar)
	defer win.Close()
	win.Resize(geo.NewSize(200, 40))
	r := cache.Renderer(bar).(*infProgressRenderer)

	assert.True(t, r.wasRunning, "a shown bar animates")

	bar.Hide()
	assert.False(t, r.wasRunning, "a hidden bar stops")

	bar.Show()
	assert.True(t, r.wasRunning, "a re-shown bar animates again")
}
