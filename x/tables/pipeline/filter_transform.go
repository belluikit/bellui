package pipeline

// ColumnFilter defines a filter for a single column.
type ColumnFilter struct {
	// ValueGetter extracts the filterable value from row data.
	ValueGetter func(rowData any) any
	// Matches returns true if the value passes the filter.
	Matches func(value any) bool
}

// FilterTransform removes rows that don't match the active filters.
type FilterTransform struct {
	Filters []ColumnFilter
}

// NewFilterTransform creates a filter transform.
func NewFilterTransform(filters ...ColumnFilter) *FilterTransform {
	return &FilterTransform{Filters: filters}
}

func (t *FilterTransform) Apply(indices []int, source DataSource) []int {
	if len(t.Filters) == 0 {
		return indices
	}

	result := make([]int, 0, len(indices))
	for _, idx := range indices {
		row := source.Row(idx)
		pass := true
		for _, f := range t.Filters {
			val := f.ValueGetter(row)
			if !f.Matches(val) {
				pass = false
				break
			}
		}
		if pass {
			result = append(result, idx)
		}
	}
	return result
}
