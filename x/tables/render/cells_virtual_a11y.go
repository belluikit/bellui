package render

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// a11yVirtualMargin is how many rows above and below the realized viewport the
// grid surfaces as virtual accessibility nodes. It bounds the window so a
// million-row grid never allocates a node per row — the full extent is reported
// through AccessibleGridDimensions instead. Navigating to rows beyond this
// margin is a platform-virtualization (lazy realization) concern tracked
// separately; see the core.AccessibleVirtualNodeProvider contract.
const a11yVirtualMargin = 20

var _ core.AccessibleVirtualNodeProvider = (*CellsWidget)(nil)

// AccessibleVirtualNodes surfaces a bounded window of off-screen rows as flat
// RoleTableCell virtual nodes (core.AccessibleVirtualNodeProvider), so assistive
// tech has coherent, source-identified cells just beyond the realized viewport
// without materializing every row.
//
// The realized (on-screen) cells are already real accessible descendants
// (cellHost), collected by the walker before this provider is consulted; this
// method covers only the margin rows OUTSIDE the realized [VisibleRows] window,
// so the two sets never overlap. Nodes carry display coordinates in the absolute
// column space (matching the realized cells and AccessibleGridDimensions) and a
// source-keyed ID so the platform tracks a record across updates. Positions are
// in this widget's own content space — the CellsWidget is the scrolled content,
// so its origin is the top of all rows, exactly like the realized cells.
func (c *CellsWidget) AccessibleVirtualNodes() []core.AccessibleVirtualNode {
	vp := c.viewport
	grid := c.Grid
	if vp == nil || grid == nil || vp.TotalRows == 0 {
		return nil
	}
	vis := vp.VisibleRows
	cols := vp.VisibleCols
	if vis.Max < vis.Min || cols.Max < cols.Min {
		return nil // nothing realized yet
	}
	pinnedLeft := grid.PinnedLeftCount()
	// The checkbox-selection column sits in its visual slot at PinnedLeftCount;
	// every main column is right of it and shifts one place right. These virtual
	// nodes only cover main columns (all >= pinnedLeft), so they always shift —
	// matching the realized cells (cellHost) and AccessibleGridDimensions.
	cbOffset := 0
	if grid.ShowCheckboxColumn() {
		cbOffset = 1
	}

	// Bounded off-screen window: margin rows above and below the realized range.
	top := vis.Min - a11yVirtualMargin
	if top < 0 {
		top = 0
	}
	bottom := vis.Max + a11yVirtualMargin
	if bottom > vp.TotalRows-1 {
		bottom = vp.TotalRows - 1
	}

	nodes := make([]core.AccessibleVirtualNode, 0, (bottom-top)*(cols.Max-cols.Min+1+cbOffset))
	emit := func(row int) {
		if grid.IsGroupRow(row) {
			return // group headers are surfaced as realized RoleTableRow nodes
		}
		y, rowH := vp.RowY(row)
		// Leading checkbox-selection cell at absolute column 0, mirroring the
		// realized checkbox host (a cellHost with sentinel internal column -1):
		// row-keyed selection state and identity via the same -1 lookup so the
		// realized and virtual channels report the same cell for a row.
		if cbOffset == 1 {
			id := grid.AccessibleCellIdentity(row, -1)
			var state core.AccessibleState
			if grid.IsSelected(row, -1) {
				state = core.StateSelected
			}
			nodes = append(nodes, core.AccessibleVirtualNode{
				AccessibleNode: core.AccessibleNode{
					Role:            core.RoleTableCell,
					Label:           grid.AccessibleCellText(row, -1),
					State:           state,
					HasCellPosition: true,
					CellRow:         row,
					CellCol:         0,
					NodeID:          id,
				},
				Pos:  geo.NewPos(0, y),
				Size: geo.NewSize(checkboxCellWidth, rowH),
				ID:   id,
			})
		}
		for col := cols.Min; col <= cols.Max; col++ {
			absCol := pinnedLeft + col
			x, colW := vp.ColX(col)
			id := grid.AccessibleCellIdentity(row, absCol)
			// A cursor parked just off-screen (within the margin) carries
			// StateFocused too, matching the realized host so the AT-focused
			// descendant node reports focus wherever it is reconciled.
			var state core.AccessibleState
			if grid.IsFocused(row, absCol) {
				state = core.StateFocused
			}
			nodes = append(nodes, core.AccessibleVirtualNode{
				AccessibleNode: core.AccessibleNode{
					Role:            core.RoleTableCell,
					Label:           grid.AccessibleCellText(row, absCol),
					State:           state,
					HasCellPosition: true,
					CellRow:         row,
					CellCol:         absCol + cbOffset,
					NodeID:          id,
				},
				Pos:  geo.NewPos(x, y),
				Size: geo.NewSize(colW, rowH),
				ID:   id,
			})
		}
	}
	for row := top; row < vis.Min; row++ {
		emit(row)
	}
	for row := vis.Max + 1; row <= bottom; row++ {
		emit(row)
	}
	return nodes
}
