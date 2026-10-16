package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
)

// Issue #76: DisplayRow is the inverse of SourceRow — it maps a stable source
// index (as delivered by OnSelectionChanged) to the display row currently
// showing it, so a caller can select/reveal that row after a sort or filter.
func TestDataGrid_DisplayRow_AcrossSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"},
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})
	// Display order is now Alice(src 1), Bob(src 2), Charlie(src 0).

	for src, wantDisp := range map[int]int{1: 0, 2: 1, 0: 2} {
		if got := g.DisplayRow(src); got != wantDisp {
			t.Errorf("DisplayRow(%d) = %d, want %d", src, got, wantDisp)
		}
		// Round trip: SourceRow is the exact inverse.
		if got := g.SourceRow(g.DisplayRow(src)); got != src {
			t.Errorf("SourceRow(DisplayRow(%d)) = %d, want %d", src, got, src)
		}
	}
}

func TestDataGrid_DisplayRow_FilteredOutAndOutOfRange(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})

	g.API().SetQuickFilter("Alice") // drops Bob (src 1) from the view

	assert.Equal(t, 0, g.DisplayRow(0), "Alice still shown")
	assert.Equal(t, -1, g.DisplayRow(1), "filtered-out source row is not in the view")
	assert.Equal(t, -1, g.DisplayRow(-1), "negative source row")
	assert.Equal(t, -1, g.DisplayRow(99), "out-of-range source row")
}

// Issue #76: ScrollToRow brings a display row into the viewport — SelectCell
// alone moves the selection/focus but never scrolls, so a selection driven from
// outside the visible range would not be seen.
func TestDataGrid_ScrollToRow(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)

	require.Equal(t, geo.NewPos(0, 0), g.scrollContent.Offset())

	g.ScrollToRow(900)
	assert.Greater(t, g.scrollContent.Offset().Y, float32(0), "scrolled down to the off-viewport row")
	assert.Equal(t, float32(0), g.scrollContent.Offset().X, "horizontal offset untouched")

	// Scrolling back up to a row above the viewport works too.
	g.ScrollToRow(0)
	assert.Equal(t, float32(0), g.scrollContent.Offset().Y, "scrolled back to the top row")

	// A row already in view does not move the viewport.
	g.ScrollToRow(900)
	before := g.scrollContent.Offset()
	g.ScrollToRow(900)
	assert.Equal(t, before, g.scrollContent.Offset(), "in-view row leaves the offset alone")

	// Out-of-range rows are ignored (no panic, no scroll).
	g.ScrollToRow(-1)
	g.ScrollToRow(5000)
	assert.Equal(t, before, g.scrollContent.Offset(), "out-of-range rows ignored")
}

// The two-way sync the issue was filed for (pre-rebrand goed tracker #67's replay scrubber): an
// external caller holding a stable source index selects and reveals that row
// after a sort has permuted the view.
func TestDataGrid_DisplayRowThenScrollToRow_RevealsSortedRow(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	g.SelectionMode = SelectionSingleCell
	layOut(t, g)

	// Sort descending: source row 0 ("row0") lands at the bottom of the view.
	g.API().SetSort(SortItem{ColID: "n", Direction: SortDescending})

	disp := g.DisplayRow(0)
	require.Greater(t, disp, 10, "row0 sorted far below the ~10 realized rows")

	g.SelectCell(disp, 0)
	assert.Equal(t, geo.NewPos(0, 0), g.scrollContent.Offset(), "SelectCell alone does not scroll")

	g.ScrollToRow(disp)
	assert.Greater(t, g.scrollContent.Offset().Y, float32(0), "revealed the selected row")
	assert.True(t, g.IsSelected(disp, 0), "selection kept on the revealed row")
}
