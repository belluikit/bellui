package desktop

import (
	"image"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// DragData is the payload carried out of the application during a drag-out to
// another application (Finder/Explorer, a text editor, a browser). Any subset of
// the flavors may be set; empty or nil fields are omitted from the drag.
//
// It is the outbound counterpart of the inbound [bellui.DropEvent].
//
// Since: 1.0
type DragData struct {
	// Text is a plain-text flavor — for example a selection dragged into a text
	// editor.
	Text string
	// URIs are file references dragged out to a file manager. Only file:// URIs
	// are carried; other schemes are skipped.
	URIs []bellui.URI
	// Image is an image flavor and, when set, is also used as the drag image
	// that follows the pointer. Nil uses a platform default (the file icon or the
	// app icon).
	Image image.Image
	// DragImageObject is an optional [core.CanvasObject] rendered to supply the
	// drag image, bridging the in-canvas drag ghost ([core.DragImageProvider])
	// so a widget can declare one drag visual for both in-canvas drag-and-drop
	// and drag-out. It is software-rendered to an image when the drag starts and
	// only consulted when Image is nil (a raw Image always wins). It carries no
	// data flavor — set Text/URIs/Image for the payload.
	//
	// Since: 1.0
	DragImageObject core.CanvasObject
	// OnCompleted is an optional callback fired once the native drag finishes,
	// reporting whether a target accepted the drop (a copy/move) versus the drag
	// being cancelled. It is always called on the main goroutine (via bellui.Do),
	// so a source can react to the outcome — for example deleting a file it
	// dragged out as a move. Nil to ignore the outcome (fire-and-forget).
	//
	// Since: 1.0
	OnCompleted func(accepted bool)
}

// DragSource is an optional capability of a [bellui.Driver] that starts a native
// drag-and-drop OUT of the application to other applications. It is the outbound
// counterpart of the inbound file-drop path ([bellui.Droppable]).
//
// GLFW exposes no drag-source API, so each desktop backend hand-rolls it against
// the native window handle; a driver or platform that cannot start a native drag
// reports false. Detect support with a type assertion, or use [StartDrag].
//
// Since: 1.0
type DragSource interface {
	// StartDragging begins a native drag session carrying data from the key
	// window. Call it from a widget's Dragged handler on the main goroutine (a
	// native drag may run its own modal loop). It returns false when the driver
	// or platform cannot start a native drag, so the caller keeps its in-app drag
	// behavior.
	StartDragging(data DragData) bool
}

// DragOutProvider is an optional extension of a drag source ([event.Draggable]):
// it lets a driver obtain the drag-out payload at gesture start without relying
// on the push-based Dragged→[StartDrag] path. It exists for backends that stop
// delivering pointer-move events once a native drag begins — notably the web
// (wasm) backend, where the browser's HTML5 Drag and Drop suppresses
// mousemove/mouseup on the source at dragstart, so the Dragged handler never
// runs during the drag and there is nothing to hand to the native drag. Such a
// backend hit-tests at pointerdown, and if the deepest Draggable under the
// pointer implements DragOutProvider it arms a native drag for that gesture,
// pulling the payload once at drag start.
//
// Desktop backends keep using the push StartDrag path unchanged; they MAY also
// consult DragOutProvider (for example to pre-arm). A plain draggable that does
// not implement it never triggers a drag-out, so in-app dragging is untouched.
//
// Since: 1.0
type DragOutProvider interface {
	// DragOutData returns the payload to hand to the OS drag and true, or a zero
	// value and false to decline (keeping the in-app drag). It is read once at
	// drag start and must not depend on prior Dragged calls.
	DragOutData() (DragData, bool)
}

// StartDrag begins a native drag-and-drop OUT of the app carrying data if the
// active driver supports it ([DragSource]), returning whether the drag started.
// It is package sugar over the driver assertion, mirroring the clipboard-flavor
// helpers. Call it from a widget's Dragged handler on the main goroutine.
//
// Since: 1.0
func StartDrag(data DragData) bool {
	if ds, ok := bellui.CurrentApp().Driver().(DragSource); ok {
		return ds.StartDragging(data)
	}
	return false
}
