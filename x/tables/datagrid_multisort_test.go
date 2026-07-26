package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
)

func TestDataGrid_MultiSortShiftClick(t *testing.T) {
	data := []map[string]any{
		{"dept": "B", "name": "Zara"},
		{"dept": "A", "name": "Bob"},
		{"dept": "B", "name": "Alice"},
		{"dept": "A", "name": "Alice"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept", HeaderName: "Dept"},
		{Field: "name", HeaderName: "Name"},
	})

	// Click dept header (no modifier) — single sort
	g.OnHeaderTapped(0, 0)
	if len(g.sortModel) != 1 {
		t.Fatalf("expected 1 sort, got %d", len(g.sortModel))
	}
	if g.sortModel[0].ColID != "dept" {
		t.Errorf("expected sort on dept, got %s", g.sortModel[0].ColID)
	}

	// Shift+click name header — add to multi-sort
	g.OnHeaderTapped(1, event.KeyModifierShift)
	if len(g.sortModel) != 2 {
		t.Fatalf("expected 2 sorts after shift+click, got %d", len(g.sortModel))
	}
	if g.sortModel[1].ColID != "name" {
		t.Errorf("expected second sort on name, got %s", g.sortModel[1].ColID)
	}

	// Verify data order: A-Alice, A-Bob, B-Alice, B-Zara
	expected := []string{"A-Alice", "A-Bob", "B-Alice", "B-Zara"}
	for i, exp := range expected {
		row := g.pipe.DisplayRow(i).(map[string]any)
		got := row["dept"].(string) + "-" + row["name"].(string)
		if got != exp {
			t.Errorf("row %d: expected %s, got %s", i, exp, got)
		}
	}
}

func TestDataGrid_MultiSortShiftClickCycle(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"}, {Field: "b"},
	})

	// Click a (ascending)
	g.OnHeaderTapped(0, 0)
	// Shift+click b (add ascending)
	g.OnHeaderTapped(1, event.KeyModifierShift)
	if len(g.sortModel) != 2 {
		t.Fatalf("expected 2 sorts, got %d", len(g.sortModel))
	}

	// Shift+click b again (cycle to descending)
	g.OnHeaderTapped(1, event.KeyModifierShift)
	if len(g.sortModel) != 2 {
		t.Fatalf("expected still 2 sorts, got %d", len(g.sortModel))
	}
	if g.sortModel[1].Direction != SortDescending {
		t.Errorf("expected b descending, got %v", g.sortModel[1].Direction)
	}

	// Shift+click b again (remove from sort)
	g.OnHeaderTapped(1, event.KeyModifierShift)
	if len(g.sortModel) != 1 {
		t.Errorf("expected 1 sort after removing b, got %d", len(g.sortModel))
	}
}

func TestDataGrid_ClickWithoutShiftResetsSortModel(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a"}, {Field: "b"},
	})

	// Build multi-sort
	g.OnHeaderTapped(0, 0)
	g.OnHeaderTapped(1, event.KeyModifierShift)
	if len(g.sortModel) != 2 {
		t.Fatalf("expected 2, got %d", len(g.sortModel))
	}

	// Click without shift — should reset to single sort
	g.OnHeaderTapped(0, 0)
	if len(g.sortModel) != 1 {
		t.Errorf("expected single sort after click without shift, got %d", len(g.sortModel))
	}
}
