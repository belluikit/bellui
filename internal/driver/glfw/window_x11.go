//go:build !wayland && ((linux && x11) || freebsd || openbsd || netbsd) && !wasm

package glfw

import (
	"github.com/belluikit/bellui/driver"

	"github.com/belluikit/glfw"
)

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	context := driver.X11WindowContext{}
	if v := w.view(); v != nil {
		context.WindowHandle = uintptr(v.GetX11Window())
	}

	f(context)
}

// doSetParentNative sets the WM_TRANSIENT_FOR hint via the glfw fork so the
// window manager keeps this window above its parent and minimizes them
// together. A nil parent clears the hint. Must run on the main goroutine.
func (w *window) doSetParentNative(parent *window) {
	var p *glfw.Window
	if parent != nil {
		p = parent.view()
	}
	w.view().SetX11Parent(p)
}

// doSetModalNative toggles _NET_WM_STATE_MODAL via the glfw fork so the window
// manager blocks input to this window's WM_TRANSIENT_FOR owner (set by
// doSetParentNative) while it is up. Must run on the main goroutine. Modality
// is expressed against the owner, so with no resolved parent it is left to the
// pure-Go scrim (#498).
func (w *window) doSetModalNative(parent *window, modal bool) {
	if parent == nil {
		return
	}
	w.view().SetX11Modal(modal)
}
