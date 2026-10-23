package tables

import (
	"io"

	"github.com/belluikit/bellui/x/tables/export"
	"github.com/belluikit/bellui/x/tables/pipeline"
)

// GridAPI provides programmatic control over a DataGrid.
type GridAPI struct {
	grid *DataGrid
}

// --- Data ---

// RefreshCells re-reads all visible cells from the data source.
func (a *GridAPI) RefreshCells() {
	a.grid.RefreshData()
}

// RefreshRow re-reads a specific row.
func (a *GridAPI) RefreshRow(row int) {
	// For now, refresh all. Per-row refresh will be optimized later.
	a.grid.RefreshData()
}

// --- Selection ---

// SelectCell selects a specific cell.
func (a *GridAPI) SelectCell(id CellID) {
	a.grid.selectCell(id)
}

// SelectRow selects the row with the given source-row index (a stable index
// into the underlying data, not a display position).
func (a *GridAPI) SelectRow(row int) {
	a.grid.selectedRows[row] = true
	a.grid.Refresh()
}

// DeselectAll clears all selection.
func (a *GridAPI) DeselectAll() {
	a.grid.selectedCell = nil
	a.grid.selCellRow, a.grid.selCellCol = -1, 0
	a.grid.selectedRows = make(map[int]bool)
	a.grid.Refresh()
}

// GetSelectedRows returns the selected rows as stable source-row indices (into
// the underlying data). These remain valid across sort/filter/paginate.
func (a *GridAPI) GetSelectedRows() []int {
	rows := make([]int, 0, len(a.grid.selectedRows))
	for row := range a.grid.selectedRows {
		rows = append(rows, row)
	}
	return rows
}

// --- Navigation ---

// ScrollToRow scrolls so the given display row is visible. It moves only the
// vertical axis (see DataGrid.ScrollToRow), leaving the horizontal offset
// where the user put it.
func (a *GridAPI) ScrollToRow(row int) {
	a.grid.ScrollToRow(row)
}

// ScrollToCell scrolls so the given cell is visible.
func (a *GridAPI) ScrollToCell(id CellID) {
	a.grid.scrollToCell(id)
}

// --- Columns ---

// SetColumnWidth changes the width of a column by its ID.
func (a *GridAPI) SetColumnWidth(colID string, width float32) {
	for _, col := range a.grid.resolvedCols {
		if col.def.ID() == colID {
			col.width = width
			break
		}
	}
	a.grid.updateViewportGeometry()
	a.grid.Refresh()
}

// SizeColumnsToFit adjusts main (non-pinned) column widths to fill the
// available scroll viewport width. Pinned columns keep their current widths.
func (a *GridAPI) SizeColumnsToFit() {
	g := a.grid
	layout := g.visibleLayout()
	mainCols := layout.main
	if g.scrollContent == nil || len(mainCols) == 0 {
		return
	}

	// Available width = grid width - pinned left - pinned right - padding gaps
	padding := g.cellPadding()
	pinnedLeftW := layout.pinnedLeftWidth(padding)
	pinnedRightW := layout.pinnedRightWidth(padding)

	available := g.Size().Width
	if pinnedLeftW > 0 {
		available -= pinnedLeftW + padding
	}
	if pinnedRightW > 0 {
		available -= pinnedRightW + padding
	}
	// Subtract padding between main columns
	available -= padding * float32(len(mainCols)-1)

	perCol := available / float32(len(mainCols))

	for _, col := range mainCols {
		w := perCol
		if col.def.MinWidth > 0 && w < col.def.MinWidth {
			w = col.def.MinWidth
		}
		if col.def.MaxWidth > 0 && w > col.def.MaxWidth {
			w = col.def.MaxWidth
		}
		col.width = w
	}
	g.updateViewportGeometry()
	g.Refresh()
}

// MoveColumn moves a column from one display position to another.
func (a *GridAPI) MoveColumn(fromIndex, toIndex int) {
	a.grid.MoveColumn(fromIndex, toIndex)
}

// SetColumnVisible shows or hides a column.
func (a *GridAPI) SetColumnVisible(colID string, visible bool) {
	a.grid.SetColumnVisible(colID, visible)
}

// SetColumnPinned pins or unpins a column.
func (a *GridAPI) SetColumnPinned(colID string, side PinSide) {
	a.grid.SetColumnPinned(colID, side)
}

// --- Selection (extended) ---

// SelectAll selects all rows (multi-row mode only).
func (a *GridAPI) SelectAll() {
	a.grid.SelectAll()
}

// GetSelectedCells returns the currently selected cell, if any.
func (a *GridAPI) GetSelectedCells() *CellID {
	return a.grid.selectedCell
}

