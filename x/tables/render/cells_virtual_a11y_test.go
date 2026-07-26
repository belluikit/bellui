package render

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func hugeViewport(g *fakeGrid, offsetY float32) *Viewport {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        g.rows,
		TotalCols:        g.cols,
		ColWidths:        g.ColumnWidths(),
	}
	vp.RecomputeColPositions()
	vp.Update(geo.NewPos(0, offsetY), geo.NewSize(400, 200))
	return vp
}

// The provider emits a bounded window of off-screen RoleTableCell nodes around
// the viewport — never one per row for a huge grid — each carrying a display
// position, a stable ID, and a row OUTSIDE the realized range.
func TestCellsWidget_VirtualNodesBoundedWindow(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 1_000_000, cols: 2}
	vp := hugeViewport(g, 500_000) // scrolled deep into the middle
	cw := NewCellsWidget(g, vp, NewCellPool())

	nodes := cw.AccessibleVirtualNodes()
	if len(nodes) == 0 {
		t.Fatal("no virtual nodes emitted for off-screen rows")
	}

	vis := vp.VisibleRows
	cols := vp.VisibleCols
	maxExpected := 2 * a11yVirtualMargin * (cols.Max - cols.Min + 1)
	if len(nodes) > maxExpected {
		t.Errorf("emitted %d nodes, exceeds bounded window %d (per-row leak?)", len(nodes), maxExpected)
	}

	above, below := false, false
	for _, n := range nodes {
		if n.Role != core.RoleTableCell {
			t.Errorf("node role = %v, want RoleTableCell", n.Role)
		}
		if !n.HasCellPosition {
			t.Errorf("node (%d,%d) missing cell position", n.CellRow, n.CellCol)
		}
		if n.CellRow >= vis.Min && n.CellRow <= vis.Max {
			t.Errorf("node row %d is inside the realized range [%d,%d] (should be margin-only)", n.CellRow, vis.Min, vis.Max)
		}
		if n.ID == nil || n.NodeID == nil {
			t.Errorf("node (%d,%d) missing stable identity", n.CellRow, n.CellCol)
		}
		// Position is content-space, matching vp.RowY.
		if wantY, _ := vp.RowY(n.CellRow); n.Pos.Y != wantY {
			t.Errorf("node row %d Pos.Y = %v, want %v", n.CellRow, n.Pos.Y, wantY)
		}
		if n.CellRow < vis.Min {
			above = true
		}
		if n.CellRow > vis.Max {
			below = true
		}
	}
	if !above || !below {
		t.Errorf("scrolled to middle: expected margin nodes both above and below (above=%v below=%v)", above, below)
	}
}

// At the top of the grid there are no rows above, so the window only extends
// below the realized range.
func TestCellsWidget_VirtualNodesClampAtTop(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 200, cols: 1}
	vp := hugeViewport(g, 0) // top of the grid
	cw := NewCellsWidget(g, vp, NewCellPool())

	for _, n := range cw.AccessibleVirtualNodes() {
		if n.CellRow < vp.VisibleRows.Min {
			t.Errorf("node row %d is above the top realized row %d", n.CellRow, vp.VisibleRows.Min)
		}
	}
}

// With a checkbox-selection column, every off-screen non-group row emits a
// leading col-0 RoleTableCell (matching the realized checkbox host), the data
// cells shift to cols >= 1, and the checkbox node carries the row's selection
// state — so the virtual window stays consistent with the realized cells and
// AccessibleGridDimensions.
func TestCellsWidget_VirtualNodesCheckboxColumn(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 200, cols: 2, checkbox: true}
	vp := hugeViewport(g, 0) // top of the grid; margin extends below
	// Mark a row guaranteed to fall in the off-screen margin selected, keyed by
	// the checkbox host's sentinel column -1.
	selRow := vp.VisibleRows.Max + 1
	g.selected = map[cellKey]bool{{selRow, -1}: true}
	cw := NewCellsWidget(g, vp, NewCellPool())

	nodes := cw.AccessibleVirtualNodes()
	if len(nodes) == 0 {
		t.Fatal("no virtual nodes emitted")
	}

	checkboxByRow := map[int]core.AccessibleVirtualNode{}
	dataCols := map[int]bool{}
	for _, n := range nodes {
		if n.CellCol == 0 { // the checkbox cell
			if _, dup := checkboxByRow[n.CellRow]; dup {
				t.Errorf("row %d emitted more than one col-0 node", n.CellRow)
			}
			checkboxByRow[n.CellRow] = n
			if n.Role != core.RoleTableCell {
				t.Errorf("checkbox node row %d role = %v, want RoleTableCell", n.CellRow, n.Role)
			}
			if !n.HasCellPosition || n.Pos.X != 0 {
				t.Errorf("checkbox node row %d position off: hasPos=%v x=%v", n.CellRow, n.HasCellPosition, n.Pos.X)
			}
			if n.ID == nil || n.NodeID == nil {
				t.Errorf("checkbox node row %d missing stable identity", n.CellRow)
			}
			continue
		}
		dataCols[n.CellCol] = true
	}

	// Data cells shifted right of the checkbox (cols 1 and 2, none at 0).
	if !dataCols[1] || !dataCols[2] {
		t.Errorf("data cells not shifted right of checkbox: cols=%v", dataCols)
	}

	// Every emitted data row also has its checkbox cell.
	dataRows := map[int]bool{}
	for _, n := range nodes {
		if n.CellCol != 0 {
			dataRows[n.CellRow] = true
		}
	}
	for row := range dataRows {
		if _, ok := checkboxByRow[row]; !ok {
			t.Errorf("row %d has data cells but no col-0 checkbox node", row)
		}
	}

	// The selected row's checkbox reports StateSelected.
	sel, ok := checkboxByRow[selRow]
	if !ok {
		t.Fatalf("selected margin row %d emitted no checkbox node", selRow)
	}
	if sel.State&core.StateSelected == 0 {
		t.Errorf("checkbox node for selected row %d missing StateSelected", selRow)
	}
}

// No realized rows yet ⇒ no virtual nodes.
func TestCellsWidget_VirtualNodesEmptyWhenNothingRealized(t *testing.T) {
	test.NewTempApp(t)
	g := &fakeGrid{rows: 0, cols: 1}
	vp := hugeViewport(g, 0)
	cw := NewCellsWidget(g, vp, NewCellPool())
	if nodes := cw.AccessibleVirtualNodes(); nodes != nil {
		t.Errorf("expected nil nodes for empty grid, got %d", len(nodes))
	}
}
