//go:build linux && !android && !wasm

package glfw

import (
	"strings"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/gsportal"
)

var _ desktop.GlobalShortcutApp = (*gLDriver)(nil)

var (
	gsMu     sync.Mutex
	gsClient *gsportal.Client
)

// RegisterGlobalShortcut implements desktop.GlobalShortcutApp. The backend is
// chosen at runtime by display server (build.IsWayland, see #542): Wayland uses
// the org.freedesktop.portal.GlobalShortcuts portal (which may prompt the user
// for consent — a nil return means the request was accepted, not necessarily
// that the binding is live), X11 grabs the key on the root window with XGrabKey.
//
// The Wayland portal requires the app to be identifiable: on GNOME the app's
// UniqueID must match an installed .desktop file (as produced by "bellui install"),
// otherwise registration fails with an "app id is required" error. A bare "go
// run" binary therefore cannot register global shortcuts under GNOME/Wayland.
func (d *gLDriver) RegisterGlobalShortcut(shortcut *desktop.CustomShortcut, handler func()) error {
	if !build.IsWayland {
		return d.registerX11GlobalShortcut(shortcut, handler)
	}
	client, err := waylandShortcutClient()
	if err != nil {
		return err
	}
	id := shortcut.ShortcutName()
	trigger := gsportal.PortalTrigger(shortcut.Mod(), shortcut.Key())
	return client.Register(id, shortcutDescription(shortcut), trigger, handler)
}

// UnregisterGlobalShortcut implements desktop.GlobalShortcutApp.
func (d *gLDriver) UnregisterGlobalShortcut(shortcut *desktop.CustomShortcut) {
	if !build.IsWayland {
		d.unregisterX11GlobalShortcut(shortcut)
		return
	}
	gsMu.Lock()
	client := gsClient
	gsMu.Unlock()
	if client != nil {
		client.Unregister(shortcut.ShortcutName())
	}
}

// waylandShortcutClient lazily creates the portal client, delivering handlers on
// the main goroutine via bellui.Do.
func waylandShortcutClient() (*gsportal.Client, error) {
	gsMu.Lock()
	defer gsMu.Unlock()
	if gsClient != nil {
		return gsClient, nil
	}
	appID := ""
	if a := bellui.CurrentApp(); a != nil {
		appID = a.UniqueID()
	}
	c, err := gsportal.NewClient(appID, func(fn func()) { bellui.Do(fn) })
	if err != nil {
		return nil, err
	}
	gsClient = c
	return c, nil
}

// shortcutDescription is the human-facing text shown in the portal consent
// dialog. The CustomShortcut carries no app-supplied label, so we derive one
// from its key combination (dropping the internal "CustomDesktop:" prefix).
func shortcutDescription(s *desktop.CustomShortcut) string {
	return "Shortcut " + strings.TrimPrefix(s.ShortcutName(), "CustomDesktop:")
}
