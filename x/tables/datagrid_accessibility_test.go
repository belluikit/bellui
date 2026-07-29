package tables

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/core"

	"github.com/belluikit/bellui/x/tables/cell"
)

func TestDataGrid_AccessibleRole(t *testing.T) {
	g := newTestGrid(t, []map[string]any{{"v": 1}}, []*ColumnDef{{Field: "v"}})

	if got := g.AccessibleRole(); got != core.RoleTable {
		t.Errorf("AccessibleRole() = %v, want %v", got, core.RoleTable)
	}
}

func TestDataGrid_AccessibleLabel(t *testing.T) {
	g := newTestGrid(t, []map[string]any{{"v": 1}}, []*ColumnDef{{Field: "v"}})

	if got := g.AccessibleLabel(); got != "" {
		t.Errorf("default AccessibleLabel() = %q, want empty", got)
	}

	g.Options.AccessibleLabel = "Problems"
	if got := g.AccessibleLabel(); got != "Problems" {
		t.Errorf("AccessibleLabel() = %q, want %q", got, "Problems")
	}
}

// Without an explicit ValueFormatter, AccessibleCellText matches the visible
// cell's formatting (Date layout, Number format) instead of a raw fmt.Sprint,
// so a screen reader announces what is shown (#28).
func TestDataGrid_AccessibleCellText_MatchesRenderedFormat(t *testing.T) {
	ts := time.Date(2026, 7, 7, 13, 30, 0, 0, time.UTC)
	data := []map[string]any{{"d": ts, "n": 3.14159, "b": true}}
	g := newTestGrid(t, data, []*ColumnDef{
		{ColID: "d", Field: "d", Type: ColumnDate},
		{ColID: "n", Field: "n", CellRenderer: &cell.NumberRenderer{Format: "%.2f"}},
		{ColID: "b", Field: "b", Type: ColumnBoolean},
	})

	// Date: the rendered layout, not the raw "2026-07-07 13:30:00 +0000 UTC".
	if got, want := g.AccessibleCellText(0, 0), "2026-07-07"; got != want {
		t.Errorf("date AccessibleCellText = %q, want %q", got, want)
	}
	// Number with a format: the formatted value, not full precision.
	if got, want := g.AccessibleCellText(0, 1), "3.14"; got != want {
		t.Errorf("number AccessibleCellText = %q, want %q", got, want)
	}
	// Boolean renderer has no accessible text of its own: falls back to Sprint.
	if got, want := g.AccessibleCellText(0, 2), "true"; got != want {
		t.Errorf("boolean AccessibleCellText = %q, want %q", got, want)
	}
}

// An explicit ValueFormatter still wins over the renderer's own text.
func TestDataGrid_AccessibleCellText_ValueFormatterWins(t *testing.T) {
	data := []map[string]any{{"n": 1000}}
	g := newTestGrid(t, data, []*ColumnDef{
		{ColID: "n", Field: "n", Type: ColumnNumber,
			ValueFormatter: func(v any) string { return "custom" }},
	})
	if got := g.AccessibleCellText(0, 0); got != "custom" {
		t.Errorf("AccessibleCellText = %q, want %q (ValueFormatter)", got, "custom")
	}
}

func TestDataGrid_AccessibleGridDimensions(t *testing.T) {
	data := []map[string]any{{"n": "a"}, {"n": "b"}, {"n": "c"}}
	g := newTestGrid(t, data, []*ColumnDef{{ColID: "n", Field: "n"}, {ColID: "x", Field: "n"}})

	rows, cols := g.AccessibleGridDimensions()
	if rows != 3 || cols != 2 {
		t.Fatalf("dimensions = (%d,%d), want (3,2)", rows, cols)
	}

	// A filter narrows the presented extent — dimensions report the current
	// view, not the unfiltered source total (visual/current-view contract).
	g.API().SetColumnFilter("n", func(v any) bool { return v == "b" })
	if rows, _ := g.AccessibleGridDimensions(); rows != 1 {
		t.Errorf("filtered rows = %d, want 1 (current-view extent)", rows)
	}
}

func TestDataGrid_AccessibleCellText_DisplayCoordinates(t *testing.T) {
	data := []map[string]any{{"n": "charlie"}, {"n": "alice"}, {"n": "bob"}}
	g := newTestGrid(t, data, []*ColumnDef{{ColID: "n", Field: "n"}})

	// Unsorted: display row 0 is the first source row.
	if got := g.AccessibleCellText(0, 0); got != "charlie" {
		t.Fatalf("cell(0,0) = %q, want %q", got, "charlie")
	}

	// After an ascending sort the label at display row 0 follows the presented
	// order — it is a display coordinate, not a stable source index.
	g.API().SetSort(SortItem{ColID: "n", Direction: SortAscending})
	if got := g.AccessibleCellText(0, 0); got != "alice" {
		t.Errorf("sorted cell(0,0) = %q, want %q (display order)", got, "alice")
	}
	if got := g.AccessibleCellText(2, 0); got != "charlie" {
		t.Errorf("sorted cell(2,0) = %q, want %q (display order)", got, "charlie")
	}

	// Out-of-range cells return empty, not a panic.
	if got := g.AccessibleCellText(99, 0); got != "" {
		t.Errorf("out-of-range cell = %q, want empty", got)
	}
}
