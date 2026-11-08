package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// pressRect is a button's shape: Touchable for its pressed state, Tappable,
// and not Draggable.
type pressRect struct {
	*canvas.Rectangle
	downs, ups, cancels int
}

func (p *pressRect) TouchDown(*mobile.TouchEvent)   { p.downs++ }
func (p *pressRect) TouchUp(*mobile.TouchEvent)     { p.ups++ }
func (p *pressRect) TouchCancel(*mobile.TouchEvent) { p.cancels++ }
func (p *pressRect) Tapped(*event.PointEvent)       {}

// draggablePressRect is an entry's shape: Touchable and Draggable.
type draggablePressRect struct {
	pressRect
	drags int
}

func (d *draggablePressRect) Dragged(*event.DragEvent) { d.drags++ }
func (d *draggablePressRect) DragEnd()                 {}

// TestTouchHandler_ScrollCancelsPress is #1085: a touch that goes down on a
// button and scrolls the list it is in cancels the button's press. The list
// moves with the finger, so the finger stayed over the button, and the button
// kept showing a press it was never tapped by.
func TestTouchHandler_ScrollCancelsPress(t *testing.T) {
	button := &pressRect{Rectangle: canvas.NewRectangle(color.Black)}
	sc := &scrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, moves, _ := newScrollTestHandler(button, sc)
	clk := useFakeClock(h)

	h.TapDown(geo.NewPos(50, 50), 1)
	assert.Equal(t, 1, button.downs)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(50, 30), 1, noopDrag)
	assert.Len(t, *moves, 1, "precondition: the move scrolls")
	assert.Equal(t, 1, button.cancels, "the scroll left the button pressed")

	clk.advance(coastStep)
	h.TapMove(geo.NewPos(50, 10), 1, noopDrag)
	assert.Equal(t, 1, button.cancels, "the press was cancelled again")
	assert.Zero(t, *tapUp(h, geo.NewPos(50, 10), 1))
}

// TestTouchHandler_DragOfAnotherCancelsPress: a touch on a button that drags
// something else, such as the carousel around it, cancels the press too.
func TestTouchHandler_DragOfAnotherCancelsPress(t *testing.T) {
	button := &pressRect{Rectangle: canvas.NewRectangle(color.Black)}
	drag := &draggableScrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, _, _ := newScrollTestHandler(button, drag)
	clk := useFakeClock(h)

	h.TapDown(geo.NewPos(50, 50), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(30, 50), 1, func(event.Draggable, *event.DragEvent) {})
	assert.Same(t, drag, h.Dragging, "precondition: the move drags the other object")
	assert.Equal(t, 1, button.cancels, "the drag left the button pressed")
}

// TestTouchHandler_OwnDragKeepsTouch: a Touchable that is dragged itself,
// such as an entry selecting text, keeps its touch: its drag is its own
// gesture.
func TestTouchHandler_OwnDragKeepsTouch(t *testing.T) {
	entry := &draggablePressRect{pressRect: pressRect{Rectangle: canvas.NewRectangle(color.Black)}}
	h, _, _ := newScrollTestHandler(entry)
	clk := useFakeClock(h)

	h.TapDown(geo.NewPos(50, 50), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(30, 50), 1, func(event.Draggable, *event.DragEvent) {})
	assert.Same(t, entry, h.Dragging, "precondition: the move drags the entry")
	assert.Zero(t, entry.cancels, "the entry's own drag cancelled its touch")
}

// TestTouchHandler_SlideOnButtonKeepsPress: a finger that slides past the tap
// slop on a button that is in no scroll keeps the press, and the release
// taps it, as on Android.
func TestTouchHandler_SlideOnButtonKeepsPress(t *testing.T) {
	button := &pressRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, _, _ := newScrollTestHandler(button)
	clk := useFakeClock(h)

	h.TapDown(geo.NewPos(50, 50), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(30, 50), 1, noopDrag)
	assert.Zero(t, button.cancels)
	assert.Equal(t, 1, *tapUp(h, geo.NewPos(30, 50), 1))
	assert.Equal(t, 1, button.ups)
}
