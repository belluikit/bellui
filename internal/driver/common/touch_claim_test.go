package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// A drag goes to the innermost Draggable at its origin that claims it
// (event.DragClaimer, #662). The finder takes the first matching object, so
// the objects are listed innermost first.

// sidewaysDraggable claims only drags that start mostly sideways, the way a
// Carousel does.
type sidewaysDraggable struct {
	*canvas.Rectangle
	drags, asked int
}

func (s *sidewaysDraggable) Dragged(*event.DragEvent) { s.drags++ }
func (s *sidewaysDraggable) DragEnd()                 {}
func (s *sidewaysDraggable) ClaimsDrag(d geo.Delta) bool {
	s.asked++
	return abs(d.DX) > abs(d.DY)
}

func abs(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// plainDraggable takes every drag: it is no DragClaimer.
type plainDraggable struct {
	*canvas.Rectangle
	drags int
}

func (p *plainDraggable) Dragged(*event.DragEvent) { p.drags++ }
func (p *plainDraggable) DragEnd()                 {}

func dispatchDrag(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }

func TestTouchHandler_DragGoesToTheInnermostClaimer(t *testing.T) {
	inner := &sidewaysDraggable{Rectangle: canvas.NewRectangle(color.Black)}
	outer := &plainDraggable{Rectangle: canvas.NewRectangle(color.Black)}

	// Sideways: the inner one claims it.
	h, _, _ := newScrollTestHandler(inner, outer)
	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(30, 48), 1, dispatchDrag)
	h.TapMove(geo.NewPos(20, 60), 1, dispatchDrag)
	assert.Equal(t, 2, inner.drags, "the claim holds for the rest of the drag, even when it turns")
	assert.Zero(t, outer.drags)
	tapUp(h, geo.NewPos(20, 60), 1)

	// Upward: the inner one leaves it, so the one around it takes it.
	inner.drags = 0
	h.TapDown(geo.NewPos(50, 50), 2)
	h.TapMove(geo.NewPos(48, 30), 2, dispatchDrag)
	h.TapMove(geo.NewPos(60, 28), 2, dispatchDrag)
	assert.Zero(t, inner.drags)
	assert.Equal(t, 2, outer.drags)
}

func TestTouchHandler_UnclaimedDragScrolls(t *testing.T) {
	inner := &sidewaysDraggable{Rectangle: canvas.NewRectangle(color.Black)}
	sc := &scrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, moves, _ := newScrollTestHandler(inner, sc)

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, dispatchDrag)

	assert.Zero(t, inner.drags)
	assert.Len(t, *moves, 1, "a drag no Draggable claims scrolls what is scrollable there")
}

// A drag after a hold-fired long-press goes to the innermost Draggable without
// asking: the held finger drags what it picked up (an Entry extends the word
// the long-press selected).
func TestTouchHandler_DragAfterLongPressIsNotOffered(t *testing.T) {
	inner := &sidewaysDraggable{Rectangle: canvas.NewRectangle(color.Black)}
	outer := &plainDraggable{Rectangle: canvas.NewRectangle(color.Black)}
	h, _, _ := newScrollTestHandler(inner, outer)

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TouchCancelLock.Lock()
	h.longPressFired[1] = true // as fireLongPress records it
	h.TouchCancelLock.Unlock()
	h.TapMove(geo.NewPos(50, 30), 1, dispatchDrag)

	assert.Zero(t, inner.asked, "no claim is asked after a long-press")
	assert.Equal(t, 1, inner.drags)
	assert.Zero(t, outer.drags)
}

// The current-position fallback (no Draggable at the origin) is held to the
// same claim: a sideways-only Draggable the finger moved onto does not take an
// upward drag.
func TestTouchHandler_DragFallbackHonoursTheClaim(t *testing.T) {
	inner := &sidewaysDraggable{Rectangle: canvas.NewRectangle(color.Black)}
	h, moves, _ := newScrollTestHandler(inner)
	h.Finder = &originlessFinder{at: geo.NewPos(50, 30), obj: inner}

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, dispatchDrag)

	assert.Zero(t, inner.drags)
	assert.Empty(t, *moves, "nothing scrollable at the origin either")
}

// originlessFinder holds one object, found only at at.
type originlessFinder struct {
	at  geo.Position
	obj core.CanvasObject
}

func (f *originlessFinder) FindObjectAtPositionMatching(pos geo.Position,
	test func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, int) {
	if pos == f.at && test(f.obj) {
		return f.obj, pos, 1
	}
	return nil, pos, 1
}

func (f *originlessFinder) CanvasForTouch() core.Canvas { return nil }
