package pipeline

import (
	"sort"

	"github.com/belluikit/bellui/lang"
)

// SortField defines a sort key and direction.
type SortField struct {
	// ValueGetter extracts the sortable value from row data.
	ValueGetter func(rowData any) any
	// Ascending indicates sort direction.
	Ascending bool
	// Comparator compares two values. Returns <0, 0, or >0.
	// If nil, a default comparator is used.
	Comparator func(a, b any) int
}

// SortTransform reorders rows by one or more sort fields.
type SortTransform struct {
	Fields []SortField
}

// NewSortTransform creates a sort transform.
func NewSortTransform(fields ...SortField) *SortTransform {
	return &SortTransform{Fields: fields}
}

func (t *SortTransform) Apply(indices []int, source DataSource) []int {
	if len(t.Fields) == 0 {
		return indices
	}

	result := make([]int, len(indices))
	copy(result, indices)

	sort.SliceStable(result, func(i, j int) bool {
		rowA := source.Row(result[i])
		rowB := source.Row(result[j])
		for _, field := range t.Fields {
			valA := field.ValueGetter(rowA)
			valB := field.ValueGetter(rowB)

			var cmp int
			if field.Comparator != nil {
				cmp = field.Comparator(valA, valB)
			} else {
				cmp = defaultCompare(valA, valB)
			}

			if cmp == 0 {
				continue
			}
			if !field.Ascending {
				cmp = -cmp
			}
			return cmp < 0
		}
		return false
	})

	return result
}

func defaultCompare(a, b any) int {
	// Compare by common types
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			// Locale-aware collation: byte order mis-sorts most non-English
			// text (German "äpfel" belongs with "a", Swedish "ö" after "z").
			// Per-ColumnDef Comparator overrides still win (handled by caller).
			return lang.Compare(av, bv)
		}
	case int:
		if bv, ok := b.(int); ok {
			return av - bv
		}
	case int64:
		if bv, ok := b.(int64); ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case float64:
		if bv, ok := b.(float64); ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case float32:
		if bv, ok := b.(float32); ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	case bool:
		if bv, ok := b.(bool); ok {
			if av == bv {
				return 0
			}
			if !av {
				return -1
			}
			return 1
		}
	}

	// Nil handling
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	return 0
}
