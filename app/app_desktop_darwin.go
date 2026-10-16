//go:build !ci && !ios && !wasm && !mobile

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#include <AppKit/AppKit.h>

bool isBundled();
void watchTheme();
*/
import "C"

import (
	"net/url"
	"os"
	"os/exec"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
)

func (a *belluiApp) OpenURL(url *url.URL) error {
	cmd := exec.Command("open", url.String())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// SetSystemTrayIcon sets a custom image for the system tray icon.
// You should have previously called `SetSystemTrayMenu` to initialise the menu icon.
func (a *belluiApp) SetSystemTrayIcon(icon bellui.Resource) {
	a.Driver().(systrayDriver).SetSystemTrayIcon(icon)
}

// SetSystemTrayMenu creates a system tray item and attaches the specified menu.
// By default, this will use the application icon.
func (a *belluiApp) SetSystemTrayMenu(menu *bellui.Menu) {
	if desk, ok := a.Driver().(systrayDriver); ok {
		desk.SetSystemTrayMenu(menu)
	}
}

// SetSystemTrayWindow assigns a window to be shown with the system tray menu is tapped.
// You should have previously called `SetSystemTrayMenu` to initialise the menu icon.
func (a *belluiApp) SetSystemTrayWindow(w bellui.Window) {
	a.Driver().(systrayDriver).SetSystemTrayWindow(w)
}

// SetSystemTrayTooltip sets the hover text of the system tray icon.
func (a *belluiApp) SetSystemTrayTooltip(text string) {
	if desk, ok := a.Driver().(systrayDriver); ok {
		desk.SetSystemTrayTooltip(text)
	}
}

// SetSystemTrayOnTapped sets the function run when the system tray icon is
// tapped (left-clicked).
func (a *belluiApp) SetSystemTrayOnTapped(fn func()) {
	if desk, ok := a.Driver().(systrayDriver); ok {
		desk.SetSystemTrayOnTapped(fn)
	}
}

// The app value is what applications type-assert to desktop.App, so it must
// carry every method of that interface, not only the driver.
var _ desktop.App = (*belluiApp)(nil)

//export themeChanged
func themeChanged() {
	bellui.CurrentApp().Settings().(*settings).setupTheme()
}

func watchTheme(_ *settings) {
	C.watchTheme()
}

func (a *belluiApp) registerRepositories() {
	// no-op
}
