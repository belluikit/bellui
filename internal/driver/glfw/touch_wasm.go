//go:build wasm

package glfw

import (
	"syscall/js"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"
)

// installTouchInput feeds finger contacts on the canvas into the shared touch
// gesture handler (touch.go), so drags, flick scrolling, long-press and
// double-tap work in phone and tablet browsers (#378). glfw-js has no touch
// support: until now only the mouse click a browser synthesises after a tap
// reached the app, and every drag went to the browser, which scrolled or
// zoomed the page instead.
//
// It uses Pointer Events and handles only pointerType "touch"; mouse and pen
// keep arriving through glfw-js's mouse callbacks. Cancelling pointerdown
// suppresses the browser's compatibility mouse events, so a tap is delivered
// once, by the touch handler. touch-action: none stops the browser panning and
// zooming the page, which also stops the address bar toggling (and resizing
// the canvas) when the user swipes inside the app.
func installTouchInput(w *window) {
	canvas := w.viewport.GetCanvas()
	style := canvas.Get("style")
	style.Call("setProperty", "touch-action", "none")
	// An inline canvas leaves a descender gap below it, which made the page a
	// few pixels taller than the viewport and therefore scrollable.
	style.Call("setProperty", "display", "block")

	listen := func(name string, phase touchPhase) {
		canvas.Call("addEventListener", name, js.FuncOf(func(_ js.Value, args []js.Value) any {
			ev := args[0]
			if ev.Get("pointerType").String() != "touch" {
				return nil
			}
			if ev.Get("cancelable").Bool() {
				ev.Call("preventDefault")
			}
			id := ev.Get("pointerId").Int()
			if phase == touchBegin {
				// Keep receiving this contact's moves and its end even when the
				// finger leaves the canvas.
				canvas.Call("setPointerCapture", id)
			}
			dpr := js.Global().Get("devicePixelRatio").Float()
			x := ev.Get("clientX").Float() * dpr // same space as glfw-js's mouse positions
			y := ev.Get("clientY").Float() * dpr
			runOnMainAsync(func() {
				pos := geo.NewPos(
					scale.ToBelluiCoordinate(w.canvas, int(w.scaleInput(x))),
					scale.ToBelluiCoordinate(w.canvas, int(w.scaleInput(y))),
				)
				w.handleTouch(id, phase, pos)
			})
			return nil
		}), map[string]any{"passive": false})
	}
	listen("pointerdown", touchBegin)
	listen("pointermove", touchMove)
	listen("pointerup", touchEnd)
	listen("pointercancel", touchCancel)
}
