package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
)

func TestDataGrid_SingleRowSelection(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}, {"v": 3}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionSingleRow

	g.selectCellWithModifiers(CellID{Row: 1, Col: 0}, 0)

	rows := g.API().GetSelectedRows()
	if len(rows) != 1 || rows[0] != 1 {
		t.Errorf("expected [1], got %v", rows)
	}

	// Selecting another row should deselect the first
	g.selectCellWithModifiers(CellID{Row: 2, Col: 0}, 0)
	rows = g.API().GetSelectedRows()
	if len(rows) != 1 || rows[0] != 2 {
		t.Errorf("expected [2], got %v", rows)
	}
}

func TestDataGrid_MultiRowCtrlClick(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}, {"v": 3}, {"v": 4}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionMultiRow

	// Click row 0
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)

	// Ctrl+click row 2
	g.selectCellWithModifiers(CellID{Row: 2, Col: 0}, event.KeyModifierControl)

	rows := g.API().GetSelectedRows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 selected rows, got %d: %v", len(rows), rows)
	}

	hasRow := func(r int) bool {
		for _, v := range rows {
			if v == r {
				return true
			}
		}
		return false
	}
	if !hasRow(0) || !hasRow(2) {
		t.Errorf("expected rows 0 and 2, got %v", rows)
	}

	// Ctrl+click row 0 again to deselect it
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, event.KeyModifierControl)
	rows = g.API().GetSelectedRows()
	if len(rows) != 1 || rows[0] != 2 {
		t.Errorf("expected [2] after toggle, got %v", rows)
	}
}

func TestDataGrid_MultiRowShiftClick(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"v": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionMultiRow

	// Click row 2 (sets anchor)
	g.selectCellWithModifiers(CellID{Row: 2, Col: 0}, 0)

	// Shift+click row 5 (select range 2-5)
	g.selectCellWithModifiers(CellID{Row: 5, Col: 0}, event.KeyModifierShift)

	rows := g.API().GetSelectedRows()
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows in range [2,5], got %d: %v", len(rows), rows)
	}
	for _, r := range []int{2, 3, 4, 5} {
		if !g.selectedRows[r] {
			t.Errorf("expected row %d to be selected", r)
		}
	}
}

func TestDataGrid_MultiRowShiftClickReverse(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"v": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionMultiRow

	// Click row 7 (sets anchor)
	g.selectCellWithModifiers(CellID{Row: 7, Col: 0}, 0)

	// Shift+click row 3 (select range 3-7, reverse direction)
	g.selectCellWithModifiers(CellID{Row: 3, Col: 0}, event.KeyModifierShift)

	rows := g.API().GetSelectedRows()
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows in range [3,7], got %d: %v", len(rows), rows)
	}
}

func TestDataGrid_SelectAll(t *testing.T) {
	data := make([]map[string]any, 5)
	for i := range data {
		data[i] = map[string]any{"v": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionMultiRow

	g.SelectAll()

	rows := g.API().GetSelectedRows()
	if len(rows) != 5 {
		t.Errorf("expected all 5 rows selected, got %d", len(rows))
	}
}

func TestDataGrid_SelectAllSingleRow(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionSingleRow

	g.SelectAll() // should be a no-op for single-row mode
	rows := g.API().GetSelectedRows()
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for single-row SelectAll, got %d", len(rows))
	}
}

func TestDataGrid_SelectionNone(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionNone

	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)
	rows := g.API().GetSelectedRows()
	if len(rows) != 0 {
		t.Errorf("expected no selection in None mode, got %v", rows)
	}
}

func TestDataGrid_SelectionCallback(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionMultiRow

	var received []int
	g.OnSelectionChanged = func(rows []int) {
		received = rows
	}

	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)
	if len(received) != 1 {
		t.Errorf("expected callback with 1 row, got %v", received)
	}
}
