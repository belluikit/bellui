package common

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// Tap handling constants shared by mobile and embedded drivers.
const (
	TapMoveDecay        = 0.92                   // scroll continue decay per frame
	TapMoveEndThreshold = 2.0                    // offset threshold to stop decay
	TapMoveThreshold    = 4.0                    // movement before drag begins
	TapSecondaryDelay   = 300 * time.Millisecond // long-press for secondary tap
	TapDoubleDelay      = 500 * time.Millisecond // max gap between double-tap
)

// ObjectFinder locates objects at a position on a canvas. This abstracts
// the difference between mobile (uses overlay stack) and embedded
// (uses FindObjectAtPositionMatching directly).
type ObjectFinder interface {
	FindObjectAtPositionMatching(pos geo.Position, test func(core.CanvasObject) bool) (core.CanvasObject, geo.Position, int)
	CanvasForTouch() core.Canvas
}

// TouchHandler implements tap, drag, and double-tap event handling logic
// shared between mobile and embedded drivers. Embed this struct and provide
// an ObjectFinder.
type TouchHandler struct {
	Finder ObjectFinder

	LastTapDown    map[int]time.Time
	LastTapDownPos map[int]geo.Position
	LastTapDelta   map[int]geo.Delta

	DragOffset geo.Position
	DragStart  geo.Position
	Dragging   event.Draggable
	// draggingID is the contact that latched Dragging, so a cancel of some
	// other contact — a palm rejected while a finger drags (#266) — does not
	// end the drag that contact never owned.
	draggingID int

	Touched         map[int]mobile.Touchable
	TouchCancelFunc context.CancelFunc
	TouchCancelLock sync.Mutex
	TouchLastTapped core.CanvasObject
	TouchTapCount   int

	// tapGeneration is bumped by CancelPendingTap. A waitForDoubleTap
	// goroutine captures it when it starts and refuses to dispatch once it
	// has moved on: the goroutine cannot be interrupted while it waits, so
	// this is how a teardown tells it that the object it captured is no
	// longer anyone's to call (#310).
	tapGeneration uint64

	// OnTapUp is an optional hook called at the start of TapUp, before
	// tap/drag processing. If it returns true, tap processing is skipped
	// (the hook handled the event). Used by mobile for menu dismissal.
	OnTapUp func(pos geo.Position) bool

	// ScrollCallback, when set by the driver, enables touch flick-scrolling of
	// scrollable content (#291): a move past the tap slop that latches no
	// Draggable but began over an event.Scrollable becomes a scroll gesture —
	// every move calls this with the gesture origin and the move's delta, and
	// the driver turns it into a touch-sourced ScrollEvent (which buys nested
	// propagation from its scroll dispatch). Drivers whose Scroll is itself
	// Draggable (mobile) leave this nil and nothing changes for them.
	ScrollCallback func(origin geo.Position, delta geo.Delta)
	// ScrollEndCallback is called when a scroll gesture's contact lifts, with
	// the last move's delta as the momentum seed. A system cancel ends the
	// gesture without it — momentum from a contact the system took over would
	// keep content moving after a palm rejection.
	ScrollEndCallback func(origin geo.Position, delta geo.Delta)

	// scrolling/scrollingID/scrollOrigin latch a scroll gesture the way
	// Dragging/draggingID latch a drag: one session at a time, owned by the
	// contact that started it.
	scrolling    bool
	scrollingID  int
	scrollOrigin geo.Position

	// LongPressCallback, when set by the driver, enables hold-fire long-press
	// (pre-rebrand tracker #758): a stationary touch dispatches LongPressed at TapSecondaryDelay
	// while the finger is still down — native mobile behavior — instead of on
	// release. The release of a hold-fired touch then dispatches
	// SecondaryTappable, so a widget implementing both gets "select on hold,
	// menu on lift". Targets that are not LongPressable keep the release-time
	// handling in TapUp unchanged.
	LongPressCallback func(event.LongPressable, *event.PointEvent)

	// longPressGen/longPressArmed/longPressFired are guarded by
	// TouchCancelLock: the hold timer fires off the event goroutine (like
	// waitForDoubleTap) before hopping back via DoFromGoroutine.
	longPressGen   int          // monotonic arm generation, never reused
	longPressArmed map[int]int  // generation of the pending fire per touch; dropping it cancels the fire
	longPressFired map[int]bool // hold-fire happened for this touch

	// contacts, pinch and pinchSpent recognise two-finger pinches (pinch.go,
	// #379): every contact that is down, the pinch in progress, and the
	// contacts used up by a pinch, whose release must not produce a tap.
	contacts   map[int]geo.Position
	pinch      *pinchState
	pinchSpent map[int]bool
}

