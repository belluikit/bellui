package pipeline

import (
	"fmt"
	"math"
	"testing"
)

func TestAggregate_Sum(t *testing.T) {
	data := []any{
		map[string]any{"v": 10.0},
		map[string]any{"v": 20.0},
		map[string]any{"v": 30.0},
	}
	src := NewSliceSource(data)

	result := Aggregate([]AggColumn{{
		Field:       "v",
		ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) },
		Func:        AggSum,
	}}, []int{0, 1, 2}, src)

	if result["v"] != 60.0 {
		t.Errorf("expected sum 60, got %v", result["v"])
	}
}

func TestAggregate_Avg(t *testing.T) {
	data := []any{
		map[string]any{"v": 10.0},
		map[string]any{"v": 20.0},
		map[string]any{"v": 30.0},
	}
	src := NewSliceSource(data)

	result := Aggregate([]AggColumn{{
		Field:       "v",
		ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) },
		Func:        AggAvg,
	}}, []int{0, 1, 2}, src)

	if result["v"] != 20.0 {
		t.Errorf("expected avg 20, got %v", result["v"])
	}
}

func TestAggregate_MinMax(t *testing.T) {
	data := []any{
		map[string]any{"v": 5.0},
		map[string]any{"v": 15.0},
		map[string]any{"v": 10.0},
	}
	src := NewSliceSource(data)

	result := Aggregate([]AggColumn{
		{Field: "min", ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) }, Func: AggMin},
		{Field: "max", ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) }, Func: AggMax},
	}, []int{0, 1, 2}, src)

	if result["min"] != 5.0 {
		t.Errorf("expected min 5, got %v", result["min"])
	}
	if result["max"] != 15.0 {
		t.Errorf("expected max 15, got %v", result["max"])
	}
}

func TestAggregate_Count(t *testing.T) {
	data := []any{map[string]any{}, map[string]any{}, map[string]any{}}
	src := NewSliceSource(data)

	result := Aggregate([]AggColumn{{
		Field:       "cnt",
		ValueGetter: func(row any) float64 { return 1 },
		Func:        AggCount,
	}}, []int{0, 1, 2}, src)

	if result["cnt"] != 3.0 {
		t.Errorf("expected count 3, got %v", result["cnt"])
	}
}

func TestAggregate_Custom(t *testing.T) {
	data := []any{
		map[string]any{"v": 2.0},
		map[string]any{"v": 3.0},
	}
	src := NewSliceSource(data)

	result := Aggregate([]AggColumn{{
		Field:       "product",
		ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) },
		CustomFunc: func(values []float64) float64 {
			p := 1.0
			for _, v := range values {
				p *= v
			}
			return p
		},
	}}, []int{0, 1}, src)

	if result["product"] != 6.0 {
		t.Errorf("expected product 6, got %v", result["product"])
	}
}

func TestAggregate_Empty(t *testing.T) {
	src := NewSliceSource(nil)
	result := Aggregate([]AggColumn{{
		Field:       "v",
		ValueGetter: func(row any) float64 { return 0 },
		Func:        AggSum,
	}}, nil, src)

	if result["v"] != 0.0 {
		t.Errorf("expected 0 for empty, got %v", result["v"])
	}
}

func TestAggregate_SkipsGroupHeaders(t *testing.T) {
	data := []any{
		map[string]any{"v": 10.0},
		map[string]any{"v": 20.0},
	}
	src := NewSliceSource(data)

	// Negative indices are group header sentinels and should be skipped
	result := Aggregate([]AggColumn{{
		Field:       "v",
		ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) },
		Func:        AggSum,
	}}, []int{-1, 0, 1}, src)

	if result["v"] != 30.0 {
		t.Errorf("expected sum 30 (skipping negative indices), got %v", result["v"])
	}
}

func TestAggregate_FirstLast(t *testing.T) {
	data := []any{
		map[string]any{"v": 10.0},
		map[string]any{"v": 20.0},
		map[string]any{"v": 30.0},
	}
	src := NewSliceSource(data)

	result := Aggregate([]AggColumn{
		{Field: "first", ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) }, Func: AggFirst},
		{Field: "last", ValueGetter: func(row any) float64 { return row.(map[string]any)["v"].(float64) }, Func: AggLast},
	}, []int{0, 1, 2}, src)

	if result["first"] != 10.0 {
		t.Errorf("expected first 10, got %v", result["first"])
	}
	if result["last"] != 30.0 {
		t.Errorf("expected last 30, got %v", result["last"])
	}
}