// --- Editing ---

// StartEditing begins editing a specific cell.
func (a *GridAPI) StartEditing(id CellID) {
	a.grid.StartEditing(id)
}

// StopEditing cancels any active edit.
func (a *GridAPI) StopEditing() {
	a.grid.StopEditing()
}

// CommitEditing commits any active edit, writing the editor's value back to
// the row.
func (a *GridAPI) CommitEditing() {
	a.grid.CommitEditing()
}

// Undo reverts the last cell edit.
func (a *GridAPI) Undo() {
	a.grid.Undo()
}

// Redo re-applies a previously undone edit.
func (a *GridAPI) Redo() {
	a.grid.Redo()
}

// --- Sorting ---

// SetSort sets the sort model, replacing any existing sort.
func (a *GridAPI) SetSort(items ...SortItem) {
	a.grid.sortModel = items
	a.grid.rebuildPipeline()
	if a.grid.OnSortChanged != nil {
		a.grid.OnSortChanged(a.grid.sortModel)
	}
}

// GetSort returns the current sort model.
func (a *GridAPI) GetSort() []SortItem {
	result := make([]SortItem, len(a.grid.sortModel))
	copy(result, a.grid.sortModel)
	return result
}

// ClearSort removes all sorting.
func (a *GridAPI) ClearSort() {
	a.grid.sortModel = nil
	a.grid.rebuildPipeline()
	if a.grid.OnSortChanged != nil {
		a.grid.OnSortChanged(nil)
	}
}

// --- Filtering ---

// SetColumnFilter sets a filter function for the given column.
// Pass nil to clear the filter for that column.
func (a *GridAPI) SetColumnFilter(colID string, matches func(value any) bool) {
	a.grid.setColumnFilter(colID, matches)
}

// ClearFilters removes all column filters and the quick filter.
func (a *GridAPI) ClearFilters() {
	a.grid.filterModel = make(map[string]filterState)
	a.grid.quickFilter = ""
	a.grid.rebuildPipeline()
}

// SetQuickFilter sets a text search across all columns.
// Pass "" to clear the quick filter.
func (a *GridAPI) SetQuickFilter(text string) {
	a.grid.setQuickFilter(text)
}

// GetQuickFilter returns the current quick filter text.
func (a *GridAPI) GetQuickFilter() string {
	return a.grid.quickFilter
}

// --- Pagination ---

// SetPage sets the current page (0-indexed). Requires Options.Pagination = true.
func (a *GridAPI) SetPage(page int) {
	g := a.grid
	if g.paginateTransform == nil {
		return
	}
	g.paginateTransform.SetPage(page)
	g.rebuildPipeline()
}

// GetPage returns the current page (0-indexed).
func (a *GridAPI) GetPage() int {
	return a.grid.CurrentPage()
}

// GetPageCount returns the total number of pages.
func (a *GridAPI) GetPageCount() int {
	return a.grid.PageCount()
}

// SetPageSize changes the page size and resets to page 0.
func (a *GridAPI) SetPageSize(size int) {
	g := a.grid
	g.Options.PageSize = size
	if g.paginateTransform != nil {
		g.paginateTransform.PageSize = size
		g.paginateTransform.SetPage(0)
	}
	g.rebuildPipeline()
}

// EnablePagination enables pagination with the given page size.
func (a *GridAPI) EnablePagination(pageSize int) {
	g := a.grid
	g.Options.Pagination = true
	g.Options.PageSize = pageSize
	g.paginateTransform = pipeline.NewPaginateTransform(pageSize)
	g.rebuildPipeline()
}

// DisablePagination disables pagination.
func (a *GridAPI) DisablePagination() {
	g := a.grid
	g.Options.Pagination = false
	g.paginateTransform = nil
	g.rebuildPipeline()
}

// --- Grouping ---

// SetRowGroupColumn sets the column to group rows by.
// Pass "" to clear grouping.
func (a *GridAPI) SetRowGroupColumn(colID string) {
	g := a.grid
	if colID == "" {
		g.groupTransform = nil
		g.rebuildPipeline()
		return
	}
	getter := g.valueGetterForColID(colID)
	if getter == nil {
		return
	}
	g.groupTransform = pipeline.NewGroupTransform(colID, getter)
	g.rebuildPipeline()
}

// ToggleGroup toggles expand/collapse of a specific group.
func (a *GridAPI) ToggleGroup(groupKey string) {
	a.grid.ToggleGroup(groupKey)
}

