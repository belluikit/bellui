//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// TestCanvasForObjectMovedWindow pins #448 with real windows: content moved
// from one open window to another belongs to the second once it is painted
// there, so its refreshes repaint the window that shows it. They used to go to
// the window it had left, which kept its entry alive.
func TestCanvasForObjectMovedWindow(t *testing.T) {
	a := createWindow("Moved from")
	defer a.Close()
	b := createWindow("Moved to")
	defer b.Close()
	img := canvas.NewImageFromImage(nil)
	img.SetMinSize(geo.NewSquareSize(20))
	moved := container.NewVBox(img)
	a.SetContent(moved)
	b.SetContent(canvas.NewRectangle(color.Black))
	repaintWindow(a)
	repaintWindow(b)
	var before core.Canvas
	runOnMain(func() { before = d.CanvasForObject(img) })
	require.Equal(t, core.Canvas(a.canvas), before)

	a.SetContent(canvas.NewRectangle(color.White))
	b.SetContent(moved)
	repaintWindow(a)
	repaintWindow(b)

	runOnMain(func() {
		c := d.CanvasForObject(img)
		assert.Equal(t, core.Canvas(b.canvas), c, "the image belongs to the window showing it")
		// Set and check in one main-thread closure: the render loop could
		// clear the flags in between otherwise.
		a.canvas.CheckDirtyAndClear()
		b.canvas.CheckDirtyAndClear()
		c.Refresh(img)
		assert.True(t, b.canvas.IsDirty(), "a refresh of it repaints the window showing it")
		assert.False(t, a.canvas.IsDirty(), "and not the window it left")
	})
}
