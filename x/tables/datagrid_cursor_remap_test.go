package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
)

// After a sort moves the selected row, the selected cell (the edit/paste/scroll
// target) must follow its SOURCE row to the new display position.
func TestDataGrid_SelectedCellFollowsSourceAcrossSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.SelectionMode = SelectionSingleCell

	// Select Charlie (display 0 == source 0).
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)

	// Sort ascending: Alice(1), Bob(2), Charlie(0) — Charlie moves to display 2.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	sel := g.API().GetSelectedCells()
	if sel == nil {
		t.Fatal("selected cell was lost across sort")
	}
	if sel.Row != 2 {
		t.Errorf("selected cell display row = %d, want 2 (Charlie's new slot)", sel.Row)
	}
	// The remapped cursor must point at the original source row's data.
	if got := g.pipe.DisplayRow(sel.Row).(map[string]any)["name"]; got != "Charlie" {
		t.Errorf("cursor now points at %v, want Charlie", got)
	}
}

// Keyboard focus must also follow its source row across a re-sort.
func TestDataGrid_KeyboardFocusFollowsSourceAcrossSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"}, // source 0
		{"name": "Alice"},   // source 1
		{"name": "Bob"},     // source 2
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})

	// Move keyboard focus down to display row 1 (Alice, source 1).
	g.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	if g.currentFocus.Row != 1 {
		t.Fatalf("focus setup: currentFocus.Row = %d, want 1", g.currentFocus.Row)
	}

	// Sort ascending: Alice(1) moves to display 0.
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	if g.currentFocus.Row != 0 {
		t.Errorf("focus display row = %d, want 0 (Alice's new slot)", g.currentFocus.Row)
	}
}

// When a filter removes the focused row from view, the cursor is cleared so
// Enter/paste act on nothing; a movement key re-enters the grid at the origin.
func TestDataGrid_CursorClearedWhenRowFilteredOut(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"},
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.SelectionMode = SelectionSingleCell

	// Select Charlie (source 0).
	g.selectCellWithModifiers(CellID{Row: 0, Col: 0}, 0)

	// Filter to only "Alice" — Charlie leaves the view.
	g.API().SetQuickFilter("Alice")

	if sel := g.API().GetSelectedCells(); sel != nil {
		t.Errorf("expected cleared selected cell, got %+v", sel)
	}
	if g.currentFocus.Row >= 0 || g.currentFocus.Col >= 0 {
		t.Errorf("expected cleared focus {-1,-1}, got %+v", g.currentFocus)
	}

	// Enter must be a no-op (no panic, nothing re-selected).
	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if sel := g.API().GetSelectedCells(); sel != nil {
		t.Errorf("Enter on a cleared cursor selected %+v, want nothing", sel)
	}

	// A movement key re-enters at the origin.
	g.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	if g.currentFocus.Row != 0 || g.currentFocus.Col != 0 {
		t.Errorf("after re-entry currentFocus = %+v, want {0,0}", g.currentFocus)
	}
}

// A grid with no active cursor pays nothing and its focus is untouched by a
// reindex (proxy for "no reverse map is built without a selection/cursor").
func TestDataGrid_NoCursorNoRemap(t *testing.T) {
	data := []map[string]any{{"name": "Charlie"}, {"name": "Alice"}, {"name": "Bob"}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	// No selection made; default focus is the origin, focusSrcRow unset.

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})

	if g.currentFocus != (CellID{Row: 0, Col: 0}) {
		t.Errorf("focus changed on a cursor-less grid: %+v", g.currentFocus)
	}
	if g.API().GetSelectedCells() != nil {
		t.Errorf("unexpected selection: %+v", g.API().GetSelectedCells())
	}
}
