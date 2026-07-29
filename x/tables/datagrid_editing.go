package tables

import (
	"fmt"
	"sort"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
)

// --- Editing ---

// StartEditing begins inline editing of the given cell, committing any edit
// already in progress. The editor comes from the column's Editor factory
// (ColumnDef.Editor); columns without one edit through a plain text Entry.
// The edit is refused — no editor appears — when the column is not editable
// or the column's per-cell CanEdit gate rejects the row.
func (g *DataGrid) StartEditing(id CellID) {
	// id.Col is an absolute display-column index; visibleCol maps it to the
	// resolved column (honouring pinned reordering), unlike raw resolvedCols[].
	rc := g.visibleCol(id.Col)
	if id.Row < 0 || rc == nil {
		return
	}

	// Check if column is editable
	if !g.columnEditable(rc) {
		return
	}

	// Starting a new edit commits the one in progress (matches spreadsheet
	// behaviour: clicking on to the next cell keeps what was typed).
	g.CommitEditing()

	// Get current value
	rowData := g.pipe.DisplayRow(id.Row)
	if rowData == nil {
		return
	}

	// Per-cell gate: an editable column can still refuse individual cells
	// (read-only summary rows, expression-valued cells). Checked against the
	// row that would actually be edited — after the pending-edit commit above,
	// whose write may have re-sorted/re-filtered the display mapping.
	if !g.rowCanEdit(rc, rowData) {
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

	factory := rc.def.Editor
	if factory == nil {
		factory = cell.NewTextEditor()
	}
	editor := factory.CreateEditor()
	factory.BeginEdit(editor, value, cell.Params{
		RowIndex:  id.Row,
		ColIndex:  id.Col,
		Value:     value,
		RowData:   rowData,
		CellWidth: rc.width,
	})

	// Wire commit gestures the factory left open (after BeginEdit, so seeding
	// the editor's initial value cannot fire an immediate commit): Enter
	// submits an entry; picking an option or toggling a check is the whole
	// edit for discrete editors, so it commits immediately.
	switch w := editor.(type) {
	case *widget.Entry:
		if w.OnSubmitted == nil {
			w.OnSubmitted = func(string) { g.CommitEditing() }
		}
		// Escape cancels from inside the focused editor — the grid's own
		// Escape handling only runs while the grid has focus.
		if w.OnEscape == nil {
			w.OnEscape = g.StopEditing
		}
	case *widget.Select:
		if w.OnChanged == nil {
			w.OnChanged = func(string) { g.CommitEditing() }
		}
	case *widget.Check:
		if w.OnChanged == nil {
			w.OnChanged = func(bool) { g.CommitEditing() }
		}
	}

	g.editFactory = factory
	g.editWidget = editor
	g.Refresh()
	g.focusEditor()
}

// columnEditable returns the column-level editable setting, inheriting from
// DefaultColDef when the column leaves it nil.
func (g *DataGrid) columnEditable(rc *resolvedColumn) bool {
	if rc.def.Editable != nil {
		return *rc.def.Editable
	}
	if g.DefaultColDef != nil {
		return g.DefaultColDef.Editable
	}
	return true
}

// rowCanEdit applies the column's per-cell CanEdit gate to a row. A nil gate
// accepts every row — the column-level setting alone decides.
func (g *DataGrid) rowCanEdit(rc *resolvedColumn, rowData any) bool {
	return rc.def.CanEdit == nil || rc.def.CanEdit(rowData)
}

// cellEditable reports whether a cell accepts writes: the column must be
// editable and the per-cell CanEdit gate must accept the row. Every mutation
// path (inline editing, paste) goes through these same gates so they cannot
// drift.
func (g *DataGrid) cellEditable(rc *resolvedColumn, rowData any) bool {
	return g.columnEditable(rc) && g.rowCanEdit(rc, rowData)
}

// StopEditing cancels the current edit without committing.
func (g *DataGrid) StopEditing() {
	g.refocusGrid()
	g.editingCell = nil
	g.editWidget = nil
	g.editFactory = nil
	g.editOldValue = nil
	g.Refresh()
}

// CommitEditing ends the active edit, extracting the value from the editor
// (EditorFactory.EndEdit) and writing it back to the row. An editor that
// reports the edit as cancelled is dismissed without a write. No-op when no
// edit is in progress.
func (g *DataGrid) CommitEditing() {
	if g.editingCell == nil || g.editFactory == nil || g.editWidget == nil {
		return
	}
	value, cancelled := g.editFactory.EndEdit(g.editWidget)
	if cancelled {
		g.StopEditing()
		return
	}
	g.commitEditValue(value)
}

// commitEdit applies raw edited text (the plain-entry path).
func (g *DataGrid) commitEdit(text string) {
	g.commitEditValue(text)
}

// commitEditValue applies the edited value produced by the active editor.
func (g *DataGrid) commitEditValue(newValue any) {
	if g.editingCell == nil {
		return
	}
	id := *g.editingCell
	rc := g.visibleCol(id.Col)
	if rc == nil {
		return
	}
	oldValue := g.editOldValue

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

	g.refocusGrid()
	g.editingCell = nil
	g.editWidget = nil
	g.editFactory = nil
	g.editOldValue = nil
	g.RefreshData()
}

// focusEditor moves keyboard focus into the active edit widget so typing goes
// to the editor rather than the grid's cursor navigation.
func (g *DataGrid) focusEditor() {
	f, ok := g.editWidget.(event.Focusable)
	if !ok {
		return
	}
	if c := bellui.CurrentApp().Driver().CanvasForObject(g); c != nil {
		c.Focus(f)
	}
}

// refocusGrid returns keyboard focus to the grid when the closing editor
// holds it, so cursor navigation resumes after the edit ends. Must run before
// the editing state is cleared.
func (g *DataGrid) refocusGrid() {
	f, ok := g.editWidget.(event.Focusable)
	if !ok {
		return
	}
	if c := bellui.CurrentApp().Driver().CanvasForObject(g); c != nil && c.Focused() == f {
		c.Focus(g)
	}
}

// layoutEditOverlay positions the active inline editor over its cell,
// mirroring the renderer's band layout. Called from the renderer's layout
// pass and from scroll updates so the editor tracks its cell; hidden when the
// cell is outside the body area.
func (g *DataGrid) layoutEditOverlay(size geo.Size) {
	if g.editWidget == nil || g.editingCell == nil {
		return
	}
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	pos, cellSize, visible := g.editingCellRect(*g.editingCell, size)
	if !visible {
		g.editWidget.Hide()
		return
	}
	g.editWidget.Show()
	g.editWidget.Move(pos)
	g.editWidget.Resize(cellSize)
}

// editingCellRect returns the grid-local rectangle of the given display cell,
// inverting the same band geometry the renderer draws with (bandAnchors):
// pinned-left fixed at the origin, main scrolled by offset.X, pinned-right a
// fixed overlay. visible is false when the cell lies outside the body area —
// scrolled off vertically, or a main cell hidden under a pinned band.
func (g *DataGrid) editingCellRect(id CellID, size geo.Size) (pos geo.Position, cellSize geo.Size, visible bool) {
	layout := g.visibleLayout()
	if id.Row < 0 || id.Row >= g.RowCount() || id.Col < 0 || id.Col >= len(layout.all) {
		return geo.Position{}, geo.Size{}, false
	}
	pad := g.cellPadding()
	mainX, pinnedRightX, pinnedRightW := g.bandAnchors(size.Width)

	var x, w float32
	nL, nM := len(layout.pinnedLeft), len(layout.main)
	switch {
	case id.Col < nL: // fixed pinned-left band at the origin
		x = bandColX(layout.pinnedLeft, id.Col, pad)
		w = layout.pinnedLeft[id.Col].width
	case id.Col < nL+nM: // main band, scrolls by offset.X
		cx, cw := g.viewport.ColX(id.Col - nL)
		x = mainX + cx - g.offset.X
		w = cw
		rightEdge := size.Width
		if pinnedRightW > 0 {
			rightEdge = pinnedRightX
		}
		if x+w <= mainX || x >= rightEdge {
			return geo.Position{}, geo.Size{}, false
		}
	default: // pinned-right fixed overlay
		i := id.Col - nL - nM
		x = pinnedRightX + bandColX(layout.pinnedRight, i, pad)
		w = layout.pinnedRight[i].width
	}

	// Vertical: the body starts below the header (and floating-filter row) and
	// scrolls by offset.Y; the row's cell band excludes any expanded detail
	// band. The 30/24 constants match dataGridRenderer.doLayout.
	top := g.HeaderHeight()
	if g.floatingFilterRow != nil {
		top += 30
	}
	bottom := size.Height
	if g.Options.ShowStatusBar {
		bottom -= 24
	}
	ry, rh := g.viewport.RowY(id.Row)
	if band := g.DetailBandHeight(id.Row); band > 0 {
		rh -= band
	}
	y := top + ry - g.offset.Y
	if y+rh <= top || y >= bottom {
		return geo.Position{}, geo.Size{}, false
	}
	return geo.NewPos(x, y), geo.NewSize(w, rh), true
}

// bandColX returns the band-local x of the i-th column within a fixed band.
func bandColX(cols []*resolvedColumn, i int, pad float32) float32 {
	x := float32(0)
	for _, c := range cols[:i] {
		x += c.width + pad
	}
	return x
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

// pasteToSelection pastes clipboard content into the selected cell, subject
// to the same editability gates as inline editing (Editable, CanEdit).
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

	// Paste is a cell mutation like an inline edit, so it passes the same
	// editability gates: a read-only cell refuses the paste — no write, no
	// undo entry, no callback.
	if !g.cellEditable(rc, rowData) {
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