// ExpandAllGroups expands all row groups.
func (a *GridAPI) ExpandAllGroups() {
	g := a.grid
	if g.groupTransform == nil {
		return
	}
	g.groupTransform.ExpandAll()
	g.rebuildPipeline()
}

// CollapseAllGroups collapses all row groups.
func (a *GridAPI) CollapseAllGroups() {
	g := a.grid
	if g.groupTransform == nil {
		return
	}
	// We need to recompute with all groups collapsed
	g.groupTransform.Collapsed = make(map[string]bool)
	// Get all group keys by running the transform once
	src := g.pipe.Source()
	n := src.Length()
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	g.groupTransform.CollapseAll(src, indices)
	g.rebuildPipeline()
}

// --- Tree data ---

// SetTreeData enables hierarchical tree data. getID returns a unique ID for a
// row; getParent returns the parent row's ID (matched against getID values), or
// a negative value for root rows. Rows whose parent ID is not found are treated
// as roots, as is the first row of a loop of rows that are each other's
// parents. The parent ID is an int, so it only matches IDs that getID returns
// as Go integers (see [pipeline.TreeTransform.GetParent]). Pass a nil getID or
// getParent to disable tree data.
//
// The hierarchy is rendered on the first main (non-pinned) column with a depth
// indent and a disclosure chevron for parent rows. Expansion state is keyed by
// node ID and survives re-sort/re-filter.
func (a *GridAPI) SetTreeData(getID func(any) any, getParent func(any) int) {
	g := a.grid
	if getID == nil || getParent == nil {
		g.treeTransform = nil
		g.rebuildPipeline()
		return
	}
	g.treeTransform = pipeline.NewTreeTransform(getID, getParent)
	g.rebuildPipeline()
}

// ToggleTreeNode toggles expand/collapse of the tree node at the given display
// row. Rows without children are ignored.
func (a *GridAPI) ToggleTreeNode(displayRow int) {
	a.grid.ToggleTreeNode(displayRow)
}

// ExpandAllNodes expands every tree node.
func (a *GridAPI) ExpandAllNodes() {
	g := a.grid
	if g.treeTransform == nil {
		return
	}
	src := g.pipe.Source()
	n := src.Length()
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	g.treeTransform.ExpandAll(indices, src)
	g.rebuildPipeline()
}

// CollapseAllNodes collapses every tree node.
func (a *GridAPI) CollapseAllNodes() {
	g := a.grid
	if g.treeTransform == nil {
		return
	}
	g.treeTransform.CollapseAll()
	g.rebuildPipeline()
}

// --- Pivot ---

// SetPivot configures a pivot table. The grid will display pivoted data with
// dynamically generated columns for each unique value in the pivot column.
// The pivotValueField values are aggregated using the given function.
// Call with empty fields to clear the pivot.
func (a *GridAPI) SetPivot(rowGroupField, pivotField, valueField string, aggFunc pipeline.AggFunc) {
	g := a.grid
	if rowGroupField == "" || pivotField == "" || valueField == "" {
		g.pivotTransform = nil
		g.rebuildPipeline()
		return
	}

	rgGetter := g.valueGetterForColID(rowGroupField)
	pvGetter := g.valueGetterForColID(pivotField)
	valGetter := g.valueGetterForColID(valueField)
	if rgGetter == nil || pvGetter == nil || valGetter == nil {
		return
	}

	g.pivotTransform = pipeline.NewPivotTransform(pipeline.PivotConfig{
		RowGroupField:  rowGroupField,
		RowGroupGetter: rgGetter,
		PivotField:     pivotField,
		PivotGetter:    pvGetter,
		ValueField:     valueField,
		ValueGetter: func(rowData any) float64 {
			v := valGetter(rowData)
			switch n := v.(type) {
			case float64:
				return n
			case float32:
				return float64(n)
			case int:
				return float64(n)
			case int64:
				return float64(n)
			}
			return 0
		},
		AggFunc: aggFunc,
	})
	g.rebuildPipeline()
}

// GetPivotColumns returns the dynamically generated pivot column names,
// or nil if no pivot is active.
func (a *GridAPI) GetPivotColumns() []string {
	g := a.grid
	if g.pivotTransform == nil || g.pivotTransform.Result == nil {
		return nil
	}
	return g.pivotTransform.Result.PivotColumns
}

// --- State ---

// GetGridState returns the current grid state for persistence.
func (a *GridAPI) GetGridState() *GridState {
	state := &GridState{
		SortModel: make([]SortItem, len(a.grid.sortModel)),
	}
	copy(state.SortModel, a.grid.sortModel)
	for i, col := range a.grid.resolvedCols {
		state.Columns = append(state.Columns, ColumnState{
			ColID:  col.def.ID(),
			Width:  col.width,
			Pinned: col.def.Pinned,
			Hidden: col.def.Hidden,
			Index:  i,
		})
	}
	return state
}

