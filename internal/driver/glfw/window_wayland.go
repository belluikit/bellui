//go:build wayland && !x11 && (linux || freebsd || openbsd || netbsd)

package glfw

import (
	"unsafe"

	"github.com/belluikit/bellui/driver"

	"github.com/belluikit/glfw"
)

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	context := driver.WaylandWindowContext{}
	if v := w.view(); v != nil {
		context.WaylandSurface = uintptr(unsafe.Pointer(v.GetWaylandWindow()))
		if h, err := v.ExportWaylandHandle(); err == nil {
			context.ExportedHandle = h
		}
	}

	f(context)
}

// doSetParentNative sets the xdg_toplevel parent via the glfw fork so the
// compositor keeps this window above its parent and minimizes them together. A
// nil parent unsets it. Must run on the main goroutine.
func (w *window) doSetParentNative(parent *window) {
	var p *glfw.Window
	if parent != nil {
		p = parent.view()
	}
	w.view().SetWaylandParent(p)
}

// doSetModalNative drives xdg_dialog_v1 via the glfw fork so a compositor
// supporting the protocol blocks input to this window's parent while it is up;
// compositors without it leave the pure-Go scrim as the only blocker. Modality
// is expressed against the owner, so a nil parent is a no-op. Must run on the
// main goroutine (pre-rebrand tracker #498).
func (w *window) doSetModalNative(parent *window, modal bool) {
	if parent == nil {
		return
	}
	w.view().SetWaylandModal(modal)
}
