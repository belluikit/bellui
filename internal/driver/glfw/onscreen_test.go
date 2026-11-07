//go:build !no_glfw && !ci && !mobile && !wasm

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
)

// TestIsOnScreen pins the driver's answer to bellui.IsOnScreen with a real
// window (#1061): the window must be shown, and the object in its last frame
// with nothing above it hidden.
func TestIsOnScreen(t *testing.T) {
	w := createWindow("On screen")
	defer w.Close()
	glow := canvas.NewRectangle(color.White)
	inner := container.NewStack(glow)
	w.SetContent(container.NewVBox(inner))
	runOnMain(w.window.Show)
	repaintWindow(w)
	runOnMain(func() {
		assert.True(t, d.IsOnScreen(glow), "in the frame of a window shown")
		inner.Hide()
		assert.False(t, d.IsOnScreen(glow), "under a hidden container")
		inner.Show()
	})
	repaintWindow(w)
	runOnMain(func() { assert.True(t, d.IsOnScreen(glow), "under the container shown again") })

	runOnMain(w.window.Hide)
	runOnMain(func() { assert.False(t, d.IsOnScreen(glow), "in a window not shown") })
	runOnMain(func() { assert.False(t, d.IsOnScreen(canvas.NewRectangle(color.White)), "in no window") })
}
