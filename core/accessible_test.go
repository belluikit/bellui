package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessibleState_Has(t *testing.T) {
	state := StateChecked | StateDisabled

	assert.True(t, state.Has(StateChecked))
	assert.True(t, state.Has(StateDisabled))
	assert.False(t, state.Has(StateExpanded))
	assert.False(t, state.Has(StateSelected))
	assert.False(t, state.Has(StateReadOnly))
}

func TestAccessibleState_Zero(t *testing.T) {
	var state AccessibleState
	assert.False(t, state.Has(StateChecked))
	assert.False(t, state.Has(StateDisabled))
}

func TestAccessibleState_AllFlags(t *testing.T) {
	all := StateChecked | StateExpanded | StateSelected | StateDisabled |
		StateFocused | StateRequired | StateReadOnly | StatePartial | StateModal

	assert.True(t, all.Has(StateChecked))
	assert.True(t, all.Has(StateExpanded))
	assert.True(t, all.Has(StateSelected))
	assert.True(t, all.Has(StateDisabled))
	assert.True(t, all.Has(StateFocused))
	assert.True(t, all.Has(StateRequired))
	assert.True(t, all.Has(StateReadOnly))
	assert.True(t, all.Has(StatePartial))
	assert.True(t, all.Has(StateModal))
}

func TestAccessibleRole_Distinct(t *testing.T) {
	// Verify key roles have distinct values (no accidental iota collision).
	roles := map[AccessibleRole]string{
		RoleNone:        "None",
		RoleButton:      "Button",
		RoleCheckbox:    "Checkbox",
		RoleTextBox:     "TextBox",
		RoleSlider:      "Slider",
		RoleLabel:       "Label",
		RoleLink:        "Link",
		RoleList:        "List",
		RoleTable:       "Table",
		RoleTree:        "Tree",
		RoleMenu:        "Menu",
		RoleDialog:      "Dialog",
		RoleProgressBar: "ProgressBar",
		RoleForm:        "Form",
		RoleToolbar:     "Toolbar",
		RoleNavigation:  "Navigation",
		RoleComboBox:    "ComboBox",
		RoleSwitch:      "Switch",
		RoleSpinButton:  "SpinButton",
		RoleSearchField: "SearchField",
		RoleAccordion:   "Accordion",
		RoleCard:        "Card",
		RoleBadge:       "Badge",
		RoleStatus:      "Status",
		RoleTooltip:     "Tooltip",
		RoleTabList:     "TabList",
		RoleRadioGroup:  "RadioGroup",
		RoleRadioButton: "RadioButton",
	}

	seen := make(map[AccessibleRole]string)
	for role, name := range roles {
		if existing, ok := seen[role]; ok {
			t.Errorf("role %d used for both %s and %s", role, existing, name)
		}
		seen[role] = name
	}
}

func TestActionConstants(t *testing.T) {
	// Ensure standard action names are non-empty and distinct.
	actions := []string{
		ActionPress, ActionToggle, ActionIncrement, ActionDecrement,
		ActionExpand, ActionCollapse, ActionSelect, ActionDismiss,
	}
	seen := make(map[string]bool)
	for _, a := range actions {
		assert.NotEmpty(t, a)
		assert.False(t, seen[a], "duplicate action: %s", a)
		seen[a] = true
	}
}
