package common

import (
	"math"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// Two-finger recognition (#379, #398). TouchHandler tracks every contact that
// is down. When a second finger lands while one is already down, the two form
// a two-finger gesture, which becomes a pinch or a pan once the fingers move
// (see pinchState). Moves of either finger then dispatch Pinched, or scroll,
// and lifting or cancelling either one ends the gesture. Both contacts are
// then spent — neither produces a tap, drag or scroll afterwards, and neither
// counts as down any more, so the surviving finger cannot start a second
// gesture with a finger that lands later.
//
// Over a Pinchable the gesture takes both fingers at once: whatever the first
// finger had started (drag, scroll, pending long-press) is cancelled. Without
// one it can only become a pan, so until it does both fingers carry on as
// single contacts — a finger tapping beside a resting thumb still taps — and
// it is dropped if they do not move together.
//
// Contacts are counted, so a release the platform never delivers would leave
// a contact down for good and pair it with every later finger. The gesture's
// own fingers are therefore dropped from contacts as soon as it ends, and a
// fresh down on an id clears whatever that id was spent on before — and ends
// the gesture it belonged to, whose release was lost.

// A two-finger gesture is classified once the fingers have moved (#398): once
// their distance or their midpoint has moved twoFingerSlop, a distance change
// at least as large as the midpoint's movement makes it a pinch (over a
// Pinchable; a one-handed pinch moves one finger, so the midpoint moves half
// as far as the distance changes), and anything else a pan, which scrolls like
// a one-finger flick (ScrollCallback, ScrollEndCallback) from the gesture's
// starting point. Scrolls pass from the widget under the fingers out to the
// scroll containers around it, so two fingers scroll a page past a widget that
// keeps one-finger drags for itself (a chart, the data grid).
//
// Platforms report each finger's move separately, so the move that crosses the
// slop has moved one finger only. The decision waits for the next move: from
// the other finger, which brings both positions up to date, or from the same
// one again, which shows the other is resting.
//
// A driver without a ScrollCallback cannot pan: two fingers over a Pinchable
// pinch from their first move, and without one they form no gesture at all.
// Every driver sets one now; the native mobile driver did not until #402.
type pinchState struct {
	target event.Pinchable // nil when no Pinchable is under the fingers
	ids    [2]int
	dist   float32      // finger distance at the previous Pinched (fixed while undecided)
	origin geo.Position // canvas position of the target's origin

	mode    twoFingerMode
	pending bool            // the slop was crossed; decide on the next move
	down    [2]geo.Position // where each finger was when the second one landed
	start   geo.Position    // midpoint when the second finger landed: the pan's scroll origin
	lastMid geo.Position    // midpoint at the previous pan step
	steps   [2]geo.Delta    // each finger's latest pan step; their sum seeds momentum
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

// tentative reports whether the gesture has not taken its fingers yet: no
// Pinchable is under them and they have not started to pan.
func (p *pinchState) tentative() bool {
	return p.target == nil && p.mode == twoFingerUndecided
}

// trackContact records a contact that went down, for pinch recognition. A
// down on an id means any earlier contact with that id is over, whether or not
// its release arrived: a gesture it was part of has ended, without momentum,
// and the id is no longer spent.
func (h *TouchHandler) trackContact(tapID int, pos geo.Position) {
	h.endPinchOf(tapID, false)
	delete(h.pinchSpent, tapID)
	h.contacts[tapID] = pos
}

// endPinchOf ends the two-finger gesture in progress if tapID is one of its
// fingers. Both fingers stop counting as down; they stay spent until they are
// released. A pan seeds momentum only if released is set: a system cancel, or
// a down that shows a release was lost, ends it where it stands (see
// ScrollEndCallback). A tentative gesture is simply dropped — its fingers were
// single contacts all along.
func (h *TouchHandler) endPinchOf(tapID int, released bool) {
	p := h.pinch
	if p == nil || (tapID != p.ids[0] && tapID != p.ids[1]) {
		return
	}
	h.pinch = nil
	if p.tentative() {
		return
	}
	delete(h.contacts, p.ids[0])
	delete(h.contacts, p.ids[1])
	switch p.mode {
	case twoFingerPinch:
		p.target.PinchEnded()
	case twoFingerPan:
		if released && h.ScrollEndCallback != nil {
			h.ScrollEndCallback(p.start, geo.Delta{DX: p.steps[0].DX + p.steps[1].DX, DY: p.steps[0].DY + p.steps[1].DY})
		}
	}
}

// startPinch turns the contact that just went down and the one already down
// into a two-finger gesture, if there are exactly two. A Pinchable under their
// midpoint, if any, is the pinch target. It reports whether the gesture took
// the new contact: without a Pinchable it is tentative, and both contacts go
// on as single ones until it turns into a pan.
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
	canPan := h.ScrollCallback != nil
	// Without a Pinchable the gesture can only pan. A driver that cannot pan
	// forms none, and neither does a first finger that already latched its own
	// drag or scroll: both fingers are then handled as single contacts.
	if target == nil && (!canPan || (h.Dragging != nil && h.draggingID == other) || (h.scrolling && h.scrollingID == other)) {
		return false
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
	if target == nil {
		return false
	}
	if !canPan {
		h.pinch.mode = twoFingerPinch
	}
	h.takeContact(other, a)
	h.pinchSpent[tapID] = true
	return true
}

// takeContact ends whatever a contact had started on its own — a touch, a
// pending long-press, a drag or scroll it latched — because it is now half of
// a two-finger gesture, and marks it spent.
func (h *TouchHandler) takeContact(tapID int, pos geo.Position) {
	h.hideToolTip()
	h.cancelLongPress(tapID)
	if wid := h.Touched[tapID]; wid != nil {
		touchEv := &mobile.TouchEvent{}
		touchEv.Position = pos
		touchEv.AbsolutePosition = pos
		wid.TouchCancel(touchEv)
		h.Touched[tapID] = nil
	}
	if h.Dragging != nil && h.draggingID == tapID {
		h.Dragging.DragEnd()
		h.Dragging = nil
		h.draggingID = 0
	}
	if h.scrolling && h.scrollingID == tapID {
		h.scrolling = false
		h.scrollingID = 0
	}
	h.pinchSpent[tapID] = true
}

// movePinch handles a move of finger tapID of the two-finger gesture:
// classifying it once the fingers have moved, then pinching or panning. It
// reports false when the gesture dissolved instead — the move must then be
// handled as an ordinary single-finger one.
func (h *TouchHandler) movePinch(tapID int) bool {
	p := h.pinch
	a, b := h.contacts[p.ids[0]], h.contacts[p.ids[1]]
	mid := midpoint(a, b)
	if p.mode == twoFingerUndecided {
		spread := float32(math.Abs(float64(distance(a, b) - p.dist)))
		moved := distance(mid, p.start)
		switch {
		case spread < twoFingerSlop && moved < twoFingerSlop:
			return true
		case !p.pending:
			p.pending = true // decide on the next move, once both positions are current
			return true
		case p.target != nil && spread >= moved:
			p.mode = twoFingerPinch
		case p.target != nil || movedTogether(p.down[0], a, p.down[1], b):
			p.mode = twoFingerPan
			if p.target == nil {
				h.takeContact(p.ids[0], a)
				h.takeContact(p.ids[1], b)
			}
		default:
			// No Pinchable and only one finger moving: not a two-finger
			// gesture after all (a resting palm or thumb beside a dragging
			// finger). Both contacts carry on as the single ones they were.
			h.pinch = nil
			return false
		}
	}
	if p.mode == twoFingerPan {
		delta := geo.Delta{DX: mid.X - p.lastMid.X, DY: mid.Y - p.lastMid.Y}
		p.lastMid = mid
		if delta == (geo.Delta{}) {
			return true
		}
		// Each finger's move shifts the midpoint by half of it, so one finger's
		// step is half the frame's motion; the two together are all of it.
		if tapID == p.ids[0] {
			p.steps[0] = delta
		} else {
			p.steps[1] = delta
		}
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

// pinchMove handles a move of a contact that is, or was, part of a two-finger
// gesture, and reports whether it did (the normal drag/scroll handling must
// then not run).
func (h *TouchHandler) pinchMove(pos geo.Position, tapID int) bool {
	if _, down := h.contacts[tapID]; down {
		h.contacts[tapID] = pos
	}
	if p := h.pinch; p != nil && (tapID == p.ids[0] || tapID == p.ids[1]) {
		if h.movePinch(tapID) {
			return true
		}
	}
	return h.pinchSpent[tapID]
}

// pinchRelease handles a contact lifting (released) or being cancelled. It
// ends a two-finger gesture the contact belonged to, and reports whether the
// contact was spent on one (its release must then not produce a tap, drag end
// or scroll).
func (h *TouchHandler) pinchRelease(tapID int, released bool) bool {
	delete(h.contacts, tapID)
	h.endPinchOf(tapID, released)
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
