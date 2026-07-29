package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
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

func TestDataGrid_CanEditGatesPerCell(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "locked": true},
		{"name": "Bob", "locked": false},
	}
	editable := true
	var gated []any
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable, CanEdit: func(rowData any) bool {
			gated = append(gated, rowData)
			locked, _ := rowData.(map[string]any)["locked"].(bool)
			return !locked
		}},
	})

	// Locked row: the edit is refused before any editor is created.
	g.StartEditing(CellID{Row: 0, Col: 0})
	if g.IsEditing() {
		t.Error("expected CanEdit=false to refuse the edit")
	}
	if g.editWidget != nil {
		t.Error("expected no editor widget for a refused edit")
	}

	// Unlocked row: editing proceeds as usual.
	g.StartEditing(CellID{Row: 1, Col: 0})
	if !g.IsEditing() {
		t.Error("expected CanEdit=true to allow the edit")
	}

	// The gate is consulted with the row's data.
	if len(gated) != 2 {
		t.Fatalf("expected CanEdit called twice, got %d", len(gated))
	}
	if m, ok := gated[0].(map[string]any); !ok || m["name"] != "Alice" {
		t.Errorf("expected first CanEdit call with Alice's row, got %v", gated[0])
	}
}

func TestDataGrid_CanEditDoesNotOverrideEditable(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := false
	canEditCalled := false
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable, CanEdit: func(any) bool {
			canEditCalled = true
			return true
		}},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})

	if g.IsEditing() {
		t.Error("CanEdit=true must not make a non-editable column editable")
	}
	if canEditCalled {
		t.Error("CanEdit should not be consulted on a non-editable column")
	}
}

func TestDataGrid_DefaultEditorIsEntry(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})

	entry, ok := g.editWidget.(*widget.Entry)
	if !ok {
		t.Fatalf("expected *widget.Entry editor, got %T", g.editWidget)
	}
	if entry.Text != "Alice" {
		t.Errorf("expected editor seeded with Alice, got %q", entry.Text)
	}

	// Enter in the focused entry commits via OnSubmitted.
	entry.SetText("Bob")
	entry.OnSubmitted(entry.Text)

	if g.IsEditing() {
		t.Error("expected editing ended after submit")
	}
	if data[0]["name"] != "Bob" {
		t.Errorf("expected data updated to Bob, got %v", data[0]["name"])
	}
}

func TestDataGrid_EscapeInEntryCancelsEdit(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})
	entry := g.editWidget.(*widget.Entry)
	entry.SetText("discarded")

	// Escape typed in the focused editor cancels without committing.
	entry.TypedKey(&event.KeyEvent{Name: event.KeyEscape})

	if g.IsEditing() {
		t.Error("expected Escape in the editor to cancel the edit")
	}
	if data[0]["name"] != "Alice" {
		t.Errorf("expected data unchanged, got %v", data[0]["name"])
	}
	if g.CanUndo() {
		t.Error("cancelled edit must not enter the undo stack")
	}
}

func TestDataGrid_ColumnEditorFactory_Select(t *testing.T) {
	data := []map[string]any{{"role": "admin"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "role", Editable: &editable, Editor: cell.NewSelectEditor([]string{"admin", "user", "guest"})},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})

	sel, ok := g.editWidget.(*widget.Select)
	if !ok {
		t.Fatalf("expected *widget.Select editor, got %T", g.editWidget)
	}
	if sel.Selected != "admin" {
		t.Errorf("expected editor seeded with admin, got %q", sel.Selected)
	}

	// Picking an option is the whole edit — it commits immediately.
	sel.SetSelected("guest")

	if g.IsEditing() {
		t.Error("expected editing ended after selecting an option")
	}
	if data[0]["role"] != "guest" {
		t.Errorf("expected data updated to guest, got %v", data[0]["role"])
	}
}

func TestDataGrid_ColumnEditorFactory_CheckboxTypedValue(t *testing.T) {
	data := []map[string]any{{"active": false}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "active", Type: ColumnBoolean, Editable: &editable, Editor: cell.NewCheckboxEditor()},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})

	check, ok := g.editWidget.(*widget.Check)
	if !ok {
		t.Fatalf("expected *widget.Check editor, got %T", g.editWidget)
	}

	// Toggling commits immediately — and writes the typed bool, not a string.
	check.SetChecked(true)

	if g.IsEditing() {
		t.Error("expected editing ended after toggling")
	}
	if v, ok := data[0]["active"].(bool); !ok || !v {
		t.Errorf("expected typed bool true, got %T %v", data[0]["active"], data[0]["active"])
	}
}

// cancellingEditor reports every edit as cancelled from EndEdit.
type cancellingEditor struct{}

func (cancellingEditor) CreateEditor() core.CanvasObject               { return widget.NewEntry() }
func (cancellingEditor) BeginEdit(core.CanvasObject, any, cell.Params) {}
func (cancellingEditor) EndEdit(core.CanvasObject) (any, bool)         { return "ignored", true }

