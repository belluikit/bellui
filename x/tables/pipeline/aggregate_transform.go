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
	Field       string
	ValueGetter func(rowData any) float64
	Func        AggFunc
	CustomFunc  func(values []float64) float64
}

// Aggregate computes aggregation values for a set of row indices.
func Aggregate(cols []AggColumn, indices []int, source DataSource) map[string]any {
	result := make(map[string]any, len(cols))

	for _, col := range cols {
		values := make([]float64, 0, len(indices))
		for _, idx := range indices {
			if idx < 0 {
				continue // skip group header sentinels
			}
			row := source.Row(idx)
			values = append(values, col.ValueGetter(row))
		}

		var aggVal float64
		if col.CustomFunc != nil {
			aggVal = col.CustomFunc(values)
		} else {
			aggVal = computeAgg(col.Func, values)
		}
		result[col.Field] = aggVal
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
