package tables

// FilterModel represents the serializable state of a column filter.
type FilterModel struct {
	Type   string // "text", "number", "date", "set"
	Filter any    // Filter-specific configuration
}
