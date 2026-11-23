package tables

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/tables/cell"
)

// selectionSpy is a CellRenderer that records, per column, which display rows
// the grid last drew as selected.
type selectionSpy struct {
	field    string
	selected map[string]map[int]bool
	hovered  map[string]map[int]bool
}

func (s *selectionSpy) CreateCell() core.CanvasObject { return canvas.NewText("", nil) }
func (s *selectionSpy) UpdateCell(_ core.CanvasObject, p cell.Params) {
	if s.selected[s.field] == nil {
		s.selected[s.field] = map[int]bool{}
	}
	s.selected[s.field][p.RowIndex] = p.IsSelected
	if s.hovered != nil {
		if s.hovered[s.field] == nil {
			s.hovered[s.field] = map[int]bool{}
		}
		s.hovered[s.field][p.RowIndex] = p.IsHovered
	}
}

// selectedDrawn lists the "field:row" cells last drawn selected.
func selectedDrawn(sel map[string]map[int]bool) []string {
	var out []string
	for f, rows := range sel {
		for r, on := range rows {
			if on {
				out = append(out, f+":"+string(rune('0'+r)))
			}
		}
	}
	sort.Strings(out)
	return out
}

func spyGrid(t *testing.T, mode SelectionMode) (*DataGrid, map[string]map[int]bool) {
	t.Helper()
	sel := map[string]map[int]bool{}
	col := func(id string, pin PinSide) *ColumnDef {
		return &ColumnDef{ColID: id, Field: "n", HeaderName: id, Pinned: pin, CellRenderer: &selectionSpy{field: id, selected: sel}}
	}
	g := newTestGrid(t, manyRows(4), []*ColumnDef{col("p", PinLeft), col("a", PinNone), col("b", PinNone)})
	g.SelectionMode = mode
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(500, 300))
	return g, sel
}

// A selected cell is drawn selected in its own column, also with a pinned
// column on its left: the main cells were asked with the main-column index
// while a CellID carries the absolute one, so the highlight landed one
// column right, and a pinned cell was never drawn selected (#1401).
func TestDataGrid_SelectedCellDrawnInItsColumn(t *testing.T) {
	g, sel := spyGrid(t, SelectionSingleCell)
	g.API().SelectCell(CellID{Row: 1, Col: 1}) // "a", the first main column
	g.Refresh()
	assert.Equal(t, []string{"a:1"}, selectedDrawn(sel))

	g.API().SelectCell(CellID{Row: 2, Col: 0}) // "p", pinned left
	g.Refresh()
	assert.Equal(t, []string{"p:2"}, selectedDrawn(sel))
}

// The hovered cell is drawn hovered in its own column too: the same main
// against absolute index mix-up (#1401).
func TestDataGrid_HoveredCellDrawnInItsColumn(t *testing.T) {
	sel, hov := map[string]map[int]bool{}, map[string]map[int]bool{}
	col := func(id string, pin PinSide) *ColumnDef {
		return &ColumnDef{ColID: id, Field: "n", HeaderName: id, Pinned: pin, CellRenderer: &selectionSpy{field: id, selected: sel, hovered: hov}}
	}
	g := newTestGrid(t, manyRows(4), []*ColumnDef{col("p", PinLeft), col("a", PinNone), col("b", PinNone)})
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(500, 300))
	g.hoveredCell = &CellID{Row: 1, Col: 1} // "a"
	g.Refresh()
	assert.Equal(t, []string{"a:1"}, selectedDrawn(hov))
	g.hoveredCell = &CellID{Row: 2, Col: 0} // "p", pinned left
	g.Refresh()
	assert.Equal(t, []string{"p:2"}, selectedDrawn(hov))
}

// SelectionMultiCell, as a mouse click applies it (#1401): a plain click
// selects one cell, Ctrl/Cmd-click toggles one and keeps the others,
// Shift-click selects the rectangle from the last clicked cell. Only the
// selected cells are drawn selected, not their rows.
func TestDataGrid_MultiCellSelection(t *testing.T) {
	g, sel := spyGrid(t, SelectionMultiCell)
	var reported []int
	fired := 0
	g.OnSelectionChanged = func(rows []int) { reported = rows; fired++ }
	click := func(row, col int, mod event.KeyModifier) {
		g.selectCellWithModifiers(CellID{Row: row, Col: col}, mod)
		g.Refresh()
	}
	ctrl, cmd, shift := event.KeyModifierControl, event.KeyModifierSuper, event.KeyModifierShift

	click(1, 1, 0) // "a" row 1
	assert.Equal(t, []string{"a:1"}, selectedDrawn(sel), "one cell, not its row")
	assert.Equal(t, []CellID{{Row: 1, Col: 1}}, g.API().SelectedCells())
	assert.Equal(t, []int{1}, reported)

	click(3, 2, ctrl) // add "b" row 3
	click(0, 0, cmd)  // add pinned "p" row 0, with Cmd
	assert.Equal(t, []string{"a:1", "b:3", "p:0"}, selectedDrawn(sel))
	assert.Equal(t, []CellID{{Row: 0, Col: 0}, {Row: 1, Col: 1}, {Row: 3, Col: 2}}, g.API().SelectedCells())
	assert.Equal(t, []int{0, 1, 3}, reported)
	assert.Empty(t, g.API().GetSelectedRows(), "cells are selected, not rows")

	click(3, 2, ctrl) // take "b" row 3 out again
	assert.Equal(t, []string{"a:1", "p:0"}, selectedDrawn(sel))
	assert.False(t, g.IsSelected(3, 2), "the cursor cell a Ctrl-click took out is not selected")
	assert.True(t, g.IsSelected(1, 1), "a screen reader hears each selected cell")

	click(1, 1, 0)     // anchor at "a" row 1
	click(2, 2, shift) // rectangle to "b" row 2
	assert.Equal(t, []string{"a:1", "a:2", "b:1", "b:2"}, selectedDrawn(sel))
	assert.Equal(t, 6, fired, "each click reports")

	click(0, 0, 0) // a plain click starts over
	assert.Equal(t, []string{"p:0"}, selectedDrawn(sel))

	g.API().DeselectAll()
	assert.Empty(t, selectedDrawn(sel))
	assert.Empty(t, g.API().SelectedCells())
}

// The selection is kept by source row, so a sort moves it with its data, and
// copy puts the selected cells on the clipboard, a line per row in the order
// shown, tab-separated by column.
func TestDataGrid_MultiCellSurvivesSortAndCopies(t *testing.T) {
	g, sel := spyGrid(t, SelectionMultiCell)
	g.selectCellWithModifiers(CellID{Row: 0, Col: 1}, 0)                        // "a" of row0
	g.selectCellWithModifiers(CellID{Row: 2, Col: 2}, event.KeyModifierControl) // "b" of row2
	g.selectCellWithModifiers(CellID{Row: 2, Col: 0}, event.KeyModifierControl) // "p" of row2

	g.copySelection()
	assert.Equal(t, "row0\nrow2\trow2", bellui.CurrentApp().Clipboard().Content())

	g.API().SetSort(SortItem{ColID: "a", Direction: SortDescending}) // row3, row2, row1, row0
	g.Refresh()
	assert.Equal(t, []string{"a:3", "b:1", "p:1"}, selectedDrawn(sel), "the cells move with their rows")
	assert.Equal(t, []CellID{{Row: 0, Col: 1}, {Row: 2, Col: 0}, {Row: 2, Col: 2}}, g.API().SelectedCells(), "source rows")
	g.copySelection()
	assert.Equal(t, "row2\trow2\nrow0", bellui.CurrentApp().Clipboard().Content(), "rows in the order shown")
}