// SetGridState restores the grid state from a previous save.
func (a *GridAPI) SetGridState(state *GridState) {
	if state == nil {
		return
	}
	for _, cs := range state.Columns {
		for _, col := range a.grid.resolvedCols {
			if col.def.ID() == cs.ColID {
				col.width = cs.Width
				col.def.Pinned = cs.Pinned
				col.def.Hidden = cs.Hidden
				break
			}
		}
	}
	// Restore sort model
	if len(state.SortModel) > 0 {
		a.grid.sortModel = make([]SortItem, len(state.SortModel))
		copy(a.grid.sortModel, state.SortModel)
	} else {
		a.grid.sortModel = nil
	}
	a.grid.rebuildPipeline()
}

// --- Export ---

// exportColumns builds the export column list from the visible layout.
func (a *GridAPI) exportColumns() []export.Column {
	layout := a.grid.visibleLayout()
	cols := make([]export.Column, len(layout.all))
	for i, rc := range layout.all {
		cols[i] = export.Column{
			Header:      rc.def.Header(),
			Field:       rc.def.Field,
			ValueGetter: rc.def.valueGetter(),
			Formatter:   rc.def.ValueFormatter,
			Type:        exportFieldType(rc.def),
		}
	}
	return cols
}

// exportFieldType resolves a column's declared field type for typed
// exports: an explicit ExportType wins (including over ValueFormatter
// — the formatter stays a display concern, PXF exports the raw typed
// value), an undeclared ValueFormatter column is string, and the
// unambiguous ColumnTypes map directly. ColumnNumber and ColumnCustom
// stay unspecified — export.PXF rejects them rather than inferring a
// type from cell values.
func exportFieldType(def *ColumnDef) export.FieldType {
	if def.ExportType != export.FieldUnspecified {
		return def.ExportType
	}
	if def.ValueFormatter != nil {
		return export.FieldString
	}
	switch def.Type {
	case ColumnText:
		return export.FieldString
	case ColumnBoolean:
		return export.FieldBool
	case ColumnDate:
		return export.FieldTimestamp
	}
	return export.FieldUnspecified
}

// ExportCSV writes the current grid data (after filters/sort) as CSV.
func (a *GridAPI) ExportCSV(w io.Writer) error {
	return export.CSV(w, a.exportColumns(), &gridDataAccess{grid: a.grid})
}

// ExportTSV writes the current grid data as tab-separated values.
func (a *GridAPI) ExportTSV(w io.Writer) error {
	return export.TSV(w, a.exportColumns(), &gridDataAccess{grid: a.grid})
}

// ExportExcel writes the current grid data as an Excel (.xlsx) file.
func (a *GridAPI) ExportExcel(w io.Writer) error {
	return export.Excel(w, a.exportColumns(), &gridDataAccess{grid: a.grid})
}

// ExportPDF writes the current grid data as a paginated PDF report:
// column headers repeated on every page, columns scaled to fit the
// page, page numbers, and an optional title block. Pass the result to
// the OS PDF viewer (e.g. via bellui.OpenURL on a file:// URL) to print.
func (a *GridAPI) ExportPDF(w io.Writer, opts export.PDFOptions) error {
	return export.PDF(w, a.exportColumns(), &gridDataAccess{grid: a.grid}, opts)
}

// ExportPXF writes the current grid data as a self-describing PXF
// document: an embedded @proto schema (protowire draft §3.4.5)
// followed by a @dataset directive (draft §3.4.4). msgType is the
// fully qualified protobuf message type the rows bind to; the visible
// column Field names become its field names, with field types taken
// from each column's declaration — ColumnDef.ExportType, or the
// unambiguous ColumnType/ValueFormatter mappings (see
// ColumnDef.ExportType). Types are never inferred from cell values:
// a ColumnNumber or ColumnCustom column without an ExportType fails
// the export, as does a cell value that does not match its column's
// declared type (see export.PXF).
func (a *GridAPI) ExportPXF(w io.Writer, msgType string) error {
	return export.PXF(w, msgType, a.exportColumns(), &gridDataAccess{grid: a.grid})
}

// gridDataAccess adapts DataGrid to export.DataAccess.
type gridDataAccess struct {
	grid *DataGrid
}

func (d *gridDataAccess) RowCount() int       { return d.grid.RowCount() }
func (d *gridDataAccess) RowData(row int) any { return d.grid.pipe.DisplayRow(row) }
