package driver

import (
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

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

// TabEscape is a window's second way out of a widget that keeps Tab: Escape,
// then Tab or Shift+Tab, as in CodeMirror and Monaco. Ctrl+Tab is the first
// way, but browsers keep Ctrl+Tab for switching their own tabs and never
// deliver it to the page, so on the web it cannot work (#357).
//
// An Escape arms it only when the Escape had nothing else to do: no pop-up or
// dialog closed (HandleEscape reported false) and the focused widget is not
// using the key for a state of its own (an event.EscapeHandler that handles
// it). So the first Escape still closes the dropdown, and a second one arms
// the way out. Any key other than a modifier disarms it.
//
// The zero value is ready to use; a driver keeps one per window.
type TabEscape struct {
	armed bool
}

// MoveTabFocus is MoveTabFocus for a driver that offers the Escape way out.
// Call it for every key press: a key other than Tab only updates the state.
func (e *TabEscape) MoveTabFocus(c TabFocuser, key event.KeyName, mods event.KeyModifier) bool {
	armed := e.armed
	if !isModifierKey(key) {
		e.armed = false
	}
	if key != event.KeyTab {
		return false
	}
	if armed && mods&^event.KeyModifierShift == 0 && focusKeepsTab(c.Focused()) {
		if mods&event.KeyModifierShift != 0 {
			c.FocusPrevious()
		} else {
			c.FocusNext()
		}
		return true
	}
	return MoveTabFocus(c, mods)
}

// EscapeNotConsumed arms the way out after an Escape that HandleEscape did not
// consume, when the focused widget keeps Tab and is not using Escape itself.
func (e *TabEscape) EscapeNotConsumed(focused event.Focusable) {
	if h, ok := focused.(event.EscapeHandler); ok && h.HandlesEscape() {
		return
	}
	e.armed = focusKeepsTab(focused)
}

func focusKeepsTab(focused event.Focusable) bool {
	t, ok := focused.(event.Tabbable)
	return ok && t.AcceptsTab()
}

// isModifierKey reports whether key is a modifier on its own, which must not
// disarm TabEscape: Shift goes down before the Tab of Shift+Tab. Drivers that
// do not name modifier keys report them as "".
func isModifierKey(key event.KeyName) bool {
	switch key {
	case "", desktop.KeyShiftLeft, desktop.KeyShiftRight, desktop.KeyControlLeft, desktop.KeyControlRight,
		desktop.KeyAltLeft, desktop.KeyAltRight, desktop.KeySuperLeft, desktop.KeySuperRight:
		return true
	}
	return false
}
