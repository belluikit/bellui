//go:build !ci && !wasm && !android && !ios && !mobile && !tinygo && !noos && !tamago && (darwin || windows || linux || openbsd || freebsd || netbsd)

package app

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
)

// The app value is what applications type-assert to desktop.App, so it must
// carry every method of that interface, not only the driver.
var _ desktop.App = (*belluiApp)(nil)

// SetSystemTrayMenu creates a system tray item and attaches the specified menu.
// By default, this will use the application icon.
func (a *belluiApp) SetSystemTrayMenu(menu *bellui.Menu) {
	if desk, ok := a.Driver().(desktop.App); ok {
		desk.SetSystemTrayMenu(menu)
	}
}

// SetSystemTrayIcon sets a custom image for the system tray icon.
// You should have previously called `SetSystemTrayMenu` to initialise the menu icon.
func (a *belluiApp) SetSystemTrayIcon(icon bellui.Resource) {
	if desk, ok := a.Driver().(desktop.App); ok {
		desk.SetSystemTrayIcon(icon)
	}
}

// SetSystemTrayWindow assigns a window to be shown with the system tray menu is tapped.
// You should have previously called `SetSystemTrayMenu` to initialise the menu icon.
func (a *belluiApp) SetSystemTrayWindow(w bellui.Window) {
	if desk, ok := a.Driver().(desktop.App); ok {
		desk.SetSystemTrayWindow(w)
	}
}

// SetSystemTrayTooltip sets the hover text of the system tray icon.
func (a *belluiApp) SetSystemTrayTooltip(text string) {
	if desk, ok := a.Driver().(desktop.App); ok {
		desk.SetSystemTrayTooltip(text)
	}
}

// SetSystemTrayOnTapped sets the function run when the system tray icon is
// tapped (left-clicked).
func (a *belluiApp) SetSystemTrayOnTapped(fn func()) {
	if desk, ok := a.Driver().(desktop.App); ok {
		desk.SetSystemTrayOnTapped(fn)
	}
}
