package tables

import (
	"fmt"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/tables/pipeline"
)

// A column's AggFunc reaches the grid's group header rows: each header draws
// its aggregates in their columns (through the column's formatter), a collapsed
// group still summarises all of its rows, and the header's accessible label
// carries them (#689).
func TestDataGrid_GroupHeaderAggregates(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "dept": "A", "salary": 100, "score": 1.0},
		{"id": 2, "dept": "B", "salary": 50, "score": 2.0},
		{"id": 3, "dept": "A", "salary": 300, "score": 2.0},
		{"id": 4, "dept": "B", "salary": 70, "score": 4.0},
		{"id": 5, "dept": "B", "salary": 90, "score": 4.0},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Width: 60, Pinned: PinLeft, AggFunc: bellui.Ptr(pipeline.AggCount)},
		{Field: "dept"},
		{
			Field: "salary", HeaderName: "Salary", AggFunc: bellui.Ptr(pipeline.AggSum),
			ValueFormatter: func(v any) string { return fmt.Sprintf("$%v", v) },
		},
		{Field: "score", HeaderName: "Score", Width: 80, Pinned: PinRight, AggFunc: bellui.Ptr(pipeline.AggAvg)},
	})
	g.API().SetRowGroupColumn("dept")
	g.API().ToggleGroup("B") // collapsed: only the header is displayed

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(700, 400))

	texts := map[string]int{}
	labels := map[string]bool{}
	walkCanvasObjects(g.groupRowOverlay, func(o core.CanvasObject) {
		if txt, ok := o.(*canvas.Text); ok && txt.Visible() {
			texts[txt.Text]++
		}
		if a, ok := o.(core.Accessible); ok && a.AccessibleRole() == core.RoleTableRow {
			labels[a.AccessibleLabel()] = true
		}
	})

	// A: count 2, sum $400, avg 1.5. B (collapsed): count 3, sum $210, avg 3.33.
	for _, want := range []string{"2", "$400", "1.5", "3", "$210", "3.33"} {
		if texts[want] != 1 {
			t.Errorf("aggregate %q drawn %d times, want once (texts: %v)", want, texts[want], texts)
		}
	}
	for _, want := range []string{
		"A (2), id count 2, Salary sum $400, Score average 1.5",
		"B (3), id count 3, Salary sum $210, Score average 3.33",
	} {
		if !labels[want] {
			t.Errorf("no group header labelled %q; got %v", want, labels)
		}
	}
}

// Aggregates follow the column definitions: a grid without AggFunc columns
// keeps the plain "key (count)" header and label.
func TestDataGrid_GroupHeaderNoAggregates(t *testing.T) {
	data := []map[string]any{{"dept": "A", "salary": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "dept"}, {Field: "salary"}})
	g.API().SetRowGroupColumn("dept")
	if aggs := g.GroupRowAggregates(0); aggs != nil {
		t.Errorf("GroupRowAggregates = %v, want nil", aggs)
	}
	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(400, 200))
	walkCanvasObjects(g.groupRowOverlay, func(o core.CanvasObject) {
		if a, ok := o.(core.Accessible); ok && a.AccessibleRole() == core.RoleTableRow {
			if l := a.AccessibleLabel(); l != "A (1)" || strings.Contains(l, ",") {
				t.Errorf("label = %q, want %q", l, "A (1)")
			}
		}
	})
}

func TestFormatAggregate(t *testing.T) {
	col := func(fn pipeline.AggFunc, def ColumnDef) *resolvedColumn {
		def.AggFunc = bellui.Ptr(fn)
		return &resolvedColumn{def: &def}
	}
	cases := []struct {
		rc   *resolvedColumn
		v    float64
		want string
	}{
		{col(pipeline.AggSum, ColumnDef{}), 1000000, "1000000"},
		{col(pipeline.AggAvg, ColumnDef{}), 2.0 / 3.0, "0.67"},
		{col(pipeline.AggCount, ColumnDef{ValueFormatter: func(any) string { return "x" }}), 3, "3"},
		{col(pipeline.AggMax, ColumnDef{ValueFormatter: func(v any) string { return fmt.Sprintf("<%v>", v) }}), 2.5, "<2.5>"},
	}
	for _, c := range cases {
		if got := formatAggregate(c.rc, c.v); got != c.want {
			t.Errorf("formatAggregate(%v, %v) = %q, want %q", *c.rc.def.AggFunc, c.v, got, c.want)
		}
	}
}

func TestAggNumber(t *testing.T) {
	type cents int64
	for _, c := range []struct {
		v    any
		want float64
	}{
		{3, 3},
		{int8(-2), -2},
		{uint16(7), 7},
		{float32(1.5), 1.5},
		{cents(250), 250},
	} {
		if got, ok := aggNumber(c.v); got != c.want || !ok {
			t.Errorf("aggNumber(%#v) = %v, %v, want %v, true", c.v, got, ok, c.want)
		}
	}
	for _, v := range []any{"12", "", nil, true} {
		if _, ok := aggNumber(v); ok {
			t.Errorf("aggNumber(%#v) reports a value, want blank", v)
		}
	}
}

// Blank and non-numeric cells are skipped: 10, 20 and a blank average 15 with
// min 10 and max 20, while count counts all 3 rows; a group whose cells are all
// blank leaves avg/min/max empty and sums to 0 (#747).
func TestDataGrid_GroupAggregatesSkipBlanks(t *testing.T) {
	data := []map[string]any{
		{"dept": "A", "v": 10, "a": 10, "lo": 10, "hi": 10, "s": 10},
		{"dept": "A", "v": 20, "a": 20, "lo": 20, "hi": 20, "s": 20},
		{"dept": "A", "v": nil, "a": "", "lo": nil, "hi": "n/a", "s": nil},
		{"dept": "B", "v": nil, "a": nil, "lo": "", "hi": nil, "s": nil},
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "dept"},
		{Field: "v", AggFunc: bellui.Ptr(pipeline.AggCount)},
		{Field: "a", AggFunc: bellui.Ptr(pipeline.AggAvg)},
		{Field: "lo", AggFunc: bellui.Ptr(pipeline.AggMin)},
		{Field: "hi", AggFunc: bellui.Ptr(pipeline.AggMax)},
		{Field: "s", AggFunc: bellui.Ptr(pipeline.AggSum)},
	})
	g.API().SetRowGroupColumn("dept")
	texts := func(row int) map[int]string {
		out := map[int]string{}
		for _, a := range g.GroupRowAggregates(row) {
			out[a.Col] = a.Text
		}
		return out
	}
	// Display columns: dept 0, v 1, a 2, lo 3, hi 4, s 5. Row 0 is A's header,
	// row 4 B's (A's three data rows lie between).
	if got, want := texts(0), map[int]string{1: "3", 2: "15", 3: "10", 4: "20", 5: "30"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("group A aggregates = %v, want %v", got, want)
	}
	if got, want := texts(4), map[int]string{1: "1", 5: "0"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("all-blank group B aggregates = %v, want %v", got, want)
	}
}
