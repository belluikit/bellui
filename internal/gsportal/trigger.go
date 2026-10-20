//go:build (linux || freebsd || openbsd || netbsd) && !android

// Package gsportal is a pure-Go client for the freedesktop
// org.freedesktop.portal.GlobalShortcuts D-Bus portal — the only way to register
// system-wide hotkeys under Wayland, where there is no client key-grab API by
// design. It is consumed by the Linux glfw driver's desktop.GlobalShortcutApp
// implementation. Kept free of cgo (and of the driver package) so the portal
// protocol logic compiles and unit-tests without a C toolchain, on every
// platform with D-Bus: Linux and the BSDs.
package gsportal

import (
	"strings"

	"github.com/belluikit/bellui/event"
)

// PortalTrigger renders a bellui key + modifiers as an XDG GlobalShortcuts
// "preferred_trigger" string (e.g. Control+Alt+K → "CTRL+ALT+k"). Per the portal
// spec the trigger is only a hint — the compositor may present it to the user
// for confirmation or reassignment — so exact fidelity is not required. Modifier
// tokens (CTRL/ALT/SHIFT/SUPER) are emitted in a stable order followed by the
// key token.
func PortalTrigger(mods event.KeyModifier, key event.KeyName) string {
	var parts []string
	if mods&event.KeyModifierControl != 0 {
		parts = append(parts, "CTRL")
	}
	if mods&event.KeyModifierAlt != 0 {
		parts = append(parts, "ALT")
	}
	if mods&event.KeyModifierShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if mods&event.KeyModifierSuper != 0 {
		parts = append(parts, "SUPER")
	}
	parts = append(parts, portalKeyToken(key))
	return strings.Join(parts, "+")
}

// portalKeyToken maps a bellui key name to the portal trigger key token. A single
// ASCII letter is lowercased (the portal/xkb convention, e.g. "a"); everything
// else (digits, "F5", "Space", …) passes through unchanged.
func portalKeyToken(key event.KeyName) string {
	s := string(key)
	if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
		return strings.ToLower(s)
	}
	return s
}
