//go:build !darwin && !windows && !linux && !wasm

package glfw

import (
	"errors"

	"github.com/belluikit/bellui/driver/desktop"
)

var _ desktop.GlobalShortcutApp = (*gLDriver)(nil)

// errGlobalShortcutUnsupported is returned on platforms with no backend yet
// (currently the BSDs — X11 XGrabKey is a follow-up). Windows is in
// globalshortcut_windows.go, macOS in globalshortcut_darwin.go, and Linux
// (Wayland portal / X11) in globalshortcut_linux.go.
var errGlobalShortcutUnsupported = errors.New("global shortcuts are not yet implemented on this platform")

// RegisterGlobalShortcut implements desktop.GlobalShortcutApp. It reports an
// error on platforms whose native backend is not yet implemented, so callers
// get an explicit signal rather than a silent no-op.
func (d *gLDriver) RegisterGlobalShortcut(shortcut *desktop.CustomShortcut, handler func()) error {
	return errGlobalShortcutUnsupported
}

// UnregisterGlobalShortcut implements desktop.GlobalShortcutApp (no-op until a
// native backend exists).
func (d *gLDriver) UnregisterGlobalShortcut(shortcut *desktop.CustomShortcut) {}