func TestDataGrid_CommitEditingCancelledDiscards(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable, Editor: cancellingEditor{}},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.CommitEditing()

	if g.IsEditing() {
		t.Error("expected editing ended after cancelled commit")
	}
	if data[0]["name"] != "Alice" {
		t.Errorf("expected data unchanged, got %v", data[0]["name"])
	}
	if g.CanUndo() {
		t.Error("cancelled edit must not enter the undo stack")
	}
}

func TestDataGrid_StartEditingCommitsPrevious(t *testing.T) {
	data := []map[string]any{{"a": "1", "b": "2"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Editable: &editable},
		{Field: "b", Editable: &editable},
	})

	g.StartEditing(CellID{Row: 0, Col: 0})
	g.editWidget.(*widget.Entry).SetText("edited")

	// Moving the edit to another cell keeps what was typed.
	g.StartEditing(CellID{Row: 0, Col: 1})

	if data[0]["a"] != "edited" {
		t.Errorf("expected previous edit committed, got %v", data[0]["a"])
	}
	if ec := g.EditingCell(); ec == nil || ec.Col != 1 {
		t.Errorf("expected editing moved to col 1, got %v", ec)
	}
}

func TestDataGrid_EditOverlayRendered(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}, {"name": "Bob"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Width: 120, Editable: &editable},
	})

	w := test.NewWindow(g)
	w.Resize(geo.NewSize(400, 300))

	g.StartEditing(CellID{Row: 1, Col: 0})

	if g.editWidget == nil || !g.editWidget.Visible() {
		t.Fatal("expected a visible edit widget after StartEditing")
	}
	pos := g.editWidget.Position()
	size := g.editWidget.Size()
	headerH := g.HeaderHeight()
	rowY, rowH := g.viewport.RowY(1)
	if pos.X != 0 || pos.Y != headerH+rowY {
		t.Errorf("expected editor at (0,%v), got %v", headerH+rowY, pos)
	}
	if size.Width != 120 || size.Height != rowH {
		t.Errorf("expected editor size 120x%v, got %v", rowH, size)
	}

	// The editor must be part of the render tree to be visible/interactive.
	r := test.TempWidgetRenderer(t, g)
	editor := g.editWidget
	found := false
	for _, o := range r.Objects() {
		if o == editor {
			found = true
		}
	}
	if !found {
		t.Error("expected edit widget among renderer objects")
	}

	// Ending the edit removes the overlay.
	g.StopEditing()
	for _, o := range r.Objects() {
		if o == editor {
			t.Error("expected edit widget removed from renderer objects")
		}
	}
}

// pasteInto selects the given cell, puts content on the clipboard, and pastes.
func pasteInto(t *testing.T, g *DataGrid, id CellID, content string) {
	t.Helper()
	g.selectedCell = &id
	bellui.CurrentApp().Clipboard().SetContent(content)
	g.pasteToSelection()
}

func TestDataGrid_PasteRespectsEditable(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := false
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})
	g.OnCellValueChanged = func(CellID, any, any) {
		t.Error("paste into a non-editable column must not fire OnCellValueChanged")
	}

	pasteInto(t, g, CellID{Row: 0, Col: 0}, "Mallory")

	if data[0]["name"] != "Alice" {
		t.Errorf("expected data unchanged, got %v", data[0]["name"])
	}
	if g.CanUndo() {
		t.Error("refused paste must not enter the undo stack")
	}
}

func TestDataGrid_PasteRespectsCanEdit(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "locked": true},
		{"name": "Bob", "locked": false},
	}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable, CanEdit: func(rowData any) bool {
			locked, _ := rowData.(map[string]any)["locked"].(bool)
			return !locked
		}},
	})

	// Locked row refuses the paste.
	pasteInto(t, g, CellID{Row: 0, Col: 0}, "Mallory")
	if data[0]["name"] != "Alice" {
		t.Errorf("expected locked row unchanged, got %v", data[0]["name"])
	}
	if g.CanUndo() {
		t.Error("refused paste must not enter the undo stack")
	}

	// Unlocked row accepts it.
	pasteInto(t, g, CellID{Row: 1, Col: 0}, "Robert")
	if data[1]["name"] != "Robert" {
		t.Errorf("expected unlocked row updated, got %v", data[1]["name"])
	}
	if !g.CanUndo() {
		t.Error("accepted paste should enter the undo stack")
	}
}

func TestDataGrid_PasteIntoEditableCell(t *testing.T) {
	data := []map[string]any{{"name": "Alice"}}
	editable := true
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "name", Editable: &editable},
	})

	var cbOld, cbNew any
	g.OnCellValueChanged = func(id CellID, oldValue, newValue any) {
		cbOld, cbNew = oldValue, newValue
	}

	pasteInto(t, g, CellID{Row: 0, Col: 0}, "Bob")

	if data[0]["name"] != "Bob" {
		t.Errorf("expected data updated to Bob, got %v", data[0]["name"])
	}
	if cbOld != "Alice" || cbNew != "Bob" {
		t.Errorf("expected callback Alice->Bob, got %v->%v", cbOld, cbNew)
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
