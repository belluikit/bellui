package driver

import "github.com/belluikit/bellui/event"

// TabDirection reports how a Tab key press moves window focus: 1 to the next
// focusable object, -1 to the previous one, or 0 to leave focus alone so the
// key reaches the focused widget or the shortcut path.
//
// Plain Tab and Shift+Tab move focus, unless the focused object keeps Tab for
// itself (an event.Tabbable whose AcceptsTab reports true, such as a
// multi-line Entry or a DAG canvas). Such an object would otherwise trap a
// keyboard user, so for it Ctrl+Tab and Ctrl+Shift+Tab move focus instead.
// On any other object Ctrl+Tab is left alone, so an app can still bind it as
// a shortcut. Alt and Super never move focus.
func TabDirection(focused event.Focusable, mods event.KeyModifier) int {
	if mods&(event.KeyModifierAlt|event.KeyModifierSuper) != 0 {
		return 0
	}

	keepsTab := false
	if t, ok := focused.(event.Tabbable); ok {
		keepsTab = t.AcceptsTab()
	}
	if (mods&event.KeyModifierControl != 0) != keepsTab {
		return 0
	}

	if mods&event.KeyModifierShift != 0 {
		return -1
	}
	return 1
}

// TabFocuser is the part of a canvas MoveTabFocus needs.
type TabFocuser interface {
	Focused() event.Focusable
	FocusNext()
	FocusPrevious()
}

// MoveTabFocus moves c's focus as TabDirection says for a Tab press with
// mods, and reports whether it did. A press it reports false for belongs to
// the focused widget or the shortcut path.
func MoveTabFocus(c TabFocuser, mods event.KeyModifier) bool {
	switch TabDirection(c.Focused(), mods) {
	case 1:
		c.FocusNext()
	case -1:
		c.FocusPrevious()
	default:
		return false
	}
	return true
}
