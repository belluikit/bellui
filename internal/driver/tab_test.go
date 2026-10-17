package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
)

type plainFocusable struct{}

func (plainFocusable) FocusGained()             {}
func (plainFocusable) FocusLost()               {}
func (plainFocusable) TypedRune(rune)           {}
func (plainFocusable) TypedKey(*event.KeyEvent) {}

type keepsTab struct {
	plainFocusable
	keep bool
}

func (k keepsTab) AcceptsTab() bool { return k.keep }

func TestTabDirection(t *testing.T) {
	const (
		shift = event.KeyModifierShift
		ctrl  = event.KeyModifierControl
		alt   = event.KeyModifierAlt
		super = event.KeyModifierSuper
	)
	ordinary := plainFocusable{}
	tabbableNo := keepsTab{keep: false}
	field := keepsTab{keep: true} // a multi-line Entry

	for _, tc := range []struct {
		name    string
		focused event.Focusable
		mods    event.KeyModifier
		want    int
	}{
		{"nothing focused, Tab", nil, 0, 1},
		{"nothing focused, Shift+Tab", nil, shift, -1},
		{"ordinary widget, Tab", ordinary, 0, 1},
		{"ordinary widget, Shift+Tab", ordinary, shift, -1},
		{"ordinary widget, Ctrl+Tab is left for shortcuts", ordinary, ctrl, 0},
		{"Tabbable that declines, Tab", tabbableNo, 0, 1},
		{"Tabbable that declines, Ctrl+Tab is left for shortcuts", tabbableNo, ctrl, 0},
		{"field keeps Tab", field, 0, 0},
		{"field keeps Shift+Tab", field, shift, 0},
		{"field: Ctrl+Tab escapes forward", field, ctrl, 1},
		{"field: Ctrl+Shift+Tab escapes backward", field, ctrl | shift, -1},
		{"Alt+Tab never moves focus", field, alt, 0},
		{"Super+Tab never moves focus", ordinary, super, 0},
		{"Ctrl+Alt+Tab never moves focus", field, ctrl | alt, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, TabDirection(tc.focused, tc.mods))
		})
	}
}

// fakeFocuser records focus moves.
type fakeFocuser struct {
	focused    event.Focusable
	next, prev int
}

func (f *fakeFocuser) Focused() event.Focusable { return f.focused }
func (f *fakeFocuser) FocusNext()               { f.next++ }
func (f *fakeFocuser) FocusPrevious()           { f.prev++ }

type escapeUser struct{ keepsTab }

func (escapeUser) HandlesEscape() bool { return true }

// Escape, then Tab or Shift+Tab, leaves a widget that keeps Tab: the way out
// that works where Ctrl+Tab never arrives, the web (#357).
func TestTabEscape(t *testing.T) {
	field := keepsTab{keep: true}
	shift := event.KeyModifierShift

	t.Run("Tab alone stays", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		assert.False(t, e.MoveTabFocus(c, event.KeyTab, 0))
		assert.Zero(t, c.next)
	})
	t.Run("Escape then Tab moves forward, once", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		assert.False(t, e.MoveTabFocus(c, event.KeyEscape, 0))
		e.EscapeNotConsumed(field)
		assert.True(t, e.MoveTabFocus(c, event.KeyTab, 0))
		assert.Equal(t, 1, c.next)
		assert.False(t, e.MoveTabFocus(c, event.KeyTab, 0), "used once")
	})
	t.Run("Escape, Shift, Shift+Tab moves back", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		e.MoveTabFocus(c, event.KeyEscape, 0)
		e.EscapeNotConsumed(field)
		e.MoveTabFocus(c, "LeftShift", shift)
		assert.True(t, e.MoveTabFocus(c, event.KeyTab, shift))
		assert.Equal(t, 1, c.prev)
	})
	t.Run("an unnamed modifier key does not disarm", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		e.EscapeNotConsumed(field)
		e.MoveTabFocus(c, "", shift)
		assert.True(t, e.MoveTabFocus(c, event.KeyTab, shift))
	})
	t.Run("another key disarms", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		e.EscapeNotConsumed(field)
		e.MoveTabFocus(c, event.KeyA, 0)
		assert.False(t, e.MoveTabFocus(c, event.KeyTab, 0))
	})
	t.Run("Alt+Tab after Escape still does not move", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		e.EscapeNotConsumed(field)
		assert.False(t, e.MoveTabFocus(c, event.KeyTab, event.KeyModifierAlt))
	})
	t.Run("a widget using Escape itself does not arm", func(t *testing.T) {
		var e TabEscape
		w := escapeUser{keepsTab{keep: true}}
		c := &fakeFocuser{focused: w}
		e.EscapeNotConsumed(w)
		assert.False(t, e.MoveTabFocus(c, event.KeyTab, 0))
	})
	t.Run("Ctrl+Tab still works without Escape", func(t *testing.T) {
		var e TabEscape
		c := &fakeFocuser{focused: field}
		assert.True(t, e.MoveTabFocus(c, event.KeyTab, event.KeyModifierControl))
		assert.Equal(t, 1, c.next)
	})
}
