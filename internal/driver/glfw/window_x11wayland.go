//go:build linux && ((!x11 && !wayland) || (x11 && wayland)) && !wasm

package glfw

import (
	"unsafe"

	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/internal/build"

	"github.com/belluikit/glfw"
)

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

// RunNative reports the native window handle for whichever backend GLFW selected
// at runtime. The default linux build compiles both the X11 and Wayland
// accessors; build.IsWayland (set by the driver after glfw.Init) picks the live
// one.
func (w *window) RunNative(f func(any)) {
	v := w.view()

	if build.IsWayland {
		context := driver.WaylandWindowContext{}
		if v != nil {
			context.WaylandSurface = uintptr(unsafe.Pointer(v.GetWaylandWindow()))
			if h, err := v.ExportWaylandHandle(); err == nil {
				context.ExportedHandle = h
			}
		}

		f(context)
		return
	}

	context := driver.X11WindowContext{}
	if v != nil {
		context.WindowHandle = uintptr(v.GetX11Window())
	}

	f(context)
}

// doSetParentNative establishes the native parent/owner relationship for
// whichever backend GLFW selected at runtime (both accessors are compiled in
// the default linux build). A nil parent clears it. Must run on the main
// goroutine.
func (w *window) doSetParentNative(parent *window) {
	var p *glfw.Window
	if parent != nil {
		p = parent.view()
	}
	if build.IsWayland {
		w.view().SetWaylandParent(p)
		return
	}
	w.view().SetX11Parent(p)
}

// doSetModalNative enforces native window-modality against the parent for the
// backend GLFW selected at runtime: on X11 it toggles _NET_WM_STATE_MODAL, on
// Wayland it drives xdg_dialog_v1 (compositors without the protocol leave the
// scrim as the only blocker) — both via the glfw fork. Modality is expressed
// against the owner, so a nil parent is a no-op. Must run on the main goroutine
// (#498).
func (w *window) doSetModalNative(parent *window, modal bool) {
	if parent == nil {
		return
	}
	if build.IsWayland {
		w.view().SetWaylandModal(modal)
		return
	}
	w.view().SetX11Modal(modal)
}
