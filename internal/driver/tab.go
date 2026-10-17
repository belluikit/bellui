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
// the way out. Any key other than a modifier disarms it, and so does any typed
// character (Disarm): some drivers give letter keys no name, so the character
// is the only sign of them. It is also armed for the widget focused at the
// Escape alone, so a focus change by pointer or by code cancels it.
//
// The zero value is ready to use; a driver keeps one per window.
type TabEscape struct {
	armed event.Focusable // the widget the Escape was pressed in, or nil
}

// MoveTabFocus is MoveTabFocus for a driver that offers the Escape way out.
// Call it for every key press: a key other than Tab only updates the state.
func (e *TabEscape) MoveTabFocus(c TabFocuser, key event.KeyName, mods event.KeyModifier) bool {
	armed := e.armed
	if !isModifierKey(key) {
		e.armed = nil
	}
	if key != event.KeyTab {
		return false
	}
	// Ctrl+Tab on a widget that keeps Tab is the first way out; the Escape
	// stands in for the Ctrl.
	if armed != nil && armed == c.Focused() && mods&^event.KeyModifierShift == 0 &&
		MoveTabFocus(c, mods|event.KeyModifierControl) {
		return true
	}
	return MoveTabFocus(c, mods)
}

// EscapeNotConsumed arms the way out after an Escape that HandleEscape did not
// consume, when the focused widget keeps Tab and is not using Escape itself.
func (e *TabEscape) EscapeNotConsumed(focused event.Focusable) {
	e.armed = nil
	if h, ok := focused.(event.EscapeHandler); ok && h.HandlesEscape() {
		return
	}
	if focusKeepsTab(focused) {
		e.armed = focused
	}
}

// Disarm cancels the way out. Drivers call it for every typed character and
// when their window loses focus.
func (e *TabEscape) Disarm() {
	e.armed = nil
}

func focusKeepsTab(focused event.Focusable) bool {
	t, ok := focused.(event.Tabbable)
	return ok && t.AcceptsTab()
}

// isModifierKey reports whether key is a modifier on its own, which must not
// disarm TabEscape: Shift goes down before the Tab of Shift+Tab. Some drivers
// do not name modifier keys and report them as "" — but some report letter
// keys as "" too, which is why typed characters disarm through Disarm.
func isModifierKey(key event.KeyName) bool {
	switch key {
	case "", desktop.KeyShiftLeft, desktop.KeyShiftRight, desktop.KeyControlLeft, desktop.KeyControlRight,
		desktop.KeyAltLeft, desktop.KeyAltRight, desktop.KeySuperLeft, desktop.KeySuperRight:
		return true
	}
	return false
}
