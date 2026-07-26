package pipeline

import "testing"

func TestGroupTransform_Basic(t *testing.T) {
	data := []any{
		map[string]any{"dept": "A", "name": "Alice"},
		map[string]any{"dept": "B", "name": "Bob"},
		map[string]any{"dept": "A", "name": "Charlie"},
		map[string]any{"dept": "B", "name": "Diana"},
	}
	src := NewSliceSource(data)
	gt := NewGroupTransform("dept", func(row any) any {
		return row.(map[string]any)["dept"]
	})

	indices := []int{0, 1, 2, 3}
	result := gt.Apply(indices, src)

	// Should have: group A header, Alice, Charlie, group B header, Bob, Diana
	// Group headers are negative indices
	if len(result) != 6 {
		t.Fatalf("expected 6 rows (2 groups + 4 data), got %d", len(result))
	}

	// First row should be group header (negative)
	if result[0] >= 0 {
		t.Errorf("expected negative index for group header, got %d", result[0])
	}

	// Second and third should be data rows from dept A
	if result[1] != 0 || result[2] != 2 {
		t.Errorf("expected data rows [0, 2] for group A, got [%d, %d]", result[1], result[2])
	}

	// GroupRows should track the structure
	if len(gt.GroupRows) != 6 {
		t.Fatalf("expected 6 GroupRows, got %d", len(gt.GroupRows))
	}
	if !gt.GroupRows[0].IsGroup || gt.GroupRows[0].GroupKey != "A" {
		t.Error("expected first GroupRow to be group A header")
	}
	if gt.GroupRows[0].ChildCount != 2 {
		t.Errorf("expected group A child count 2, got %d", gt.GroupRows[0].ChildCount)
	}
}

func TestGroupTransform_Collapse(t *testing.T) {
	data := []any{
		map[string]any{"dept": "A", "name": "Alice"},
		map[string]any{"dept": "B", "name": "Bob"},
		map[string]any{"dept": "A", "name": "Charlie"},
	}
	src := NewSliceSource(data)
	gt := NewGroupTransform("dept", func(row any) any {
		return row.(map[string]any)["dept"]
	})

	// Collapse group A
	gt.ToggleGroup("A")
	if !gt.IsCollapsed("A") {
		t.Error("expected group A to be collapsed")
	}

	indices := []int{0, 1, 2}
	result := gt.Apply(indices, src)

	// Group A collapsed: header only (no children)
	// Group B expanded: header + Bob
	// Total: 3 (A header, B header, Bob)
	if len(result) != 3 {
		t.Fatalf("expected 3 rows after collapsing A, got %d", len(result))
	}

	// Expand group A
	gt.ToggleGroup("A")
	result = gt.Apply(indices, src)
	if len(result) != 5 {
		t.Fatalf("expected 5 rows after expanding A, got %d", len(result))
	}
}

func TestGroupTransform_ExpandCollapseAll(t *testing.T) {
	data := []any{
		map[string]any{"dept": "A"},
		map[string]any{"dept": "B"},
		map[string]any{"dept": "C"},
	}
	src := NewSliceSource(data)
	gt := NewGroupTransform("dept", func(row any) any {
		return row.(map[string]any)["dept"]
	})

	indices := []int{0, 1, 2}

	// Collapse all
	gt.CollapseAll(src, indices)
	result := gt.Apply(indices, src)
	// Only group headers
	if len(result) != 3 {
		t.Errorf("expected 3 group headers only, got %d rows", len(result))
	}

	// Expand all
	gt.ExpandAll()
	result = gt.Apply(indices, src)
	// 3 headers + 3 data rows
	if len(result) != 6 {
		t.Errorf("expected 6 rows after expand all, got %d", len(result))
	}
}
