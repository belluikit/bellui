//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"testing"

	"github.com/belluikit/bellui/core"
)

func TestAtspiRole_KeyRoles(t *testing.T) {
	cases := []struct {
		role core.AccessibleRole
		num  uint32
		name string
	}{
		{core.RoleButton, atspiRolePushButton, "push button"},
		{core.RoleCheckbox, atspiRoleCheckBox, "check box"},
		{core.RoleRadioButton, atspiRoleRadioButton, "radio button"},
		{core.RoleSwitch, atspiRoleToggleBtn, "toggle button"},
		{core.RoleLabel, atspiRoleLabel, "label"},
		{core.RoleTextBox, atspiRoleEntry, "entry"},
		{core.RolePasswordField, atspiRolePasswordTxt, "password text"},
		{core.RoleList, atspiRoleList, "list"},
		{core.RoleListItem, atspiRoleListItem, "list item"},
		{core.RoleTree, atspiRoleTree, "tree"},
		{core.RoleTreeItem, atspiRoleTreeItem, "tree item"},
		{core.RoleTable, atspiRoleTable, "table"},
		{core.RoleTableCell, atspiRoleTableCell, "table cell"},
		{core.RoleTab, atspiRolePageTab, "page tab"},
		{core.RoleTabList, atspiRolePageTabList, "page tab list"},
		{core.RoleSlider, atspiRoleSlider, "slider"},
		{core.RoleHeading, atspiRoleHeading, "heading"},
		{core.RoleLink, atspiRoleLink, "link"},
		{core.RoleDialog, atspiRoleDialog, "dialog"},
	}
	for _, c := range cases {
		num, name := atspiRole(c.role)
		if num != uint32(c.num) || name != c.name {
			t.Errorf("atspiRole(%v) = (%d, %q); want (%d, %q)", c.role, num, name, c.num, c.name)
		}
	}
}

func TestAtspiRole_UnknownFallsBack(t *testing.T) {
	num, name := atspiRole(core.AccessibleRole(9999))
	if num != atspiRoleUnknown || name != "unknown" {
		t.Errorf("unknown role = (%d, %q); want (%d, %q)", num, name, atspiRoleUnknown, "unknown")
	}
}

// hasState reports whether bit b is set in the two-word bitfield.
func hasState(s [2]uint32, b uint) bool { return s[b/32]&(1<<(b%32)) != 0 }

func TestAtspiStates_BaselineAndFocus(t *testing.T) {
	// A plain enabled, focusable, focused control.
	in := nodeStateInput{focusable: true}
	s := atspiStates(&in, true)
	for _, bit := range []uint{atspiStateVisible, atspiStateShowing, atspiStateEnabled, atspiStateSensitive, atspiStateFocusable, atspiStateFocused} {
		if !hasState(s, bit) {
			t.Errorf("expected state bit %d set", bit)
		}
	}
	// Not focused → no focused bit.
	s = atspiStates(&in, false)
	if hasState(s, atspiStateFocused) {
		t.Error("focused bit should be clear when focused=false")
	}
}

func TestAtspiStates_Disabled(t *testing.T) {
	in := nodeStateInput{disabled: true, focusable: true}
	s := atspiStates(&in, false)
	if hasState(s, atspiStateEnabled) || hasState(s, atspiStateSensitive) {
		t.Error("disabled node must not report enabled/sensitive")
	}
}

func TestAtspiStates_Checkable(t *testing.T) {
	in := nodeStateInput{checkable: true, checked: true}
	s := atspiStates(&in, false)
	for _, bit := range []uint{atspiStateCheckable, atspiStateChecked, atspiStatePressed} {
		if !hasState(s, bit) {
			t.Errorf("checked node missing bit %d", bit)
		}
	}
	// A checkable-but-unchecked node advertises checkable, not checked.
	in = nodeStateInput{checkable: true}
	s = atspiStates(&in, false)
	if !hasState(s, atspiStateCheckable) || hasState(s, atspiStateChecked) {
		t.Error("unchecked checkable node state wrong")
	}
}

func TestAtspiStates_HighBits(t *testing.T) {
	// required/read-only/indeterminate live in word 1 (bit >= 32).
	in := nodeStateInput{required: true, readOnly: true, checkable: true, indeterminate: true}
	s := atspiStates(&in, false)
	if s[1] == 0 {
		t.Fatal("expected high-word state bits to be set")
	}
	for _, bit := range []uint{atspiStateRequired, atspiStateReadOnly, atspiStateIndeterminate} {
		if !hasState(s, bit) {
			t.Errorf("missing high-word bit %d", bit)
		}
	}
}

func TestAtspiStates_ExpandCollapseAndSelect(t *testing.T) {
	in := nodeStateInput{expandable: true, expanded: true, selectable: true, selected: true}
	s := atspiStates(&in, false)
	for _, bit := range []uint{atspiStateExpandable, atspiStateExpanded, atspiStateSelectable, atspiStateSelected} {
		if !hasState(s, bit) {
			t.Errorf("missing bit %d", bit)
		}
	}
	if hasState(s, atspiStateCollapsed) {
		t.Error("expanded node must not also be collapsed")
	}
	in = nodeStateInput{expandable: true}
	s = atspiStates(&in, false)
	if !hasState(s, atspiStateCollapsed) || hasState(s, atspiStateExpanded) {
		t.Error("unexpanded expandable node should be collapsed")
	}
}
