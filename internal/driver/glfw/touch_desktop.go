//go:build !wasm

package glfw

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
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
//     mobile's momentum decay. Content flick-scrolling is not wired: on
//     desktop builds Scroll deliberately does not implement Draggable
//     (scroller_mobile.go), and the legacy Windows emulation never provided
//     it either, so nothing regresses — whether desktop touch should scroll
//     content mobile-style is a separate UX decision.
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
