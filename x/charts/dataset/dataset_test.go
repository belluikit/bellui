package dataset

import "testing"

func TestDataset_AppendAndQuery(t *testing.T) {
	ds := New(
		Dimension{Name: "x", Type: DimFloat},
		Dimension{Name: "y", Type: DimFloat},
		Dimension{Name: "label", Type: DimString},
	)

	if err := ds.AppendRow(1.0, 10.0, "a"); err != nil {
		t.Fatal(err)
	}
	if err := ds.AppendRow(2.0, 20.0, "b"); err != nil {
		t.Fatal(err)
	}

	if ds.RowCount() != 2 {
		t.Errorf("RowCount = %d, want 2", ds.RowCount())
	}

	xs := ds.FloatColumn(0)
	if len(xs) != 2 || xs[0] != 1 || xs[1] != 2 {
		t.Errorf("FloatColumn(0) = %v, want [1, 2]", xs)
	}

	labels := ds.StringColumn(2)
	if len(labels) != 2 || labels[0] != "a" || labels[1] != "b" {
		t.Errorf("StringColumn(2) = %v, want [a, b]", labels)
	}
}

func TestDataset_WrongType(t *testing.T) {
	ds := New(Dimension{Name: "x", Type: DimFloat})
	if err := ds.AppendRow("not a number"); err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestFromFloat64(t *testing.T) {
	ds := FromFloat64([][]float64{
		{1, 10},
		{2, 20},
		{3, 30},
	})
	if ds.RowCount() != 3 {
		t.Errorf("RowCount = %d, want 3", ds.RowCount())
	}
	if len(ds.Dimensions) != 2 {
		t.Errorf("Dimensions = %d, want 2", len(ds.Dimensions))
	}
	ys := ds.FloatColumn(1)
	if ys[2] != 30 {
		t.Errorf("FloatColumn(1)[2] = %v, want 30", ys[2])
	}
}

func TestExtractXY(t *testing.T) {
	ds := FromFloat64([][]float64{{1, 10}, {2, 20}})
	xs, ys := ExtractXY(ds, DefaultEncode())
	if len(xs) != 2 || xs[0] != 1 {
		t.Errorf("xs = %v", xs)
	}
	if len(ys) != 2 || ys[1] != 20 {
		t.Errorf("ys = %v", ys)
	}
}

func TestFilterTransform(t *testing.T) {
	ds := FromFloat64([][]float64{{1, 10}, {2, 20}, {3, 30}})
	f := &FilterTransform{
		Dimension: 0,
		Predicate: func(v float64) bool { return v >= 2 },
	}
	result, err := f.Apply(ds)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount() != 2 {
		t.Errorf("filtered RowCount = %d, want 2", result.RowCount())
	}
	xs := result.FloatColumn(0)
	if xs[0] != 2 || xs[1] != 3 {
		t.Errorf("filtered xs = %v, want [2, 3]", xs)
	}
}

func TestSortTransform(t *testing.T) {
	ds := FromFloat64([][]float64{{3, 30}, {1, 10}, {2, 20}})
	s := &SortTransform{Dimension: 0}
	result, err := s.Apply(ds)
	if err != nil {
		t.Fatal(err)
	}
	xs := result.FloatColumn(0)
	if xs[0] != 1 || xs[1] != 2 || xs[2] != 3 {
		t.Errorf("sorted xs = %v, want [1, 2, 3]", xs)
	}
	ys := result.FloatColumn(1)
	if ys[0] != 10 || ys[1] != 20 || ys[2] != 30 {
		t.Errorf("sorted ys = %v, want [10, 20, 30]", ys)
	}
}

func TestPipeline(t *testing.T) {
	ds := FromFloat64([][]float64{{3, 30}, {1, 10}, {2, 20}, {4, 40}})
	p := &Pipeline{Steps: []Transform{
		&FilterTransform{Dimension: 0, Predicate: func(v float64) bool { return v <= 3 }},
		&SortTransform{Dimension: 0},
	}}
	result, err := p.Apply(ds)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount() != 3 {
		t.Errorf("pipeline RowCount = %d, want 3", result.RowCount())
	}
	xs := result.FloatColumn(0)
	if xs[0] != 1 || xs[2] != 3 {
		t.Errorf("pipeline xs = %v, want [1, 2, 3]", xs)
	}
}