// InitTouchHandler initializes the maps. Call from the canvas constructor.
func (h *TouchHandler) InitTouchHandler(finder ObjectFinder) {
	h.Finder = finder
	h.LastTapDown = make(map[int]time.Time)
	h.LastTapDownPos = make(map[int]geo.Position)
	h.LastTapDelta = make(map[int]geo.Delta)
	h.Touched = make(map[int]mobile.Touchable)
	h.longPressArmed = make(map[int]int)
	h.longPressFired = make(map[int]bool)
	h.contacts = make(map[int]geo.Position)
	h.pinchSpent = make(map[int]bool)
}

// TapDown handles a pointer-down event.
func (h *TouchHandler) TapDown(pos geo.Position, tapID int) {
	h.trackContact(tapID, pos)
	if h.startPinch(tapID) {
		return
	}
	h.LastTapDown[tapID] = time.Now()
	h.LastTapDownPos[tapID] = pos
	h.Dragging = nil
	h.scrolling = false
	h.scrollingID = 0

	// Tap-through overlays (pre-rebrand tracker #472/#569): when the top overlay opted in and the
	// press lands outside its content, dismiss it now so the hit tests below
	// (and the following TapUp) resolve this same press against the layer
	// beneath — the mobile mirror of the desktop driver's
	// dismissTapThroughOverlay. Matched structurally to avoid a widget import.
	if c := h.Finder.CanvasForTouch(); c != nil {
		if td, ok := c.Overlays().Top().(interface {
			TapThroughDismiss(geo.Position) bool
		}); ok {
			td.TapThroughDismiss(pos)
		}
	}

	// Long-press tooltip (pre-rebrand tracker #827): arm the ToolTipable under the finger. The
	// tooltip controller's own show-delay doubles as the press-and-hold
	// threshold — a quick tap's TapUp cancels it before it appears, while a hold
	// past the delay reveals the bubble (the touch mirror of the desktop hover
	// path, which shows a tooltip after the same delay). A nil target clears any
	// stale tooltip from a previous touch.
	if c := h.Finder.CanvasForTouch(); c != nil {
		tipObj, _, _ := h.Finder.FindObjectAtPositionMatching(pos, func(object core.CanvasObject) bool {
			tt, ok := object.(core.ToolTipable)
			return ok && tt.ToolTip() != ""
		})
		core.NotifyToolTip(tipObj, c)
	}

	co, objPos, layer := h.Finder.FindObjectAtPositionMatching(pos, func(object core.CanvasObject) bool {
		switch object.(type) {
		case mobile.Touchable, event.Focusable:
			return true
		}
		return false
	})

	if wid, ok := co.(mobile.Touchable); ok {
		touchEv := &mobile.TouchEvent{}
		touchEv.Position = objPos
		touchEv.AbsolutePosition = pos
		wid.TouchDown(touchEv)
		h.Touched[tapID] = wid
	}

	if layer != 1 { // 0 - overlay, 1 - window head / menu, 2 - content
		canvas := h.Finder.CanvasForTouch()
		if wid, ok := co.(event.Focusable); !ok || wid != canvas.Focused() {
			// Objects that adjust the focused widget (selection handles) opt
			// out of the unfocus, so touching them doesn't yank the keyboard
			// away from the editor they serve. Matched structurally to avoid
			// a widget import (like TapThroughDismiss above).
			if rf, ok := co.(interface{ RetainsFocus() bool }); !ok || !rf.RetainsFocus() {
				canvas.Unfocus()
			}
		}
	}

	h.armLongPress(pos, tapID)
}

// armLongPress schedules a hold-fire long-press for the touch that just went
// down. The pending fire is cancelled by lifting the finger or moving it past
// the tap slop; see LongPressCallback.
func (h *TouchHandler) armLongPress(pos geo.Position, tapID int) {
	if h.LongPressCallback == nil {
		return
	}
	h.TouchCancelLock.Lock()
	h.longPressGen++
	seq := h.longPressGen
	h.longPressArmed[tapID] = seq
	delete(h.longPressFired, tapID)
	h.TouchCancelLock.Unlock()
	time.AfterFunc(TapSecondaryDelay, func() {
		bellui.CurrentApp().Driver().DoFromGoroutine(func() {
			h.fireLongPress(pos, tapID, seq)
		}, true)
	})
}

