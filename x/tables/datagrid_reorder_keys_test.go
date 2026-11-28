package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"

	"github.com/belluikit/bellui/x/tables/pipeline"
)

// reorderGrid is a grid of five rows "a" to "e" whose OnRowMoved reorders
// the data the documented way: the row at from goes where to is.
func reorderGrid(t *testing.T) (*DataGrid, *[]map[string]any) {
	t.Helper()
	data := []map[string]any{{"v": "a"}, {"v": "b"}, {"v": "c"}, {"v": "d"}, {"v": "e"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.Options.RowDragEnabled = true
	g.OnRowMoved = func(from, to int) {
		item := data[from]
		rest := append(append([]map[string]any{}, data[:from]...), data[from+1:]...)
		moved := append(append(append([]map[string]any{}, rest[:to]...), item), rest[to:]...)
		copy(data, moved)
	}
	layOut(t, g)
	return g, &data
}

// order is the grid's rows as it shows them, by their v.
func order(g *DataGrid) string {
	s := ""
	for r := 0; r < g.RowCount(); r++ {
		s += g.pipe.DisplayRow(r).(map[string]any)["v"].(string)
	}
	return s
}

var altDown = &desktop.CustomShortcut{KeyName: event.KeyDown, Modifier: event.KeyModifierAlt}

// Alt+Down, which the desktop delivers as a shortcut, moves the focused row
// down one place, and the cursor stays on it, so a second press moves it on
// (#1512). Alt+Up moves it back.
func TestDataGrid_ReorderKeysMoveTheFocusedRow(t *testing.T) {
	g, _ := reorderGrid(t)
	g.currentFocus = CellID{Row: 1, Col: 0}

	g.TypedShortcut(altDown)
	assert.Equal(t, "acbde", order(g))
	assert.Equal(t, 2, g.currentFocus.Row, "the cursor went with the row")
	g.TypedShortcut(altDown)
	assert.Equal(t, "acdbe", order(g))

	g.TypedShortcut(&desktop.CustomShortcut{KeyName: event.KeyUp, Modifier: event.KeyModifierAlt})
	assert.Equal(t, "acbde", order(g))
	assert.Equal(t, 2, g.currentFocus.Row)

	// At the last row there is nowhere to go.
	g.currentFocus.Row = 4
	g.TypedShortcut(altDown)
	assert.Equal(t, "acbde", order(g))
	assert.Equal(t, 4, g.currentFocus.Row)
}

// A hardware keyboard on a touch device reports Alt+Down as a key with its
// modifier; it moves the row too. A plain Down still only moves the cursor.
func TestDataGrid_ReorderKeyWithModifier(t *testing.T) {
	g, _ := reorderGrid(t)
	g.currentFocus = CellID{Row: 0, Col: 0}

	g.TypedKey(&event.KeyEvent{Name: event.KeyDown, Modifier: event.KeyModifierAlt})
	assert.Equal(t, "bacde", order(g))
	assert.Equal(t, 1, g.currentFocus.Row)

	g.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, "bacde", order(g), "a plain Down moved no row")
	assert.Equal(t, 2, g.currentFocus.Row)

	g.TypedKey(&event.KeyEvent{Name: event.KeyDown, Modifier: event.KeyModifierAlt | event.KeyModifierShift})
	assert.Equal(t, "bacde", order(g), "Alt with another modifier is not the reorder key")
}

// The selection stays on the record it was on when rows move under it.
func TestDataGrid_ReorderKeepsTheSelectionOnItsRecord(t *testing.T) {
	g, _ := reorderGrid(t)
	g.SelectionMode = SelectionMultiRow
	g.selectedRows = map[int]bool{1: true, 3: true} // "b" and "d"
	g.currentFocus = CellID{Row: 1, Col: 0}

	g.TypedShortcut(altDown) // "b" past "c"
	g.TypedShortcut(altDown) // and past "d"
	require.Equal(t, "acdbe", order(g))
	assert.Equal(t, map[int]bool{3: true, 2: true}, g.selectedRows, "b and d stay selected where they now are")
}

// Under a sort, a row grouping or tree data, or with RowDragEnabled off, the
// grid decides the order, so the keys move nothing and the cursor stays.
func TestDataGrid_ReorderOffWhereTheGridOrdersRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*DataGrid)
	}{
		{"drag off", func(g *DataGrid) { g.Options.RowDragEnabled = false }},
		{"sorted", func(g *DataGrid) { g.API().SetSort(SortItem{ColID: "v", Direction: SortDescending}) }},
		{"grouped", func(g *DataGrid) { g.API().SetRowGroupColumn("v") }},
		{"tree", func(g *DataGrid) {
			g.API().SetTreeData(func(r any) any { return r.(map[string]any)["v"] }, func(any) int { return -1 })
		}},
		{"pivot", func(g *DataGrid) { g.API().SetPivot("v", "v", "v", pipeline.AggCount) }},
		{"disabled", func(g *DataGrid) { g.Disable() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, data := reorderGrid(t)
			tc.set(g)
			g.currentFocus = CellID{Row: 1, Col: 0}
			before := append([]map[string]any{}, (*data)...)
			g.TypedShortcut(altDown)
			assert.Equal(t, before, *data, "no row moved")
			assert.False(t, g.rowsReorderable())
			assert.Empty(t, g.accessibleMoveActions(1))
		})
	}
}

