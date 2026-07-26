//go:build !darwin

package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
)

const (
	textModifierAlt     = "Alt+"
	textModifierControl = "Ctrl+"
	textModifierShift   = "Shift+"
	textModifierSuper   = "Super+"
)

var (
	styleModifiers   = bellui.TextStyle{}
	defaultStyleKeys = bellui.TextStyle{}
)

var keyTexts = map[event.KeyName]string{
	event.KeyBackspace: "Backspace",
	event.KeyDelete:    "Del",
	event.KeyDown:      "↓",
	event.KeyEnd:       "End",
	event.KeyEnter:     "Enter",
	event.KeyEscape:    "Esc",
	event.KeyHome:      "Home",
	event.KeyLeft:      "←",
	event.KeyPageDown:  "PgDn",
	event.KeyPageUp:    "PgUp",
	event.KeyReturn:    "Return",
	event.KeyRight:     "→",
	event.KeySpace:     "Space",
	event.KeyTab:       "Tab",
	event.KeyUp:        "↑",
}