// fireLongPress runs on the event goroutine when the hold timer elapses. It
// dispatches LongPressed to the LongPressable under the still-stationary
// finger, if any; otherwise the touch keeps its release-time handling.
func (h *TouchHandler) fireLongPress(pos geo.Position, tapID, seq int) {
	h.TouchCancelLock.Lock()
	current, armed := h.longPressArmed[tapID]
	if armed && current == seq {
		delete(h.longPressArmed, tapID) // this timer has resolved, whatever it decides below
	}
	h.TouchCancelLock.Unlock()
	if !armed || current != seq || h.Dragging != nil {
		return // lifted, moved, or re-pressed since armed
	}

	co, objPos, _ := h.Finder.FindObjectAtPositionMatching(pos, func(object core.CanvasObject) bool {
		_, ok := object.(event.LongPressable)
		return ok
	})
	wid, ok := co.(event.LongPressable)
	if !ok {
		return
	}

	h.TouchCancelLock.Lock()
	h.longPressFired[tapID] = true
	h.TouchCancelLock.Unlock()
	ev := &event.PointEvent{
		Position:         objPos,
		AbsolutePosition: pos,
	}
	h.LongPressCallback(wid, ev)
}

// cancelLongPress invalidates any pending hold-fire for the touch. The armed
// generation is dropped rather than bumped, so the entry does not outlive the
// contact — and since generations are never reused, a timer armed before this
// call can never match a later arming of the same id.
func (h *TouchHandler) cancelLongPress(tapID int) {
	if h.longPressArmed == nil {
		return
	}
	h.TouchCancelLock.Lock()
	delete(h.longPressArmed, tapID)
	h.TouchCancelLock.Unlock()
}

// hideToolTip dismisses any pending or visible long-press tooltip (pre-rebrand tracker #827). Called
// when the touch ends or turns into a drag, so a tooltip never lingers past the
// gesture that summoned it.
func (h *TouchHandler) hideToolTip() {
	if c := h.Finder.CanvasForTouch(); c != nil {
		core.NotifyToolTip(nil, c)
	}
}

// TapMove handles a pointer-move event (potential drag).
func (h *TouchHandler) TapMove(pos geo.Position, tapID int,
	dragCallback func(event.Draggable, *event.DragEvent),
) {
	if h.pinchMove(pos, tapID) {
		return
	}
	previousPos := h.LastTapDownPos[tapID]
	deltaX := pos.X - previousPos.X
	deltaY := pos.Y - previousPos.Y

	scrollingThis := h.scrolling && h.scrollingID == tapID
	if h.Dragging == nil && !scrollingThis && (math.Abs(float64(deltaX)) < TapMoveThreshold && math.Abs(float64(deltaY)) < TapMoveThreshold) {
		return
	}

	// The finger moved past the tap slop — this is a drag/scroll, not a
	// press-and-hold, so cancel any pending tooltip and hold-fire long-press.
	h.hideToolTip()
	h.cancelLongPress(tapID)
	h.LastTapDownPos[tapID] = pos
	offset := geo.Delta{DX: deltaX, DY: deltaY}
	h.LastTapDelta[tapID] = offset

	co, objPos, _ := h.Finder.FindObjectAtPositionMatching(pos, func(object core.CanvasObject) bool {
		if _, ok := object.(event.Draggable); ok {
			return true
		} else if _, ok := object.(mobile.Touchable); ok {
			return true
		}
		return false
	})

	if h.Touched[tapID] != nil {
		if touch, ok := co.(mobile.Touchable); !ok || h.Touched[tapID] != touch {
			touchEv := &mobile.TouchEvent{}
			touchEv.Position = objPos
			touchEv.AbsolutePosition = pos
			h.Touched[tapID].TouchCancel(touchEv)
			h.Touched[tapID] = nil
		}
	}

	if h.Dragging == nil {
		if scrollingThis {
			h.ScrollCallback(h.scrollOrigin, offset)
			return
		}
		// Latch the drag target at the gesture origin, not the current finger
		// position (matching the desktop driver): a fast first move can jump
		// clear across a small Draggable — a selection handle, a slider thumb —
		// and would otherwise start dragging whatever lies beyond it (pre-rebrand tracker #945
		// device finding).
		dragCo, dragPos := co, objPos
		if originCo, originPos, _ := h.Finder.FindObjectAtPositionMatching(previousPos, func(object core.CanvasObject) bool {
			_, ok := object.(event.Draggable)
			return ok
		}); originCo != nil {
			dragCo, dragPos = originCo, originPos
		}
		if drag, ok := dragCo.(event.Draggable); ok {
			h.Dragging = drag
			h.draggingID = tapID
			h.DragOffset = previousPos.Subtract(dragPos)
			h.DragStart = dragCo.Position()
		} else {
			h.tryLatchScroll(previousPos, offset, tapID)
			return
		}
	}

	ev := &event.DragEvent{}
	draggedObjDelta := h.DragStart.Subtract(h.Dragging.(core.CanvasObject).Position())
	ev.Position = pos.Subtract(h.DragOffset).Add(draggedObjDelta)
	ev.AbsolutePosition = pos // the glfw driver sets it on mouse drags too
	ev.Dragged = offset

	dragCallback(h.Dragging, ev)
}

