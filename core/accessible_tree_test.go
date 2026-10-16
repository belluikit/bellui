package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
)

// mockAccessible is a minimal accessible canvas object for testing.
type mockAccessible struct {
	role     AccessibleRole
	label    string
	visible  bool
	children []CanvasObject
}

func (m *mockAccessible) AccessibleRole() AccessibleRole { return m.role }
func (m *mockAccessible) AccessibleLabel() string        { return m.label }

func (m *mockAccessible) MinSize() geo.Size      { return geo.Size{} }
func (m *mockAccessible) Move(geo.Position)      {}
func (m *mockAccessible) Position() geo.Position { return geo.Position{} }
func (m *mockAccessible) Resize(geo.Size)        {}
func (m *mockAccessible) Size() geo.Size         { return geo.Size{} }
func (m *mockAccessible) Hide()                  { m.visible = false }
func (m *mockAccessible) Visible() bool          { return m.visible }
func (m *mockAccessible) Show()                  { m.visible = true }
func (m *mockAccessible) Refresh()               {}

func mockChildren(obj CanvasObject) []CanvasObject {
	if m, ok := obj.(*mockAccessible); ok {
		return m.children
	}
	return nil
}

func TestWalkAccessibleTree_Simple(t *testing.T) {
	child1 := &mockAccessible{role: RoleButton, label: "OK", visible: true}
	child2 := &mockAccessible{role: RoleLabel, label: "Status", visible: true}
	root := &mockAccessible{
		role:     RoleForm,
		label:    "My Form",
		visible:  true,
		children: []CanvasObject{child1, child2},
	}

	var visited []AccessibleNode
	var depths []int
	WalkAccessibleTree(root, mockChildren, func(node AccessibleNode, depth int) bool {
		visited = append(visited, node)
		depths = append(depths, depth)
		return false
	})

	assert.Len(t, visited, 3)
	assert.Equal(t, RoleForm, visited[0].Role)
	assert.Equal(t, "My Form", visited[0].Label)
	assert.Equal(t, 0, depths[0])

	assert.Equal(t, RoleButton, visited[1].Role)
	assert.Equal(t, "OK", visited[1].Label)
	assert.Equal(t, 1, depths[1])

	assert.Equal(t, RoleLabel, visited[2].Role)
	assert.Equal(t, "Status", visited[2].Label)
	assert.Equal(t, 1, depths[2])
}

func TestWalkAccessibleTree_SkipsHidden(t *testing.T) {
	hidden := &mockAccessible{role: RoleButton, label: "Hidden", visible: false}
	visible := &mockAccessible{role: RoleButton, label: "Visible", visible: true}
	root := &mockAccessible{
		role:     RoleForm,
		label:    "",
		visible:  true,
		children: []CanvasObject{hidden, visible},
	}

	var labels []string
	WalkAccessibleTree(root, mockChildren, func(node AccessibleNode, _ int) bool {
		labels = append(labels, node.Label)
		return false
	})

	assert.Equal(t, []string{"", "Visible"}, labels)
}

func TestWalkAccessibleTree_EarlyTermination(t *testing.T) {
	child1 := &mockAccessible{role: RoleButton, label: "First", visible: true}
	child2 := &mockAccessible{role: RoleButton, label: "Second", visible: true}
	root := &mockAccessible{
		role:     RoleForm,
		label:    "",
		visible:  true,
		children: []CanvasObject{child1, child2},
	}

	count := 0
	WalkAccessibleTree(root, mockChildren, func(node AccessibleNode, _ int) bool {
		count++
		return count >= 2 // stop after 2 visits
	})

	assert.Equal(t, 2, count)
}

func TestWalkAccessibleTree_NilRoot(t *testing.T) {
	count := 0
	WalkAccessibleTree(nil, mockChildren, func(_ AccessibleNode, _ int) bool {
		count++
		return false
	})
	assert.Equal(t, 0, count)
}

// mockNonAccessible is a canvas object that does NOT implement Accessible.
type mockNonAccessible struct {
	visible  bool
	children []CanvasObject
}

func (m *mockNonAccessible) MinSize() geo.Size      { return geo.Size{} }
func (m *mockNonAccessible) Move(geo.Position)      {}
func (m *mockNonAccessible) Position() geo.Position { return geo.Position{} }
func (m *mockNonAccessible) Resize(geo.Size)        {}
func (m *mockNonAccessible) Size() geo.Size         { return geo.Size{} }
func (m *mockNonAccessible) Hide()                  { m.visible = false }
func (m *mockNonAccessible) Visible() bool          { return m.visible }
func (m *mockNonAccessible) Show()                  { m.visible = true }
func (m *mockNonAccessible) Refresh()               {}

func mockChildrenExt(obj CanvasObject) []CanvasObject {
	switch m := obj.(type) {
	case *mockAccessible:
		return m.children
	case *mockNonAccessible:
		return m.children
	}
	return nil
}

