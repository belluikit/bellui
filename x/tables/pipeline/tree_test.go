package pipeline

import "testing"

func TestTreeTransform_Basic(t *testing.T) {
	data := []any{
		map[string]any{"id": 1, "parent": -1, "name": "Root A"},
		map[string]any{"id": 2, "parent": 1, "name": "Child A1"},
		map[string]any{"id": 3, "parent": 1, "name": "Child A2"},
		map[string]any{"id": 4, "parent": -1, "name": "Root B"},
		map[string]any{"id": 5, "parent": 4, "name": "Child B1"},
	}
	src := NewSliceSource(data)
	tt := NewTreeTransform(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)

	// Expand all
	for _, d := range data {
		tt.Expanded[d.(map[string]any)["id"]] = true
	}

	result := tt.Apply([]int{0, 1, 2, 3, 4}, src)

	// Should have all 5 rows in tree order
	if len(result) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(result))
	}

	// Check depths
	if tt.TreeNodes[0].Depth != 0 || tt.TreeNodes[1].Depth != 1 {
		t.Error("expected root at depth 0, child at depth 1")
	}
}

func TestTreeTransform_Collapsed(t *testing.T) {
	data := []any{
		map[string]any{"id": 1, "parent": -1, "name": "Root"},
		map[string]any{"id": 2, "parent": 1, "name": "Child 1"},
		map[string]any{"id": 3, "parent": 1, "name": "Child 2"},
	}
	src := NewSliceSource(data)
	tt := NewTreeTransform(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)

	// Root collapsed (default)
	result := tt.Apply([]int{0, 1, 2}, src)

	// Only root visible
	if len(result) != 1 {
		t.Fatalf("expected 1 row (root only), got %d", len(result))
	}

	// Expand root
	tt.ToggleNode(1)
	result = tt.Apply([]int{0, 1, 2}, src)
	if len(result) != 3 {
		t.Fatalf("expected 3 rows after expand, got %d", len(result))
	}
}

func TestTreeTransform_CollapseAll(t *testing.T) {
	data := []any{
		map[string]any{"id": 1, "parent": -1},
		map[string]any{"id": 2, "parent": 1},
	}
	src := NewSliceSource(data)
	tt := NewTreeTransform(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)

	tt.ExpandAll([]int{0, 1}, src)
	result := tt.Apply([]int{0, 1}, src)
	if len(result) != 2 {
		t.Fatalf("expected 2 after expand all, got %d", len(result))
	}

	tt.CollapseAll()
	result = tt.Apply([]int{0, 1}, src)
	if len(result) != 1 {
		t.Fatalf("expected 1 after collapse all, got %d", len(result))
	}
}

// GetParent returns the parent's ID, not its row index (#339: the doc comment
// said row index). Here the two differ: row 2's parent ID is 10, which is row
// 0's ID, while row index 10 does not exist.
func TestTreeTransform_ParentIsAnIDNotAnIndex(t *testing.T) {
	data := []any{
		map[string]any{"id": 10, "parent": -1},
		map[string]any{"id": 20, "parent": -1},
		map[string]any{"id": 30, "parent": 10},
	}
	src := NewSliceSource(data)
	tt := NewTreeTransform(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)
	tt.ExpandAll([]int{0, 1, 2}, src)

	got := tt.Apply([]int{0, 1, 2}, src)
	want := []int{0, 2, 1} // id 30 sits under id 10
	if len(got) != len(want) {
		t.Fatalf("Apply = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Apply = %v, want %v", got, want)
		}
	}
	if tt.TreeNodes[1].Depth != 1 {
		t.Errorf("row with parent ID 10 at depth %d, want 1", tt.TreeNodes[1].Depth)
	}
}

// The parent ID is an int compared with GetID's values as an interface, so
// IDs of any other type never match and every row becomes a root. This pins
// the limitation the GetParent doc comment states; changing GetParent's type
// would lift it.
func TestTreeTransform_NonIntIDsNeverMatch(t *testing.T) {
	data := []any{
		map[string]any{"id": int64(1), "parent": -1},
		map[string]any{"id": int64(2), "parent": 1},
	}
	src := NewSliceSource(data)
	tt := NewTreeTransform(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)
	tt.ExpandAll([]int{0, 1}, src)
	tt.Apply([]int{0, 1}, src)
	for i, n := range tt.TreeNodes {
		if n.Depth != 0 {
			t.Errorf("node %d at depth %d; int64 IDs were expected never to match an int parent", i, n.Depth)
		}
	}
}
