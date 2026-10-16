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
