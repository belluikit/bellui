//go:build linux && !android && wayland && !wasm

package glfw

import (
	"errors"

	"github.com/belluikit/bellui/driver/desktop"
)

// A forced Wayland-only build (-tags wayland) does not link X11, so the XGrabKey
// backend is not compiled. The Wayland portal path is used at runtime there
// anyway (build.IsWayland is forced true); these stubs keep the dispatch in
// globalshortcut_linux.go compiling.

var errGlobalShortcutX11NotBuilt = errors.New("global shortcuts: X11 backend not compiled in this Wayland-only build")

func (d *gLDriver) registerX11GlobalShortcut(_ *desktop.CustomShortcut, _ func()) error {
	return errGlobalShortcutX11NotBuilt
}

func (d *gLDriver) unregisterX11GlobalShortcut(_ *desktop.CustomShortcut) {}
