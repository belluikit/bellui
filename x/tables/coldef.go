package tables

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/tables/cell"
)

// ColumnDef defines the configuration for a single column in the data grid.
type ColumnDef struct {
	// Field is the key used to extract the value from row data.
	// Supports dot notation for nested fields (e.g. "address.city").
	Field string

	// HeaderName is the display text shown in the column header.
	// Defaults to Field if empty.
	HeaderName string

	// ColID is a unique identifier for this column. Defaults to Field if empty.
	ColID string

	// Type determines the default renderer, editor, filter, and comparator.
	Type ColumnType

	// Width is the fixed width in pixels. Zero means use default or Flex.
	Width float32
	// MinWidth is the minimum allowed column width.
	MinWidth float32
	// MaxWidth is the maximum allowed column width. Zero means no max.
	MaxWidth float32
	// Flex is the flex grow factor. Zero means no flex.
	Flex float32

	// Sortable controls whether this column can be sorted. Nil inherits from DefaultColumnDef.
	Sortable *bool
	// Resizable controls whether this column can be resized by dragging. Nil inherits.
	Resizable *bool
	// Filterable controls whether this column can be filtered. Nil inherits.
	Filterable *bool
	// Editable controls whether cells in this column can be edited. Nil inherits.
	Editable *bool

	// Pinned indicates which side this column is pinned to.
	Pinned PinSide
	// Hidden hides the column from display.
	Hidden bool

	// ValueGetter extracts a cell value from the row data.
	// If nil, the default getter uses Field as a map key or struct field name.
	ValueGetter func(rowData any) any

	// ValueFormatter formats the cell value for display.
	// If nil, the value is converted to string using fmt.Sprint.
	ValueFormatter func(value any) string

	// CellRenderer overrides the default cell renderer for this column.
	CellRenderer cell.RendererFactory

	// Comparator provides a custom sort comparator.
	// Returns negative if a < b, zero if equal, positive if a > b.
	Comparator func(a, b any) int

	// ColSpan returns the number of columns this cell should span for a given row.
	// If nil or returns 1, the cell occupies a single column.
	ColSpan func(row int) int

	// RowSpan returns the number of rows this cell should span for a given row.
	// If nil or returns 1, the cell occupies a single row.
	RowSpan func(row int) int

	// TooltipGetter returns tooltip text for a cell. If nil, no tooltip is shown.
	TooltipGetter func(rowData any) string

	// Movable controls whether this column can be reordered by drag. Nil inherits.
	Movable *bool

	// HeaderComponent returns a custom widget for this column's header.
	// If nil, a default text header is used.
	HeaderComponent func() core.CanvasObject

	// Children defines sub-columns for a column group.
	// When Children is set, this is a column group header.
	Children []*ColumnDef

	// OpenByDefault controls whether a column group starts expanded.
	OpenByDefault bool
}

// DefaultColumnDef provides shared defaults applied to all columns.
type DefaultColumnDef struct {
	Sortable   bool
	Resizable  bool
	Filterable bool
	Editable   bool
	MinWidth   float32
	MaxWidth   float32
	Width      float32
	Flex       float32
}

// ID returns the effective column ID, falling back to Field.
func (c *ColumnDef) ID() string {
	if c.ColID != "" {
		return c.ColID
	}
	return c.Field
}

// Header returns the effective header name, falling back to Field.
func (c *ColumnDef) Header() string {
	if c.HeaderName != "" {
		return c.HeaderName
	}
	return c.Field
}

// IsGroup returns true if this column definition has children (is a column group).
func (c *ColumnDef) IsGroup() bool {
	return len(c.Children) > 0
}

// isSortable returns the effective sortable setting, using the default if nil.
func (c *ColumnDef) isSortable(def *DefaultColumnDef) bool {
	if c.Sortable != nil {
		return *c.Sortable
	}
	if def != nil {
		return def.Sortable
	}
	return true
}

// isResizable returns the effective resizable setting, using the default if nil.
func (c *ColumnDef) isResizable(def *DefaultColumnDef) bool {
	if c.Resizable != nil {
		return *c.Resizable
	}
	if def != nil {
		return def.Resizable
	}
	return true
}

// isFilterable returns the effective filterable setting, using the default if nil.
func (c *ColumnDef) isFilterable(def *DefaultColumnDef) bool {
	if c.Filterable != nil {
		return *c.Filterable
	}
	if def != nil {
		return def.Filterable
	}
	return true
}

// valueGetter returns the effective value getter for this column.
func (c *ColumnDef) valueGetter() func(any) any {
	if c.ValueGetter != nil {
		return c.ValueGetter
	}
	field := c.Field
	return func(rowData any) any {
		return defaultFieldAccess(rowData, field)
	}
}

func defaultFieldAccess(rowData any, field string) any {
	if m, ok := rowData.(map[string]any); ok {
		return m[field]
	}
	if m, ok := rowData.(map[string]string); ok {
		return m[field]
	}
	return nil
}
