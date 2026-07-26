package desktop

import "github.com/belluikit/bellui"

// App defines the desktop specific extensions to a bellui.App.
//
// System tray support (the SetSystemTray* methods below) is delivered through
// the platform's status-item facilities: a native status item on macOS and
// Windows, and StatusNotifierItem over D-Bus on Linux/BSD. The Linux path is
// the fragile one: an item only appears if the desktop environment runs an SNI
// host. KDE Plasma, most Wayland compositors, and anything with an SNI host
// show it; stock GNOME shows nothing unless the user installs the AppIndicator
// (KStatusNotifierItem) shell extension, and legacy XEmbed-only trays (some
// minimal window managers) are not supported at all. This is a property of the
// desktop, not a bellui bug, and every non-GTK toolkit shares it. Therefore:
// never make the tray the only route to a feature — always keep a window or
// menu path (see the "Show Window" advice on SetSystemTrayWindow).
//
// Since: 2.2
type App interface {
	// SetSystemTrayMenu sets a menu to be shown from the system tray icon.
	// On Linux, see the tray-delivery note on the App interface: the menu is
	// only reachable where the desktop environment provides an SNI host.
	SetSystemTrayMenu(menu *bellui.Menu)
	// SetSystemTrayIcon sets the icon to be used in system tray.
	// If you pass a `ThemedResource` then any OS that adjusts look to match theme will adapt the icon.
	// On Linux, see the tray-delivery note on the App interface: the icon is
	// only visible where the desktop environment provides an SNI host (stock
	// GNOME needs the AppIndicator extension; XEmbed-only trays are unsupported).
	SetSystemTrayIcon(icon bellui.Resource)

	// SetSystemTrayWindow optionally sets a window that this system tray will help to control.
	// On systems that support it (Windows, macOS and most Linux) the window will be shown on left-tap.
	// If the window is decorated (a regular window) tapping will show it only, however for a splash window
	// (without window decorations) tapping when the window is visible will hide it.
	// If you also have a menu set this will be triggered with right-mouse tap.
	// Note that your menu should probably include a "Show Window" menu item for less-compliant Linux
	// systems: on Linux the tray relies on the desktop environment providing a StatusNotifierItem host
	// (stock GNOME requires the AppIndicator extension; XEmbed-only trays are unsupported), so the icon
	// may never appear and the window needs another way to be reached.
	//
	// Since: 2.7
	SetSystemTrayWindow(bellui.Window)

	// SetSystemTrayTooltip sets the hover tooltip text shown for the system tray icon.
	// It may be called before the tray has started; the value is applied once it does.
	// Not every platform renders a tray tooltip (notably it is unreliable on some Linux
	// desktops), so treat it as a best-effort hint.
	//
	// Since: 2.19
	SetSystemTrayTooltip(text string)

	// SetSystemTrayOnTapped sets a custom handler for a left-click (primary tap) on the
	// system tray icon, for actions other than showing a window. Pass nil to clear it.
	//
	// This shares the single underlying tap slot with SetSystemTrayWindow, so the two are
	// mutually exclusive: whichever was set last wins. On macOS, when a menu is set the
	// status item opens the menu on click, so a tap handler cannot be promised alongside a
	// menu there.
	//
	// The handler is invoked on the main goroutine.
	//
	// Since: 2.19
	SetSystemTrayOnTapped(fn func())
}
