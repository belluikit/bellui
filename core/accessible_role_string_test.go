package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessibleRoleString(t *testing.T) {
	assert.Equal(t, "None", RoleNone.String())
	assert.Equal(t, "Button", RoleButton.String())
	assert.Equal(t, "MultiLineTextBox", RoleMultiLineTextBox.String())
	assert.Equal(t, "Heading", RoleHeading.String())

	// Every declared role has a non-empty, unique-looking name.
	for r := RoleNone; r <= RoleHeading; r++ {
		assert.NotEmpty(t, r.String(), "role %d should have a name", int(r))
	}

	// Out-of-range values fall back to the numeric form.
	assert.Equal(t, "AccessibleRole(999)", AccessibleRole(999).String())
	assert.Equal(t, "AccessibleRole(-1)", AccessibleRole(-1).String())
}