func TestWalkAccessibleTree_SkipsNonAccessible(t *testing.T) {
	// A non-accessible parent with an accessible child — walker should
	// visit the child but not the parent.
	child := &mockAccessible{role: RoleButton, label: "Click", visible: true}
	parent := &mockNonAccessible{visible: true, children: []CanvasObject{child}}

	var visited []AccessibleRole
	WalkAccessibleTree(parent, mockChildrenExt, func(node AccessibleNode, _ int) bool {
		visited = append(visited, node.Role)
		return false
	})

	assert.Equal(t, []AccessibleRole{RoleButton}, visited)
}

func TestSnapshotAccessible(t *testing.T) {
	obj := &mockAccessible{role: RoleButton, label: "OK", visible: true}
	node, ok := SnapshotAccessible(obj)
	assert.True(t, ok)
	assert.Equal(t, RoleButton, node.Role)
	assert.Equal(t, "OK", node.Label)
	assert.Equal(t, CanvasObject(obj), node.Object)

	_, ok = SnapshotAccessible(&mockNonAccessible{visible: true})
	assert.False(t, ok)
}

// mockIdentified is an accessible object that advertises a stable data
// identity (AccessibleIdentified, pre-rebrand tracker #635).
type mockIdentified struct {
	mockAccessible
	identity any
}

func (m *mockIdentified) AccessibleIdentity() any { return m.identity }

func TestSnapshotAccessible_NodeID(t *testing.T) {
	// A plain accessible object carries no identity: bridges key it by object.
	plain, _ := SnapshotAccessible(&mockAccessible{role: RoleButton, visible: true})
	assert.Nil(t, plain.NodeID)

	// An AccessibleIdentified object surfaces its stable key as NodeID.
	id := &mockIdentified{
		mockAccessible: mockAccessible{role: RoleTableCell, visible: true},
		identity:       [2]int{5000, 2},
	}
	node, ok := SnapshotAccessible(id)
	assert.True(t, ok)
	assert.Equal(t, [2]int{5000, 2}, node.NodeID)

	// A nil identity falls back to object keying.
	nilID := &mockIdentified{mockAccessible: mockAccessible{role: RoleTableCell, visible: true}}
	node, _ = SnapshotAccessible(nilID)
	assert.Nil(t, node.NodeID)
}

// mockValuer is an accessible object implementing AccessibleValuer with a
// configurable range.
type mockValuer struct {
	mockAccessible
	min, max, current float64
	value             string
}

func (m *mockValuer) AccessibleValue() string { return m.value }
func (m *mockValuer) AccessibleRange() (min, max, current float64) {
	return m.min, m.max, m.current
}

func TestSnapshotAccessible_HasRange(t *testing.T) {
	// A real numeric range sets HasRange.
	slider := &mockValuer{
		mockAccessible: mockAccessible{role: RoleSlider, visible: true},
		min:            0, max: 100, current: 42,
		value: "42",
	}
	node, ok := SnapshotAccessible(slider)
	assert.True(t, ok)
	assert.True(t, node.HasRange)
	assert.Equal(t, 100.0, node.RangeMax)
	assert.Equal(t, 42.0, node.RangeCurrent)
	assert.Equal(t, "42", node.Value)

	// The (0, 0, 0) sentinel means "no numeric range" — text widgets
	// implement AccessibleValuer for their string value only.
	entry := &mockValuer{
		mockAccessible: mockAccessible{role: RoleTextBox, visible: true},
		value:          "hello",
	}
	node, ok = SnapshotAccessible(entry)
	assert.True(t, ok)
	assert.False(t, node.HasRange, "the (0,0,0) sentinel must not report a range")
	assert.Equal(t, "hello", node.Value)

	// A non-zero current value alone is still a range (e.g. progress 0..0
	// is degenerate, but current != 0 distinguishes it from the sentinel).
	prog := &mockValuer{
		mockAccessible: mockAccessible{role: RoleProgressBar, visible: true},
		min:            -1, max: 0, current: -0.5,
	}
	node, _ = SnapshotAccessible(prog)
	assert.True(t, node.HasRange)
}

func TestAccessibleRole_ChildrenPresentational(t *testing.T) {
	// WAI-ARIA "children presentational" leaf controls...
	assert.True(t, RoleButton.ChildrenPresentational())
	assert.True(t, RoleTab.ChildrenPresentational())
	assert.True(t, RoleHeading.ChildrenPresentational())

	// ...while composite containers keep their accessible descendants.
	assert.False(t, RoleNone.ChildrenPresentational())
	assert.False(t, RoleGroup.ChildrenPresentational())
	assert.False(t, RoleList.ChildrenPresentational())
	assert.False(t, RoleListItem.ChildrenPresentational())
	assert.False(t, RoleDialog.ChildrenPresentational())
	assert.False(t, RoleForm.ChildrenPresentational())

	// Text-bearing roles moved to the selective treatment: only their
	// non-interactive descendants are presentational (pre-rebrand tracker #522).
	for _, r := range []AccessibleRole{
		RoleLabel, RoleTextBox, RoleMultiLineTextBox, RolePasswordField,
		RoleSearchField, RoleComboBox,
	} {
		assert.False(t, r.ChildrenPresentational(), "role %v", r)
		assert.True(t, r.NonInteractiveChildrenPresentational(), "role %v", r)
	}
	assert.False(t, RoleButton.NonInteractiveChildrenPresentational())
	assert.False(t, RoleGroup.NonInteractiveChildrenPresentational())
}
