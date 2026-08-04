package desktop

import "github.com/belluikit/bellui"

// Window describes desktop specific window features.
//
// The method set matches the upstream Bellui 2.8 desktop.Window interface so
// that code written against upstream asserts and works unchanged here.
//
// Since: 1.0
type Window interface {
	// RequestFullScreenSecondary asks for a window to fullscreen on a non-primary monitor.
	// When there is only one screen it will fullscreen on the primary monitor.
	RequestFullScreenSecondary()

	// RequestAlwaysOnTop asks for the window to remain above other windows, call before [bellui.Window.Show].
	// This is subject to support on the current system and with the caveat that the window manager
	// may decide that other windows or panels remain above this one.
	RequestAlwaysOnTop()

	// RequestPosition asks the operating system to position this window at the x, y coordinate
	// of the native system. These values may be scaled and so not necessarily pixel values.
	// This request may be ignored (for example Linux Wayland).
	// There is no contract of what these values represent and on multiple monitors may be greater
	// than the coordinates of the current monitor.
	RequestPosition(x, y int)
}

// PositionableWindow extends [Window] with position read-back and a move
// subscription, so applications can tile windows, or save and restore window
// geometry across runs. Coordinates use the same loose native-system contract
// as [Window.RequestPosition] — a position read from [PositionableWindow.Position]
// can be passed back to RequestPosition verbatim, but carries no unit
// guarantee beyond that round-trip.
//
// On Linux Wayland the compositor never reveals global window positions, so
// Position only echoes back the last requested position (zero if none was
// requested) and OnMoved callbacks do not fire.
//
// Windows on mobile and web targets do not implement this interface.
//
// Since: 1.0
type PositionableWindow interface {
	Window

	// Position returns the most recently known position of the window on the
	// virtual desktop, in the same native coordinates as [Window.RequestPosition].
	// While the window is fullscreen this keeps reporting the last windowed
	// position, so a save-on-exit records a useful restore point.
	// Before the window has been shown it reports the last requested position,
	// or (0, 0) if none was requested.
	Position() (x, y int)

	// SetOnMoved sets a callback that runs on the main goroutine every time
	// the window is moved, with the new position in the same native
	// coordinates as [Window.RequestPosition]. Moves caused by entering
	// fullscreen are not reported. Pass nil to remove the callback.
	SetOnMoved(func(x, y int))
}

// AttentionWindow extends [Window] with a request for the user's attention
// without stealing focus — a politer signal than [bellui.Window.RequestFocus]
// for "a background task finished" or "this window needs you". It maps to the
// platform's non-intrusive attention affordance: a dock-icon bounce on macOS,
// a taskbar-button flash on Windows, and the urgency hint
// (_NET_WM_STATE_DEMANDS_ATTENTION) on Linux/X11.
//
// The request is advisory: the window manager decides how (and whether) to
// surface it, and it is typically cleared automatically once the window gains
// focus. On Linux Wayland support depends on the compositor.
//
// Windows on mobile and web targets do not implement this interface.
//
// Since: 1.0
type AttentionWindow interface {
	Window

	// RequestAttention asks the system to draw the user's attention to this
	// window without raising or focusing it. Safe to call before or after the
	// window is shown; a call before Show is applied once the window maps.
	RequestAttention()
}

// ProgressWindow extends [Window] with a task-progress indicator drawn on the
// window's taskbar button (Windows, the flagship for this) or the application's
// launcher/dock icon on other desktops. It is the natural home for "download is
// 40% done" or "export running" feedback without a modal progress dialog.
//
// The indicator is advisory and best-effort: platforms without a native
// progress affordance (currently macOS, and Linux desktops that do not honour
// the Unity LauncherEntry protocol) treat it as a no-op.
//
// Windows on mobile and web targets do not implement this interface.
//
// Since: 1.0
type ProgressWindow interface {
	Window

	// SetProgress shows task progress in the range 0..1 (values outside the
	// range are clamped). A negative value clears the indicator. When
	// indeterminate is true the platform shows a busy/marquee animation and the
	// value is ignored. Safe to call before or after the window is shown; a call
	// before Show is applied once the window maps.
	SetProgress(value float64, indeterminate bool)
}

// ChildWindow extends [Window] with a native parent/owner relationship: the
// window manager keeps this window stacked above its parent, minimizes and
// restores it together with the parent, and treats it as owned by the parent
// rather than as an independent top-level window.
//
// This is the native counterpart to the pure-Go modal scrim used by
// window-hosted dialogs; drivers that support it use both.
//
// Windows on mobile and web targets do not implement this interface.
//
// Since: 1.0
type ChildWindow interface {
	Window

	// SetParent establishes parent as this window's owner. Call before Show
	// for the relationship to take effect as the window maps; calling it while
	// already shown applies it immediately where the platform allows. Pass nil
	// to clear the relationship.
	//
	// On platforms or sessions that cannot express window ownership the call is
	// a no-op. On Linux Wayland the relationship is applied by the compositor,
	// so — unlike [Window.RequestPosition] — it is honoured there.
	SetParent(parent bellui.Window)
}

// ModalWindow extends [ChildWindow] with native window-modality: while this
// window is shown the window manager blocks input to its parent window (its
// title bar is greyed and keyboard focus cannot reach it), enforcing at the OS
// level the modality that the pure-Go modal scrim provides inside the toolkit.
//
// It is the native counterpart to the application-level scrim used by
// window-hosted dialogs; drivers that support it use both, so the OS and the
// canvas agree that the parent is blocked.
//
// Windows on mobile and web targets do not implement this interface.
//
// Since: 1.0
type ModalWindow interface {
	ChildWindow

	// SetModal enables or disables native window-modality against the parent
	// established with [ChildWindow.SetParent]; call SetParent first. Enabling
	// it before Show takes effect as the window maps; toggling it while shown
	// applies immediately where the platform allows. Disabling it — or closing
	// the window — restores input to the parent.
	//
	// On platforms or sessions that cannot express window-modality the call is
	// a no-op (the scrim still blocks the parent inside the toolkit).
	SetModal(modal bool)
}