// tryLatchScroll begins a scroll gesture for a drag that latched no Draggable
// (#291): if the driver opted in and an event.Scrollable sits under the gesture
// origin, the contact becomes a scroll session and this first past-the-slop
// move is dispatched. The origin is latched like a drag target's — the finger
// may wander off the scroller mid-gesture and content keeps following it. Only
// the existence of a Scrollable is tested here; the driver's dispatch re-walks
// the scrollables under the origin per delta, propagating to outer scrollers
// when the inner one is at its boundary.
func (h *TouchHandler) tryLatchScroll(origin geo.Position, offset geo.Delta, tapID int) {
	if h.ScrollCallback == nil || h.scrolling {
		return
	}
	co, _, _ := h.Finder.FindObjectAtPositionMatching(origin, func(object core.CanvasObject) bool {
		_, ok := object.(event.Scrollable)
		return ok
	})
	if co == nil {
		return
	}
	h.scrolling = true
	h.scrollingID = tapID
	h.scrollOrigin = origin
	h.ScrollCallback(origin, offset)
}

// TapUp handles a pointer-up event (tap, secondary tap, long-press, or drag end).
func (h *TouchHandler) TapUp(pos geo.Position, tapID int,
	tapCallback func(event.Tappable, *event.PointEvent),
	tapAltCallback func(event.SecondaryTappable, *event.PointEvent),
	longPressCallback func(event.LongPressable, *event.PointEvent),
	doubleTapCallback func(event.DoubleTappable, *event.PointEvent),
	dragCallback func(event.Draggable, *event.DragEvent),
) {
	if h.pinchRelease(tapID) {
		return
	}
	// The gesture is over: dismiss any long-press tooltip it summoned (pre-rebrand tracker #827)
	// and cancel any pending hold-fire, whatever kind of tap/drag it turns
	// out to be.
	h.hideToolTip()
	h.cancelLongPress(tapID)
	h.TouchCancelLock.Lock()
	longPressed := h.longPressFired[tapID]
	delete(h.longPressFired, tapID)
	h.TouchCancelLock.Unlock()
	defer h.forgetTouch(tapID) // the contact is gone whichever branch dispatches it

	// Pre-hook for platform-specific handling (e.g., mobile menu dismissal).
	if h.OnTapUp != nil && h.OnTapUp(pos) {
		return
	}

	if h.scrolling && h.scrollingID == tapID {
		h.scrolling = false
		h.scrollingID = 0
		if h.ScrollEndCallback != nil {
			h.ScrollEndCallback(h.scrollOrigin, h.LastTapDelta[tapID])
		}
		return // a scroll gesture's release is not a tap
	}

	if h.Dragging != nil {
		previousDelta := h.LastTapDelta[tapID]
		ev := &event.DragEvent{Dragged: previousDelta}
		draggedObjDelta := h.DragStart.Subtract(h.Dragging.(core.CanvasObject).Position())
		ev.Position = pos.Subtract(h.DragOffset).Add(draggedObjDelta)
		ev.AbsolutePosition = pos
		dragCallback(h.Dragging, ev)

		h.Dragging = nil
		h.draggingID = 0
		return
	}

	duration := time.Since(h.LastTapDown[tapID])

	co, objPos, _ := h.Finder.FindObjectAtPositionMatching(pos, func(object core.CanvasObject) bool {
		if _, ok := object.(event.Tappable); ok {
			return true
		} else if _, ok := object.(event.SecondaryTappable); ok {
			return true
		} else if _, ok := object.(event.LongPressable); ok {
			return true
		} else if _, ok := object.(mobile.Touchable); ok {
			return true
		} else if _, ok := object.(event.DoubleTappable); ok {
			return true
		}
		return false
	})

	if wid, ok := co.(mobile.Touchable); ok {
		touchEv := &mobile.TouchEvent{}
		touchEv.Position = objPos
		touchEv.AbsolutePosition = pos
		wid.TouchUp(touchEv)
		h.Touched[tapID] = nil
	}

	ev := &event.PointEvent{
		Position:         objPos,
		AbsolutePosition: pos,
	}

	if longPressed {
		// The hold already dispatched LongPressed; releasing in place now
		// offers the secondary-tap surface (e.g. the Entry context menu),
		// matching the native pattern of the toolbar appearing on lift.
		if wid, ok := co.(event.SecondaryTappable); ok {
			tapAltCallback(wid, ev)
		}
		return
	}

	if duration < TapSecondaryDelay {
		_, doubleTap := co.(event.DoubleTappable)
		if doubleTap {
			h.TouchCancelLock.Lock()
			h.TouchTapCount++
			h.TouchLastTapped = co
			cancel := h.TouchCancelFunc
			// Read the generation HERE, not inside the goroutine: between
			// `go` and the goroutine's first statement a teardown can bump it,
			// and a generation captured after the bump looks current (#310).
			gen := h.tapGeneration
			h.TouchCancelLock.Unlock()
			if cancel != nil {
				cancel()
				return
			}
			go h.waitForDoubleTap(co, ev, gen, tapCallback, doubleTapCallback)
		} else {
			if wid, ok := co.(event.Tappable); ok {
				tapCallback(wid, ev)
			}
		}
	} else {
		if wid, ok := co.(event.LongPressable); ok {
			longPressCallback(wid, ev)
		} else if wid, ok := co.(event.SecondaryTappable); ok {
			tapAltCallback(wid, ev)
		}
	}
}

