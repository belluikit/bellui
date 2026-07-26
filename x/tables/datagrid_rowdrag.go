package tables

// OnRowDragged is called when the user drags row data to a new position.
// This is only meaningful for in-memory data sources where the underlying
// slice order can be changed.
type OnRowDragged func(fromRow, toRow int)

// MoveRow moves a row from one display position to another.
// This triggers a data callback (if the data source supports reordering)
// and refreshes the display. The OnRowMoved callback is invoked if set.
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

	g.pipe.MarkDirty()
	g.RefreshData()
}

// OnRowMoved is a callback field — set it on the DataGrid.
// It's stored in datagrid.go's struct definition. Adding a dedicated accessor here.

// SetOnRowMoved sets the callback for when a row is dragged to a new position.
func (g *DataGrid) SetOnRowMoved(fn func(fromSourceRow, toSourceRow int)) {
	g.OnRowMoved = fn
}
