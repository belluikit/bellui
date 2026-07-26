//go:build accessibility && linux && !android && !wasm && !js

package glfw

import (
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/accessibility"
)

func semNode(role core.AccessibleRole, state core.AccessibleState) *accessibility.Node {
	return &accessibility.Node{AccessibleNode: core.AccessibleNode{Role: role, State: state}}
}

func TestDecodeNodeState_Checkbox(t *testing.T) {
	in := decodeNodeState(semNode(core.RoleCheckbox, core.StateChecked))
	if !in.checkable || !in.checked {
		t.Errorf("checkbox: got %+v", in)
	}
	in = decodeNodeState(semNode(core.RoleCheckbox, core.StatePartial))
	if !in.checkable || !in.indeterminate || in.checked {
		t.Errorf("indeterminate checkbox: got %+v", in)
	}
}

func TestDecodeNodeState_TextEditability(t *testing.T) {
	in := decodeNodeState(semNode(core.RoleTextBox, 0))
	if !in.editable || in.multiLine {
		t.Errorf("single-line entry: got %+v", in)
	}
	in = decodeNodeState(semNode(core.RoleMultiLineTextBox, 0))
	if !in.editable || !in.multiLine {
		t.Errorf("multi-line entry: got %+v", in)
	}
	// A read-only text box is not editable.
	in = decodeNodeState(semNode(core.RoleTextBox, core.StateReadOnly))
	if in.editable || !in.readOnly {
		t.Errorf("read-only entry: got %+v", in)
	}
}

func TestDecodeNodeState_ExpandAndSelect(t *testing.T) {
	// A tree item declaring the expand action is expandable+collapsed.
	n := semNode(core.RoleTreeItem, core.StateSelected)
	n.Actions = []core.AccessibleAction{{Name: core.ActionExpand}}
	in := decodeNodeState(n)
	if !in.expandable || in.expanded {
		t.Errorf("collapsed tree item: got %+v", in)
	}
	if !in.selectable || !in.selected {
		t.Errorf("selected tree item: got %+v", in)
	}
	// StateExpanded (or the collapse action) → expanded.
	n = semNode(core.RoleTreeItem, core.StateExpanded)
	in = decodeNodeState(n)
	if !in.expandable || !in.expanded {
		t.Errorf("expanded tree item: got %+v", in)
	}
}

func TestPathsEqualAndIndex(t *testing.T) {
	a := []dbus.ObjectPath{"/a", "/b", "/c"}
	if !pathsEqual(a, []dbus.ObjectPath{"/a", "/b", "/c"}) {
		t.Error("equal slices reported unequal")
	}
	if pathsEqual(a, []dbus.ObjectPath{"/a", "/b"}) {
		t.Error("different-length slices reported equal")
	}
	if pathsEqual(a, []dbus.ObjectPath{"/a", "/x", "/c"}) {
		t.Error("differing slices reported equal")
	}
	if got := indexOfPath(a, "/b"); got != 1 {
		t.Errorf("indexOfPath = %d; want 1", got)
	}
	if got := indexOfPath(a, "/z"); got != -1 {
		t.Errorf("indexOfPath(missing) = %d; want -1", got)
	}
}
