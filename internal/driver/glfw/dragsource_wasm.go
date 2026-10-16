//go:build wasm

package glfw

import (
	"image"
	"image/draw"
	"syscall/js"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalapp "github.com/belluikit/bellui/internal/app"
	internalclipboard "github.com/belluikit/bellui/internal/clipboard"
	"github.com/belluikit/bellui/internal/scale"
)

// Web (wasm/js) drag-out via the browser's HTML5 Drag and Drop API (pre-rebrand tracker #857).
//
// GLFW exposes no drag-source API, and unlike the desktop backends the web
// backend cannot use the push desktop.StartDrag path: the browser only starts a
// native drag from a user gesture, and once dragstart fires it stops delivering
// mousemove/mouseup on the source — so a widget's Dragged handler (the only
// place the push model produces the payload) never runs during the drag. The
// web backend is therefore pull-based (pre-rebrand tracker #874): a synchronous DOM mousedown
// listener hit-tests the bellui tree, and if the deepest event.Draggable under the
// pointer implements desktop.DragOutProvider it arms the canvas element
// (draggable=true) for that gesture; dragstart then pulls DragOutData() and
// populates dataTransfer / setDragImage; dragend reports the outcome. A plain
// draggable (slider, scrollbar, in-canvas DnD) keeps draggable=false, so in-app
// dragging is untouched.
//
// This is pure syscall/js in bellui's wasm driver — it does not touch glfw-js.
// The push startNativeDrag stays a no-op on wasm (dragsource_other.go returns
// false), because there is no native drag session to attach to when Dragged runs.

// dragOut holds the per-gesture drag-out state. The web target is single
// threaded and single window, so package-level state is safe (matching the
// clipboard and IME wasm code).
var dragOut struct {
	provider desktop.DragOutProvider // armed at mousedown, pulled at dragstart
	data     desktop.DragData        // pulled at dragstart, held for the dragend outcome
	active   bool                    // a native drag is in flight (payload accepted at dragstart)
}

// installNativeDragOut wires the HTML5 drag-and-drop listeners onto the window's
// canvas element. It is called once from window.create(). The js.Func callbacks
// live for the app lifetime (one window), so they are never released — matching
// connectComposition.
func installNativeDragOut(w *window) {
	canvasEl := w.viewport.GetCanvas()
	if !canvasEl.Truthy() {
		return
	}

	// mousedown runs synchronously inside the DOM dispatch — before the browser
	// decides whether the following mousemove starts a native drag — so it is the
	// only place we can arm draggable in time. glfw-js delivers its own mouse
	// events asynchronously (runOnMainAsync), which is too late.
	mousedown := js.FuncOf(func(_ js.Value, args []js.Value) any {
		armDragOut(w, canvasEl, args)
		return nil
	})
	canvasEl.Call("addEventListener", "mousedown", mousedown)

	dragstart := js.FuncOf(func(_ js.Value, args []js.Value) any {
		onDragStart(canvasEl, args)
		return nil
	})
	canvasEl.Call("addEventListener", "dragstart", dragstart)

	dragend := js.FuncOf(func(_ js.Value, args []js.Value) any {
		onDragEnd(canvasEl, args)
		return nil
	})
	canvasEl.Call("addEventListener", "dragend", dragend)
}

// armDragOut hit-tests the bellui tree at the pointer and arms a native drag when
// the deepest draggable implements desktop.DragOutProvider. It runs
// synchronously in the DOM mousedown callback; the hit-test is pure computation
// (no channel ops) so it is safe to run inline on the single-threaded web
// runtime without the runOnMainAsync deadlock the async callbacks avoid (pre-rebrand tracker #113).
func armDragOut(w *window, canvasEl js.Value, args []js.Value) {
	// Default to not-a-drag-source so ordinary in-canvas drags are untouched.
	canvasEl.Set("draggable", false)
	dragOut.provider = nil

	if len(args) == 0 {
		return
	}
	ev := args[0]
	if ev.Get("button").Int() != 0 { // primary button only
		return
	}

	pos := clientToBellui(w, ev)
	// Find the deepest event.Draggable under the pointer; only arm a native drag
	// when that top-most draggable is itself a DragOutProvider — so a plain
	// draggable sitting over a provider still gets its in-canvas drag.
	obj, _, _ := w.findObjectAtPositionMatching(w.canvas, pos, func(o core.CanvasObject) bool {
		_, ok := o.(event.Draggable)
		return ok
	})
	if p, ok := obj.(desktop.DragOutProvider); ok {
		dragOut.provider = p
		canvasEl.Set("draggable", true)
	}
}

// clientToBellui converts a mouse event's client coordinates into bellui canvas
// coordinates, matching glfw-js's own cursor math (clientX×devicePixelRatio,
// then scale.ToBelluiCoordinate) so the hit-test agrees with the async mouse path.
func clientToBellui(w *window, ev js.Value) geo.Position {
	dpr := js.Global().Get("devicePixelRatio").Float()
	px := int(ev.Get("clientX").Float() * dpr)
	py := int(ev.Get("clientY").Float() * dpr)
	return geo.NewPos(scale.ToBelluiCoordinate(w.canvas, px), scale.ToBelluiCoordinate(w.canvas, py))
}

