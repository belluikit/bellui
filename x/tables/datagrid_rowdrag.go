package tables

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
)

// OnRowDragged is called when the user drags row data to a new position.
// This is only meaningful for in-memory data sources where the underlying
// slice order can be changed.
type OnRowDragged func(fromRow, toRow int)

// MoveRow moves a row from one display position to another.
// This triggers a data callback (if the data source supports reordering)
// and refreshes the display. The OnRowMoved callback is invoked if set.
//
// The move means what OnRowMoved says: the row at fromDisplayRow ends up
// where toDisplayRow is now. The grid's own state kept by source row (the
// selection, the keyboard cursor, expanded detail rows) moves with the rows,
// on the assumption that the app reorders its data that way.
func (g *DataGrid) MoveRow(fromDisplayRow, toDisplayRow int) {
	if fromDisplayRow == toDisplayRow {
		return
	}
	if fromDisplayRow < 0 || fromDisplayRow >= g.RowCount() {
		return
	}
	if toDisplayRow < 0 || toDisplayRow >= g.RowCount() {
		return
	}

	// Get source indices
	idx := g.pipe.Index()
	fromSource := idx.SourceRow(fromDisplayRow)
	toSource := idx.SourceRow(toDisplayRow)
	if fromSource < 0 || toSource < 0 {
		return
	}

	// Fire callback so the app can reorder its backing data
	if g.OnRowMoved != nil {
		g.OnRowMoved(fromSource, toSource)
	}
	g.remapMovedSource(fromSource, toSource)

	g.pipe.MarkDirty()
	g.RefreshData()
}

// OnRowMoved is a callback field — set it on the DataGrid.
// It's stored in datagrid.go's struct definition. Adding a dedicated accessor here.

// SetOnRowMoved sets the callback for when a row is dragged to a new position.
func (g *DataGrid) SetOnRowMoved(fn func(fromSourceRow, toSourceRow int)) {
	g.OnRowMoved = fn
}

// rowsReorderable reports whether the user may move rows now (#1512):
// GridOptions.RowDragEnabled is set, the grid is enabled, and no sort, row
// grouping, pivot or tree data is active. Under those the grid decides the
// order itself, so a dropped row would jump straight back (owner decision,
// 2026-11-28). A filter or pagination leaves it on: a moved row still has a
// clear place among the rows shown.
func (g *DataGrid) rowsReorderable() bool {
	return g.Options.RowDragEnabled && !g.disabled && len(g.sortModel) == 0 &&
		g.groupTransform == nil && g.pivotTransform == nil && g.treeTransform == nil
}

// movedSource is where source row s is after the row at from moved to to's
// place: the rows between them close the gap it left.
func movedSource(s, from, to int) int {
	switch {
	case s == from:
		return to
	case from < to && s > from && s <= to:
		return s - 1
	case to < from && s >= to && s < from:
		return s + 1
	}
	return s
}

// remapMovedSource moves the grid's source-keyed state with a row move, so
// the selection, the cursor and the expanded detail rows stay on their
// records.
func (g *DataGrid) remapMovedSource(from, to int) {
	remap := func(s int) int {
		if s < 0 {
			return s
		}
		return movedSource(s, from, to)
	}
	if len(g.selectedRows) > 0 {
		moved := make(map[int]bool, len(g.selectedRows))
		for s, on := range g.selectedRows {
			moved[remap(s)] = on
		}
		g.selectedRows = moved
	}
	if len(g.selectedCells) > 0 {
		moved := make(map[selCell]bool, len(g.selectedCells))
		for k, on := range g.selectedCells {
			moved[selCell{remap(k.src), k.col}] = on
		}
		g.selectedCells = moved
	}
	if len(g.expandedDetails) > 0 {
		moved := make(expandedDetails, len(g.expandedDetails))
		for s, on := range g.expandedDetails {
			moved[remap(s)] = on
		}
		g.expandedDetails = moved
	}
	if len(g.detailContent) > 0 {
		moved := make(map[int]core.CanvasObject, len(g.detailContent))
		for s, o := range g.detailContent {
			moved[remap(s)] = o
		}
		g.detailContent = moved
	}
	g.selCellRow = remap(g.selCellRow)
	g.focusSrcRow = remap(g.focusSrcRow)
}

// moveFocusedRow moves the row the keyboard cursor is on by delta rows, up
// for a negative delta, and keeps the cursor on it, so pressing the key again
// moves it further. It reports whether the key was taken.
func (g *DataGrid) moveFocusedRow(delta int) bool {
	if !g.rowsReorderable() || g.IsEditing() {
		return false
	}
	from := g.currentFocus.Row
	to := from + delta
	if from < 0 || to < 0 || to >= g.RowCount() {
		return from >= 0 // at the end: taken, and nothing moves
	}
	g.MoveRow(from, to)
	g.currentFocus.Row = to
	g.focusSrcRow = g.sourceRow(to)
	g.scrollToCell(g.currentFocus)
	g.Refresh()
	return true
}

// reorderKey reports the row move a key asks for (#1512): Alt+Up and
// Alt+Down, Option on a Mac, move the focused row one place. delta is 0 for
// any other key.
func reorderKey(name event.KeyName, mod event.KeyModifier) int {
	if mod != event.KeyModifierAlt {
		return 0
	}
	switch name {
	case event.KeyUp:
		return -1
	case event.KeyDown:
		return 1
	}
	return 0
}

// reorderShortcut is reorderKey for a desktop key combination, which the
// desktop driver delivers as a shortcut rather than as a key.
func reorderShortcut(s core.Shortcut) int {
	if cs, ok := s.(*desktop.CustomShortcut); ok {
		return reorderKey(cs.KeyName, cs.Modifier)
	}
	return 0
}

// Accessible names of the row moves a screen reader offers (#1512).
const (
	moveRowUpName   = "move-row-up"
	moveRowDownName = "move-row-down"
)

// accessibleMoveActions returns "Move up" and "Move down" for display row row
// while rows can be moved, leaving out the one that would move it past the
// first or last row.
func (g *DataGrid) accessibleMoveActions(row int) []core.AccessibleAction {
	if !g.rowsReorderable() || row < 0 || row >= g.RowCount() {
		return nil
	}
	var out []core.AccessibleAction
	if row > 0 {
		out = append(out, core.AccessibleAction{Name: moveRowUpName, Description: "Move up"})
	}
	if row < g.RowCount()-1 {
		out = append(out, core.AccessibleAction{Name: moveRowDownName, Description: "Move down"})
	}
	return out
}

// performAccessibleMove runs a move action of display row row, and reports
// whether name was one. The keyboard cursor goes with the row, as with
// Alt+Up and Alt+Down.
func (g *DataGrid) performAccessibleMove(row int, name string) bool {
	delta := 0
	switch name {
	case moveRowUpName:
		delta = -1
	case moveRowDownName:
		delta = 1
	default:
		return false
	}
	if !g.rowsReorderable() || row < 0 || row >= g.RowCount() {
		return true
	}
	g.currentFocus.Row = row
	g.currentFocus.Col = max(g.currentFocus.Col, 0)
	g.moveFocusedRow(delta)
	return true
}
