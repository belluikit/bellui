package tables

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

func newTestGrid(t *testing.T, data []map[string]any, cols []*ColumnDef) *DataGrid {
	t.Helper()
	test.NewTempApp(t)
	return NewDataGrid(
		cols,
		func() int { return len(data) },
		func(row int) any { return data[row] },
	)
}

func TestDataGrid_SortAscending(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie", "age": 30},
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 35},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
		{Field: "age", HeaderName: "Age", Type: ColumnNumber},
	})

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	if g.RowCount() != 3 {
		t.Fatalf("expected 3 rows, got %d", g.RowCount())
	}

	// Verify sorted order: Alice, Bob, Charlie
	expected := []string{"Alice", "Bob", "Charlie"}
	for i, exp := range expected {
		row := g.pipe.DisplayRow(i).(map[string]any)
		if row["name"] != exp {
			t.Errorf("row %d: expected %s, got %v", i, exp, row["name"])
		}
	}
}

func TestDataGrid_SortDescending(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice"},
		{"name": "Bob"},
		{"name": "Charlie"},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", HeaderName: "Name"},
	})

	g.API().SetSort(SortItem{ColID: "name", Direction: SortDescending})

	expected := []string{"Charlie", "Bob", "Alice"}
	for i, exp := range expected {
		row := g.pipe.DisplayRow(i).(map[string]any)
		if row["name"] != exp {
			t.Errorf("row %d: expected %s, got %v", i, exp, row["name"])
		}
	}
}

func TestDataGrid_SortCycle(t *testing.T) {
	data := []map[string]any{
		{"v": 3}, {"v": 1}, {"v": 2},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val", Type: ColumnNumber},
	})

	// First click: ascending
	g.cycleSortColumn("v")
	if len(g.sortModel) != 1 || g.sortModel[0].Direction != SortAscending {
		t.Fatalf("expected ascending, got %v", g.sortModel)
	}
	row := g.pipe.DisplayRow(0).(map[string]any)
	if row["v"] != 1 {
		t.Errorf("expected 1 first in asc, got %v", row["v"])
	}

	// Second click: descending
	g.cycleSortColumn("v")
	if g.sortModel[0].Direction != SortDescending {
		t.Fatalf("expected descending, got %v", g.sortModel)
	}
	row = g.pipe.DisplayRow(0).(map[string]any)
	if row["v"] != 3 {
		t.Errorf("expected 3 first in desc, got %v", row["v"])
	}

	// Third click: no sort
	g.cycleSortColumn("v")
	if len(g.sortModel) != 0 {
		t.Fatalf("expected no sort, got %v", g.sortModel)
	}
}

func TestDataGrid_MultiSort(t *testing.T) {
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

	g.API().SetSort(
		SortItem{ColID: "dept", Direction: SortAscending},
		SortItem{ColID: "name", Direction: SortAscending},
	)

	expected := []string{"A-Alice", "A-Bob", "B-Alice", "B-Zara"}
	for i, exp := range expected {
		row := g.pipe.DisplayRow(i).(map[string]any)
		got := row["dept"].(string) + "-" + row["name"].(string)
		if got != exp {
			t.Errorf("row %d: expected %s, got %s", i, exp, got)
		}
	}
}

func TestDataGrid_SortCallback(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val"},
	})

	var received []SortItem
	g.OnSortChanged = func(model []SortItem) {
		received = model
	}

	g.API().SetSort(SortItem{ColID: "v", Direction: SortAscending})
	if len(received) != 1 || received[0].Direction != SortAscending {
		t.Errorf("expected callback with ascending, got %v", received)
	}
}

func TestDataGrid_SortUnsortableColumn(t *testing.T) {
	data := []map[string]any{{"v": 2}, {"v": 1}}
	sortable := false
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val", Sortable: &sortable},
	})

	g.OnHeaderTapped(0, 0) // no modifier

	// Should not have added any sort
	if len(g.sortModel) != 0 {
		t.Errorf("expected no sort on unsortable column, got %v", g.sortModel)
	}
}

func TestDataGrid_SortState(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val"},
	})

	if g.SortState(0) != 0 { // SortIndicatorNone
		t.Error("expected no sort initially")
	}

	g.API().SetSort(SortItem{ColID: "v", Direction: SortAscending})
	if g.SortState(0) != 1 { // SortIndicatorAsc
		t.Error("expected asc sort indicator")
	}

	g.API().SetSort(SortItem{ColID: "v", Direction: SortDescending})
	if g.SortState(0) != 2 { // SortIndicatorDesc
		t.Error("expected desc sort indicator")
	}
}

func TestDataGrid_ClearSort(t *testing.T) {
	data := []map[string]any{{"v": 2}, {"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", HeaderName: "Val"},
	})

	g.API().SetSort(SortItem{ColID: "v", Direction: SortAscending})
	g.API().ClearSort()

	if len(g.sortModel) != 0 {
		t.Errorf("expected empty sort model after clear, got %v", g.sortModel)
	}

	// Data should be back in original order
	row := g.pipe.DisplayRow(0).(map[string]any)
	if row["v"] != 2 {
		t.Errorf("expected original order restored, first row v=%v", row["v"])
	}
}
