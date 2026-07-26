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

	Touched         map[int]mobile.Touchable
	TouchCancelFunc context.CancelFunc
	TouchCancelLock sync.Mutex
	TouchLastTapped core.CanvasObject
	TouchTapCount   int

	// OnTapUp is an optional hook called at the start of TapUp, before
	// tap/drag processing. If it returns true, tap processing is skipped
	// (the hook handled the event). Used by mobile for menu dismissal.
	OnTapUp func(pos geo.Position) bool

	// LongPressCallback, when set by the driver, enables hold-fire long-press
	// (#758): a stationary touch dispatches LongPressed at TapSecondaryDelay
	// while the finger is still down — native mobile behavior — instead of on
	// release. The release of a hold-fired touch then dispatches
	// SecondaryTappable, so a widget implementing both gets "select on hold,
	// menu on lift". Targets that are not LongPressable keep the release-time
	// handling in TapUp unchanged.
	LongPressCallback func(event.LongPressable, *event.PointEvent)

	// longPressSeq/longPressFired are guarded by TouchCancelLock: the hold
	// timer fires off the event goroutine (like waitForDoubleTap) before
	// hopping back via DoFromGoroutine.
	longPressSeq   map[int]int  // arm generation per touch; bumping cancels a pending fire
	longPressFired map[int]bool // hold-fire happened for this touch
}

// InitTouchHandler initializes the maps. Call from the canvas constructor.
func (h *TouchHandler) InitTouchHandler(finder ObjectFinder) {
	h.Finder = finder
	h.LastTapDown = make(map[int]time.Time)
	h.LastTapDownPos = make(map[int]geo.Position)
	h.LastTapDelta = make(map[int]geo.Delta)
	h.Touched = make(map[int]mobile.Touchable)
	h.longPressSeq = make(map[int]int)
	h.longPressFired = make(map[int]bool)
}

// TapDown handles a pointer-down event.
func (h *TouchHandler) TapDown(pos geo.Position, tapID int) {
	h.LastTapDown[tapID] = time.Now()
	h.LastTapDownPos[tapID] = pos
	h.Dragging = nil

	// Tap-through overlays (#472/#569): when the top overlay opted in and the
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

	// Long-press tooltip (#827): arm the ToolTipable under the finger. The
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
	h.longPressSeq[tapID]++
	seq := h.longPressSeq[tapID]
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
	current := h.longPressSeq[tapID]
	h.TouchCancelLock.Unlock()
	if current != seq || h.Dragging != nil {
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

// cancelLongPress invalidates any pending hold-fire for the touch.
func (h *TouchHandler) cancelLongPress(tapID int) {
	if h.longPressSeq == nil {
		return
	}
	h.TouchCancelLock.Lock()
	h.longPressSeq[tapID]++
	h.TouchCancelLock.Unlock()
}

// hideToolTip dismisses any pending or visible long-press tooltip (#827). Called
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
	previousPos := h.LastTapDownPos[tapID]
	deltaX := pos.X - previousPos.X
	deltaY := pos.Y - previousPos.Y

	if h.Dragging == nil && (math.Abs(float64(deltaX)) < TapMoveThreshold && math.Abs(float64(deltaY)) < TapMoveThreshold) {
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
		// Latch the drag target at the gesture origin, not the current finger
		// position (matching the desktop driver): a fast first move can jump
		// clear across a small Draggable — a selection handle, a slider thumb —
		// and would otherwise start dragging whatever lies beyond it (#945
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
			h.DragOffset = previousPos.Subtract(dragPos)
			h.DragStart = dragCo.Position()
		} else {
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

// TapUp handles a pointer-up event (tap, secondary tap, long-press, or drag end).
func (h *TouchHandler) TapUp(pos geo.Position, tapID int,
	tapCallback func(event.Tappable, *event.PointEvent),
	tapAltCallback func(event.SecondaryTappable, *event.PointEvent),
	longPressCallback func(event.LongPressable, *event.PointEvent),
	doubleTapCallback func(event.DoubleTappable, *event.PointEvent),
	dragCallback func(event.Draggable, *event.DragEvent),
) {
	// The gesture is over: dismiss any long-press tooltip it summoned (#827)
	// and cancel any pending hold-fire, whatever kind of tap/drag it turns
	// out to be.
	h.hideToolTip()
	h.cancelLongPress(tapID)
	h.TouchCancelLock.Lock()
	longPressed := h.longPressFired[tapID]
	delete(h.longPressFired, tapID)
	h.TouchCancelLock.Unlock()

	// Pre-hook for platform-specific handling (e.g., mobile menu dismissal).
	if h.OnTapUp != nil && h.OnTapUp(pos) {
		return
	}

	if h.Dragging != nil {
		previousDelta := h.LastTapDelta[tapID]
		ev := &event.DragEvent{Dragged: previousDelta}
		draggedObjDelta := h.DragStart.Subtract(h.Dragging.(core.CanvasObject).Position())
		ev.Position = pos.Subtract(h.DragOffset).Add(draggedObjDelta)
		ev.AbsolutePosition = pos
		dragCallback(h.Dragging, ev)

		h.Dragging = nil
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
			h.TouchCancelLock.Unlock()
			if cancel != nil {
				cancel()
				return
			}
			go h.waitForDoubleTap(co, ev, tapCallback, doubleTapCallback)
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

func (h *TouchHandler) waitForDoubleTap(co core.CanvasObject, ev *event.PointEvent,
	tapCallback func(event.Tappable, *event.PointEvent),
	doubleTapCallback func(event.DoubleTappable, *event.PointEvent),
) {
	ctx, cancel := context.WithDeadline(context.TODO(), time.Now().Add(TapDoubleDelay))
	h.TouchCancelLock.Lock()
	h.TouchCancelFunc = cancel
	h.TouchCancelLock.Unlock()
	defer cancel()

	<-ctx.Done()
	bellui.CurrentApp().Driver().DoFromGoroutine(func() {
		h.TouchCancelLock.Lock()
		touchCount := h.TouchTapCount
		touchLast := h.TouchLastTapped
		h.TouchCancelLock.Unlock()

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
