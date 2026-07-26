package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/software"
)

// gLDriver satisfies desktop.DragSource on every platform; startNativeDrag is the
// per-platform bridge (real on macOS, a no-op returning false elsewhere until the
// Windows/Linux backends land — see docs/design/515-drag-out.md, #515 Part B).
var _ desktop.DragSource = (*gLDriver)(nil)

// StartDragging begins a native drag-and-drop out of the app. It returns false
// when the platform backend cannot start a native drag, so the caller keeps its
// in-app drag behavior.
//
// The DragData.OnCompleted callback is delivered by each platform backend from
// its own native completion signal, on the main goroutine via bellui.Do — the
// goroutine-driven Windows/Linux backends capture it in their drag goroutine,
// while macOS parks it (AppKit reports completion asynchronously through an
// exported C callback that cannot capture a Go closure). It is not handled here
// so this shared entry point stays dead-code-free on platforms with no backend.
func (d *gLDriver) StartDragging(data desktop.DragData) bool {
	// Resolve the drag *cursor* image separately from the data flavors. An
	// explicit DragData.Image doubles as the cursor image AND is offered as an
	// image data flavor by the backends; the in-canvas ghost (#819
	// DragImageProvider, rendered here) is a visual ONLY — per DragImageObject's
	// contract it carries no data flavor, so it must never land in data.Image
	// (which the backends would then offer as a droppable image, letting the
	// cursor ghost be dropped instead of the real payload). Keep them separate.
	cursorImg := data.Image
	if cursorImg == nil && data.DragImageObject != nil {
		cursorImg = software.Render(data.DragImageObject, bellui.CurrentApp().Settings().Theme())
	}

	ok := startNativeDrag(data, cursorImg)
	if ok {
		nativeDragOutStarted = true
	}
	return ok
}

// nativeDragOutStarted records that StartDragging handed the current pointer
// gesture to a platform drag session. window.mouseMoved consumes it right after
// the Dragged dispatch that triggered it, to end the bellui-side gesture (#933).
// Main-goroutine confined, like all pointer state.
var nativeDragOutStarted bool

// consumeNativeDragOutStarted reports and clears the flag.
func consumeNativeDragOutStarted() bool {
	s := nativeDragOutStarted
	nativeDragOutStarted = false
	return s
}

// filePaths extracts the local file-system paths from the file:// URIs, skipping
// any non-file scheme (only file references can be dragged to a file manager).
// Shared by the Linux/BSD (dragsource_xdg.go) and web (dragsource_wasm.go)
// backends, which both encode files as a text/uri-list flavor.
func filePaths(uris []bellui.URI) []string {
	var out []string
	for _, u := range uris {
		if u == nil || u.Scheme() != "file" {
			continue
		}
		out = append(out, u.Path())
	}
	return out
}
