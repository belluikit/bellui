package tables

import "github.com/belluikit/bellui/x/tables/theme"

// GridOptions configures the behaviour and appearance of a DataGrid.
type GridOptions struct {
	// RowHeight is the fixed row height in pixels. Zero uses the default from the cell template.
	RowHeight float32

	// RowHeightFunc, when set, returns the height in pixels for a given SOURCE
	// row, enabling variable per-row heights. Return 0 (or a negative value) to
	// use the default row height for that row. Keyed by source row so heights
	// follow rows across sort/filter/paginate. Leaving it nil keeps every row at
	// the uniform RowHeight (the O(1) fast path).
	RowHeightFunc func(sourceRow int) float32
	// HeaderHeight is the fixed header height. Zero uses the default.
	HeaderHeight float32

	// Density controls the visual density of the grid.
	Density theme.DensityMode

	// Pagination enables paginated display.
	Pagination bool
	// PageSize is the number of rows per page when pagination is enabled.
	PageSize int

	// HideSeparators hides the grid lines between cells.
	HideSeparators bool
	// RowAlternation enables alternating row background colors.
	RowAlternation bool

	// CheckboxSelection shows checkboxes in the first column for row selection.
	CheckboxSelection bool

	// ShowStatusBar shows a status bar at the bottom with selection count and aggregations.
	ShowStatusBar bool

	// FloatingFilters shows inline filter input fields below each column header.
	FloatingFilters bool

	// RowDragEnabled allows rows to be reordered by dragging.
	RowDragEnabled bool

	// AccessibleLabel is the grid's human-readable name reported to assistive
	// technologies via core.Accessible. Empty leaves the grid unnamed.
	AccessibleLabel string

	// PinnedTopRows is the number of rows pinned to the top (always visible).
	PinnedTopRows int
	// PinnedBottomRows is the number of rows pinned to the bottom.
	PinnedBottomRows int
}
