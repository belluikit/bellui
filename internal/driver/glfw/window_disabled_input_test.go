//go:build !no_glfw && !ci && !mobile && !wasm

package glfw

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/assert"
)

// A click on a tappable that cannot be disabled, inside a disabled container,
// reaches nothing (#1378): through the tree walk while the canvas is dirty,
// and through the spatial index once it has been painted.
func TestWindow_DisabledContainerDropsClick(t *testing.T) {
	w := createWindow("Test")
	inside := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
	inside.SetMinSize(geo.NewSize(100, 100))
	tabs := container.NewAppTabs(container.NewTabItem("a", inside))
	w.SetContent(tabs)
	w.Resize(geo.NewSize(300, 300))
	repaintWindow(w)

	var at geo.Position
	runOnMain(func() { at = d.AbsolutePositionForObject(inside).AddXY(50, 50) })
	click := func() {
		w.mouseMoved(w.viewport, float64(at.X), float64(at.Y))
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Press, 0)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Release, 0)
	}

	runOnMain(func() {
		click()
		assert.NotNil(t, inside.popTapEvent(), "the probe click missed")

		tabs.Disable() // the canvas is dirty now: the tree walk answers
		click()
		assert.Nil(t, inside.popTapEvent(), "a click reached a widget inside disabled tabs (tree walk)")
	})
	repaintWindow(w)
	runOnMain(func() {
		// The run loop clears the dirty flag before it paints; repaintWindow
		// does not. Clean, the canvas answers from the spatial index the
		// paint just built.
		w.canvas.CheckDirtyAndClear()
		w.mouseMoved(w.viewport, float64(at.X), float64(at.Y))
		clean := !w.canvas.IsDirty() && w.canvas.SpatialIndex() != nil
		assert.True(t, clean, "the click would not go through the spatial index")
		t.Logf("spatial index answers the click: %v", clean)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Press, 0)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Release, 0)
		assert.Nil(t, inside.popTapEvent(), "a click reached a widget inside disabled tabs (spatial index)")

		tabs.Enable()
	})
	repaintWindow(w)
	runOnMain(func() {
		click()
		assert.NotNil(t, inside.popTapEvent(), "enabling the tabs brings the clicks back")
	})
}
