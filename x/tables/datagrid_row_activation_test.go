package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
)

// pre-rebrand Issue pre-rebrand tracker #72: SourceRow / DisplayRowData map a display-space row (as delivered in
// CellID.Row by the click callbacks) back to the underlying source row/data, so
// consumers can act on the clicked row after a sort/filter permutes the view.
func TestDataGrid_SourceRowAndDisplayRowData_AcrossSort(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"},
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})

	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})
	// Display order is now Alice(src 1), Bob(src 2), Charlie(src 0).

	wantSrc := []int{1, 2, 0}
	wantName := []string{"Alice", "Bob", "Charlie"}
	for disp, src := range wantSrc {
		if got := g.SourceRow(disp); got != src {
			t.Errorf("SourceRow(%d) = %d, want %d", disp, got, src)
		}
		row, ok := g.DisplayRowData(disp).(map[string]any)
		if !ok {
			t.Fatalf("DisplayRowData(%d) = %v, want map", disp, g.DisplayRowData(disp))
		}
		if row["name"] != wantName[disp] {
			t.Errorf("DisplayRowData(%d)[name] = %v, want %s", disp, row["name"], wantName[disp])
		}
	}
}

func TestDataGrid_SourceRow_OutOfRange(t *testing.T) {
	g := newTestGrid(t, []map[string]any{{"v": 1}}, []*ColumnDef{{Field: "v"}})
	if got := g.SourceRow(5); got != -1 {
		t.Errorf("SourceRow(out-of-range) = %d, want -1", got)
	}
	if got := g.DisplayRowData(5); got != nil {
		t.Errorf("DisplayRowData(out-of-range) = %v, want nil", got)
	}
}

// pre-rebrand Issue pre-rebrand tracker #73: Enter/Return and Space on a focused data row fire OnRowActivated
// with the row's stable source index (like OnSelectionChanged), after a sort has
// permuted display<->source.
func TestDataGrid_OnRowActivated_KeyboardEnterAndSpace(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"},
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})
	// Display order: Alice(src 1), Bob(src 2), Charlie(src 0).

	var activated []int
	g.OnRowActivated = func(src int) { activated = append(activated, src) }

	g.focused = true
	g.currentFocus = CellID{Row: 2, Col: 0} // Charlie, source 0
	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})

	g.currentFocus = CellID{Row: 0, Col: 0} // Alice, source 1
	g.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	want := []int{0, 1}
	if len(activated) != len(want) {
		t.Fatalf("activated = %v, want %v", activated, want)
	}
	for i, s := range want {
		if activated[i] != s {
			t.Errorf("activated[%d] = %d, want %d", i, activated[i], s)
		}
	}
}

// A cleared cursor (row filtered out after a re-sort) must not fire activation.
func TestDataGrid_OnRowActivated_NoFireWhenCursorCleared(t *testing.T) {
	g := newTestGrid(t, []map[string]any{{"v": 1}}, []*ColumnDef{{Field: "v"}})
	fired := false
	g.OnRowActivated = func(int) { fired = true }

	g.focused = true
	g.currentFocus = CellID{Row: -1, Col: -1}
	g.TypedKey(&event.KeyEvent{Name: event.KeyReturn})

	if fired {
		t.Error("OnRowActivated fired on a cleared cursor")
	}
}
