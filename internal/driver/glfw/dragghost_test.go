//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/glfw"
)

// ghostDraggable is a drag source that opts into the pre-rebrand tracker #819 ghost affordance.
type ghostDraggable struct {
	*canvas.Rectangle
	draggable
	ghost core.CanvasObject
}

func (g *ghostDraggable) DragImage() core.CanvasObject { return g.ghost }

var _ core.DragImageProvider = (*ghostDraggable)(nil)

func TestWindow_DragGhostLifecycle(t *testing.T) {
	w := createWindow("Test")
	ghost := canvas.NewRectangle(color.White)
	ghost.SetMinSize(geo.NewSize(20, 20))
	d := &ghostDraggable{Rectangle: canvas.NewRectangle(color.White), ghost: ghost}
	d.SetMinSize(geo.NewSize(20, 20))
	w.SetContent(container.NewHBox(d))

	repaintWindow(w)
	runOnMain(func() {
		before := len(w.canvas.Overlays().List())

		// Hover then press over the draggable — no ghost until the drag starts.
		w.mouseMoved(w.viewport, 10, 10)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Press, 0)
		assert.Nil(t, w.dragGhost, "no ghost before the drag threshold is crossed")

		// Move past the threshold — the drag starts and the ghost overlay appears.
		w.mouseMoved(w.viewport, 8, 8)
		assert.NotNil(t, w.dragGhost, "ghost created on drag start")
		assert.Equal(t, before+1, len(w.canvas.Overlays().List()), "ghost overlay added")

		// A further move now applies the grabbing cursor (mouseDragged is set
		// after the cursor is computed on the drag-start frame).
		w.mouseMoved(w.viewport, 7, 7)
		assert.Equal(t, desktop.Cursor(desktop.GrabbingCursor), w.cursor, "grabbing cursor during a DnD drag")

		// Release — the ghost is removed.
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Release, 0)
		assert.Nil(t, w.dragGhost, "ghost removed on drag end")
		assert.Equal(t, before, len(w.canvas.Overlays().List()), "ghost overlay removed")
	})
}
