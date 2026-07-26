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
