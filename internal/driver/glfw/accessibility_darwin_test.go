//go:build accessibility && darwin && !wasm

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/stretchr/testify/assert"
)

func actionNode(names ...string) *accessibility.Node {
	n := &accessibility.Node{}
	for _, name := range names {
		n.Actions = append(n.Actions, core.AccessibleAction{Name: name, Description: "do " + name})
	}
	return n
}

func customNames(n *accessibility.Node) []string {
	var names []string
	for _, a := range a11yCustomActions(n) {
		names = append(names, a.Name)
	}
	return names
}

func TestA11yCustomActions(t *testing.T) {
	// A tree row (pre-rebrand tracker #544): AXPress consumes "select", increment/decrement do
	// not apply, everything else surfaces as a custom action in order.
	row := actionNode(core.ActionSelect, core.ActionExpand, core.ActionRename, core.ActionDelete)
	assert.Equal(t, []string{core.ActionExpand, core.ActionRename, core.ActionDelete}, customNames(row))

	// A slider: increment/decrement are consumed by AXIncrement/AXDecrement.
	slider := actionNode(core.ActionIncrement, core.ActionDecrement)
	assert.Empty(t, customNames(slider))
	assert.True(t, a11yHasAction(slider, core.ActionIncrement))
	assert.True(t, a11yHasAction(slider, core.ActionDecrement))
	assert.False(t, a11yHasAction(slider, core.ActionPress))

	// A plain button: its single press-y action maps to AXPress, no customs.
	button := actionNode(core.ActionPress)
	assert.Empty(t, customNames(button))

	// Two press-family actions: AXPress consumes only the first; the second
	// stays reachable as a custom action.
	toggleAndDismiss := actionNode(core.ActionToggle, core.ActionDismiss)
	assert.Equal(t, core.ActionToggle, a11yPressAction(toggleAndDismiss))
	assert.Equal(t, []string{core.ActionDismiss}, customNames(toggleAndDismiss))

	assert.Empty(t, customNames(actionNode()))
}

func TestA11ySubroleAndDisclosed(t *testing.T) {
	role := func(r core.AccessibleRole) *accessibility.Node {
		n := &accessibility.Node{}
		n.Role = r
		return n
	}
	// Numeric BelluiAXSubrole values (cgo constants are unavailable in tests):
	// 0 none, 1 secure text field, 2 content list, 3 outline row.
	assert.EqualValues(t, 1, a11ySubrole(role(core.RolePasswordField)))
	assert.EqualValues(t, 2, a11ySubrole(role(core.RoleList)))
	assert.EqualValues(t, 3, a11ySubrole(role(core.RoleTreeItem)))
	assert.EqualValues(t, 0, a11ySubrole(role(core.RoleButton)))

	// Disclosure state: a branch declares exactly one of expand/collapse.
	assert.Equal(t, 0, a11yDisclosed(actionNode(core.ActionSelect, core.ActionExpand)))
	assert.Equal(t, 1, a11yDisclosed(actionNode(core.ActionSelect, core.ActionCollapse)))
	assert.Equal(t, -1, a11yDisclosed(actionNode(core.ActionSelect)))

	expanded := actionNode(core.ActionSelect)
	expanded.State = core.StateExpanded
	assert.Equal(t, 1, a11yDisclosed(expanded))
}
