package tables

import (
	"testing"
)

func TestDataGrid_StartEditing(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})

	if !g.IsEditing() {
		t.Error("expected editing state after StartEditing")
	}
	if g.EditingCell().Row != 0 || g.EditingCell().Col != 0 {
		t.Errorf("expected editing cell {0,0}, got %v", g.EditingCell())
	}
}

func TestDataGrid_StopEditing(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.StopEditing()

	if g.IsEditing() {
		t.Error("expected editing stopped")
	}
}

func TestDataGrid_EditNotEditable(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := false
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})

	if g.IsEditing() {
		t.Error("should not be editing a non-editable column")
	}
}

func TestDataGrid_EditDefaultEditable(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name"}, // Editable is nil, DefaultColDef.Editable is false
	})
	g.DefaultColDef = &DefaultColumnDef{Editable: false}

	g.StartEditing(CellID{Row: 0, Col: 0})
	if g.IsEditing() {
		t.Error("should not be editing when DefaultColDef.Editable is false")
	}
}

func TestDataGrid_CommitEdit(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	var callbackFired bool
	var cbOld, cbNew any
	g.OnCellValueChanged = func(id CellID, oldValue, newValue any) {
		callbackFired = true
		cbOld = oldValue
		cbNew = newValue
	}

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("Bob")

	if g.IsEditing() {
		t.Error("should not be editing after commit")
	}
	if !callbackFired {
		t.Error("expected OnCellValueChanged callback")
	}
	if cbOld != "Alice" {
		t.Errorf("expected old value Alice, got %v", cbOld)
	}
	if cbNew != "Bob" {
		t.Errorf("expected new value Bob, got %v", cbNew)
	}

	// Verify data was updated
	if data[0]["name"] != "Bob" {
		t.Errorf("expected data updated to Bob, got %v", data[0]["name"])
	}
}

func TestDataGrid_Undo(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("Bob")

	if data[0]["name"] != "Bob" {
		t.Fatalf("expected Bob, got %v", data[0]["name"])
	}

	g.Undo()
	if data[0]["name"] != "Alice" {
		t.Errorf("expected Alice after undo, got %v", data[0]["name"])
	}
}

func TestDataGrid_Redo(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("Bob")
	g.Undo()
	g.Redo()

	if data[0]["name"] != "Bob" {
		t.Errorf("expected Bob after redo, got %v", data[0]["name"])
	}
}

func TestDataGrid_UndoRedoMultiple(t *testing.T) {
	data := []map[string]any{{"name": "A"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	// Edit A -> B -> C
	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("B")
	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("C")

	if data[0]["name"] != "C" {
		t.Fatalf("expected C, got %v", data[0]["name"])
	}

	// Undo twice: C -> B -> A
	g.Undo()
	if data[0]["name"] != "B" {
		t.Errorf("expected B after first undo, got %v", data[0]["name"])
	}
	g.Undo()
	if data[0]["name"] != "A" {
		t.Errorf("expected A after second undo, got %v", data[0]["name"])
	}

	// Redo once: A -> B
	g.Redo()
	if data[0]["name"] != "B" {
		t.Errorf("expected B after redo, got %v", data[0]["name"])
	}

	// New edit after undo should truncate redo stack
	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("D")
	if !g.CanUndo() {
		t.Error("expected can undo after new edit")
	}
	if g.CanRedo() {
		t.Error("expected no redo after new edit truncated stack")
	}
}

func TestDataGrid_CanUndoRedo(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "v", Editable: &editable},
	})

	if g.CanUndo() {
		t.Error("should not be able to undo with empty stack")
	}
	if g.CanRedo() {
		t.Error("should not be able to redo with empty stack")
	}

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.commitEdit("2")

	if !g.CanUndo() {
		t.Error("should be able to undo after edit")
	}
	if g.CanRedo() {
		t.Error("should not be able to redo at top of stack")
	}

	g.Undo()
	if g.CanUndo() {
		t.Error("should not be able to undo at bottom of stack")
	}
	if !g.CanRedo() {
		t.Error("should be able to redo after undo")
	}
}
