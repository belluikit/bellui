//go:build !wasm

package glfw

import (
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
// origin-latched drags, double-tap, and hold-fire long-press (pre-rebrand tracker #758), the
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
// bellui coordinates and the glfw phase to touchPhase, then dispatches through
// the shared handleTouch (touch.go).
func (w *window) touchEvent(_ *glfw.Window, id int, phase glfw.TouchPhase, x, y float64) {
	pos := geo.NewPos(scale.ToBelluiCoordinate(w.canvas, int(x)), scale.ToBelluiCoordinate(w.canvas, int(y)))

	switch phase {
	case glfw.TouchBegin:
		w.handleTouch(id, touchBegin, pos)
	case glfw.TouchMove:
		w.handleTouch(id, touchMove, pos)
	case glfw.TouchEnd:
		w.handleTouch(id, touchEnd, pos)
	case glfw.TouchCancel:
		w.handleTouch(id, touchCancel, pos)
	}
}
