//go:build !wasm

package glfw

import (
	"math"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/internal/scale"

	"github.com/belluikit/glfw"
)

// Desktop touchscreen gesture dispatch (#266). The glfw fork's touch callback
// (belluikit/glfw#1, v1.10.0) reports touchscreen contacts independently of
// the emulated mouse on Windows (WM_POINTER) and Wayland (wl_touch); on X11
// and macOS it never fires and touch keeps arriving as an emulated mouse.
// The contacts feed the same common.TouchHandler the mobile driver runs, so
// desktop touch gets mobile's gesture vocabulary — slop-cancelled taps,
// origin-latched drags, double-tap, and hold-fire long-press (#758), the
// #259 selection-mode entry that was unreachable on desktop touchscreens.
//
// Deliberate consequences and limits:
//   - On Windows, registering the callback suppresses the system's legacy
//     mouse emulation for touches (tap-to-left-click and press-and-hold
//     right-click promotion) — bellui now owns those gestures: tap → Tapped,
//     hold-fire → LongPressed with the release offering TappedSecondary.
//   - Touch dispatches the mobile vocabulary, not the desktop-mouse one:
//     no Hoverable, no desktop.Mouseable, no cursor updates — exactly like
//     the mobile driver. Widgets needing raw contacts implement
//     mobile.Touchable, which is honoured here too.
//   - Touch drags move Draggables (scrollbar thumbs, sliders, selection
//     handles) without the mouse path's DnD ghost/drag-out, and without
//     mobile's momentum decay — desktop Draggables never had momentum.
//   - Content flick-scrolls under touch (#291): a drag that latches no
//     Draggable but starts over scrollable content dispatches touch-sourced
//     ScrollEvents through the same nested-propagation chain as the wheel,
//     with mobile's momentum decay on release. Mouse drags keep NOT
//     scrolling content — direct manipulation is a touch idiom; the mouse
//     has the wheel, the scrollbar and keyboard paging.
//
// All of this runs on the glfw main thread, like the mouse callbacks; the
// touch state is only ever touched from there.

// touchEvent is the glfw callback. It converts the content-area position to
// bellui coordinates and routes by phase, ignoring ids that never began — a
// callback set or removed while contacts are down truncates their sequences
// (see glfw.SetTouchCallback), and a rare system cancel may carry an unknown
// id too.
func (w *window) touchEvent(_ *glfw.Window, id int, phase glfw.TouchPhase, x, y float64) {
	pos := geo.NewPos(scale.ToBelluiCoordinate(w.canvas, int(x)), scale.ToBelluiCoordinate(w.canvas, int(y)))

	switch phase {
	case glfw.TouchBegin:
		w.ensureTouchHandler()
		w.touchScrollGen++ // touch-to-stop: a new contact halts any in-flight flick momentum
		w.touchActive[id] = true
		w.hideToolTip() // a press dismisses any visible/pending hover tooltip
		w.touch.TapDown(pos, id)
	case glfw.TouchMove:
		if !w.touchActive[id] {
			return
		}
		w.touch.TapMove(pos, id, func(wid event.Draggable, ev *event.DragEvent) {
			wid.Dragged(ev)
		})
	case glfw.TouchEnd:
		if !w.touchActive[id] {
			return
		}
		delete(w.touchActive, id)
		w.processTouchEnd(pos, id)
	case glfw.TouchCancel:
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
