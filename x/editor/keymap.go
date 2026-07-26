package editor

import (
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// KeyMap holds the editor's configurable interaction shortcuts — the "smart
// editing" chords whose fixed defaults were historically macOS-hostile (#110).
//
// A host customizes a binding by assigning to [Editor.KeyMap] before the editor
// is first shown. Start from [DefaultKeyMap] and adjust only the fields you care
// about; a binding left at its zero value (empty key) is unbound, disabling that
// action:
//
//	km := editor.DefaultKeyMap()
//	km.Completion = desktop.CustomShortcut{
//		KeyName:  event.KeyI,
//		Modifier: event.KeyModifierShortcutDefault, // Cmd on macOS, Ctrl elsewhere
//	}
//	editor.KeyMap = km
//
// Only these interaction shortcuts are configurable via the keymap. Find, save,
// run and the hover chord keep their fixed bindings. Completion also triggers
// automatically as you type, independent of [KeyMap.Completion].
//
// [KeyMap.GoToDefinition] is the keyboard "go to definition" trigger; its
// default (F12) carries no modifier, so it is delivered through the editor's key
// dispatcher rather than the shortcut registry (the platform driver only
// synthesizes shortcuts for modified chords). A host may rebind it to a modified
// chord — e.g. Cmd/Ctrl+F12 — which then routes through the shortcut registry
// like the others; both paths call the same action.
type KeyMap struct {
	// Completion explicitly requests the completion popup.
	Completion desktop.CustomShortcut
	// SignatureHelp requests signature help at the cursor.
	SignatureHelp desktop.CustomShortcut
	// CycleOverloadNext and CycleOverloadPrev step through overloaded signatures
	// while the signature card is shown.
	CycleOverloadNext desktop.CustomShortcut
	CycleOverloadPrev desktop.CustomShortcut
	// GoToDefinition fires [Editor.GoToDefinitionAtCursor] — the keyboard
	// counterpart to Cmd/Ctrl+click go-to-definition. Its default (F12) is
	// modifier-less; see the type doc for how that is dispatched.
	GoToDefinition desktop.CustomShortcut
}

// DefaultKeyMap returns the built-in interaction shortcuts with platform-aware
// modifiers:
//
//   - Completion: Ctrl+Space on every platform. macOS reserves Ctrl+Space for
//     the input-source switcher, so mac hosts may want to rebind it — completion
//     also auto-triggers as you type, so it stays usable regardless.
//   - SignatureHelp: Cmd+Shift+Space on macOS, Ctrl+Shift+Space elsewhere.
//   - CycleOverloadNext / CycleOverloadPrev: Alt(Option)+Down / Alt(Option)+Up.
//   - GoToDefinition: F12 (no modifier), matching VS Code's default.
//
// [event.KeyModifierShortcutDefault] resolves to Cmd on macOS and Ctrl on other
// platforms, mirroring the modifier the editor's find/save/run shortcuts use.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Completion: desktop.CustomShortcut{
			KeyName:  event.KeySpace,
			Modifier: event.KeyModifierControl,
		},
		SignatureHelp: desktop.CustomShortcut{
			KeyName:  event.KeySpace,
			Modifier: event.KeyModifierShortcutDefault | event.KeyModifierShift,
		},
		CycleOverloadNext: desktop.CustomShortcut{
			KeyName:  event.KeyDown,
			Modifier: event.KeyModifierAlt,
		},
		CycleOverloadPrev: desktop.CustomShortcut{
			KeyName:  event.KeyUp,
			Modifier: event.KeyModifierAlt,
		},
		GoToDefinition: desktop.CustomShortcut{
			KeyName: event.KeyF12,
			// No modifier: delivered via the key dispatcher, not the shortcut
			// registry (see the KeyMap type doc).
		},
	}
}
