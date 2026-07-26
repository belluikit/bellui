package tables

// Issue #15: with pinned columns, the a11y column space is absolute display
// order (pinned-left + main + pinned-right), so dimensions report the total and
// AccessibleCellText resolves by that absolute index.

import "testing"

func pinnedGrid(t *testing.T) *DataGrid {
	t.Helper()
	data := []map[string]any{{"a": "AA", "b": "BB", "c": "CC"}}
	return newTestGrid(t, data, []*ColumnDef{
		{ColID: "a", Field: "a", Pinned: PinLeft},
		{ColID: "b", Field: "b"},
		{ColID: "c", Field: "c", Pinned: PinRight},
	})
}

// AccessibleGridDimensions reports every visible column, including pinned ones —
// not just the main-column count.
func TestDataGrid_AccessibleGridDimensions_CountsPinned(t *testing.T) {
	g := pinnedGrid(t)
	_, cols := g.AccessibleGridDimensions()
	if cols != 3 {
		t.Errorf("cols = %d, want 3 (pinned-left + main + pinned-right)", cols)
	}
}

// AccessibleCellText resolves the column by its absolute display index: 0 is the
// pinned-left column, 1 the main column, 2 the pinned-right column.
func TestDataGrid_AccessibleCellText_AbsoluteColumnOrder(t *testing.T) {
	g := pinnedGrid(t)

	cases := []struct {
		col  int
		want string
	}{
		{0, "AA"}, // pinned-left
		{1, "BB"}, // main
		{2, "CC"}, // pinned-right
	}
	for _, c := range cases {
		if got := g.AccessibleCellText(0, c.col); got != c.want {
			t.Errorf("AccessibleCellText(0, %d) = %q, want %q", c.col, got, c.want)
		}
	}

	// Out-of-range absolute column returns empty, not a panic.
	if got := g.AccessibleCellText(0, 3); got != "" {
		t.Errorf("out-of-range col = %q, want empty", got)
	}
}
