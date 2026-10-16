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
