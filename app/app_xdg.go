//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago

package app

import (
	"net/url"
	"os"
	"os/exec"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"github.com/rymdport/portal/notification"
	"github.com/rymdport/portal/openuri"
	portalSettings "github.com/rymdport/portal/settings"
	"github.com/rymdport/portal/settings/appearance"

	"github.com/belluikit/bellui"
	internalapp "github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/theme"
)

const systemTheme = bellui.ThemeVariant(99)

func (a *belluiApp) OpenURL(url *url.URL) error {
	if build.IsFlatpak {
		err := openuri.OpenURI("", url.String(), nil)
		if err != nil {
			bellui.LogError("Opening url in portal failed", err)
		}
		return err
	}

	cmd := exec.Command("xdg-open", url.String())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Start()
}

// fetch color variant from dbus portal desktop settings.
func findFreedesktopColorScheme() bellui.ThemeVariant {
	colorScheme, err := appearance.GetColorScheme()
	if err != nil {
		return systemTheme
	}

	return colorSchemeToThemeVariant(colorScheme)
}

func colorSchemeToThemeVariant(colorScheme appearance.ColorScheme) bellui.ThemeVariant {
	switch colorScheme {
	case appearance.Light:
		return theme.VariantLight
	case appearance.Dark:
		return theme.VariantDark
	}

	// Default to light theme to support Gnome's default see https://github.com/fyne-io/fyne/pull/3561
	return theme.VariantLight
}

func (a *belluiApp) SendNotification(n *bellui.Notification) {
	if build.IsFlatpak {
		err := a.sendNotificationThroughPortal(n)
		if err != nil {
			bellui.LogError("Sending notification using portal failed", err)
		}
		return
	}

	conn, err := dbus.SessionBus() // shared connection, don't close
	if err != nil {
		bellui.LogError("Unable to connect to session D-Bus", err)
		return
	}

	appIcon := a.cachedIconPath()
	timeout := int32(0) // we don't support this yet

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, a.uniqueID, uint32(0),
		appIcon, n.Title, n.Content, []string{}, map[string]dbus.Variant{}, timeout)
	if call.Err != nil {
		bellui.LogError("Failed to send message to bus", call.Err)
	}
}

// Sending with same ID replaces the old notification.
var notificationID atomic.Uint64

// See https://flatpak.github.io/xdg-desktop-portal/docs/#gdbus-org.freedesktop.portal.Notification.
func (a *belluiApp) sendNotificationThroughPortal(n *bellui.Notification) error {
	return notification.Add(
		uint(notificationID.Add(1)),
		notification.Content{
			Title: n.Title,
			Body:  n.Content,
			Icon:  a.uniqueID,
		},
	)
}

// SetSystemTrayMenu creates a system tray item and attaches the specified menu.
// By default, this will use the application icon.
func (a *belluiApp) SetSystemTrayMenu(menu *bellui.Menu) {
	if desk, ok := a.Driver().(systrayDriver); ok { // don't use this on mobile tag
		desk.SetSystemTrayMenu(menu)
	}
}

// SetSystemTrayIcon sets a custom image for the system tray icon.
// You should have previously called `SetSystemTrayMenu` to initialise the menu icon.
func (a *belluiApp) SetSystemTrayIcon(icon bellui.Resource) {
	if desk, ok := a.Driver().(systrayDriver); ok { // don't use this on mobile tag
		desk.SetSystemTrayIcon(icon)
	}
}

// SetSystemTrayWindow assigns a window to be shown with the system tray menu is tapped.
// You should have previously called `SetSystemTrayMenu` to initialise the menu icon.
func (a *belluiApp) SetSystemTrayWindow(w bellui.Window) {
	a.Driver().(systrayDriver).SetSystemTrayWindow(w)
}

// SetBadge sets the launcher-entry count badge via the Unity LauncherEntry
// D-Bus protocol, honoured by KDE Plasma and several other Linux desktops and
// silently ignored elsewhere. A count <= 0 clears the badge. Part of the
// [bellui.BadgeApp] optional interface; best-effort (errors are non-fatal).
func (a *belluiApp) SetBadge(count int) {
	conn, err := dbus.SessionBus() // shared connection, don't close
	if err != nil {
		return
	}

	appURI := "application://" + a.UniqueID() + ".desktop"
	props := map[string]dbus.Variant{
		"count":         dbus.MakeVariant(int64(count)),
		"count-visible": dbus.MakeVariant(count > 0),
	}

	// Launchers match the LauncherEntry Update signal by the app URI argument,
	// not by the emitting object path, so a fixed path is fine.
	_ = conn.Emit("/com/belluikit/bellui/LauncherEntry",
		"com.canonical.Unity.LauncherEntry.Update", appURI, props)
}

func watchTheme(s *settings) {
	go func() {
		// Theme lookup hangs on some desktops. Update theme variant cache from within goroutine.
		themeVariant := findFreedesktopColorScheme()
		if themeVariant != systemTheme {
			internalapp.CurrentVariant.Store(uint64(themeVariant))
			bellui.Do(func() { s.applyVariant(themeVariant) })
		}

		portalSettings.OnSignalSettingChanged(func(changed portalSettings.Changed) {
			if changed.Namespace == appearance.Namespace && changed.Key == "color-scheme" {
				themeVariant := colorSchemeToThemeVariant(appearance.ColorScheme(changed.Value.(uint32)))
				internalapp.CurrentVariant.Store(uint64(themeVariant))
				bellui.Do(func() { s.applyVariant(themeVariant) })
			}
		})
	}()
}

func (a *belluiApp) registerRepositories() {
	// no-op
}

func (s *settings) applyVariant(variant bellui.ThemeVariant) {
	s.variant = variant
	s.apply()
}
