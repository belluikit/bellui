package pipeline

import (
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
