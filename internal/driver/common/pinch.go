package common

import (
	"math"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// Two-finger pinch recognition (#379). TouchHandler tracks every contact that
// is down. When a second finger lands while one is already down and an
// event.Pinchable lies under the midpoint, the two contacts become a pinch:
// whatever the first finger had started (drag, scroll, pending long-press) is
// cancelled, moves of either finger dispatch Pinched with the change in their
// distance, and lifting or cancelling either one dispatches PinchEnded. Both
// contacts are then spent — neither produces a tap, drag or scroll afterwards.

type pinchState struct {
	target event.Pinchable
	ids    [2]int
	dist   float32      // finger distance at the previous Pinched
	origin geo.Position // canvas position of the target's origin
}

// trackContact records where a contact is, for pinch recognition.
func (h *TouchHandler) trackContact(tapID int, pos geo.Position) {
	if h.contacts == nil {
		h.contacts = map[int]geo.Position{}
		h.pinchSpent = map[int]bool{}
	}
	h.contacts[tapID] = pos
}

// startPinch turns the contact that just went down and the one already down
// into a pinch, if there are exactly two and a Pinchable is under their
// midpoint. It reports whether it did.
func (h *TouchHandler) startPinch(tapID int) bool {
	if h.pinch != nil || len(h.contacts) != 2 {
		return false
	}
	other := 0
	for id := range h.contacts {
		if id != tapID {
			other = id
		}
	}
	a, b := h.contacts[other], h.contacts[tapID]
	mid := midpoint(a, b)
	co, objPos, _ := h.Finder.FindObjectAtPositionMatching(mid, func(o core.CanvasObject) bool {
		_, ok := o.(event.Pinchable)
		return ok
	})
	target, ok := co.(event.Pinchable)
	if !ok {
		return false
	}

	// The first finger's gesture is over: it is now half of a pinch.
	h.hideToolTip()
	h.cancelLongPress(other)
	if h.Dragging != nil && h.draggingID == other {
		h.Dragging.DragEnd()
		h.Dragging = nil
		h.draggingID = 0
	}
	if h.scrolling && h.scrollingID == other {
		h.scrolling = false
		h.scrollingID = 0
	}

	h.pinch = &pinchState{
		target: target,
		ids:    [2]int{other, tapID},
		dist:   distance(a, b),
		origin: mid.Subtract(objPos),
	}
	h.pinchSpent[other] = true
	h.pinchSpent[tapID] = true
	return true
}

// movePinch dispatches one Pinched for a move of either pinch finger.
func (h *TouchHandler) movePinch() {
	p := h.pinch
	a, b := h.contacts[p.ids[0]], h.contacts[p.ids[1]]
	d := distance(a, b)
	if p.dist <= 0 || d <= 0 || d == p.dist {
		return
	}
	scale := d / p.dist
	p.dist = d
	mid := midpoint(a, b)
	p.target.Pinched(&event.PinchEvent{
		PointEvent: event.PointEvent{AbsolutePosition: mid, Position: mid.Subtract(p.origin)},
		Scale:      scale,
	})
}

// pinchMove handles a move of a contact that is, or was, part of a pinch, and
// reports whether it did (the normal drag/scroll handling must then not run).
func (h *TouchHandler) pinchMove(pos geo.Position, tapID int) bool {
	if _, down := h.contacts[tapID]; down {
		h.contacts[tapID] = pos
	}
	if p := h.pinch; p != nil && (tapID == p.ids[0] || tapID == p.ids[1]) {
		h.movePinch()
		return true
	}
	return h.pinchSpent[tapID]
}

// pinchRelease handles a contact lifting or being cancelled. It ends a pinch
// the contact belonged to, and reports whether the contact was spent on a
// pinch (its release must then not produce a tap, drag end or scroll).
func (h *TouchHandler) pinchRelease(tapID int) bool {
	delete(h.contacts, tapID)
	if p := h.pinch; p != nil && (tapID == p.ids[0] || tapID == p.ids[1]) {
		h.pinch = nil
		p.target.PinchEnded()
	}
	if !h.pinchSpent[tapID] {
		return false
	}
	delete(h.pinchSpent, tapID)
	h.hideToolTip()
	h.cancelLongPress(tapID)
	h.TouchCancelLock.Lock()
	delete(h.longPressFired, tapID)
	h.TouchCancelLock.Unlock()
	h.forgetTouch(tapID)
	return true
}

func midpoint(a, b geo.Position) geo.Position {
	return geo.NewPos((a.X+b.X)/2, (a.Y+b.Y)/2)
}

func distance(a, b geo.Position) float32 {
	return float32(math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)))
}
