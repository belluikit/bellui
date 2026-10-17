package common

import (
	"math"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// Two-finger pinch recognition (#379). TouchHandler tracks every contact that
// is down. When a second finger lands while one is already down and an
// event.Pinchable lies under the midpoint, the two contacts become a pinch:
// whatever the first finger had started (drag, scroll, pending long-press) is
// cancelled, moves of either finger dispatch Pinched with the change in their
// distance, and lifting or cancelling either one dispatches PinchEnded. Both
// contacts are then spent — neither produces a tap, drag or scroll afterwards,
// and neither counts as down any more, so the surviving finger cannot start a
// second pinch with a finger that lands later.
//
// Contacts are counted, so a release the platform never delivers would leave
// a contact down for good and pair it with every later finger. The pinch's
// own fingers are therefore dropped from contacts as soon as it ends, and a
// fresh down on an id clears whatever that id was spent on before — and ends
// the pinch it belonged to, whose release was lost.

// Two fingers make a two-finger gesture whether or not a Pinchable is under
// them, classified once the fingers have moved (#398): once their distance or
// their midpoint has moved twoFingerSlop, a distance change at least as large
// as the midpoint's movement makes it a pinch (over a Pinchable; a one-handed
// pinch moves one finger, so the midpoint moves half as far as the distance
// changes), and anything else a pan, which scrolls like a one-finger flick
// (ScrollCallback, ScrollEndCallback) from the gesture's starting point.
// Scrolls pass from the widget under the fingers out to the scroll containers
// around it, so two fingers scroll a page past a widget that keeps one-finger
// drags for itself (a chart, the data grid).
type pinchState struct {
	target event.Pinchable // nil when no Pinchable is under the fingers
	ids    [2]int
	dist   float32      // finger distance at the previous Pinched (fixed while undecided)
	origin geo.Position // canvas position of the target's origin

	mode    twoFingerMode
	down    [2]geo.Position // where each finger was when the second one landed
	start   geo.Position    // midpoint when the second finger landed: the pan's scroll origin
	lastMid geo.Position    // midpoint at the previous pan step
	lastPan geo.Delta       // the previous pan step, the momentum seed on release
}

type twoFingerMode int

const (
	twoFingerUndecided twoFingerMode = iota
	twoFingerPinch
	twoFingerPan
)

// twoFingerSlop is how far the finger distance or the midpoint must move
// before a two-finger gesture is classified as a pinch or a pan.
const twoFingerSlop = 10

// trackContact records a contact that went down, for pinch recognition. A
// down on an id means any earlier contact with that id is over, whether or not
// its release arrived: a pinch it was part of has ended, and the id is no
// longer spent.
func (h *TouchHandler) trackContact(tapID int, pos geo.Position) {
	h.endPinchOf(tapID)
	delete(h.pinchSpent, tapID)
	h.contacts[tapID] = pos
}

// endPinchOf ends the pinch in progress if tapID is one of its fingers. Both
// fingers stop counting as down; they stay spent until they are released.
func (h *TouchHandler) endPinchOf(tapID int) {
	p := h.pinch
	if p == nil || (tapID != p.ids[0] && tapID != p.ids[1]) {
		return
	}
	h.pinch = nil
	delete(h.contacts, p.ids[0])
	delete(h.contacts, p.ids[1])
	switch p.mode {
	case twoFingerPinch:
		p.target.PinchEnded()
	case twoFingerPan:
		if h.ScrollEndCallback != nil {
			h.ScrollEndCallback(p.start, p.lastPan)
		}
	}
}

// startPinch turns the contact that just went down and the one already down
// into a two-finger gesture, if there are exactly two. It reports whether it
// did. A Pinchable under their midpoint, if any, is the pinch target.
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
	target, _ := co.(event.Pinchable)
	// Without a Pinchable the gesture can only pan, and a first finger that
	// already latched its own drag or scroll keeps it: a second contact then
	// (a palm, a resting thumb) must not take the drag away.
	if target == nil && ((h.Dragging != nil && h.draggingID == other) || (h.scrolling && h.scrollingID == other)) {
		return false
	}

	// The first finger's gesture is over: it is now half of a pinch.
	h.hideToolTip()
	h.cancelLongPress(other)
	if wid := h.Touched[other]; wid != nil {
		touchEv := &mobile.TouchEvent{}
		touchEv.Position = a
		touchEv.AbsolutePosition = a
		wid.TouchCancel(touchEv)
		h.Touched[other] = nil
	}
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
		target:  target,
		ids:     [2]int{other, tapID},
		dist:    distance(a, b),
		origin:  mid.Subtract(objPos),
		down:    [2]geo.Position{a, b},
		start:   mid,
		lastMid: mid,
	}
	h.pinchSpent[other] = true
	h.pinchSpent[tapID] = true
	return true
}

