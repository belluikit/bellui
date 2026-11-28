package tables

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/tables/render"
)

// What the grid paints is what is selected (#1524): a cell selected on its
// own is highlighted by itself, in whichever band it sits; a selected row is
// highlighted whole; and a row a click deselected is not highlighted at all.

// paintGrid is a grid with two pinned-left columns, three main ones and a
// pinned-right one, all of which fit the window, so every cell is in view.
// Columns: 0 id and 1 k pinned left, 2 a, 3 b, 4 c, 5 z pinned right.
func paintGrid(t *testing.T, mode SelectionMode, dir bellui.TextDirection) *DataGrid {
	t.Helper()
	data := make([]map[string]any, 5)
	for i := range data {
		data[i] = map[string]any{"id": i, "k": i, "a": fmt.Sprint("a", i), "b": fmt.Sprint("b", i), "c": fmt.Sprint("c", i), "z": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 40, Pinned: PinLeft},
		{Field: "k", Width: 40, Pinned: PinLeft},
		{Field: "a", Width: 70},
		{Field: "b", Width: 70},
		{Field: "c", Width: 70},
		{Field: "z", Width: 50, Pinned: PinRight},
	})
	g.SelectionMode = mode
	w := test.NewWindow(container.NewDirectionScope(g, dir))
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(500, 300))
	return g
}

// painted reports whether a selection-coloured rectangle is drawn under the
// centre of a display cell.
func painted(t *testing.T, g *DataGrid, id CellID) bool {
	t.Helper()
	pos, size, visible := g.editingCellRect(id, g.Size())
	require.True(t, visible, "cell %v is in view", id)
	at := pos.AddXY(size.Width/2, size.Height/2)
	sel := render.ResolveThemeColors().Selection
	found := false
	var walk func(o core.CanvasObject, origin geo.Position)
	walk = func(o core.CanvasObject, origin geo.Position) {
		if found || !o.Visible() {
			return
		}
		p := origin.Add(o.Position())
		if r, ok := o.(*canvas.Rectangle); ok && sameColor(r.FillColor, sel) {
			s := r.Size()
			if at.X >= p.X && at.X < p.X+s.Width && at.Y >= p.Y && at.Y < p.Y+s.Height {
				found = true
				return
			}
		}
		var kids []core.CanvasObject
		switch v := o.(type) {
		case *bellui.Container:
			kids = v.Objects
		case core.Widget:
			kids = test.WidgetRenderer(v).Objects()
		}
		for _, k := range kids {
			walk(k, p)
		}
	}
	for _, o := range test.WidgetRenderer(g).Objects() {
		walk(o, geo.Position{})
	}
	return found
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return false
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// paintedCells lists the cells of rows 0..4 and columns 0..5 drawn selected.
func paintedCells(t *testing.T, g *DataGrid) []CellID {
	t.Helper()
	var out []CellID
	for row := range 5 {
		for col := range 6 {
			if painted(t, g, CellID{Row: row, Col: col}) {
				out = append(out, CellID{Row: row, Col: col})
			}
		}
	}
	return out
}

func press(g *DataGrid, id CellID, mod event.KeyModifier) {
	pos, size, _ := g.editingCellRect(id, g.Size())
	at := pos.AddXY(size.Width/2, size.Height/2)
	ev := &desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary, Modifier: mod}
	g.MouseDown(ev)
	g.MouseUp(ev)
}

// One selected cell is drawn, in every band, in both directions.
func TestDataGrid_SelectedCellPainted(t *testing.T) {
	for _, dir := range []bellui.TextDirection{bellui.DirectionLTR, bellui.DirectionRTL} {
		g := paintGrid(t, SelectionSingleCell, dir)
		require.Equal(t, dir == bellui.DirectionRTL, g.rtl)
		for _, id := range []CellID{{Row: 2, Col: 3}, {Row: 1, Col: 0}, {Row: 1, Col: 1}, {Row: 3, Col: 5}} {
			press(g, id, 0)
			assert.Equal(t, []CellID{id}, paintedCells(t, g), "dir %v: only the selected cell %v is drawn selected", dir, id)
		}
	}
}

