package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// The touch scroll gesture (#291) is opt-in per driver and only reachable where
// Scroll is not itself Draggable, which is exactly the build the glfw driver
// tests cannot run under (scroller_mobile.go makes it Draggable for ci/mobile).
// These tests drive the shared handler directly instead, so the latch, the
// no-tap-on-release rule and the cancel-without-momentum rule stay covered on
// every leg.

// touchScrollFinder is a minimal ObjectFinder over a fixed object list: the
// first object matching the caller's test wins, and there is no canvas — the
// scroll path never needs one.
type touchScrollFinder struct {
	objects []core.CanvasObject
}

func (f *touchScrollFinder) FindObjectAtPositionMatching(pos geo.Position,
	test func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, int) {
	for _, o := range f.objects {
		if test(o) {
			return o, pos, 1 // layer 1: window head, so TapDown skips the unfocus path
		}
	}
	return nil, pos, 1
}

func (f *touchScrollFinder) CanvasForTouch() core.Canvas { return nil }

// scrollableRect is Scrollable but not Draggable — the desktop Scroll's shape.
// It is Tappable too, so the no-tap-on-release rule is tested against a target
// that would otherwise have been tapped.
type scrollableRect struct {
	*canvas.Rectangle
}

func (s *scrollableRect) Scrolled(*event.ScrollEvent) {}
func (s *scrollableRect) Tapped(*event.PointEvent)    {}

// tappableRect is the plain non-scrollable target a drag can end over.
type tappableRect struct {
	*canvas.Rectangle
}

func (tr *tappableRect) Tapped(*event.PointEvent) {}

// draggableScrollableRect is both, so the latch order is unambiguous where both
// apply (a scrollbar thumb or a slider inside scrollable content).
type draggableScrollableRect struct {
	*canvas.Rectangle
	drags int
}

func (d *draggableScrollableRect) Scrolled(*event.ScrollEvent) {}
func (d *draggableScrollableRect) Dragged(*event.DragEvent)    { d.drags++ }
func (d *draggableScrollableRect) DragEnd()                    {}

type scrollCall struct {
	origin geo.Position
	delta  geo.Delta
}

// newScrollTestHandler wires a handler over objects with the scroll callbacks
// the glfw driver sets, recording every dispatch.
func newScrollTestHandler(objects ...core.CanvasObject) (*TouchHandler, *[]scrollCall, *[]scrollCall) {
	h := &TouchHandler{}
	h.InitTouchHandler(&touchScrollFinder{objects: objects})
	moves := &[]scrollCall{}
	ends := &[]scrollCall{}
	h.ScrollCallback = func(origin geo.Position, delta geo.Delta) {
		*moves = append(*moves, scrollCall{origin, delta})
	}
	h.ScrollEndCallback = func(origin geo.Position, delta geo.Delta) {
		*ends = append(*ends, scrollCall{origin, delta})
	}
	return h, moves, ends
}

func noopDrag(event.Draggable, *event.DragEvent) {}

// tapUp runs TapUp with recording tap callbacks, returning how many taps it
// dispatched.
func tapUp(h *TouchHandler, pos geo.Position, tapID int) *int {
	taps := new(int)
	h.TapUp(pos, tapID, func(event.Tappable, *event.PointEvent) { *taps++ },
		func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {},
		func(event.DoubleTappable, *event.PointEvent) {},
		noopDrag)
	return taps
}

func TestTouchHandler_ScrollLatchesOverScrollable(t *testing.T) {
	sc := &scrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, moves, ends := newScrollTestHandler(sc)

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, noopDrag)
	require.Len(t, *moves, 1, "a past-slop move over a Scrollable latches a scroll gesture")
	assert.Equal(t, geo.NewPos(50, 50), (*moves)[0].origin, "the gesture origin is latched, not the finger position")
	assert.Equal(t, geo.NewDelta(0, -20), (*moves)[0].delta)

	// Once latched, sub-slop moves keep scrolling — a slow finger must not stutter.
	h.TapMove(geo.NewPos(50, 29), 1, noopDrag)
	require.Len(t, *moves, 2, "a sub-slop move keeps scrolling once latched")
	assert.Equal(t, geo.NewDelta(0, -1), (*moves)[1].delta)

	taps := tapUp(h, geo.NewPos(50, 29), 1)
	require.Len(t, *ends, 1, "the release seeds momentum with the last move's delta")
	assert.Equal(t, geo.NewDelta(0, -1), (*ends)[0].delta)
	assert.Zero(t, *taps, "a scroll gesture's release is not a tap")
}

func TestTouchHandler_ScrollNeedsAScrollableAtTheOrigin(t *testing.T) {
	h, moves, ends := newScrollTestHandler(&tappableRect{Rectangle: canvas.NewRectangle(color.Black)})

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, noopDrag)
	taps := tapUp(h, geo.NewPos(50, 30), 1)

	assert.Empty(t, *moves, "a drag over nothing scrollable is no scroll gesture")
	assert.Empty(t, *ends)
	assert.Equal(t, 1, *taps, "and its release keeps the unchanged tap handling")
}

func TestTouchHandler_ScrollOptOutLeavesDragsAlone(t *testing.T) {
	sc := &scrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := &TouchHandler{} // no ScrollCallback: the mobile/embedded drivers' wiring
	h.InitTouchHandler(&touchScrollFinder{objects: []core.CanvasObject{sc}})

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, noopDrag)
	taps := tapUp(h, geo.NewPos(50, 30), 1)
	assert.Equal(t, 1, *taps, "a driver that never opts in keeps the pre-#291 tap on release")
}

func TestTouchHandler_ScrollDraggableWinsTheLatch(t *testing.T) {
	drag := &draggableScrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, moves, ends := newScrollTestHandler(drag)

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) })

	assert.Equal(t, 1, drag.drags, "a Draggable under the finger takes the gesture")
	assert.Empty(t, *moves, "no content scroll under a latched drag")

	h.TapUp(geo.NewPos(50, 30), 1, func(event.Tappable, *event.PointEvent) {},
		func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {},
		func(event.DoubleTappable, *event.PointEvent) {},
		func(wid event.Draggable, ev *event.DragEvent) { wid.DragEnd() })
	assert.Empty(t, *ends)
}

func TestTouchHandler_ScrollCancelSeedsNoMomentum(t *testing.T) {
	sc := &scrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, moves, ends := newScrollTestHandler(sc)

	h.TapDown(geo.NewPos(50, 50), 1)
	h.TapMove(geo.NewPos(50, 30), 1, noopDrag)
	require.Len(t, *moves, 1)

	// A system cancel (palm rejection, edge gesture) ends the gesture where it
	// stands: content must not fly on from a contact the system took over.
	h.TapCancel(geo.NewPos(50, 30), 1)
	assert.Empty(t, *ends, "a cancelled scroll seeds no momentum")

	// ...and the single scroll slot is free for the next contact.
	h.TapDown(geo.NewPos(50, 50), 2)
	h.TapMove(geo.NewPos(50, 30), 2, noopDrag)
	assert.Len(t, *moves, 2, "the cancelled gesture released the scroll slot")
}
