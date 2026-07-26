package render

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// CellsWidget is the inner scrollable content widget that hosts visible cells.
// It is placed inside a Scroll widget and only materializes cells in the visible range.
type CellsWidget struct {
	widget.BaseWidget

	// Grid is a back-reference to the DataGrid for accessing data and configuration.
	// Set by the DataGrid during initialization.
	Grid GridAccess

	viewport *Viewport
	pool     *CellPool
}

// GridAccess is the interface the CellsWidget needs from the DataGrid.
// This avoids a circular dependency between the render and root packages.
type GridAccess interface {
	// ColumnCount returns the number of visible columns.
	ColumnCount() int
	// RowCount returns the number of display rows (after pipeline transforms).
	RowCount() int
	// ColumnTypeName returns the type name for the given column index (for pool keying).
	ColumnTypeName(col int) string
	// CreateCell creates a new cell template for the given column.
	CreateCell(col int) core.CanvasObject
	// UpdateCell updates a cell template with data for the given display row and column.
	UpdateCell(row, col int, cell core.CanvasObject)
	// RowHeight returns the row height to use.
	RowHeight() float32
	// CellPadding returns the padding between cells.
	CellPadding() float32
	// ColumnWidths returns the resolved widths for all columns.
	ColumnWidths() []float32
	// IsSelected returns true if the given cell is selected.
	IsSelected(row, col int) bool
	// IsFocused reports whether the given cell is the grid's keyboard-focus
	// cursor cell — the moving cell a screen reader should announce as the cursor
	// travels. It backs the cell host's core.StateFocused, mirroring IsSelected
	// for StateSelected, and pairs with the DataGrid designating that same cell as
	// its AT-focused descendant (fyne#688). row/col are display + absolute column
	// coordinates, matching IsSelected.
	IsFocused(row, col int) bool
	// SelectCell selects the cell at the given display row and absolute (layout)
	// column, as a pointer tap would. It backs the cell host's accessible
	// "select" action; a negative column (the checkbox host's sentinel) selects
	// the row.
	SelectCell(row, col int)
	// AccessibleCellText returns the full untruncated text a screen reader
	// should announce for the cell at the given display row and absolute column.
	AccessibleCellText(row, col int) string
	// AccessibleCellIdentity returns a stable, comparable identity for the cell
	// at the given display row and absolute column — keyed by SOURCE row so a
	// screen reader's cursor follows the record across sort/scroll/filter. It
	// returns nil for synthetic rows (e.g. group headers) that have no source
	// identity, so the platform falls back to keying by object.
	AccessibleCellIdentity(row, col int) any
	// IsHovered returns true if the given cell is hovered.
	IsHovered(row, col int) bool
	// IsGroupRow returns true if the given display row is a group header.
	IsGroupRow(row int) bool
	// GroupRowInfo returns the group key and child count for a group header row.
	GroupRowInfo(row int) (key string, childCount int, expanded bool, depth int)
	// ToggleGroup expands or collapses the row group with the given key, routing
	// through the same path a pointer tap on the group header uses. It backs the
	// group-header row's accessible expand/collapse action.
	ToggleGroup(groupKey string)
	// IsAlternateRow returns true if the row should use alternate coloring.
	IsAlternateRow(row int) bool
	// ShowCheckboxColumn returns true if a checkbox column should be rendered.
	ShowCheckboxColumn() bool
	// ColSpanAt returns the column span for a cell.
	ColSpanAt(row, col int) int
	// PinnedLeftCount returns the number of pinned-left columns.
	PinnedLeftCount() int
	// PinnedRightCount returns the number of pinned-right columns.
	PinnedRightCount() int

	// Master-detail. DetailToggleWidth is the left indent reserved for the
	// disclosure chevron (0 when master-detail is inactive). IsMasterRow reports
	// whether a display row is expandable. DetailBandHeight returns the expanded
	// detail band height beneath a row's cells (0 when collapsed). DetailContent
	// returns the cached detail object for an expanded row, or nil. ToggleDetail
	// expands/collapses a master row's detail band, backing the master row's
	// accessible expand/collapse action.
	DetailToggleWidth() float32
	IsMasterRow(row int) bool
	DetailBandHeight(row int) float32
	DetailContent(row int) core.CanvasObject
	ToggleDetail(row int)

	// Tree data. IsTreeRow reports whether a display row is a tree node (tree
	// data active). TreeRowInfo returns the node's depth (0 = root), whether it
	// has children, and whether it is expanded. TreeFirstColIndent is the left
	// indent applied to the first main column of a tree row (depth indentation
	// plus the chevron slot; 0 when tree data is inactive). ToggleTreeNode
	// expands/collapses the tree node at a display row, backing the tree row's
	// accessible expand/collapse action and the disclosure-chevron click.
	IsTreeRow(row int) bool
	TreeRowInfo(row int) (depth int, hasChildren, expanded bool)
	TreeFirstColIndent(row int) float32
	ToggleTreeNode(row int)

	// Methods for pinned columns — use absolute display index (not main-only).
	PinnedColumnTypeName(displayCol int) string
	PinnedCreateCell(displayCol int) core.CanvasObject
	PinnedUpdateCell(row, displayCol int, cell core.CanvasObject)

	// HeaderColumnWidths returns widths for ALL visible columns (pinned + main + right).
	HeaderColumnWidths() []float32
}

// NewCellsWidget creates a new CellsWidget.
func NewCellsWidget(grid GridAccess, viewport *Viewport, pool *CellPool) *CellsWidget {
	c := &CellsWidget{
		Grid:     grid,
		viewport: viewport,
		pool:     pool,
	}
	c.ExtendBaseWidget(c)
	return c
}

// CreateRenderer returns the CellsRenderer that manages visible cell objects.
func (c *CellsWidget) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	return NewCellsRenderer(c)
}

// MinSize returns the total content size (all rows * all columns).
func (c *CellsWidget) MinSize() geo.Size {
	return c.viewport.ContentSize()
}