// TapCancel handles a system-cancelled touch sequence (the desktop driver's
// glfw TouchCancel, #266): the system took the contact over — e.g. a palm
// rejection or an edge gesture — so the gesture must end without dispatching
// any tap. A pending hold-fire and tooltip are cancelled, a Touchable gets
// TouchCancel, and a drag this contact latched gets DragEnd (the drag's effect
// so far stands; there is no undo protocol for Draggable) — a drag some other
// contact owns is left alone, which is the point of a palm rejection arriving
// mid-drag. pos is the last known contact position — it may be zero where the
// platform does not report one, so it is used for the cancel event only, never
// for hit testing.
func (h *TouchHandler) TapCancel(pos geo.Position, tapID int) {
	if h.pinchRelease(tapID) {
		return
	}
	h.hideToolTip()
	h.cancelLongPress(tapID)
	h.TouchCancelLock.Lock()
	delete(h.longPressFired, tapID)
	h.TouchCancelLock.Unlock()

	if wid := h.Touched[tapID]; wid != nil {
		touchEv := &mobile.TouchEvent{}
		touchEv.Position = pos
		touchEv.AbsolutePosition = pos
		wid.TouchCancel(touchEv)
	}
	if h.Dragging != nil && h.draggingID == tapID {
		h.Dragging.DragEnd()
		h.Dragging = nil
		h.draggingID = 0
	}
	if h.scrolling && h.scrollingID == tapID {
		// The scroll this contact latched ends where it stands — no momentum
		// seeded from a contact the system took over.
		h.scrolling = false
		h.scrollingID = 0
	}
	h.forgetTouch(tapID)
}

// forgetTouch drops the per-contact state of a finished sequence. Touch ids are
// not recycled everywhere the handler runs — Windows reports a fresh WM_POINTER
// id per contact (#266) — so entries left behind would grow these maps for the
// life of the process.
func (h *TouchHandler) forgetTouch(tapID int) {
	delete(h.LastTapDown, tapID)
	delete(h.LastTapDownPos, tapID)
	delete(h.LastTapDelta, tapID)
	delete(h.Touched, tapID)
}

