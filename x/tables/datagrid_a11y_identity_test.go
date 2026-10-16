package tables

// pre-rebrand Issue pre-rebrand tracker #16: realized cells and off-screen virtual nodes share one stable,
// source-keyed identity channel, so a screen reader's cursor follows a record
// across sort/filter/scroll rather than the display slot.

import "testing"

// AccessibleCellIdentity is keyed by source row: it is stable for a record as it
// moves display positions under a sort, and distinct for different records.
func TestDataGrid_AccessibleCellIdentity_FollowsSource(t *testing.T) {
	data := []map[string]any{{"n": "charlie"}, {"n": "alice"}, {"n": "bob"}}
	g := newTestGrid(t, data, []*ColumnDef{{ColID: "n", Field: "n"}})

	// Charlie is source 0, currently display row 0.
	charlie := g.AccessibleCellIdentity(0, 0)
	if charlie == nil {
		t.Fatal("identity for a real cell should not be nil")
	}

	g.API().SetSort(SortItem{ColID: "n", Direction: SortAscending})

	// After sort Charlie is display row 2 — same identity as before.
	if got := g.AccessibleCellIdentity(2, 0); got != charlie {
		t.Errorf("identity changed across sort: %v -> %v", charlie, got)
	}
	// Display row 0 is now Alice (source 1) — a different identity.
	if got := g.AccessibleCellIdentity(0, 0); got == charlie {
		t.Errorf("different source row shares Charlie's identity: %v", got)
	}
	// Same source row, different column ⇒ different identity.
	if g.AccessibleCellIdentity(2, 0) == g.AccessibleCellIdentity(2, 1) {
		t.Error("cells in different columns must have distinct identities")
	}
}

// Out-of-range (synthetic) rows have no source identity.
func TestDataGrid_AccessibleCellIdentity_NilForSynthetic(t *testing.T) {
	g := newTestGrid(t, []map[string]any{{"n": "a"}}, []*ColumnDef{{ColID: "n", Field: "n"}})
	if got := g.AccessibleCellIdentity(99, 0); got != nil {
		t.Errorf("out-of-range identity = %v, want nil", got)
	}
}
