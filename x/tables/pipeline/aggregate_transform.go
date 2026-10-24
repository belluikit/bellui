package pipeline

import "math"

// AggFunc is a named aggregation function.
type AggFunc int

const (
	AggSum AggFunc = iota
	AggAvg
	AggMin
	AggMax
	AggCount
	AggFirst
	AggLast
)

// AggColumn defines an aggregation to compute on a column.
type AggColumn struct {
	Field string
	// ValueGetter reads a row's value. Every row has one: a getter that has
	// nothing to report must return a number anyway, which then counts in
	// the aggregate (0 lowers an average). Use ValueOK to skip such rows.
	ValueGetter func(rowData any) float64
	Func        AggFunc
	CustomFunc  func(values []float64) float64
	// ValueOK reads a row's value and reports whether it has one (#747).
	// When set it is used instead of ValueGetter, which may then be nil.
	// A row it reports as having no value (ok false), such as a blank or
	// non-numeric cell, is left out of the aggregate: AggAvg, AggMin and
	// AggMax are taken over the remaining rows, AggSum adds nothing for it
	// (the same as adding 0), AggFirst and AggLast take the first and last
	// row that has a value, and CustomFunc receives only the values present.
	// AggCount still counts every row, blank or not.
	//
	// When no row has a value (or there are no rows), AggAvg, AggMin, AggMax,
	// AggFirst and AggLast have no value: Aggregate maps the field to nil
	// rather than to a number. AggSum is then 0 and AggCount the row count.
	ValueOK func(rowData any) (float64, bool)
}

// Aggregate computes aggregation values for a set of row indices. Each value
// is a float64, or nil for a column whose ValueOK reported no value for any
// row and whose function has no value over nothing (see AggColumn.ValueOK).
func Aggregate(cols []AggColumn, indices []int, source DataSource) map[string]any {
	result := make(map[string]any, len(cols))

	for _, col := range cols {
		values := make([]float64, 0, len(indices))
		rows := 0
		for _, idx := range indices {
			if idx < 0 {
				continue // skip group header sentinels
			}
			rows++
			row := source.Row(idx)
			if col.ValueOK != nil {
				if v, ok := col.ValueOK(row); ok {
					values = append(values, v)
				}
				continue
			}
			values = append(values, col.ValueGetter(row))
		}

		switch {
		case col.CustomFunc != nil:
			result[col.Field] = col.CustomFunc(values)
		case col.ValueOK == nil:
			result[col.Field] = computeAgg(col.Func, values)
		case col.Func == AggCount:
			result[col.Field] = float64(rows) // rows, blank ones included
		case len(values) == 0 && col.Func != AggSum:
			result[col.Field] = nil // no value to average, compare or pick
		default:
			result[col.Field] = computeAgg(col.Func, values)
		}
	}

	return result
}

func computeAgg(fn AggFunc, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	switch fn {
	case AggSum:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case AggAvg:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case AggMin:
		m := math.Inf(1)
		for _, v := range values {
			if v < m {
				m = v
			}
		}
		return m
	case AggMax:
		m := math.Inf(-1)
		for _, v := range values {
			if v > m {
				m = v
			}
		}
		return m
	case AggCount:
		return float64(len(values))
	case AggFirst:
		return values[0]
	case AggLast:
		return values[len(values)-1]
	}
	return 0
}
