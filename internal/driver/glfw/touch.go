package glfw

import (
	"math"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
)

// Touch gesture dispatch shared by the desktop touchscreen path
// (touch_desktop.go, #266) and the browser (touch_wasm.go, #378). Each feeds
// raw contacts — an id, a phase and a position in bellui coordinates — into
// handleTouch, which runs them through common.TouchHandler, the mobile
// driver's gesture state machine. Main-goroutine state only.

// touchPhase is the platform-neutral phase of one contact.
type touchPhase int

const (
	touchBegin touchPhase = iota
	touchMove
	touchEnd
	touchCancel
)

// handleTouch routes one contact event, ignoring ids that never began — a
// callback set or removed while contacts are down truncates their sequences,
// and a rare system cancel may carry an unknown id too.
func (w *window) handleTouch(id int, phase touchPhase, pos geo.Position) {
	switch phase {
	case touchBegin:
		w.ensureTouchHandler()
		w.touchScrollGen++ // touch-to-stop: a new contact halts any in-flight flick momentum
		w.touchActive[id] = true
		w.hideToolTip() // a press dismisses any visible/pending hover tooltip
		w.touch.TapDown(pos, id)
	case touchMove:
		if !w.touchActive[id] {
			return
		}
		w.touch.TapMove(pos, id, func(wid event.Draggable, ev *event.DragEvent) {
			wid.Dragged(ev)
		})
	case touchEnd:
		if !w.touchActive[id] {
			return
		}
		delete(w.touchActive, id)
		w.processTouchEnd(pos, id)
	case touchCancel:
		if !w.touchActive[id] {
			return
		}
		delete(w.touchActive, id)
		w.touch.TapCancel(pos, id)
	}
}

// ensureTouchHandler initializes the touch state machine on the first
// contact. Most desktop sessions never see a touch, so the maps and the
// finder are not built with the window.
func (w *window) ensureTouchHandler() {
	if w.touchActive != nil {
		return
	}
	w.touchActive = make(map[int]bool)
	w.touch.InitTouchHandler(&touchFinder{w: w})
	w.touch.LongPressCallback = func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}
	w.touch.ScrollCallback = func(origin geo.Position, delta geo.Delta) {
		w.dispatchScroll(origin, delta, event.ScrollSourceTouch)
	}
	w.touch.ScrollEndCallback = w.startTouchScrollMomentum
}

// startTouchScrollMomentum continues a flick's scrolling past the release with
// mobile's decay curve (common.DragDecay at ~60 ticks/s), dispatching each
// decayed delta at the gesture origin through the same propagation chain the
// live gesture used. The loop stops when the delta decays out, when the scroll
// stops being consumed (boundary reached — content must not appear stuck to a
// wall it is being flung against), or when the generation moves on: a new
// contact (touch-to-stop) or window close. The generation is main-goroutine
// state; the timer goroutine only reads it inside runOnMain.
func (w *window) startTouchScrollMomentum(origin geo.Position, delta geo.Delta) {
	if math.Abs(float64(delta.DX)) <= common.TapMoveEndThreshold && math.Abs(float64(delta.DY)) <= common.TapMoveEndThreshold {
		return // a controlled drag's release, not a flick
	}
	gen := w.touchScrollGen
	go func() {
		for {
			time.Sleep(time.Millisecond * 16)
			delta = common.DragDecay(delta)
			if delta == (geo.Delta{}) {
				return
			}
			stop := false
			runOnMain(func() {
				if w.touchScrollGen != gen || w.isClosing() {
					stop = true
					return
				}
				if !w.dispatchScroll(origin, delta, event.ScrollSourceTouch) {
					stop = true
				}
			})
			if stop {
				return
			}
		}
	}()
}

// processTouchEnd dispatches the release through the shared handler with the
// desktop flavour of the callbacks: activation notification like the mouse
// click path, and a plain DragEnd — desktop drags (scrollbar thumbs, sliders)
// have no momentum, unlike the mobile driver's flick decay.
func (w *window) processTouchEnd(pos geo.Position, id int) {
	w.touch.TapUp(pos, id, func(wid event.Tappable, ev *event.PointEvent) {
		wid.Tapped(ev)
		if co, ok := wid.(core.CanvasObject); ok {
			core.NotifyActivation(co, false)
		}
	}, func(wid event.SecondaryTappable, ev *event.PointEvent) {
		wid.TappedSecondary(ev)
	}, func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}, func(wid event.DoubleTappable, ev *event.PointEvent) {
		wid.DoubleTapped(ev)
	}, func(wid event.Draggable, ev *event.DragEvent) {
		wid.DragEnd()
	})
}

// touchFinder adapts the window's canvas to the shared handler's
// ObjectFinder: hit testing resolves through the canvas — overlays first —
// exactly like the mouse path's findObjectAtPositionMatching.
type touchFinder struct {
	w *window
}

func (f *touchFinder) FindObjectAtPositionMatching(pos geo.Position, test func(core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	return f.w.findObjectAtPositionMatching(f.w.canvas, pos, test)
}

func (f *touchFinder) CanvasForTouch() core.Canvas {
	return f.w.canvas
}
