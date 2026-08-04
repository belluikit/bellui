//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	pubdriver "github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
)

// The public pick wrappers probe through the canvas's dispatch hit-walk
// (#245), so the canvas menu bar — a dispatch root the plain
// overlays+content walk misses — is reachable through them.
func TestWindow_ObjectAtPositionMatching_MenuBar(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS uses the native menu bar; there is no canvas menu to hit")
	}

	w := createWindow("Test")
	w.SetContent(belluicanvas.NewRectangle(color.Black))
	w.Resize(geo.NewSize(300, 200))

	runOnMain(func() {
		w.SetMainMenu(bellui.NewMainMenu(bellui.NewMenu("File", bellui.NewMenuItem("Item", func() {}))))
	})

	var (
		obj    core.CanvasObject
		bounds geo.Rectangle
		ok     bool
	)
	runOnMain(func() {
		menu := w.canvas.menu
		require.NotNil(t, menu, "SetMainMenu installs a canvas menu bar off darwin")
		obj, bounds, ok = pubdriver.ObjectAtPositionMatching(w.canvas, geo.NewPos(5, 5),
			func(o core.CanvasObject) bool { return o == menu })
	})
	require.True(t, ok, "the public pick reaches the menu bar")
	assert.Same(t, w.canvas.menu, obj)
	assert.Equal(t, w.canvas.menu.Size(), bounds.Size)
}
