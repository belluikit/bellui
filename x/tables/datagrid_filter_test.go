package tables

import (
	"fmt"
	"testing"
)

func TestDataGrid_ColumnFilter(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 35},
		{"name": "Charlie", "age": 30},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	g.API().SetColumnFilter("age", func(v any) bool {
		n, _ := v.(int)
		return n >= 30
	})

	if g.RowCount() != 2 {
		t.Fatalf("expected 2 rows after filter, got %d", g.RowCount())
	}

	row0 := g.pipe.DisplayRow(0).(map[string]any)
	row1 := g.pipe.DisplayRow(1).(map[string]any)
	if row0["name"] != "Bob" {
		t.Errorf("expected Bob, got %v", row0["name"])
	}
	if row1["name"] != "Charlie" {
		t.Errorf("expected Charlie, got %v", row1["name"])
	}
}

func TestDataGrid_ClearColumnFilter(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice"}, {"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
	})

	g.API().SetColumnFilter("name", func(v any) bool {
		return v == "Alice"
	})
	if g.RowCount() != 1 {
		t.Fatalf("expected 1 row after filter, got %d", g.RowCount())
	}

	// Clear the filter
	g.API().SetColumnFilter("name", nil)
	if g.RowCount() != 2 {
		t.Fatalf("expected 2 rows after clear, got %d", g.RowCount())
	}
}

func TestDataGrid_ClearAllFilters(t *testing.T) {
	data := []map[string]any{
		{"a": 1, "b": "x"},
		{"a": 2, "b": "y"},
		{"a": 3, "b": "z"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Type: ColumnNumber},
		{Field: "b"},
	})

	g.API().SetColumnFilter("a", func(v any) bool { return v.(int) > 1 })
	g.API().SetColumnFilter("b", func(v any) bool { return v == "y" })
	if g.RowCount() != 1 {
		t.Fatalf("expected 1 row with both filters, got %d", g.RowCount())
	}

	g.API().ClearFilters()
	if g.RowCount() != 3 {
		t.Fatalf("expected 3 rows after clear all, got %d", g.RowCount())
	}
}

func TestDataGrid_FilterThenSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie", "age": 30},
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 35},
		{"name": "Diana", "age": 20},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	// Filter: age >= 25
	g.API().SetColumnFilter("age", func(v any) bool {
		n, _ := v.(int)
		return n >= 25
	})

	// Sort: name ascending
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	if g.RowCount() != 3 {
		t.Fatalf("expected 3 rows, got %d", g.RowCount())
	}

	expected := []string{"Alice", "Bob", "Charlie"}
	for i, exp := range expected {
		row := g.pipe.DisplayRow(i).(map[string]any)
		if row["name"] != exp {
			t.Errorf("row %d: expected %s, got %v", i, exp, row["name"])
		}
	}
}

func TestDataGrid_QuickFilter(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "city": "New York"},
		{"name": "Bob", "city": "London"},
		{"name": "Charlie", "city": "New Delhi"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "city", HeaderName: "City"},
	})

	g.API().SetQuickFilter("new")

	if g.RowCount() != 2 {
		t.Fatalf("expected 2 rows matching 'new', got %d", g.RowCount())
	}

	// Both rows should contain "New" in city field
	for i := 0; i < g.RowCount(); i++ {
		row := g.pipe.DisplayRow(i).(map[string]any)
		city := row["city"].(string)
		if !containsLower(city, "new") {
			t.Errorf("row %d city %q doesn't contain 'new'", i, city)
		}
	}
}

func TestDataGrid_QuickFilterCaseInsensitive(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice"},
		{"name": "BOB"},
		{"name": "Charlie"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
	})

	g.API().SetQuickFilter("bob")
	if g.RowCount() != 1 {
		t.Fatalf("expected 1 row matching 'bob' (case insensitive), got %d", g.RowCount())
	}
}

func TestDataGrid_QuickFilterClear(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice"}, {"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
	})

	g.API().SetQuickFilter("alice")
	if g.RowCount() != 1 {
		t.Fatalf("expected 1, got %d", g.RowCount())
	}

	g.API().SetQuickFilter("")
	if g.RowCount() != 2 {
		t.Fatalf("expected 2 after clear, got %d", g.RowCount())
	}
}

func TestDataGrid_FilterCallback(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val"},
	})

	called := false
	g.OnFilterChanged = func() {
		called = true
	}

	g.API().SetColumnFilter("v", func(v any) bool { return true })
	if !called {
		t.Error("expected OnFilterChanged callback")
	}
}

func TestDataGrid_IsFilterActive(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val"},
	})

	if g.IsFilterActive(0) {
		t.Error("expected no filter active initially")
	}

	g.API().SetColumnFilter("v", func(v any) bool { return true })
	if !g.IsFilterActive(0) {
		t.Error("expected filter active after set")
	}

	g.API().SetColumnFilter("v", nil)
	if g.IsFilterActive(0) {
		t.Error("expected filter inactive after clear")
	}
}

func TestDataGrid_FilterLargeDataset(t *testing.T) {
	n := 10000
	data := make([]map[string]any, n)
	for i := range data {
		data[i] = map[string]any{"i": i, "name": fmt.Sprintf("row_%d", i)}
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "i", Type: ColumnNumber},
		{Field: "name"},
	})

	// Filter to only even indices
	g.API().SetColumnFilter("i", func(v any) bool {
		n, _ := v.(int)
		return n%2 == 0
	})

	if g.RowCount() != n/2 {
		t.Errorf("expected %d rows, got %d", n/2, g.RowCount())
	}

	// Sort descending
	g.API().SetSort(SortItem{ColID: "i", Direction: SortDescending})

	// First row should be the largest even number
	row := g.pipe.DisplayRow(0).(map[string]any)
	if row["i"] != n-2 {
		t.Errorf("expected %d, got %v", n-2, row["i"])
	}
}
