package pipeline

import (
	"testing"
)

func TestPipeline_Identity(t *testing.T) {
	data := []any{"a", "b", "c", "d", "e"}
	src := NewSliceSource(data)
	p := New(src)

	if p.DisplayLength() != 5 {
		t.Errorf("expected 5, got %d", p.DisplayLength())
	}

	for i := 0; i < 5; i++ {
		if p.DisplayRow(i) != data[i] {
			t.Errorf("row %d: expected %v, got %v", i, data[i], p.DisplayRow(i))
		}
	}
}

func TestPipeline_SortTransform(t *testing.T) {
	data := []any{
		map[string]any{"name": "Charlie", "age": 30},
		map[string]any{"name": "Alice", "age": 25},
		map[string]any{"name": "Bob", "age": 35},
	}
	src := NewSliceSource(data)
	p := New(src)

	p.AddTransform(NewSortTransform(SortField{
		ValueGetter: func(row any) any { return row.(map[string]any)["name"] },
		Ascending:   true,
	}))

	if p.DisplayLength() != 3 {
		t.Fatalf("expected 3 rows, got %d", p.DisplayLength())
	}

	// Expect sorted: Alice, Bob, Charlie
	expected := []string{"Alice", "Bob", "Charlie"}
	for i, exp := range expected {
		row := p.DisplayRow(i).(map[string]any)
		if row["name"] != exp {
			t.Errorf("row %d: expected %s, got %v", i, exp, row["name"])
		}
	}
}

func TestPipeline_SortDescending(t *testing.T) {
	data := []any{
		map[string]any{"v": 1},
		map[string]any{"v": 3},
		map[string]any{"v": 2},
	}
	src := NewSliceSource(data)
	p := New(src)

	p.AddTransform(NewSortTransform(SortField{
		ValueGetter: func(row any) any { return row.(map[string]any)["v"] },
		Ascending:   false,
	}))

	expected := []int{3, 2, 1}
	for i, exp := range expected {
		row := p.DisplayRow(i).(map[string]any)
		if row["v"] != exp {
			t.Errorf("row %d: expected %d, got %v", i, exp, row["v"])
		}
	}
}

func TestPipeline_FilterTransform(t *testing.T) {
	data := []any{
		map[string]any{"name": "Alice", "age": 25},
		map[string]any{"name": "Bob", "age": 35},
		map[string]any{"name": "Charlie", "age": 30},
	}
	src := NewSliceSource(data)
	p := New(src)

	p.AddTransform(NewFilterTransform(ColumnFilter{
		ValueGetter: func(row any) any { return row.(map[string]any)["age"] },
		Matches:     func(v any) bool { return v.(int) >= 30 },
	}))

	if p.DisplayLength() != 2 {
		t.Fatalf("expected 2 rows, got %d", p.DisplayLength())
	}

	row0 := p.DisplayRow(0).(map[string]any)
	row1 := p.DisplayRow(1).(map[string]any)
	if row0["name"] != "Bob" {
		t.Errorf("expected Bob, got %v", row0["name"])
	}
	if row1["name"] != "Charlie" {
		t.Errorf("expected Charlie, got %v", row1["name"])
	}
}

func TestPipeline_FilterThenSort(t *testing.T) {
	data := []any{
		map[string]any{"name": "Charlie", "age": 30},
		map[string]any{"name": "Alice", "age": 25},
		map[string]any{"name": "Bob", "age": 35},
		map[string]any{"name": "Diana", "age": 20},
	}
	src := NewSliceSource(data)
	p := New(src)

	// Filter: age >= 25
	p.AddTransform(NewFilterTransform(ColumnFilter{
		ValueGetter: func(row any) any { return row.(map[string]any)["age"] },
		Matches:     func(v any) bool { return v.(int) >= 25 },
	}))

	// Sort: name ascending
	p.AddTransform(NewSortTransform(SortField{
		ValueGetter: func(row any) any { return row.(map[string]any)["name"] },
		Ascending:   true,
	}))

	if p.DisplayLength() != 3 {
		t.Fatalf("expected 3 rows, got %d", p.DisplayLength())
	}

	expected := []string{"Alice", "Bob", "Charlie"}
	for i, exp := range expected {
		row := p.DisplayRow(i).(map[string]any)
		if row["name"] != exp {
			t.Errorf("row %d: expected %s, got %v", i, exp, row["name"])
		}
	}
}

func TestPipeline_MarkDirty(t *testing.T) {
	data := []any{"a", "b"}
	src := NewSliceSource(data)
	p := New(src)

	_ = p.DisplayLength() // force compute
	src.SetData([]any{"x", "y", "z"})
	p.MarkDirty()

	if p.DisplayLength() != 3 {
		t.Errorf("expected 3 after dirty, got %d", p.DisplayLength())
	}
}

func TestRowIndex_SourceRow(t *testing.T) {
	idx := NewRowIndex([]int{5, 2, 8})
	if idx.Length() != 3 {
		t.Errorf("expected 3, got %d", idx.Length())
	}
	if idx.SourceRow(0) != 5 {
		t.Errorf("expected 5, got %d", idx.SourceRow(0))
	}
	if idx.SourceRow(1) != 2 {
		t.Errorf("expected 2, got %d", idx.SourceRow(1))
	}
	if idx.SourceRow(-1) != -1 {
		t.Errorf("expected -1 for out of bounds, got %d", idx.SourceRow(-1))
	}
	if idx.SourceRow(10) != -1 {
		t.Errorf("expected -1 for out of bounds, got %d", idx.SourceRow(10))
	}
}

func TestCallbackSource(t *testing.T) {
	items := []string{"a", "b", "c"}
	src := NewCallbackSource(
		func() int { return len(items) },
		func(i int) any { return items[i] },
	)
	if src.Length() != 3 {
		t.Errorf("expected 3, got %d", src.Length())
	}
	if src.Row(1) != "b" {
		t.Errorf("expected b, got %v", src.Row(1))
	}
}

func TestMultiSort(t *testing.T) {
	data := []any{
		map[string]any{"dept": "B", "name": "Zara"},
		map[string]any{"dept": "A", "name": "Bob"},
		map[string]any{"dept": "B", "name": "Alice"},
		map[string]any{"dept": "A", "name": "Alice"},
	}
	src := NewSliceSource(data)
	p := New(src)

	p.AddTransform(NewSortTransform(
		SortField{
			ValueGetter: func(row any) any { return row.(map[string]any)["dept"] },
			Ascending:   true,
		},
		SortField{
			ValueGetter: func(row any) any { return row.(map[string]any)["name"] },
			Ascending:   true,
		},
	))

	expected := []string{"A-Alice", "A-Bob", "B-Alice", "B-Zara"}
	for i, exp := range expected {
		row := p.DisplayRow(i).(map[string]any)
		got := row["dept"].(string) + "-" + row["name"].(string)
		if got != exp {
			t.Errorf("row %d: expected %s, got %s", i, exp, got)
		}
	}
}
