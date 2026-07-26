package desktop

import (
	"runtime"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
)

// Declare conformity with Shortcut interface
var (
	_ core.Shortcut           = (*CustomShortcut)(nil)
	_ bellui.KeyboardShortcut = (*CustomShortcut)(nil)
)

// CustomShortcut describes a shortcut desktop event.
type CustomShortcut struct {
	event.KeyName
	Modifier event.KeyModifier
}

// Key returns the key name of this shortcut.
func (cs *CustomShortcut) Key() event.KeyName {
	return cs.KeyName
}

// Mod returns the modifier of this shortcut.
func (cs *CustomShortcut) Mod() event.KeyModifier {
	return cs.Modifier
}

// ShortcutName returns the shortcut name associated to the event.
func (cs *CustomShortcut) ShortcutName() string {
	id := &strings.Builder{}
	id.WriteString("CustomDesktop:")
	writeModifiers(id, cs.Modifier)
	id.WriteString(string(cs.KeyName))
	return id.String()
}

func writeModifiers(w *strings.Builder, mods event.KeyModifier) {
	if (mods & event.KeyModifierShift) != 0 {
		w.WriteString("Shift+")
	}
	if (mods & event.KeyModifierControl) != 0 {
		w.WriteString("Control+")
	}
	if (mods & event.KeyModifierAlt) != 0 {
		w.WriteString("Alt+")
	}
	if (mods & event.KeyModifierSuper) != 0 {
		if runtime.GOOS == "darwin" {
			w.WriteString("Command+")
		} else {
			w.WriteString("Super+")
		}
	}
}
