//go:build gl && !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
)

// shadowCache is the GL painter's view of its box-shadow cache (#444).
type shadowCache interface {
	CachedShadows() int
	ReleaseCaches()
}

// releaseProbe wraps the window's painter for Close and records what the
// cache release saw.
type releaseProbe struct {
	gpu.Painter
	window          *glfw.Window
	called, current bool
	before, after   int
}

func (r *releaseProbe) ReleaseCaches() {
	cache := r.Painter.(shadowCache)
	r.called = true
	r.current = glfw.GetCurrentContext() == r.window
	r.before = cache.CachedShadows()
	cache.ReleaseCaches()
	r.after = cache.CachedShadows()
}

// TestWindow_CloseReleasesShadowCache: closing a window whose GL painter holds
// cached box shadows deletes them, with that window's context current — the
// textures belong to it, and Close runs when no context is current (#444).
func TestWindow_CloseReleasesShadowCache(t *testing.T) {
	w := createWindow("Test")
	card := func(c color.Color) *canvas.BoxShadow {
		s := canvas.NewBoxShadow(c, 6)
		s.CornerRadius = 8
		return s
	}
	content := container.NewGridWithColumns(2,
		card(color.NRGBA{A: 80}), card(color.NRGBA{A: 80}), card(color.NRGBA{A: 120}))
	w.SetContent(content)
	w.Resize(geo.NewSize(300, 200))
	repaintWindow(w)

	var painter gpu.Painter
	runOnMain(func() { painter = w.canvas.Painter() })
	cache, ok := painter.(shadowCache)
	require.True(t, ok, "the GL painter caches box shadows")
	require.Equal(t, 2, cache.CachedShadows(), "two distinct shadows, one shared by two cards")

	probe := &releaseProbe{Painter: painter, window: w.viewport}
	runOnMain(func() { w.canvas.SetPainter(probe) })
	w.Close()

	assert.True(t, probe.called, "Close releases the painter's caches")
	assert.True(t, probe.current, "the window's context is current during the release")
	assert.Equal(t, 2, probe.before)
	assert.Zero(t, probe.after)
	runOnMain(func() { assert.Nil(t, glfw.GetCurrentContext(), "Close leaves no context current") })
}
