package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
)

const (
	textModifierAlt     = "⌥"
	textModifierControl = "⌃"
	textModifierShift   = "⇧"
	textModifierSuper   = "⌘"
)

var (
	styleModifiers   = bellui.TextStyle{Symbol: true}
	defaultStyleKeys = bellui.TextStyle{Monospace: true}
)

var keyTexts = map[event.KeyName]string{
	event.KeyBackspace: "⌫",
	event.KeyDelete:    "⌦",
	event.KeyDown:      "↓",
	event.KeyEnd:       "↘",
	event.KeyEnter:     "↩",
	event.KeyEscape:    "⎋",
	event.KeyHome:      "↖",
	event.KeyLeft:      "←",
	event.KeyPageDown:  "⇟",
	event.KeyPageUp:    "⇞",
	event.KeyReturn:    "↩",
	event.KeyRight:     "→",
	event.KeySpace:     "␣",
	event.KeyTab:       "⇥",
	event.KeyUp:        "↑",
}