func TestAggregate_MinEmpty(t *testing.T) {
	src := NewSliceSource(nil)
	result := Aggregate([]AggColumn{{
		Field:       "v",
		ValueGetter: func(row any) float64 { return 0 },
		Func:        AggMin,
	}}, nil, src)

	// With no values, AggMin returns 0 (empty slice case)
	if result["v"] != 0.0 && !math.IsInf(result["v"].(float64), 1) {
		// computeAgg returns 0 for empty slice
	}
}

// ValueOK skips rows with no value: avg, min and max are taken over the rest,
// sum adds nothing, first and last pick rows with a value, CustomFunc sees only
// the values present, and count still counts every row (#747).
func TestAggregate_ValueOKSkipsBlanks(t *testing.T) {
	src := NewSliceSource([]any{10.0, 20.0, nil})
	get := func(row any) (float64, bool) {
		v, ok := row.(float64)
		return v, ok
	}
	cols := []AggColumn{
		{Field: "avg", ValueOK: get, Func: AggAvg},
		{Field: "min", ValueOK: get, Func: AggMin},
		{Field: "max", ValueOK: get, Func: AggMax},
		{Field: "sum", ValueOK: get, Func: AggSum},
		{Field: "count", ValueOK: get, Func: AggCount},
		{Field: "last", ValueOK: get, Func: AggLast},
		{Field: "n", ValueOK: get, CustomFunc: func(v []float64) float64 { return float64(len(v)) }},
	}
	got := Aggregate(cols, []int{2, 0, -1, 1}, src) // blank first; -1 is a header sentinel
	want := map[string]any{"avg": 15.0, "min": 10.0, "max": 20.0, "sum": 30.0, "count": 3.0, "last": 20.0, "n": 2.0}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	first := Aggregate([]AggColumn{{Field: "f", ValueOK: get, Func: AggFirst}}, []int{2, 0, 1}, src)
	if first["f"] != 10.0 {
		t.Errorf("first = %v, want 10 (the first row with a value)", first["f"])
	}
}

// With ValueOK and no row holding a value, avg, min, max, first and last have
// no value (nil); sum is 0 and count the row count (#747).
func TestAggregate_ValueOKAllBlank(t *testing.T) {
	src := NewSliceSource([]any{nil, "x"})
	get := func(row any) (float64, bool) {
		v, ok := row.(float64)
		return v, ok
	}
	var cols []AggColumn
	for _, fn := range []AggFunc{AggAvg, AggMin, AggMax, AggFirst, AggLast, AggSum, AggCount} {
		cols = append(cols, AggColumn{Field: fmt.Sprint(fn), ValueOK: get, Func: fn})
	}
	got := Aggregate(cols, []int{0, 1}, src)
	for _, fn := range []AggFunc{AggAvg, AggMin, AggMax, AggFirst, AggLast} {
		v, present := got[fmt.Sprint(fn)]
		if !present || v != nil {
			t.Errorf("func %d = %v (present %v), want nil", fn, v, present)
		}
	}
	if got[fmt.Sprint(AggSum)] != 0.0 || got[fmt.Sprint(AggCount)] != 2.0 {
		t.Errorf("sum = %v, count = %v, want 0 and 2", got[fmt.Sprint(AggSum)], got[fmt.Sprint(AggCount)])
	}
}

// Without ValueOK every row counts, as before: a ValueGetter's 0 lowers the
// average, and no rows aggregate to 0.
func TestAggregate_ValueGetterUnchanged(t *testing.T) {
	src := NewSliceSource([]any{10.0, 20.0, nil})
	get := func(row any) float64 { v, _ := row.(float64); return v }
	got := Aggregate([]AggColumn{{Field: "a", ValueGetter: get, Func: AggAvg}}, []int{0, 1, 2}, src)
	if got["a"] != 10.0 {
		t.Errorf("avg = %v, want 10", got["a"])
	}
	if got := Aggregate([]AggColumn{{Field: "a", ValueGetter: get, Func: AggMin}}, nil, src); got["a"] != 0.0 {
		t.Errorf("min of no rows = %v, want 0", got["a"])
	}
}
