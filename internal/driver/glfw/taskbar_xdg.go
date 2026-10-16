//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago

package glfw

import (
	"github.com/godbus/dbus/v5"

	"github.com/belluikit/bellui"
)

// applyProgressNative drives launcher/taskbar progress on Linux and the BSDs via
// the Unity LauncherEntry D-Bus protocol (pre-rebrand tracker #721) — the same channel App.SetBadge
// uses for the count badge. It is honoured by KDE Plasma and several other
// desktops and silently ignored elsewhere.
//
// Unlike the per-window Windows taskbar button, LauncherEntry is keyed by the
// app's .desktop file, so progress is an application-level affordance shared by
// all of the app's windows. There is no marquee in the protocol, so an
// indeterminate request just shows the (clamped) bar rather than an animation.
// A negative value clears the indicator. Best-effort: D-Bus errors are ignored.
func (w *window) applyProgressNative(value float64, indeterminate bool) {
	conn, err := dbus.SessionBus() // shared connection, don't close
	if err != nil {
		return
	}

	visible := indeterminate || value >= 0
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}

	appURI := "application://" + bellui.CurrentApp().UniqueID() + ".desktop"
	props := map[string]dbus.Variant{
		"progress":         dbus.MakeVariant(value),
		"progress-visible": dbus.MakeVariant(visible),
	}

	// Launchers match the LauncherEntry Update signal by the app URI argument,
	// not by the emitting object path, so a fixed path is fine (matches SetBadge).
	_ = conn.Emit("/com/belluikit/bellui/LauncherEntry",
		"com.canonical.Unity.LauncherEntry.Update", appURI, props)
}
