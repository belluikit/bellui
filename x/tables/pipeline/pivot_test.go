package pipeline

import "testing"

func TestPivotTransform_Basic(t *testing.T) {
	data := []any{
		map[string]any{"region": "North", "product": "A", "revenue": 100.0},
		map[string]any{"region": "North", "product": "B", "revenue": 200.0},
		map[string]any{"region": "South", "product": "A", "revenue": 150.0},
		map[string]any{"region": "South", "product": "B", "revenue": 250.0},
		map[string]any{"region": "North", "product": "A", "revenue": 50.0},
	}
	src := NewSliceSource(data)

	pt := NewPivotTransform(PivotConfig{
		RowGroupField:  "region",
		RowGroupGetter: func(row any) any { return row.(map[string]any)["region"] },
		PivotField:     "product",
		PivotGetter:    func(row any) any { return row.(map[string]any)["product"] },
		ValueField:     "revenue",
		ValueGetter:    func(row any) float64 { return row.(map[string]any)["revenue"].(float64) },
		AggFunc:        AggSum,
	})

	result := pt.Apply([]int{0, 1, 2, 3, 4}, src)

	if pt.Result == nil {
		t.Fatal("expected PivotResult")
	}

	// Should have 2 pivot columns: A_revenue, B_revenue
	if len(pt.Result.PivotColumns) != 2 {
		t.Fatalf("expected 2 pivot columns, got %d: %v", len(pt.Result.PivotColumns), pt.Result.PivotColumns)
	}

	// Should have 2 rows: North, South
	if len(pt.Result.Rows) != 2 {
		t.Fatalf("expected 2 pivoted rows, got %d", len(pt.Result.Rows))
	}

	// Result indices should be 0, 1 (into pivot result)
	if len(result) != 2 || result[0] != 0 || result[1] != 1 {
		t.Errorf("expected result [0,1], got %v", result)
	}

	// North: A=100+50=150, B=200
	north := pt.Result.Rows[0]
	if north["region"] != "North" {
		t.Errorf("expected North, got %v", north["region"])
	}
	if north["A_revenue"] != 150.0 {
		t.Errorf("expected North A_revenue=150, got %v", north["A_revenue"])
	}
	if north["B_revenue"] != 200.0 {
		t.Errorf("expected North B_revenue=200, got %v", north["B_revenue"])
	}

	// South: A=150, B=250
	south := pt.Result.Rows[1]
	if south["A_revenue"] != 150.0 {
		t.Errorf("expected South A_revenue=150, got %v", south["A_revenue"])
	}
	if south["B_revenue"] != 250.0 {
		t.Errorf("expected South B_revenue=250, got %v", south["B_revenue"])
	}
}

func TestPivotTransform_Avg(t *testing.T) {
	data := []any{
		map[string]any{"g": "X", "p": "A", "v": 10.0},
		map[string]any{"g": "X", "p": "A", "v": 20.0},
	}
	src := NewSliceSource(data)

	pt := NewPivotTransform(PivotConfig{
		RowGroupField:  "g",
		RowGroupGetter: func(row any) any { return row.(map[string]any)["g"] },
		PivotField:     "p",
		PivotGetter:    func(row any) any { return row.(map[string]any)["p"] },
		ValueField:     "v",
		ValueGetter:    func(row any) float64 { return row.(map[string]any)["v"].(float64) },
		AggFunc:        AggAvg,
	})

	pt.Apply([]int{0, 1}, src)

	if pt.Result.Rows[0]["A_v"] != 15.0 {
		t.Errorf("expected avg 15, got %v", pt.Result.Rows[0]["A_v"])
	}
}

func TestPivotTransform_Empty(t *testing.T) {
	src := NewSliceSource(nil)
	pt := NewPivotTransform(PivotConfig{
		RowGroupField:  "g",
		RowGroupGetter: func(row any) any { return nil },
		PivotField:     "p",
		PivotGetter:    func(row any) any { return nil },
		ValueField:     "v",
		ValueGetter:    func(row any) float64 { return 0 },
		AggFunc:        AggSum,
	})

	result := pt.Apply(nil, src)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestPivotDataSource(t *testing.T) {
	result := &PivotResult{
		Rows: []map[string]any{
			{"x": 1},
			{"x": 2},
		},
	}
	ds := &PivotDataSource{Result: result}

	if ds.Length() != 2 {
		t.Errorf("expected 2, got %d", ds.Length())
	}
	if ds.Row(0).(map[string]any)["x"] != 1 {
		t.Error("expected row 0 x=1")
	}
}
