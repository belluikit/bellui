package tables

import (
	"testing"
)

func TestDataGrid_ColumnPinning(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", HeaderName: "A", Pinned: PinLeft},
		{Field: "b", HeaderName: "B"},
		{Field: "c", HeaderName: "C", Pinned: PinRight},
	})

	layout := g.visibleLayout()
	if len(layout.pinnedLeft) != 1 || layout.pinnedLeft[0].def.Field != "a" {
		t.Errorf("expected 'a' pinned left, got %v", layout.pinnedLeft)
	}
	if len(layout.main) != 1 || layout.main[0].def.Field != "b" {
		t.Errorf("expected 'b' in main, got %v", layout.main)
	}
	if len(layout.pinnedRight) != 1 || layout.pinnedRight[0].def.Field != "c" {
		t.Errorf("expected 'c' pinned right, got %v", layout.pinnedRight)
	}

	// Display order should be: a (left), b (main), c (right)
	if len(layout.all) != 3 {
		t.Fatalf("expected 3 visible columns, got %d", len(layout.all))
	}
	expected := []string{"a", "b", "c"}
	for i, exp := range expected {
		if layout.all[i].def.Field != exp {
			t.Errorf("display order[%d]: expected %s, got %s", i, exp, layout.all[i].def.Field)
		}
	}
}

func TestDataGrid_PinColumnViaAPI(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"}, {Field: "b"}, {Field: "c"},
	})

	g.API().SetColumnPinned("b", PinLeft)
	layout := g.visibleLayout()
	if len(layout.pinnedLeft) != 1 || layout.pinnedLeft[0].def.Field != "b" {
		t.Errorf("expected 'b' pinned left after API call")
	}

	g.API().SetColumnPinned("b", PinNone)
	layout = g.visibleLayout()
	if len(layout.pinnedLeft) != 0 {
		t.Errorf("expected no pinned left after unpin")
	}
}

func TestDataGrid_PinnedWidth(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Width: 80, Pinned: PinLeft},
		{Field: "b", Width: 120},
	})

	layout := g.visibleLayout()
	padding := g.cellPadding()
	leftW := layout.pinnedLeftWidth(padding)
	if leftW != 80 { // single column, no trailing padding
		t.Errorf("expected pinned left width 80, got %f", leftW)
	}
}

func TestDataGrid_ColumnHidden(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"}, {Field: "b", Hidden: true}, {Field: "c"},
	})

	if g.ColumnCount() != 2 {
		t.Errorf("expected 2 visible columns, got %d", g.ColumnCount())
	}

	layout := g.visibleLayout()
	if layout.all[0].def.Field != "a" || layout.all[1].def.Field != "c" {
		t.Error("expected 'a' and 'c' visible, 'b' hidden")
	}
}

func TestDataGrid_SetColumnVisible(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"}, {Field: "b"},
	})

	if g.ColumnCount() != 2 {
		t.Fatalf("expected 2 columns initially")
	}

	g.API().SetColumnVisible("a", false)
	if g.ColumnCount() != 1 {
		t.Errorf("expected 1 visible column after hiding 'a', got %d", g.ColumnCount())
	}

	g.API().SetColumnVisible("a", true)
	if g.ColumnCount() != 2 {
		t.Errorf("expected 2 columns after showing 'a', got %d", g.ColumnCount())
	}
}

func TestDataGrid_MoveColumn(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"}, {Field: "b"}, {Field: "c"},
	})

	// Move 'a' from position 0 to position 2
	g.MoveColumn(0, 2)

	layout := g.visibleLayout()
	expected := []string{"b", "c", "a"}
	for i, exp := range expected {
		if layout.all[i].def.Field != exp {
			t.Errorf("after move, position %d: expected %s, got %s", i, exp, layout.all[i].def.Field)
		}
	}
}

func TestDataGrid_MoveColumnNotMovable(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	movable := false
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Movable: &movable}, {Field: "b"},
	})

	g.MoveColumn(0, 1)

	layout := g.visibleLayout()
	if layout.all[0].def.Field != "a" {
		t.Error("non-movable column should not have been moved")
	}
}

func TestDataGrid_ColumnGroups(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{
			HeaderName: "Group",
			ColID:      "grp",
			Children: []*ColumnDef{
				{Field: "a", HeaderName: "A"},
				{Field: "b", HeaderName: "B"},
			},
		},
		{Field: "c", HeaderName: "C"},
	})

	// All 3 leaf columns should be visible
	if g.ColumnCount() != 3 {
		t.Errorf("expected 3 visible columns, got %d", g.ColumnCount())
	}
}

func TestDataGrid_ColumnGroupCollapse(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{
			HeaderName: "Group",
			ColID:      "grp",
			Children: []*ColumnDef{
				{Field: "a", HeaderName: "A"},
				{Field: "b", HeaderName: "B"},
			},
		},
		{Field: "c", HeaderName: "C"},
	})

	// Initially all visible
	if g.ColumnCount() != 3 {
		t.Fatalf("expected 3 columns, got %d", g.ColumnCount())
	}

	// Collapse the group
	g.ToggleColumnGroup("grp")
	if g.ColumnCount() != 1 {
		t.Errorf("expected 1 column after collapse, got %d", g.ColumnCount())
	}
	if !g.IsGroupCollapsed("grp") {
		t.Error("expected group to be collapsed")
	}

	// Expand again
	g.ToggleColumnGroup("grp")
	if g.ColumnCount() != 3 {
		t.Errorf("expected 3 columns after expand, got %d", g.ColumnCount())
	}
}

func TestDataGrid_CollapseExpandAll(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3, "d": 4}}
	g := newTestGrid(t, data, []*ColumnDef{
		{ColID: "g1", Children: []*ColumnDef{
			{Field: "a"}, {Field: "b"},
		}},
		{ColID: "g2", Children: []*ColumnDef{
			{Field: "c"}, {Field: "d"},
		}},
	})

	if g.ColumnCount() != 4 {
		t.Fatalf("expected 4 columns initially, got %d", g.ColumnCount())
	}

	g.CollapseAllGroups()
	if g.ColumnCount() != 0 {
		t.Errorf("expected 0 columns after collapse all, got %d", g.ColumnCount())
	}

	g.ExpandAllGroups()
	if g.ColumnCount() != 4 {
		t.Errorf("expected 4 columns after expand all, got %d", g.ColumnCount())
	}
}

func TestDataGrid_ColSpan(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2, "c": 3}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", ColSpan: func(row int) int { return 2 }},
		{Field: "b"},
		{Field: "c"},
	})

	span := g.colSpanAt(0, 0)
	if span != 2 {
		t.Errorf("expected col span 2, got %d", span)
	}

	// Normal column should span 1
	span = g.colSpanAt(0, 1)
	if span != 1 {
		t.Errorf("expected col span 1, got %d", span)
	}
}

func TestDataGrid_ColSpanClamped(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"},
		{Field: "b", ColSpan: func(row int) int { return 5 }}, // more than available
	})

	span := g.colSpanAt(0, 1)
	if span != 1 { // only 1 column remaining after position 1
		t.Errorf("expected clamped span 1, got %d", span)
	}
}

func TestDataGrid_StateWithPinning(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft}, {Field: "b"},
	})

	state := g.API().GetGridState()
	if len(state.Columns) != 2 {
		t.Fatalf("expected 2 columns in state, got %d", len(state.Columns))
	}
	if state.Columns[0].Pinned != PinLeft {
		t.Error("expected column 'a' pinned left in state")
	}
}
