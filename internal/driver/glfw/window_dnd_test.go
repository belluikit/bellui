//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/glfw"
)

// droppableObject is a minimal bellui.Droppable canvas object for the drop tests.
type droppableObject struct {
	*canvas.Rectangle
	dropped *bellui.DropEvent
}

func (d *droppableObject) Dropped(ev *bellui.DropEvent) {
	d.dropped = ev
}

// hoverDroppableObject additionally implements bellui.DropHoverable and records the
// drag-motion phases it receives, for the pre-rebrand tracker #708 hover-routing tests.
type hoverDroppableObject struct {
	*canvas.Rectangle
	dropped               *bellui.DropEvent
	enters, overs, leaves int
	lastEnter, lastOver   *bellui.DragHoverEvent
}

func (d *hoverDroppableObject) Dropped(ev *bellui.DropEvent) { d.dropped = ev }

func (d *hoverDroppableObject) DragEnter(ev *bellui.DragHoverEvent) {
	d.enters++
	d.lastEnter = ev
}

func (d *hoverDroppableObject) DragOver(ev *bellui.DragHoverEvent) {
	d.overs++
	d.lastOver = ev
}

func (d *hoverDroppableObject) DragLeave() { d.leaves++ }

func TestWindow_ProcessDropped_RoutesToWidget(t *testing.T) {
	w := createWindow("Test")
	d := &droppableObject{Rectangle: canvas.NewRectangle(color.White)}
	d.SetMinSize(geo.NewSize(40, 40))
	w.SetContent(d)
	w.Resize(geo.NewSize(60, 60))
	repaintWindow(w)

	windowGot := false
	w.SetOnDropped(func(geo.Position, []bellui.URI) { windowGot = true })

	runOnMain(func() {
		w.mouseMoved(w.viewport, 30, 30)
		w.processDropped(w.viewport, []string{"/tmp/a.txt", "/tmp/b.png"})
	})

	require.NotNil(t, d.dropped, "widget should have received the drop")
	assert.False(t, windowGot, "window fallback should not fire when a widget claims the drop")
	require.Len(t, d.dropped.URIs, 2)
	assert.Equal(t, "file:///tmp/a.txt", d.dropped.URIs[0].String())
	assert.Equal(t, "file:///tmp/b.png", d.dropped.URIs[1].String())
	// Relative position is the drop point minus the object's absolute origin.
	assert.Equal(t, w.mousePos.Subtract(d.Position()), d.dropped.Position)
}

func TestWindow_ProcessDropped_FallsBackToWindow(t *testing.T) {
	w := createWindow("Test")
	w.SetContent(canvas.NewRectangle(color.Black)) // plain, non-Droppable
	w.Resize(geo.NewSize(60, 60))
	repaintWindow(w)

	var gotPos geo.Position
	var gotURIs []bellui.URI
	w.SetOnDropped(func(pos geo.Position, items []bellui.URI) {
		gotPos = pos
		gotURIs = items
	})

	runOnMain(func() {
		w.mouseMoved(w.viewport, 15, 15)
		w.processDropped(w.viewport, []string{"/tmp/c.txt"})
	})

	require.Len(t, gotURIs, 1)
	assert.Equal(t, "file:///tmp/c.txt", gotURIs[0].String())
	assert.Equal(t, w.mousePos, gotPos)
}

func TestWindow_ProcessDropped_NoTargetNoCallback(t *testing.T) {
	w := createWindow("Test")
	w.SetContent(canvas.NewRectangle(color.Black))
	w.Resize(geo.NewSize(60, 60))
	repaintWindow(w)

	// No SetOnDropped and no Droppable widget: must not panic.
	runOnMain(func() {
		assert.NotPanics(t, func() {
			w.processDropped(w.viewport, []string{"/tmp/x.txt"})
		})
	})
}

func TestWindow_ProcessDragMotion_EnterOverLeave(t *testing.T) {
	w := createWindow("Test")
	d := &hoverDroppableObject{Rectangle: canvas.NewRectangle(color.White)}
	d.SetMinSize(geo.NewSize(40, 40))
	w.SetContent(d)
	w.Resize(geo.NewSize(60, 60))
	repaintWindow(w)

	runOnMain(func() {
		// First position over the object -> DragEnter (once), no DragOver yet.
		w.processDragMotion(w.viewport, glfw.DragEnter, 30, 30)
		// Subsequent positions over the same object -> DragOver.
		w.processDragMotion(w.viewport, glfw.DragOver, 32, 32)
		w.processDragMotion(w.viewport, glfw.DragOver, 34, 34)
		// The drag leaves the window -> DragLeave.
		w.processDragMotion(w.viewport, glfw.DragLeave, 0, 0)
	})

	assert.Equal(t, 1, d.enters, "one DragEnter")
	assert.Equal(t, 2, d.overs, "two DragOver")
	assert.Equal(t, 1, d.leaves, "one DragLeave")
	require.NotNil(t, d.lastEnter)
	// Relative position is the pointer minus the object's absolute origin.
	assert.Equal(t, geo.NewPos(30, 30).Subtract(d.Position()), d.lastEnter.Position)
}

func TestWindow_ProcessDragMotion_MoveOffTargetLeaves(t *testing.T) {
	w := createWindow("Test")
	d := &hoverDroppableObject{Rectangle: canvas.NewRectangle(color.White)}
	d.SetMinSize(geo.NewSize(20, 20))
	// Put the droppable in the top-left; the rest of the window is empty. A
	// no-layout container keeps d where we place it (a plain SetContent would
	// stretch it to fill the window).
	w.SetContent(container.NewWithoutLayout(d))
	w.Resize(geo.NewSize(80, 80))
	runOnMain(func() {
		d.Move(geo.NewPos(0, 0))
		d.Resize(geo.NewSize(20, 20))
	})
	repaintWindow(w)

	runOnMain(func() {
		w.processDragMotion(w.viewport, glfw.DragEnter, 5, 5)  // over d
		w.processDragMotion(w.viewport, glfw.DragOver, 60, 60) // off d, empty area
	})

	assert.Equal(t, 1, d.enters)
	assert.Equal(t, 1, d.leaves, "moving off the target fires DragLeave")
	assert.Equal(t, 0, d.overs, "no DragOver once the pointer left the target")
}

func TestWindow_ProcessDragMotion_DropClearsHover(t *testing.T) {
	w := createWindow("Test")
	d := &hoverDroppableObject{Rectangle: canvas.NewRectangle(color.White)}
	d.SetMinSize(geo.NewSize(40, 40))
	w.SetContent(d)
	w.Resize(geo.NewSize(60, 60))
	repaintWindow(w)

	runOnMain(func() {
		w.mouseMoved(w.viewport, 30, 30)
		w.processDragMotion(w.viewport, glfw.DragEnter, 30, 30)
		w.processDropped(w.viewport, []string{"/tmp/a.txt"})
	})

	require.NotNil(t, d.dropped, "the drop is delivered")
	assert.Equal(t, 1, d.enters)
	assert.Equal(t, 1, d.leaves, "the drop clears the hover highlight via DragLeave")
}
