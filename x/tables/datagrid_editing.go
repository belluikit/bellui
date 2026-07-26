package tables

import (
	"fmt"
	"sort"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
)

// --- Editing ---

// StartEditing begins inline editing of the given cell.
func (g *DataGrid) StartEditing(id CellID) {
	// id.Col is an absolute display-column index; visibleCol maps it to the
	// resolved column (honouring pinned reordering), unlike raw resolvedCols[].
	rc := g.visibleCol(id.Col)
	if id.Row < 0 || rc == nil {
		return
	}

	// Check if column is editable
	editable := true
	if rc.def.Editable != nil {
		editable = *rc.def.Editable
	} else if g.DefaultColDef != nil {
		editable = g.DefaultColDef.Editable
	}
	if !editable {
		return
	}

	// Get current value
	rowData := g.pipe.DisplayRow(id.Row)
	if rowData == nil {
		return
	}
	var value any
	if rc.def.ValueGetter != nil {
		value = rc.def.ValueGetter(rowData)
	} else {
		value = defaultFieldAccess(rowData, rc.def.Field)
	}

	g.editingCell = &id
	g.editOldValue = value

	// Create an Entry widget for editing
	entry := widget.NewEntry()
	entry.SetText(fmt.Sprint(value))
	entry.OnSubmitted = func(text string) {
		g.commitEdit(text)
	}

	g.editWidget = entry
	g.Refresh()
}

// StopEditing cancels the current edit without committing.
func (g *DataGrid) StopEditing() {
	g.editingCell = nil
	g.editWidget = nil
	g.editOldValue = nil
	g.Refresh()
}

// commitEdit applies the edited value.
func (g *DataGrid) commitEdit(text string) {
	if g.editingCell == nil {
		return
	}
	id := *g.editingCell
	rc := g.visibleCol(id.Col)
	if rc == nil {
		return
	}
	oldValue := g.editOldValue

	// Convert text to appropriate type
	var newValue any = text

	// Write back to data source
	rowData := g.pipe.DisplayRow(id.Row)
	if rowData != nil {
		if m, ok := rowData.(map[string]any); ok {
			m[rc.def.Field] = newValue
		}
	}

	// Record in undo stack
	g.pushUndo(undoEntry{
		Cell:     id,
		OldValue: oldValue,
		NewValue: newValue,
	})

	// Fire callback
	if g.OnCellValueChanged != nil {
		g.OnCellValueChanged(id, oldValue, newValue)
	}

	g.editingCell = nil
	g.editWidget = nil
	g.editOldValue = nil
	g.RefreshData()
}

// IsEditing returns true if a cell is currently being edited.
func (g *DataGrid) IsEditing() bool {
	return g.editingCell != nil
}

// EditingCell returns the cell currently being edited, or nil.
func (g *DataGrid) EditingCell() *CellID {
	return g.editingCell
}

// --- Undo/Redo ---

func (g *DataGrid) pushUndo(entry undoEntry) {
	// Truncate any redo entries beyond current position
	if g.undoPos < len(g.undoStack) {
		g.undoStack = g.undoStack[:g.undoPos]
	}
	g.undoStack = append(g.undoStack, entry)
	g.undoPos = len(g.undoStack)
}

// Undo reverts the last cell edit.
func (g *DataGrid) Undo() {
	if g.undoPos <= 0 {
		return
	}
	g.undoPos--
	entry := g.undoStack[g.undoPos]

	// Restore old value
	rowData := g.pipe.DisplayRow(entry.Cell.Row)
	if rowData != nil {
		if m, ok := rowData.(map[string]any); ok {
			if rc := g.visibleCol(entry.Cell.Col); rc != nil {
				m[rc.def.Field] = entry.OldValue
			}
		}
	}

	if g.OnCellValueChanged != nil {
		g.OnCellValueChanged(entry.Cell, entry.NewValue, entry.OldValue)
	}

	g.RefreshData()
}

