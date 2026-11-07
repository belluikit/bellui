//go:build !no_glfw && !ci && !mobile && !wasm

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
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

// TestIsOnScreenTabReturnFirstFrame is #1069: the tick before the first frame
// of a tab selected again reads the tab's content as on screen, so that frame
// draws what the tick applied, not what the tab showed when it was hidden.
func TestIsOnScreenTabReturnFirstFrame(t *testing.T) {
	w := createWindow("Tab return")
	defer w.Close()
	glow := canvas.NewRectangle(color.White)
	tabs := container.NewAppTabs(
		container.NewTabItem("Player", glow),
		container.NewTabItem("Library", widget.NewLabel("songs")),
	)
	w.SetContent(tabs)
	w.Resize(geo.NewSize(200, 200))
	runOnMain(w.window.Show)
	repaintWindow(w)
	runOnMain(func() { tabs.SelectIndex(1) })
	repaintWindow(w)

	red := color.NRGBA{R: 0xff, A: 0xff}
	runOnMain(func() {
		tabs.SelectIndex(0)
		// The tick, which runs before the frame's walk (processEventTick).
		if assert.True(t, d.IsOnScreen(glow), "not on screen on the tick before the first frame back") {
			glow.FillColor = red
			glow.Refresh()
		}
	})
	repaintWindow(w)
	img := captureOrSkip(t, w) // what that frame drew; skips while the window is occluded
	var at geo.Position
	runOnMain(func() { at = glow.Position().Add(geo.NewPos(glow.Size().Width/2, glow.Size().Height/2)) })
	scale := float32(img.Bounds().Dx()) / w.Canvas().Size().Width
	r, g, _, _ := img.At(int(at.X*scale), int(at.Y*scale)).RGBA()
	assert.True(t, r > 0xf000 && g < 0x1000, "the first frame back drew the glow as it was when hidden: %v", img.At(int(at.X*scale), int(at.Y*scale)))
}
