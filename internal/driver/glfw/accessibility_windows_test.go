//go:build accessibility && windows && !wasm

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/stretchr/testify/assert"
)

func stateNode(role core.AccessibleRole, state core.AccessibleState, actions ...string) *accessibility.Node {
	n := &accessibility.Node{}
	n.Role = role
	n.State = state
	for _, name := range actions {
		n.Actions = append(n.Actions, core.AccessibleAction{Name: name})
	}
	return n
}

func TestA11yControlType(t *testing.T) {
	assert.Equal(t, uiaControlButton, a11yControlType(core.RoleButton))
	assert.Equal(t, uiaControlCheckBox, a11yControlType(core.RoleSwitch))
	assert.Equal(t, uiaControlEdit, a11yControlType(core.RolePasswordField))
	assert.Equal(t, uiaControlText, a11yControlType(core.RoleHeading))
	assert.Equal(t, uiaControlTab, a11yControlType(core.RoleTabList))
	assert.Equal(t, uiaControlTabItem, a11yControlType(core.RoleTab))
	assert.Equal(t, uiaControlTable, a11yControlType(core.RoleTable))
	assert.Equal(t, uiaControlDataItem, a11yControlType(core.RoleTableCell))
	assert.Equal(t, uiaControlPane, a11yControlType(core.RoleDialog))
	assert.Equal(t, uiaControlGroup, a11yControlType(core.RoleCard), "unmapped roles fall back to Group")
}

func TestA11yToggleState(t *testing.T) {
	assert.Equal(t, 0, a11yToggleState(stateNode(core.RoleCheckbox, 0)))
	assert.Equal(t, 1, a11yToggleState(stateNode(core.RoleSwitch, core.StateChecked)))
	assert.Equal(t, 2, a11yToggleState(stateNode(core.RoleCheckbox, core.StatePartial)))
	// Radio buttons and tabs use SelectionItem, not Toggle (UIA convention).
	assert.Equal(t, -1, a11yToggleState(stateNode(core.RoleRadioButton, core.StateChecked)))
	assert.Equal(t, -1, a11yToggleState(stateNode(core.RoleButton, 0)))
}

func TestA11ySelectable(t *testing.T) {
	// One-of-N roles get the SelectionItem pattern from the role alone.
	assert.Equal(t, 0, a11ySelectable(stateNode(core.RoleTab, 0)))
	assert.Equal(t, 1, a11ySelectable(stateNode(core.RoleTab, core.StateSelected)))
	assert.Equal(t, 1, a11ySelectable(stateNode(core.RoleRadioButton, core.StateChecked)))
	assert.Equal(t, 0, a11ySelectable(stateNode(core.RoleListItem, 0, core.ActionSelect)))
	// Anything else needs a declared "select" action (table cells, day cells).
	assert.Equal(t, 1, a11ySelectable(stateNode(core.RoleTableCell, core.StateSelected, core.ActionSelect)))
	assert.Equal(t, -1, a11ySelectable(stateNode(core.RoleTableCell, 0)))
	assert.Equal(t, -1, a11ySelectable(stateNode(core.RoleButton, 0)))
}