// CancelPendingTap abandons a double-tap wait that has not fired yet. Call it
// when the canvas those taps were dispatched against goes away — a window
// closing, a driver shutting down.
//
// Cancelling the context alone is not enough and never was: that is how a
// SECOND tap tells the waiter to fire early as a double tap, so it makes the
// dispatch happen rather than preventing it. The generation bump is what
// distinguishes "fire now" from "do not fire at all".
func (h *TouchHandler) CancelPendingTap() {
	h.TouchCancelLock.Lock()
	cancel := h.TouchCancelFunc
	h.TouchCancelFunc = nil
	h.TouchTapCount = 0
	h.TouchLastTapped = nil
	h.tapGeneration++
	h.TouchCancelLock.Unlock()

	if cancel != nil {
		cancel() // wake the waiter; the generation check stops it dispatching
	}
}

// tapTargetLive reports whether co is still on the canvas the taps came from.
//
// A pending double-tap outlives its own gesture by TapDoubleDelay, and in that
// window the object can be dismissed — a dialog closed, an overlay popped —
// leaving a widget whose renderer the cache has destroyed. Calling Tapped on it
// then dereferences freed state (#310). Nothing cancels on that path, because
// the canvas itself is still alive, so the check has to be here.
func (h *TouchHandler) tapTargetLive(co core.CanvasObject) bool {
	if h.Finder == nil {
		return true // no finder to ask — behave as before
	}
	c := h.Finder.CanvasForTouch()
	if c == nil {
		return false
	}

	found := false
	seek := func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if o == co {
			found = true
		}
		return found
	}

	driver.WalkVisibleObjectTree(c.Content(), seek, nil)
	if found {
		return true
	}
	for _, o := range c.Overlays().List() {
		driver.WalkVisibleObjectTree(o, seek, nil)
		if found {
			return true
		}
	}
	return false
}

func (h *TouchHandler) waitForDoubleTap(co core.CanvasObject, ev *event.PointEvent, gen uint64,
	tapCallback func(event.Tappable, *event.PointEvent),
	doubleTapCallback func(event.DoubleTappable, *event.PointEvent),
) {
	ctx, cancel := context.WithDeadline(context.TODO(), time.Now().Add(TapDoubleDelay))
	h.TouchCancelLock.Lock()
	if h.tapGeneration != gen { // abandoned before this goroutine got going
		h.TouchCancelLock.Unlock()
		cancel()
		return
	}
	h.TouchCancelFunc = cancel
	h.TouchCancelLock.Unlock()
	defer cancel()

	<-ctx.Done()
	bellui.CurrentApp().Driver().DoFromGoroutine(func() {
		h.TouchCancelLock.Lock()
		stale := h.tapGeneration != gen
		touchCount := h.TouchTapCount
		touchLast := h.TouchLastTapped
		h.TouchCancelLock.Unlock()

		// Abandoned while we waited — the canvas these taps belonged to is
		// gone — or the object has since left the tree. Either way co is no
		// longer ours to call (#310).
		if stale || !h.tapTargetLive(co) {
			return
		}

		if touchCount == 2 && touchLast == co {
			if wid, ok := co.(event.DoubleTappable); ok {
				doubleTapCallback(wid, ev)
			}
		} else {
			if wid, ok := co.(event.Tappable); ok {
				tapCallback(wid, ev)
			}
		}

		h.TouchCancelLock.Lock()
		h.TouchTapCount = 0
		h.TouchCancelFunc = nil
		h.TouchLastTapped = nil
		h.TouchCancelLock.Unlock()
	}, true)
}

// DragDecay implements momentum-based scroll decay after a drag ends.
// Call this from a ticker/animation callback, passing the last drag delta.
// Returns the decayed delta, or zero if below threshold.
func DragDecay(delta geo.Delta) geo.Delta {
	dx := delta.DX * TapMoveDecay
	dy := delta.DY * TapMoveDecay
	if math.Abs(float64(dx)) < TapMoveEndThreshold && math.Abs(float64(dy)) < TapMoveEndThreshold {
		return geo.Delta{}
	}
	return geo.Delta{DX: dx, DY: dy}
}
