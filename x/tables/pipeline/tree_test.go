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

// GetID may return any integer kind: database rows commonly carry int64 IDs.
// They match the int parent ID by value. Only non-integer IDs, such as
// strings, can never match, so their rows are all roots.
func TestTreeTransform_IntegerIDsOfAnyKindMatch(t *testing.T) {
	for _, tc := range []struct {
		name      string
		root, kid any
		wantDepth int
	}{
		{"int64", int64(1), int64(2), 1},
		{"int32", int32(1), int32(2), 1},
		{"uint", uint(1), uint(2), 1},
		{"string", "1", "2", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []any{
				map[string]any{"id": tc.root, "parent": -1},
				map[string]any{"id": tc.kid, "parent": 1},
			}
			src := NewSliceSource(data)
			tt := NewTreeTransform(
				func(row any) any { return row.(map[string]any)["id"] },
				func(row any) int { return row.(map[string]any)["parent"].(int) },
			)
			tt.ExpandAll([]int{0, 1}, src)
			if got := tt.Apply([]int{0, 1}, src); len(got) != 2 {
				t.Fatalf("Apply = %v, want both rows", got)
			}
			if d := tt.TreeNodes[1].Depth; d != tc.wantDepth {
				t.Errorf("child row at depth %d, want %d", d, tc.wantDepth)
			}
		})
	}
}

// A row that is its own parent, or one of a loop of rows that are each
// other's parents, has a parent that matches a row, yet no root leads to it.
// Such rows must still be shown: the first of each loop, in row order,
// becomes a root.
func TestTreeTransform_ParentLoopsStayVisible(t *testing.T) {
	data := []any{
		map[string]any{"id": 1, "parent": -1},
		map[string]any{"id": 5, "parent": 5}, // its own parent
		map[string]any{"id": 7, "parent": 8}, // 7 and 8 are each other's parent
		map[string]any{"id": 8, "parent": 7},
		map[string]any{"id": 9, "parent": 8}, // hangs off the loop
	}
	src := NewSliceSource(data)
	tt := NewTreeTransform(
		func(row any) any { return row.(map[string]any)["id"] },
		func(row any) int { return row.(map[string]any)["parent"].(int) },
	)
	all := []int{0, 1, 2, 3, 4}
	tt.ExpandAll(all, src)

	got := tt.Apply(all, src)
	want := []int{0, 1, 2, 3, 4} // 7 is the loop's root, 8 under it, 9 under 8
	if len(got) != len(want) {
		t.Fatalf("Apply = %v, want every row %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Apply = %v, want %v", got, want)
		}
	}
	wantDepth := []int{0, 0, 0, 1, 2}
	for i, n := range tt.TreeNodes {
		if n.Depth != wantDepth[i] {
			t.Errorf("row %d at depth %d, want %d", n.SourceIndex, n.Depth, wantDepth[i])
		}
	}
	if tt.TreeNodes[1].HasChildren {
		t.Error("the row that is its own parent reports children; it has none once it is a root")
	}

	tt.CollapseAll()
	if got := tt.Apply(all, src); len(got) != 3 {
		t.Errorf("collapsed Apply = %v, want the three roots", got)
	}
}
