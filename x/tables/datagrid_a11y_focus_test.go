package tables

// pre-rebrand Issue pre-rebrand tracker #42: the grid's keyboard cursor (currentFocus) is exposed to assistive
// technology. The focused cell reports StateFocused (via IsFocused) and the grid
// designates it as its AT-focused descendant (AccessibleActiveDescendant), so a
// screen reader follows arrow-key navigation and announces the focused cell. The
// descendant identity equals the cell's stable, source-keyed AccessibleCellIdentity
// so it resolves to the same node as the cursor moves and survives sort/filter.

import "testing"

func focusGrid(t *testing.T) *DataGrid {
	t.Helper()
	data := []map[string]any{{"n": "charlie"}, {"n": "alice"}, {"n": "bob"}}
	return newTestGrid(t, data, []*ColumnDef{{ColID: "n", Field: "n"}, {ColID: "u", Field: "n"}})
}

// While the grid holds focus, the cursor cell — and only that cell — reports
// IsFocused, and AccessibleActiveDescendant is exactly that cell's identity.
func TestDataGrid_ActiveDescendant_ReflectsCursor(t *testing.T) {
	g := focusGrid(t)
	g.focused = true
	g.currentFocus = CellID{Row: 1, Col: 0}

	if !g.IsFocused(1, 0) {
		t.Error("cursor cell (1,0) should report IsFocused")
	}
	if g.IsFocused(1, 1) || g.IsFocused(0, 0) {
		t.Error("non-cursor cells must not report IsFocused")
	}

	want := g.AccessibleCellIdentity(1, 0)
	if want == nil {
		t.Fatal("cursor cell identity should not be nil")
	}
	if got := g.AccessibleActiveDescendant(); got != want {
		t.Errorf("active descendant = %v, want the cursor cell identity %v", got, want)
	}
}

// AccessibleActiveDescendant is nil (AT focus stays on the grid container) when
// the grid does not hold focus or the cursor is cleared, and IsFocused matches.
func TestDataGrid_ActiveDescendant_NilWhenUnfocusedOrCleared(t *testing.T) {
	g := focusGrid(t)

	// Cursor set but the grid does not hold keyboard focus.
	g.focused = false
	g.currentFocus = CellID{Row: 1, Col: 0}
	if got := g.AccessibleActiveDescendant(); got != nil {
		t.Errorf("unfocused grid active descendant = %v, want nil", got)
	}
	if g.IsFocused(1, 0) {
		t.Error("unfocused grid must not report a focused cell")
	}

	// Focused but the cursor is cleared (its row was filtered out).
	g.focused = true
	g.currentFocus = CellID{Row: -1, Col: -1}
	if got := g.AccessibleActiveDescendant(); got != nil {
		t.Errorf("cleared cursor active descendant = %v, want nil", got)
	}
	if g.IsFocused(-1, -1) {
		t.Error("a cleared cursor must not report any focused cell")
	}
}

// The designated descendant follows the record across a re-sort: the cursor is
// remapped to the record's new display row, and AccessibleActiveDescendant keeps
// returning that record's stable identity — so AT focus tracks the row, not the
// display slot.
func TestDataGrid_ActiveDescendant_FollowsSourceAcrossSort(t *testing.T) {
	g := focusGrid(t)
	g.focused = true

	// Park the cursor on Charlie (source 0, display row 0) and record identity.
	g.currentFocus = CellID{Row: 0, Col: 0}
	g.focusSrcRow = g.sourceRow(0)
	charlie := g.AccessibleActiveDescendant()
	if charlie == nil {
		t.Fatal("active descendant should not be nil with a live cursor")
	}

	g.API().SetSort(SortItem{ColID: "n", Direction: SortAscending})

	// Charlie is now display row 2; the cursor was remapped there.
	if g.currentFocus.Row != 2 {
		t.Fatalf("cursor row after sort = %d, want 2 (remapped to Charlie)", g.currentFocus.Row)
	}
	if got := g.AccessibleActiveDescendant(); got != charlie {
		t.Errorf("active descendant changed across sort: %v -> %v", charlie, got)
	}
	// The identity is the same as querying Charlie's new display position.
	if got := g.AccessibleCellIdentity(2, 0); got != charlie {
		t.Errorf("active descendant %v != Charlie's cell identity at its new row %v", charlie, got)
	}
}