// Each of several selected cells is drawn, and nothing else.
func TestDataGrid_SelectedCellsPaintedMultiCell(t *testing.T) {
	g := paintGrid(t, SelectionMultiCell, bellui.DirectionLTR)
	press(g, CellID{Row: 1, Col: 2}, 0)
	press(g, CellID{Row: 3, Col: 4}, event.KeyModifierControl)
	press(g, CellID{Row: 4, Col: 0}, event.KeyModifierControl)
	assert.Equal(t, []CellID{{Row: 1, Col: 2}, {Row: 3, Col: 4}, {Row: 4, Col: 0}}, paintedCells(t, g))
}

// A selected row is drawn whole, across all three bands.
func TestDataGrid_SelectedRowPaintedWhole(t *testing.T) {
	for _, mode := range []SelectionMode{SelectionSingleRow, SelectionMultiRow} {
		g := paintGrid(t, mode, bellui.DirectionLTR)
		press(g, CellID{Row: 2, Col: 3}, 0)
		want := []CellID{{Row: 2, Col: 0}, {Row: 2, Col: 1}, {Row: 2, Col: 2}, {Row: 2, Col: 3}, {Row: 2, Col: 4}, {Row: 2, Col: 5}}
		assert.Equal(t, want, paintedCells(t, g), "mode %v", mode)

		// The row's background is its highlight: no cell of it gets one of
		// its own on top (a rectangle per cell for every selected row).
		sel := render.ResolveThemeColors().Selection
		for _, o := range test.WidgetRenderer(g.cellsWidget).Objects() {
			r, ok := o.(*canvas.Rectangle)
			if !ok || !r.Visible() || !sameColor(r.FillColor, sel) {
				continue
			}
			for col := range 3 { // the main columns
				x, w := g.viewport.ColX(col)
				assert.False(t, r.Position().X == x && r.Size().Width == w,
					"mode %v: main column %d of the selected row has a highlight of its own", mode, col)
			}
		}
	}
}

// A row a click deselects is no longer drawn selected, nor any of its cells,
// and its checkbox clears. The cursor cell the click left behind is no
// selection in the row modes.
func TestDataGrid_DeselectedRowNotPainted(t *testing.T) {
	for _, mode := range []SelectionMode{SelectionSingleRow, SelectionMultiRow} {
		g := paintGrid(t, mode, bellui.DirectionLTR)
		g.ToggleRowSelected(2)
		require.Equal(t, []int{2}, g.API().GetSelectedRows())
		g.ToggleRowSelected(2)
		require.Empty(t, g.API().GetSelectedRows(), "mode %v: the row is deselected", mode)
		assert.Empty(t, paintedCells(t, g), "mode %v: nothing is drawn selected", mode)
		for col := range 6 {
			assert.False(t, g.IsSelected(2, col), "mode %v: cell (2,%d) is not selected", mode, col)
		}
	}

	g := selectionColumnGrid(t, 400, 4, "alpha", "beta")
	g.SelectionMode = SelectionSingleRow
	g.Refresh()
	clickBox(t, g, 1)
	clickBox(t, g, 1)
	box, _ := rowBox(t, g, 1)
	assert.False(t, box.Checked, "the deselected row's checkbox is clear")
}

// A selected first cell's highlight is drawn under the row's disclosure
// chevron, not over it.
func TestDataGrid_CellHighlightUnderChevron(t *testing.T) {
	data := []map[string]any{{"a": "x", "b": "y"}, {"a": "p", "b": "q"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a", Width: 120}, {Field: "b", Width: 120}})
	g.MasterDetail = MasterDetailConfig{
		Enabled: true, DetailHeight: 40,
		DetailRenderer: func(int) core.CanvasObject { return canvas.NewRectangle(color.Black) },
	}
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	g.SelectionMode = SelectionSingleCell
	press(g, CellID{Row: 0, Col: 0}, 0)

	objs := test.WidgetRenderer(g.cellsWidget).Objects()
	sel := render.ResolveThemeColors().Selection
	highlight, chevron := -1, -1
	for i, o := range objs {
		switch v := o.(type) {
		case *canvas.Rectangle:
			if v.Visible() && sameColor(v.FillColor, sel) && highlight < 0 {
				highlight = i
			}
		case *canvas.Image:
			if v.Visible() && chevron < 0 {
				chevron = i
			}
		}
	}
	require.GreaterOrEqual(t, highlight, 0, "the selected cell is highlighted")
	require.GreaterOrEqual(t, chevron, 0, "row 0 has a chevron")
	assert.Less(t, highlight, chevron, "the highlight is drawn before, so under, the chevron")
}
