package tables

import (
	"reflect"
	"strings"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/tables/cell"
	"github.com/belluikit/bellui/x/tables/export"
	"github.com/belluikit/bellui/x/tables/pipeline"
)

// ColumnDef defines the configuration for a single column in the data grid.
type ColumnDef struct {
	// Field is the key used to extract the value from row data.
	// Supports dot notation for nested fields (e.g. "address.city"),
	// walking string-keyed maps and exported struct fields (through any
	// pointers) one segment at a time. A map row holding the whole dotted
	// string as a literal key resolves to that entry instead.
	Field string

	// HeaderName is the display text shown in the column header.
	// Defaults to Field if empty.
	HeaderName string

	// ColID is a unique identifier for this column. Defaults to Field if empty.
	ColID string

	// Type determines the default renderer, editor, filter, and comparator.
	Type ColumnType

	// ExportType declares the column's field type for typed exports
	// (PXF). When set, it wins — including over ValueFormatter, which
	// stays a display/CSV concern while PXF exports the raw typed
	// value. When unset, the unambiguous ColumnTypes map directly:
	// ColumnText to export.FieldString, ColumnBoolean to
	// export.FieldBool, ColumnDate to export.FieldTimestamp, and any
	// column with a ValueFormatter to export.FieldString (formatted
	// cells export as strings). ColumnNumber and ColumnCustom have no
	// unambiguous field type — PXF export fails for them unless
	// ExportType is set; types are never inferred from values.
	ExportType export.FieldType

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

	// AggFunc aggregates this column over each row group's rows when the grid
	// is grouped (GridAPI.SetRowGroupColumn); the group header row shows the
	// result in this column and includes it in its accessible label. Nil
	// means no aggregate. Wrap the function with bellui.Ptr, since AggSum is
	// the zero value: AggFunc: bellui.Ptr(pipeline.AggAvg).
	//
	// The cell values are read as numbers (any Go integer or float type); a
	// value of any other type, nil included, counts as 0. AggCount counts the
	// group's rows. A collapsed group still aggregates all of its rows. The
	// aggregate, a float64, is shown through ValueFormatter when set (which
	// must therefore accept a float64), else through a NumberRenderer's
	// Format, else rounded to two decimals. A count is always shown as a
	// whole number, never through the formatter.
	AggFunc *pipeline.AggFunc

	// CellRenderer overrides the default cell renderer for this column.
	CellRenderer cell.RendererFactory

	// Editor overrides the inline cell editor for this column (e.g.
	// cell.NewSelectEditor for a dropdown of candidate values). If nil, a
	// plain text Entry is used.
	Editor cell.EditorFactory

	// CanEdit gates editing per cell. On an editable column, a non-nil
	// CanEdit is consulted with the row's data before an editor is created;
	// returning false refuses the edit and no editor appears. It further
	// restricts Editable — it cannot make a non-editable column editable.
	// Nil means the column's Editable setting alone decides.
	CanEdit func(rowData any) bool

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

// defaultFieldAccess resolves ColumnDef.Field against a row. Dot notation
// walks nested values one segment at a time (e.g. "address.city"), except
// that a map row holding the whole dotted field as a literal key resolves to
// that entry. Each segment resolves a map key for string-keyed map values or
// an exported struct field (through any pointers) otherwise. Any segment that
// fails to resolve — non-map/struct value, missing key, missing/unexported
// field, nil pointer — yields nil, and display/sort/filter treat the cell as
// empty; set ColumnDef.ValueGetter for rows that need custom access.
func defaultFieldAccess(rowData any, field string) any {
	if strings.Contains(field, ".") {
		if m, ok := rowData.(map[string]any); ok {
			if v, ok := m[field]; ok {
				return v
			}
		}
		if m, ok := rowData.(map[string]string); ok {
			if v, ok := m[field]; ok {
				return v
			}
		}
	}
	value := rowData
	for field != "" {
		var segment string
		segment, field, _ = strings.Cut(field, ".")
		value = fieldSegment(value, segment)
		if value == nil {
			return nil
		}
	}
	return value
}

// fieldSegment resolves one dot-notation segment against a value: a map key
// for string-keyed maps, an exported struct field (through any pointers)
// otherwise, nil when neither applies.
func fieldSegment(value any, name string) any {
	if m, ok := value.(map[string]any); ok {
		return m[name]
	}
	if m, ok := value.(map[string]string); ok {
		return m[name]
	}
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		if f := v.FieldByName(name); f.IsValid() && f.CanInterface() {
			return f.Interface()
		}
	case reflect.Map:
		if v.Type().Key().Kind() == reflect.String {
			key := reflect.ValueOf(name).Convert(v.Type().Key())
			if e := v.MapIndex(key); e.IsValid() {
				return e.Interface()
			}
		}
	}
	return nil
}
