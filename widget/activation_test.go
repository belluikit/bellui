package widget

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/stretchr/testify/assert"
)

// TestActivatable_Contract verifies the button-like controls declare keyboard
// activation via event.Activatable so the driver can fire the central
// activation signal in parity with a pointer tap (#700). ActivatedByKey must be
// true only for the key the widget actually acts on (Space), not for navigation
// keys, so analytics never records an activation that did not happen.
func TestActivatable_Contract(t *testing.T) {
	cases := []struct {
		name string
		obj  event.Activatable
	}{
		{"Button", NewButton("ok", nil)},
		{"Hyperlink", NewHyperlink("bellui", nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, tc.obj.ActivatedByKey(&event.KeyEvent{Name: event.KeySpace}),
				"Space activates %s", tc.name)
			assert.False(t, tc.obj.ActivatedByKey(&event.KeyEvent{Name: event.KeyReturn}),
				"Return does not activate %s", tc.name)
			assert.False(t, tc.obj.ActivatedByKey(&event.KeyEvent{Name: event.KeyDown}),
				"navigation keys do not activate %s", tc.name)
		})
	}
}