// Redo re-applies a previously undone edit.
func (g *DataGrid) Redo() {
	if g.undoPos >= len(g.undoStack) {
		return
	}
	entry := g.undoStack[g.undoPos]
	g.undoPos++

	// Apply new value again
	rowData := g.pipe.DisplayRow(entry.Cell.Row)
	if rowData != nil {
		if m, ok := rowData.(map[string]any); ok {
			if rc := g.visibleCol(entry.Cell.Col); rc != nil {
				m[rc.def.Field] = entry.NewValue
			}
		}
	}

	if g.OnCellValueChanged != nil {
		g.OnCellValueChanged(entry.Cell, entry.OldValue, entry.NewValue)
	}

	g.RefreshData()
}

// CanUndo returns true if there are edits to undo.
func (g *DataGrid) CanUndo() bool { return g.undoPos > 0 }

// CanRedo returns true if there are edits to redo.
func (g *DataGrid) CanRedo() bool { return g.undoPos < len(g.undoStack) }

// --- Clipboard ---

// copySelection copies the selected cells' values to the system clipboard.
func (g *DataGrid) copySelection() {
	if len(g.selectedRows) == 0 && g.selCellRow < 0 {
		return
	}

	// Collect selected source rows, then map them to display positions so the
	// clipboard emits rows in the order they are currently shown. Selection is
	// keyed by source identity (survives re-sort); the reverse lookup goes
	// through the index's lazily-built source->display map.
	var srcRows []int
	if len(g.selectedRows) > 0 {
		for s := range g.selectedRows {
			srcRows = append(srcRows, s)
		}
	} else if g.selCellRow >= 0 {
		srcRows = []int{g.selCellRow}
	}

	idx := g.pipe.Index()
	var rows []int
	for _, s := range srcRows {
		if d := idx.DisplayRow(s); d >= 0 {
			rows = append(rows, d)
		}
	}
	sort.Ints(rows)

	var sb strings.Builder
	for ri, row := range rows {
		if ri > 0 {
			sb.WriteByte('\n')
		}
		rowData := g.pipe.DisplayRow(row)
		if rowData == nil {
			continue
		}
		for ci, rc := range g.resolvedCols {
			if ci > 0 {
				sb.WriteByte('\t')
			}
			var value any
			if rc.def.ValueGetter != nil {
				value = rc.def.ValueGetter(rowData)
			} else {
				value = defaultFieldAccess(rowData, rc.def.Field)
			}
			if rc.def.ValueFormatter != nil {
				sb.WriteString(rc.def.ValueFormatter(value))
			} else {
				sb.WriteString(fmt.Sprint(value))
			}
		}
	}

	bellui.CurrentApp().Clipboard().SetContent(sb.String())
}

// pasteToSelection pastes clipboard content into the selected cell.
func (g *DataGrid) pasteToSelection() {
	if g.selectedCell == nil {
		return
	}

	content := bellui.CurrentApp().Clipboard().Content()
	if content == "" {
		return
	}

	id := *g.selectedCell
	// id.Col is an absolute display index; resolve it through visibleCol.
	rc := g.visibleCol(id.Col)
	if rc == nil {
		return
	}

	// Write value to data
	rowData := g.pipe.DisplayRow(id.Row)
	if rowData == nil {
		return
	}

	var oldValue any
	if rc.def.ValueGetter != nil {
		oldValue = rc.def.ValueGetter(rowData)
	} else {
		oldValue = defaultFieldAccess(rowData, rc.def.Field)
	}

	// Write the pasted text
	if m, ok := rowData.(map[string]any); ok {
		m[rc.def.Field] = content
	}

	g.pushUndo(undoEntry{
		Cell:     id,
		OldValue: oldValue,
		NewValue: content,
	})

	if g.OnCellValueChanged != nil {
		g.OnCellValueChanged(id, oldValue, content)
	}

	g.RefreshData()
}

// --- Cell editor types ---

// Ensure CellRendererFactory conformance for editor-related renderers.
var _ cell.RendererFactory = (*cell.TextRenderer)(nil)
