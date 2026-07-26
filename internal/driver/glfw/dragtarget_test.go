//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/glfw"
)

// payloadDraggable is a drag source declaring an in-app payload (#819).
type payloadDraggable struct {
	*canvas.Rectangle
	draggable
	payload any
}

func (p *payloadDraggable) DragPayload() any { return p.payload }

// DragImage makes the source also raise a ghost overlay, so this exercises that
// the pass-through ghost does not shadow the DragTarget hit-testing beneath it.
func (p *payloadDraggable) DragImage() core.CanvasObject {
	g := canvas.NewRectangle(color.White)
	g.SetMinSize(geo.NewSize(20, 20))
	return g
}

var (
	_ core.DragPayloadProvider = (*payloadDraggable)(nil)
	_ core.DragImageProvider   = (*payloadDraggable)(nil)
)

// dropTarget records DragTarget callbacks.
type dropTarget struct {
	*canvas.Rectangle
	enters, overs, leaves int
	dropped               *core.DragTargetEvent
}

func (d *dropTarget) DragEnter(*core.DragTargetEvent) { d.enters++ }
func (d *dropTarget) DragOver(*core.DragTargetEvent)  { d.overs++ }
func (d *dropTarget) DragLeave()                      { d.leaves++ }
func (d *dropTarget) Drop(e *core.DragTargetEvent)    { d.dropped = e }

var _ core.DragTarget = (*dropTarget)(nil)

func TestWindow_DragTargetRouting(t *testing.T) {
	w := createWindow("Test")
	src := &payloadDraggable{Rectangle: canvas.NewRectangle(color.White), payload: "card-42"}
	src.SetMinSize(geo.NewSize(20, 20))
	tgt := &dropTarget{Rectangle: canvas.NewRectangle(color.Black)}
	tgt.SetMinSize(geo.NewSize(20, 20))
	w.SetContent(container.NewHBox(src, tgt))

	repaintWindow(w)
	runOnMain(func() {
		targetX := float64(tgt.Position().X + 5)

		// Start a drag on the source.
		w.mouseMoved(w.viewport, 10, 10)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Press, 0)
		w.mouseMoved(w.viewport, 8, 8) // cross the threshold -> drag starts
		assert.True(t, w.dndDrag, "source with a payload marks a DnD drag")

		// Move onto the target -> DragEnter.
		w.mouseMoved(w.viewport, targetX, 10)
		assert.Equal(t, 1, tgt.enters, "DragEnter on entering the target")
		assert.Equal(t, core.DragTarget(tgt), w.dragTarget)

		// Move within the target -> DragOver.
		w.mouseMoved(w.viewport, targetX+1, 10)
		assert.GreaterOrEqual(t, tgt.overs, 1, "DragOver while over the target")

		// Move back over the source (off the target) -> DragLeave.
		w.mouseMoved(w.viewport, 10, 10)
		assert.Equal(t, 1, tgt.leaves, "DragLeave on leaving the target")
		assert.Nil(t, w.dragTarget)

		// Move back onto the target and release -> Drop with the payload.
		w.mouseMoved(w.viewport, targetX, 10)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Release, 0)
		if assert.NotNil(t, tgt.dropped, "Drop fires on release over the target") {
			assert.Equal(t, "card-42", tgt.dropped.Payload, "the source payload reaches the target")
		}
		assert.False(t, w.dndDrag, "DnD state cleared after drop")
		assert.Nil(t, w.dragTarget)
	})
}

// TestWindow_DragTargetIgnoresPlainDrag verifies a plain draggable (no DnD
// opt-in) never routes to a DragTarget.
func TestWindow_DragTargetIgnoresPlainDrag(t *testing.T) {
	w := createWindow("Test")
	src := &draggableObject{Rectangle: canvas.NewRectangle(color.White)}
	src.SetMinSize(geo.NewSize(20, 20))
	tgt := &dropTarget{Rectangle: canvas.NewRectangle(color.Black)}
	tgt.SetMinSize(geo.NewSize(20, 20))
	w.SetContent(container.NewHBox(src, tgt))

	repaintWindow(w)
	runOnMain(func() {
		targetX := float64(tgt.Position().X + 5)
		w.mouseMoved(w.viewport, 10, 10)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Press, 0)
		w.mouseMoved(w.viewport, 8, 8)
		assert.False(t, w.dndDrag, "plain draggable is not a DnD drag")

		w.mouseMoved(w.viewport, targetX, 10)
		w.mouseClicked(w.viewport, glfw.MouseButton1, glfw.Release, 0)
		assert.Zero(t, tgt.enters, "plain draggable must not reach a DragTarget")
		assert.Nil(t, tgt.dropped)
	})
}