// A filter leaves reordering on: the visible rows' source indices are not
// next to each other, and the move still lands where the documented meaning
// says, with the cursor on the moved row.
func TestDataGrid_ReorderUnderAFilter(t *testing.T) {
	g, _ := reorderGrid(t)
	g.API().SetColumnFilter("v", func(v any) bool { return v != "b" && v != "d" }) // shows a, c, e
	require.Equal(t, "ace", order(g))
	require.True(t, g.rowsReorderable())
	g.currentFocus = CellID{Row: 0, Col: 0}

	g.TypedShortcut(altDown) // "a" to where "c" is
	assert.Equal(t, "cae", order(g))
	assert.Equal(t, 1, g.currentFocus.Row)
}

// A screen reader gets "Move up" and "Move down" on each row, without the one
// past either end, and running one moves the row (#1512).
func TestDataGrid_ReorderAccessibleActions(t *testing.T) {
	g, _ := reorderGrid(t)

	names := func(row int) []string {
		var out []string
		for _, a := range g.AccessibleRowActions(row) {
			out = append(out, a.Name)
		}
		return out
	}
	assert.Equal(t, []string{moveRowDownName}, names(0))
	assert.Equal(t, []string{moveRowUpName, moveRowDownName}, names(2))
	assert.Equal(t, []string{moveRowUpName}, names(4))

	g.PerformAccessibleRowAction(3, moveRowUpName)
	assert.Equal(t, "abdce", order(g))
	assert.Equal(t, 2, g.currentFocus.Row, "the cursor is on the moved row")
	g.PerformAccessibleRowAction(0, moveRowUpName)
	assert.Equal(t, "abdce", order(g), "the first row does not move up")
}

// A mouse press arms no row drag where the grid orders the rows itself.
func TestDataGrid_ReorderMousePressUnderASort(t *testing.T) {
	g, _ := reorderGrid(t)
	g.API().SetSort(SortItem{ColID: "v", Direction: SortAscending})
	g.MouseDown(pressAt(rowAt(g, 1), 0))
	assert.Equal(t, -1, g.dragRow)
}

func TestMovedSource(t *testing.T) {
	// Row 1 moves to 3: rows 2 and 3 close the gap.
	for s, want := range map[int]int{0: 0, 1: 3, 2: 1, 3: 2, 4: 4} {
		assert.Equal(t, want, movedSource(s, 1, 3), "s=%d", s)
	}
	// Row 3 moves to 1: rows 1 and 2 make room.
	for s, want := range map[int]int{0: 0, 1: 2, 2: 3, 3: 1, 4: 4} {
		assert.Equal(t, want, movedSource(s, 3, 1), "s=%d", s)
	}
}
