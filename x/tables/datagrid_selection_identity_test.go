package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/lang"
)

// displayRowOf returns the display position currently showing the given source
// row, or -1 if it is not displayed.
func displayRowOf(g *DataGrid, sourceRow int) int {
	idx := g.pipe.Index()
	for d, s := range idx.Indices() {
		if s == sourceRow {
			return d
		}
	}
	return -1
}

// TestSelection_RowSurvivesReSort verifies that a row selection tracks the
// underlying source data across a re-sort instead of sticking to a display
// position (the bug in pre-rebrand issue #2).
func TestSelection_RowSurvivesReSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.SelectionMode = SelectionSingleRow

	// Before any sort, display order == source order: display 0 is Charlie (src 0).
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)

	if got := g.API().GetSelectedRows(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("expected selected source row [0], got %v", got)
	}

	// Sort ascending: display order becomes Alice(1), Bob(2), Charlie(0).
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	// Selection identity is unchanged: still source row 0 (Charlie).
	if got := g.API().GetSelectedRows(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("after re-sort expected stable source row [0], got %v", got)
	}

	// Charlie moved to display row 2 — the highlight must follow it there...
	charlieDisplay := displayRowOf(g, 0)
	if charlieDisplay != 2 {
		t.Fatalf("expected Charlie (src 0) at display row 2, got %d", charlieDisplay)
	}
	if !g.isCellSelected(2, 0) {
		t.Errorf("expected display row 2 (Charlie) to be highlighted after re-sort")
	}
	// ...and must NOT remain on the old display position (now Alice).
	if g.isCellSelected(0, 0) {
		t.Errorf("display row 0 (Alice) must not be highlighted after re-sort")
	}
}

// TestSelection_SingleCellSurvivesReSort verifies the selected-cell highlight is
// keyed by source identity too.
func TestSelection_SingleCellSurvivesReSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.SelectionMode = SelectionSingleCell

	g.selectCellWithModifiers(CellID{Row: 1, Col: 0}, 0) // display 1 == Alice (src 1)

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	// Alice is now display row 0.
	if got := displayRowOf(g, 1); got != 0 {
		t.Fatalf("expected Alice (src 1) at display row 0, got %d", got)
	}
	if !g.isCellSelected(0, 0) {
		t.Errorf("expected Alice's cell (now display 0) to remain selected")
	}
	if g.isCellSelected(1, 0) {
		t.Errorf("old display position 1 must not be selected after re-sort")
	}
}

// TestSelection_MultiRowSurvivesReSort verifies a multi-row selection keeps
// pointing at the same source rows after a re-sort.
func TestSelection_MultiRowSurvivesReSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
		{"name": "Dave"},    // source 3
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.SelectionMode = SelectionMultiRow

	// Select Charlie (src 0) and Bob (src 2) via ctrl-click.
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)
	g.selectCellWithModifiers(CellID{Row: 2, Col: 0}, event.KeyModifierControl)

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	got := map[int]bool{}
	for _, r := range g.API().GetSelectedRows() {
		got[r] = true
	}
	if len(got) != 2 || !got[0] || !got[2] {
		t.Fatalf("expected stable source rows {0,2}, got %v", got)
	}

	// Highlights follow the data: Charlie and Bob, wherever they now display.
	for _, src := range []int{0, 2} {
		if d := displayRowOf(g, src); d < 0 || !g.isCellSelected(d, 0) {
			t.Errorf("source row %d not highlighted at its new display position %d", src, d)
		}
	}
	// Alice (src 1) and Dave (src 3) are not selected.
	for _, src := range []int{1, 3} {
		if d := displayRowOf(g, src); d >= 0 && g.isCellSelected(d, 0) {
			t.Errorf("source row %d should not be highlighted", src)
		}
	}
}

// TestSelection_SurvivesFilter verifies selection identity holds across a filter
// that removes rows (the selected source row stays selected while visible).
func TestSelection_SurvivesFilter(t *testing.T) {
	data := []map[string]any{
		{"name": "Alice", "active": true}, // source 0
		{"name": "Bob", "active": false},  // source 1
		{"name": "Carol", "active": true}, // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}, {Field: "active"}})
	g.SelectionMode = SelectionMultiRow

	// Select Carol (src 2) at its current display position (2).
	g.selectCellWithModifiers(CellID{Row: 2, Col: 0}, 0)
	if got := g.API().GetSelectedRows(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("expected source row [2], got %v", got)
	}

	// Filter to active==true, dropping Bob (src 1). Carol survives at display 1.
	g.API().SetColumnFilter("active", func(v any) bool { b, _ := v.(bool); return b })

	if got := g.API().GetSelectedRows(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("after filter expected stable source row [2], got %v", got)
	}
	carolDisplay := displayRowOf(g, 2)
	if carolDisplay < 0 || !g.isCellSelected(carolDisplay, 0) {
		t.Errorf("Carol (src 2) should still be highlighted at display %d after filter", carolDisplay)
	}
}

// TestSort_LocaleAwareStrings verifies string columns sort by locale collation
// (pre-rebrand issue #2, part 2): 'ä' collates near 'a' (before 'z'), not in byte order
// where its code point (228) would sort after 'z' (122).
func TestSort_LocaleAwareStrings(t *testing.T) {
	lang.SetLocaleForTests(t, "en")

	data := []map[string]any{
		{"name": "z"},
		{"name": "ä"},
		{"name": "a"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	want := []string{"a", "ä", "z"}
	for i, exp := range want {
		row := g.pipe.DisplayRow(i).(map[string]any)
		if row["name"] != exp {
			t.Errorf("display row %d: expected %q, got %v (byte-order would give a, z, ä)", i, exp, row["name"])
		}
	}
}
