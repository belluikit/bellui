//go:build linux && !android && !wasm && !wayland

package glfw

// This file holds only the //export callback. cgo forbids C *definitions* in the
// preamble of a file that uses //export (it is emitted into two translation
// units), so the C helper definitions live in globalshortcut_x11_linux.go and
// this preamble stays empty.

import "C"

import "github.com/belluikit/bellui"

//export belluiX11HotkeyFired
func belluiX11HotkeyFired(keycode, state C.uint) {
	// Runs on the pump goroutine (called from belluiX11HandleOne), so x11Grabs is
	// accessed single-threaded. Mask off the lock bits to match the base grab
	// (x11ModifierBits is defined in globalshortcut_x11_linux.go where the X11
	// headers are included).
	mask := state & x11ModifierBits
	if h := x11Grabs[x11GrabKey{keycode: keycode, mask: mask}]; h != nil {
		bellui.Do(h)
	}
}
