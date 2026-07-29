package tables

import (
	"testing"

	"github.com/belluikit/bellui/test"
)

type nestedTestAddress struct {
	City string
}

type nestedTestRow struct {
	Name    string
	Address nestedTestAddress
	Contact *nestedTestAddress
	Tags    map[string]string
	Scores  map[string]int
}

// ColumnDef.Field has documented dot notation for nested fields since the
// initial import, but defaultFieldAccess only ever resolved a single segment —
// a dotted field silently rendered blank cells and no-op sort/filter (#84).
func TestDefaultFieldAccess_DotNotation(t *testing.T) {
	row := nestedTestRow{
		Name:    "Ann",
		Address: nestedTestAddress{City: "Oslo"},
		Contact: &nestedTestAddress{City: "Bergen"},
		Tags:    map[string]string{"tier": "gold"},
		Scores:  map[string]int{"math": 7},
	}

	cases := []struct {
		field string
		row   any
		want  any
	}{
		{"Address.City", row, "Oslo"},
		{"Contact.City", row, "Bergen"}, // pointer segment dereferenced
		{"Tags.tier", row, "gold"},
		{"Scores.math", row, 7}, // string-keyed map via reflection
		{"Address.City", &row, "Oslo"},
		{"address.city", map[string]any{"address": map[string]any{"city": "Paris"}}, "Paris"},
		{"a.b.c", map[string]any{"a": map[string]any{"b": map[string]string{"c": "deep"}}}, "deep"},
		{"outer.Name", map[string]any{"outer": nestedTestRow{Name: "Bo"}}, "Bo"},
	}
	for _, c := range cases {
		if got := defaultFieldAccess(c.row, c.field); got != c.want {
			t.Errorf("defaultFieldAccess(%T, %q) = %v, want %v", c.row, c.field, got, c.want)
		}
	}
}

// A map row holding the whole dotted field as a literal key keeps resolving to
// that entry — that was the only meaning a dotted Field ever had for map rows.
func TestDefaultFieldAccess_LiteralDottedKeyWins(t *testing.T) {
	row := map[string]any{
		"address.city": "Literal",
		"address":      map[string]any{"city": "Nested"},
	}
	if got := defaultFieldAccess(row, "address.city"); got != "Literal" {
		t.Fatalf("literal dotted key = %v, want Literal", got)
	}
	if got := defaultFieldAccess(map[string]string{"a.b": "lit"}, "a.b"); got != "lit" {
		t.Fatalf("literal dotted key (string map) = %v, want lit", got)
	}
}

// Any segment that fails to resolve yields nil (blank cell), never a panic.
func TestDefaultFieldAccess_DotNotationEdges(t *testing.T) {
	row := nestedTestRow{Name: "Ann"}
	cases := []struct {
		name  string
		row   any
		field string
	}{
		{"missing intermediate map key", map[string]any{}, "address.city"},
		{"nil pointer segment", row, "Contact.City"},
		{"segment into scalar", row, "Name.length"},
		{"missing struct field", row, "Address.Zip"},
		{"nil map segment", row, "Scores.math"},
		{"segment into string map value", nestedTestRow{Tags: map[string]string{"a": "x"}}, "Tags.a.b"},
		{"non-string-keyed map", map[string]any{"m": map[int]string{1: "x"}}, "m.1"},
	}
	for _, c := range cases {
		if got := defaultFieldAccess(c.row, c.field); got != nil {
			t.Errorf("%s: defaultFieldAccess(%q) = %v, want nil", c.name, c.field, got)
		}
	}

	// map[string]string keeps its single-segment semantics at the leaf: a
	// missing key yields the zero string, which still displays as a blank cell.
	tagged := nestedTestRow{Tags: map[string]string{"a": "x"}}
	if got := defaultFieldAccess(tagged, "Tags.missing"); got != "" {
		t.Errorf("missing string-map leaf = %v, want empty string", got)
	}
}

// End-to-end: a dotted Field drives display, sort, and quick filter.
func TestDataGrid_NestedFieldColumn(t *testing.T) {
	rows := []nestedTestRow{
		{Name: "Charlie", Address: nestedTestAddress{City: "Zurich"}, Scores: map[string]int{"math": 3}},
		{Name: "Alice", Address: nestedTestAddress{City: "Amsterdam"}, Scores: map[string]int{"math": 9}},
		{Name: "Bob", Address: nestedTestAddress{City: "Madrid"}, Scores: map[string]int{"math": 5}},
	}
	test.NewTempApp(t)
	g := NewDataGrid(
		[]*ColumnDef{
			{Field: "Name"},
			{Field: "Address.City", HeaderName: "City"},
			{Field: "Scores.math", HeaderName: "Math", Type: ColumnNumber},
		},
		func() int { return len(rows) },
		func(row int) any { return rows[row] },
	)

	if got := g.AccessibleCellText(0, 1); got != "Zurich" {
		t.Fatalf("cell(0,1) = %q, want Zurich", got)
	}

	g.API().SetSort(SortItem{ColID: "Address.City", Direction: SortAscending})
	wantSrc := []int{1, 2, 0} // Amsterdam, Madrid, Zurich
	for disp, src := range wantSrc {
		if got := g.SourceRow(disp); got != src {
			t.Errorf("after city sort, SourceRow(%d) = %d, want %d", disp, got, src)
		}
	}

	g.API().SetSort(SortItem{ColID: "Scores.math", Direction: SortDescending})
	wantSrc = []int{1, 2, 0} // 9, 5, 3
	for disp, src := range wantSrc {
		if got := g.SourceRow(disp); got != src {
			t.Errorf("after math sort, SourceRow(%d) = %d, want %d", disp, got, src)
		}
	}

	g.API().ClearSort()
	g.API().SetQuickFilter("madrid")
	if got := g.RowCount(); got != 1 {
		t.Fatalf("quick filter on nested field kept %d rows, want 1", got)
	}
	if got := g.SourceRow(0); got != 2 {
		t.Fatalf("filtered view shows source %d, want 2 (Bob)", got)
	}
}