// onDragStart pulls the armed provider's payload and populates the native drag's
// dataTransfer and drag image. If the provider declines it cancels the drag with
// preventDefault so the in-canvas behavior is kept.
func onDragStart(canvasEl js.Value, args []js.Value) {
	if dragOut.provider == nil || len(args) == 0 {
		return
	}
	ev := args[0]
	data, ok := dragOut.provider.DragOutData()
	dragOut.provider = nil
	if !ok {
		canvasEl.Set("draggable", false)
		ev.Call("preventDefault")
		return
	}

	dt := ev.Get("dataTransfer")
	if !dt.Truthy() {
		return
	}
	dt.Set("effectAllowed", "copy")

	if data.Text != "" {
		dt.Call("setData", "text/plain", data.Text)
	}
	if paths := filePaths(data.URIs); len(paths) > 0 {
		// file:// URIs as an RFC 2483 uri-list. A genuine file-to-OS drag needs
		// the Chromium-only DownloadURL convention plus a fetchable URL, which
		// the web target has no local file to serve — documented limit (pre-rebrand tracker #857).
		dt.Call("setData", "text/uri-list", string(internalclipboard.EncodeURIList(paths)))
	}

	// Drag image: an explicit Image wins, else render the DragImageObject ghost
	// (pre-rebrand tracker #842). dataTransfer.setData is string-only, so an image is offered only as
	// the drag ghost, not as a droppable image flavor (browser limit).
	img := data.Image
	if img == nil && data.DragImageObject != nil {
		img = renderDragGhost(data.DragImageObject)
	}
	if img != nil {
		if el, iw, ih := imageToCanvasElement(img); el.Truthy() {
			dt.Call("setDragImage", el, iw/2, ih/2)
		}
	}

	dragOut.data = data
	dragOut.active = true
}

// onDragEnd reports the drop outcome (dropEffect "none" ⇒ not accepted) to the
// payload's OnCompleted on the main goroutine, then clears the drag state.
func onDragEnd(canvasEl js.Value, args []js.Value) {
	canvasEl.Set("draggable", false)
	if !dragOut.active {
		dragOut.provider = nil
		return
	}
	data := dragOut.data
	dragOut.active = false
	dragOut.data = desktop.DragData{}
	dragOut.provider = nil

	accepted := false
	if len(args) > 0 {
		if dt := args[0].Get("dataTransfer"); dt.Truthy() {
			accepted = dt.Get("dropEffect").String() != "none"
		}
	}
	if data.OnCompleted != nil {
		cb := data.OnCompleted
		// runOnMainAsync (not bellui.Do): a synchronous JS callback must not take
		// bellui.Do's blocking fallback on the single-threaded web runtime (pre-rebrand tracker #113).
		runOnMainAsync(func() { cb(accepted) })
	}
}

// renderDragGhost software-renders a DragImageObject to an image for use as the
// drag ghost. Unlike driver/software.Render it does NOT mutate the global app
// theme: SetTheme re-applies settings across every window (RescaleContext,
// texture invalidation), which crashes when run reentrantly inside this DOM
// event callback on the single-threaded web runtime. The ghost is rendered with
// the current app theme via a throwaway software canvas — CPU-only, no GL, no
// global mutation — so it is safe to call inline here.
func renderDragGhost(obj core.CanvasObject) image.Image {
	c := software.NewCanvas()
	c.SetPadded(false)
	c.SetContent(obj)
	internalapp.ApplyThemeTo(obj, c)
	return c.Capture()
}

// imageToCanvasElement renders img into a detached HTML <canvas> via putImageData
// so setDragImage has a synchronously-ready element (an <img> with a data URL is
// not decoded in time for the dragstart snapshot). Returns the element and its
// pixel size, or an undefined value for an empty image.
func imageToCanvasElement(img image.Image) (js.Value, int, int) {
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw == 0 || ih == 0 {
		return js.Undefined(), 0, 0
	}
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Bounds().Min != (image.Point{}) {
		conv := image.NewRGBA(image.Rect(0, 0, iw, ih))
		draw.Draw(conv, conv.Bounds(), img, b.Min, draw.Src)
		rgba = conv
	}

	doc := js.Global().Get("document")
	cv := doc.Call("createElement", "canvas")
	cv.Set("width", iw)
	cv.Set("height", ih)
	ctx := cv.Call("getContext", "2d")

	arr := js.Global().Get("Uint8ClampedArray").New(len(rgba.Pix))
	js.CopyBytesToJS(arr, rgba.Pix)
	imgData := js.Global().Get("ImageData").New(arr, iw, ih)
	ctx.Call("putImageData", imgData, 0, 0)
	return cv, iw, ih
}
