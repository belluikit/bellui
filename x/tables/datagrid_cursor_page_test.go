package tables

// Tests for issue #9: with pagination, the interaction cursor follows its source
// row to whatever page it lands on after a re-sort (Option A), and still clears
// when the row is filtered out of every page.

import "testing"

// paginatedNameGrid builds a 6-row grid paginated 2-per-page. Source 0 is "F"
// (sorts last) so it can be pushed from page 0 to page 2 by an ascending sort.
func paginatedNameGrid(t *testing.T) *DataGrid {
	t.Helper()
	data := []map[string]any{
		{"name": "F"}, // source 0
		{"name": "A"}, // source 1
		{"name": "B"}, // source 2
		{"name": "C"}, // source 3
		{"name": "D"}, // source 4
		{"name": "E"}, // source 5
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.API().EnablePagination(2)
	return g
}

// A selected/focused row on page 0 follows to its new page after a sort pushes
// it there, and the cursor re-resolves to the row on that page.
func TestDataGrid_CursorFollowsRowToNewPage(t *testing.T) {
	g := paginatedNameGrid(t)

	// Select "F" (source 0), currently display 0 on page 0.
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)
	if g.CurrentPage() != 0 {
		t.Fatalf("setup: CurrentPage = %d, want 0", g.CurrentPage())
	}

	// Ascending sort → A,B,C,D,E,F; "F" is now global position 5 → page 2.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	if g.CurrentPage() != 2 {
		t.Fatalf("cursor did not follow to its page: CurrentPage = %d, want 2", g.CurrentPage())
	}
	// On page 2 the rows are [E(src5), F(src0)] → "F" is display 1.
	if g.currentFocus.Row != 1 {
		t.Errorf("focus display row = %d, want 1", g.currentFocus.Row)
	}
	if g.selectedCell == nil || g.selectedCell.Row != 1 {
		t.Errorf("selected cell = %+v, want row 1", g.selectedCell)
	}
	if got := g.pipe.DisplayRow(1).(map[string]any)["name"]; got != "F" {
		t.Errorf("cursor points at %v, want F", got)
	}
}

// A cursor whose row is filtered out of every page still clears — follow only
// applies when the row genuinely exists somewhere.
func TestDataGrid_CursorClearsWhenFilteredAcrossPages(t *testing.T) {
	g := paginatedNameGrid(t)

	// Focus "F" (source 0) on page 0.
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)

	// Filter to rows containing "A" — removes "F" entirely.
	g.API().SetQuickFilter("A")

	if g.currentFocus.Row != -1 || g.focusSrcRow != -1 {
		t.Errorf("filtered-out cursor not cleared: focus=%d src=%d", g.currentFocus.Row, g.focusSrcRow)
	}
	if g.selectedCell != nil {
		t.Errorf("filtered-out selected cell not cleared: %+v", g.selectedCell)
	}
}

// A cursor already on the current page is untouched (no spurious page change).
func TestDataGrid_CursorStaysWhenRowRemainsOnPage(t *testing.T) {
	g := paginatedNameGrid(t)

	// Focus "A" (source 1), display 1 on page 0.
	g.selectCellWithModifiers(CellID{Row: 1, Col: 0}, 0)

	// Ascending sort → "A" becomes global position 0, still on page 0.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	if g.CurrentPage() != 0 {
		t.Errorf("page changed unnecessarily: CurrentPage = %d, want 0", g.CurrentPage())
	}
	if g.currentFocus.Row != 0 {
		t.Errorf("focus display row = %d, want 0 (A's new slot)", g.currentFocus.Row)
	}
	if got := g.pipe.DisplayRow(0).(map[string]any)["name"]; got != "A" {
		t.Errorf("cursor points at %v, want A", got)
	}
}

// pageForSource maps a source row to the page it lands on in the pre-paginate
// order, and returns -1 for a filtered-out row.
func TestDataGrid_PageForSource(t *testing.T) {
	g := paginatedNameGrid(t)
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})
	// Sorted A,B,C,D,E,F → source 0 ("F") at position 5 → page 2; source 1 ("A")
	// at position 0 → page 0.
	if p := g.pageForSource(0); p != 2 {
		t.Errorf("pageForSource(0) = %d, want 2", p)
	}
	if p := g.pageForSource(1); p != 0 {
		t.Errorf("pageForSource(1) = %d, want 0", p)
	}

	g.API().SetQuickFilter("A") // drop everything but "A"
	if p := g.pageForSource(0); p != -1 {
		t.Errorf("pageForSource(filtered-out) = %d, want -1", p)
	}
}