// movePinch handles a move of either finger of the two-finger gesture:
// classifying it on the first decisive move, then pinching or panning. It
// reports false when the gesture dissolved instead — the move must then be
// handled as an ordinary single-finger one.
func (h *TouchHandler) movePinch() bool {
	p := h.pinch
	a, b := h.contacts[p.ids[0]], h.contacts[p.ids[1]]
	mid := midpoint(a, b)
	if p.mode == twoFingerUndecided {
		spread := float32(math.Abs(float64(distance(a, b) - p.dist)))
		moved := distance(mid, p.start)
		switch {
		case spread < twoFingerSlop && moved < twoFingerSlop:
			return true
		case p.target != nil && spread >= moved:
			p.mode = twoFingerPinch
		case p.target != nil || movedTogether(p.down[0], a, p.down[1], b):
			p.mode = twoFingerPan
		default:
			// No Pinchable and only one finger moving: not a two-finger
			// gesture after all (a resting palm or thumb beside a dragging
			// finger). Hand both contacts back to single-finger handling.
			h.dissolveTwoFinger()
			return false
		}
	}
	if p.mode == twoFingerPan {
		delta := geo.Delta{DX: mid.X - p.lastMid.X, DY: mid.Y - p.lastMid.Y}
		p.lastMid = mid
		if delta == (geo.Delta{}) {
			return true
		}
		p.lastPan = delta
		if h.ScrollCallback != nil {
			h.ScrollCallback(p.start, delta)
		}
		return true
	}

	// A pinch. A move that does not change the distance still reports the new
	// focus, with scale 1, so the content follows the fingers.
	scale := float32(1)
	if d := distance(a, b); d > 0 {
		if p.dist > 0 {
			scale = d / p.dist
		}
		p.dist = d // fingers that landed together are measured from here
	}
	p.target.Pinched(&event.PinchEvent{
		PointEvent: event.PointEvent{AbsolutePosition: mid, Position: mid.Subtract(p.origin)},
		Scale:      scale,
	})
	return true
}

// pinchMove handles a move of a contact that is, or was, part of a pinch, and
// reports whether it did (the normal drag/scroll handling must then not run).
func (h *TouchHandler) pinchMove(pos geo.Position, tapID int) bool {
	if _, down := h.contacts[tapID]; down {
		h.contacts[tapID] = pos
	}
	if p := h.pinch; p != nil && (tapID == p.ids[0] || tapID == p.ids[1]) {
		if h.movePinch() {
			return true
		}
	}
	return h.pinchSpent[tapID]
}

// pinchRelease handles a contact lifting or being cancelled. It ends a pinch
// the contact belonged to, and reports whether the contact was spent on a
// pinch (its release must then not produce a tap, drag end or scroll).
func (h *TouchHandler) pinchRelease(tapID int) bool {
	delete(h.contacts, tapID)
	h.endPinchOf(tapID)
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

// dissolveTwoFinger abandons an undecided two-finger gesture: both contacts
// stop being spent and go down again as single contacts where they first
// touched, so the moving one drags or scrolls as it would have alone.
func (h *TouchHandler) dissolveTwoFinger() {
	p := h.pinch
	h.pinch = nil
	for i, id := range p.ids {
		delete(h.pinchSpent, id)
		h.tapDownSingle(p.down[i], id)
	}
}

// movedTogether reports whether both fingers moved, in roughly the same
// direction — a pan — rather than one moving beside a resting one.
func movedTogether(a0, a, b0, b geo.Position) bool {
	ax, ay := a.X-a0.X, a.Y-a0.Y
	bx, by := b.X-b0.X, b.Y-b0.Y
	half := float32(twoFingerSlop / 2)
	if distance(a0, a) < half || distance(b0, b) < half {
		return false
	}
	return ax*bx+ay*by > 0
}

func midpoint(a, b geo.Position) geo.Position {
	return geo.NewPos((a.X+b.X)/2, (a.Y+b.Y)/2)
}

func distance(a, b geo.Position) float32 {
	return float32(math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)))
}
