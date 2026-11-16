package tables

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// TestSortingOrderIsTheTapsCycle: a column's SortingOrder is what header
// taps move it through. Asc and Desc alone never clear the sort, as a list
// that always comes in that column's order wants (DaySong's History, whose
// sort arrow showed a third, empty state); the order may start descending;
// DefaultColumnDef's applies to columns without their own; with none set, the
// cycle is ascending, descending, none, as before. A column sorted from the
// start (API().SetSort) moves on from there, and Shift follows the order too.
func TestSortingOrderIsTheTapsCycle(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 1, "c": 1}, {"a": 2, "b": 2, "c": 2}}
	grid := func(t *testing.T, def *DefaultColumnDef, cols ...*ColumnDef) (*DataGrid, func(x float32, shift bool)) {
		t.Helper()
		g := newTestGrid(t, data, cols)
		g.DefaultColDef = def
		w := test.NewWindow(g)
		t.Cleanup(w.Close)
		w.Resize(geo.NewSize(400, 300))
		y := g.HeaderHeight() / 2
		tap := func(x float32, shift bool) {
			mod := event.KeyModifier(0)
			if shift {
				mod = event.KeyModifierShift
			}
			g.headerWidget.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, y)}, Modifier: mod})
			g.headerWidget.Tapped(&event.PointEvent{Position: geo.NewPos(x, y)})
		}
		return g, tap
	}
	sorts := func(g *DataGrid) string { return fmt.Sprint(g.API().GetSort()) }
	a := func(d SortDirection) string { return fmt.Sprint([]SortItem{{ColID: "a", Direction: d}}) }
	const none = "[]"
	twoWay := []SortDirection{SortAscending, SortDescending}
	cases := []struct {
		name  string
		def   *DefaultColumnDef
		cols  []*ColumnDef
		start []SortItem
		shift bool
		want  []string // after each tap on column a
	}{
		{name: "the default: ascending, descending, none", cols: []*ColumnDef{{Field: "a", Width: 100}},
			want: []string{a(SortAscending), a(SortDescending), none, a(SortAscending)}},
		{name: "ascending and descending only: never none", cols: []*ColumnDef{{Field: "a", Width: 100, SortingOrder: twoWay}},
			want: []string{a(SortAscending), a(SortDescending), a(SortAscending), a(SortDescending)}},
		{name: "descending first", cols: []*ColumnDef{{Field: "a", Width: 100, SortingOrder: []SortDirection{SortDescending, SortAscending}}},
			want: []string{a(SortDescending), a(SortAscending), a(SortDescending)}},
		{name: "DefaultColumnDef's order", def: &DefaultColumnDef{Sortable: true, SortingOrder: twoWay}, cols: []*ColumnDef{{Field: "a", Width: 100}},
			want: []string{a(SortAscending), a(SortDescending), a(SortAscending)}},
		{name: "the column's own before DefaultColumnDef's", def: &DefaultColumnDef{Sortable: true, SortingOrder: twoWay},
			cols: []*ColumnDef{{Field: "a", Width: 100, SortingOrder: []SortDirection{SortDescending, SortNone}}},
			want: []string{a(SortDescending), none, a(SortDescending)}},
		{name: "sorted from the start: the first tap moves on", cols: []*ColumnDef{{Field: "a", Width: 100, SortingOrder: twoWay}},
			start: []SortItem{{ColID: "a", Direction: SortAscending}}, want: []string{a(SortDescending), a(SortAscending)}},
		{name: "Shift follows the order too", cols: []*ColumnDef{{Field: "a", Width: 100, SortingOrder: twoWay}}, shift: true,
			want: []string{a(SortAscending), a(SortDescending), a(SortAscending)}},
	}
	ran, taps := 0, 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ran++
			g, tap := grid(t, c.def, c.cols...)
			if c.start != nil {
				g.API().SetSort(c.start...)
			}
			for i, w := range c.want {
				taps++
				tap(10, c.shift)
				if got := sorts(g); got != w {
					t.Fatalf("tap %d: sort %s, want %s", i+1, got, w)
				}
			}
		})
	}
	if ran != 7 || taps != 22 {
		t.Fatalf("ran %d cases, %d taps; want 7, 22", ran, taps)
	}
}
