package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// Issue #101: the Menu (application) key fires OnCellContextMenu for the
// focused cell — display-space id, window-absolute anchor at the cell's
// bottom-left — and selects it like the pointer path does.
func TestDataGrid_ContextMenuKey_FiresWithFocusedCellAndAnchor(t *testing.T) {
	data := []map[string]any{{"a": 1, "b": 1}, {"a": 2, "b": 2}, {"a": 3, "b": 3}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "a"}, {Field: "b"}})
	g.SelectionMode = SelectionSingleCell
	layOut(t, g)

	var gotID CellID
	var gotAnchor geo.Position
	fired := 0
	g.OnCellContextMenu = func(id CellID, anchor geo.Position) {
		fired++
		gotID = id
		gotAnchor = anchor
	}

	g.focused = true
	g.currentFocus = CellID{Row: 1, Col: 1}
	g.TypedKey(&event.KeyEvent{Name: desktop.KeyMenu})

	if fired != 1 {
		t.Fatalf("expected callback to fire once, got %d", fired)
	}
	if gotID.Row != 1 || gotID.Col != 1 {
		t.Errorf("callback id = %+v, want {Row:1 Col:1}", gotID)
	}
	// The key also selects the cell (parity with TappedSecondary).
	if sel := g.API().GetSelectedCells(); sel == nil || sel.Row != 1 || sel.Col != 1 {
		t.Errorf("expected cell {1,1} selected, got %+v", sel)
	}

	// Anchor = grid's window-absolute origin + the cell's bottom-left. The
	// small grid fits the 300x300 window, so no scroll offset is in play.
	if g.offset != geo.NewPos(0, 0) {
		t.Fatalf("expected no scroll offset, got %+v", g.offset)
	}
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(g)
	rowPitch := g.rowHeight() + g.cellPadding()
	wantY := abs.Y + g.HeaderHeight() + rowPitch + g.rowHeight() // bottom of row 1
	wantX := abs.X + g.HeaderColumnWidths()[0] + g.cellPadding() // left edge of col 1
	if gotAnchor.X != wantX || gotAnchor.Y != wantY {
		t.Errorf("anchor = %+v, want {%v %v}", gotAnchor, wantX, wantY)
	}
}

// Shift+F10 is the chorded equivalent of the Menu key; plain F10 stays
// unhandled and neither fires nor selects.
func TestDataGrid_ContextMenuKey_ShiftF10AndPlainF10(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionSingleCell
	layOut(t, g)

	var gotID CellID
	fired := 0
	g.OnCellContextMenu = func(id CellID, anchor geo.Position) {
		fired++
		gotID = id
	}

	g.focused = true
	g.currentFocus = CellID{Row: 1, Col: 0}

	g.TypedKey(&event.KeyEvent{Name: event.KeyF10})
	if fired != 0 {
		t.Fatalf("plain F10 fired the callback %d times, want 0", fired)
	}
	if sel := g.API().GetSelectedCells(); sel != nil {
		t.Errorf("plain F10 selected %+v, want no selection", sel)
	}

	g.TypedKey(&event.KeyEvent{Name: event.KeyF10, Modifier: event.KeyModifierShift})
	if fired != 1 {
		t.Fatalf("Shift+F10 fired the callback %d times, want 1", fired)
	}
	if gotID.Row != 1 || gotID.Col != 0 {
		t.Errorf("callback id = %+v, want {Row:1 Col:0}", gotID)
	}
	if sel := g.API().GetSelectedCells(); sel == nil || sel.Row != 1 || sel.Col != 0 {
		t.Errorf("expected cell {1,0} selected, got %+v", sel)
	}
}

// A cleared keyboard cursor (its row was filtered out) makes Menu/Shift+F10 a
// no-op, like the other Enter/Space/F2 focus verbs.
func TestDataGrid_ContextMenuKey_NoFocusNoOp(t *testing.T) {
	data := []map[string]any{{"v": 1}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionSingleCell

	fired := 0
	g.OnCellContextMenu = func(id CellID, anchor geo.Position) { fired++ }

	g.focused = true
	g.currentFocus = CellID{Row: -1, Col: -1} // cleared, as after a re-filter
	g.TypedKey(&event.KeyEvent{Name: desktop.KeyMenu})
	g.TypedKey(&event.KeyEvent{Name: event.KeyF10, Modifier: event.KeyModifierShift})

	if fired != 0 {
		t.Errorf("cleared cursor fired the callback %d times, want 0", fired)
	}
	if sel := g.API().GetSelectedCells(); sel != nil {
		t.Errorf("cleared cursor selected %+v, want no selection", sel)
	}
}

// A nil callback must not panic, and the key still selects the focused cell —
// exactly what TappedSecondary does with a nil OnCellSecondaryTapped.
func TestDataGrid_ContextMenuKey_NilCallbackStillSelects(t *testing.T) {
	data := []map[string]any{{"v": 1}, {"v": 2}}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "v"}})
	g.SelectionMode = SelectionSingleCell
	layOut(t, g)

	g.focused = true
	g.currentFocus = CellID{Row: 1, Col: 0}
	g.TypedKey(&event.KeyEvent{Name: desktop.KeyMenu})

	if sel := g.API().GetSelectedCells(); sel == nil || sel.Row != 1 || sel.Col != 0 {
		t.Errorf("expected cell {1,0} selected despite nil callback, got %+v", sel)
	}
}

// On a sorted grid the keyboard path hands out the same display-space CellID
// the pointer path does — SourceRow recovers the same underlying row from
// either, so a consumer's context-menu wiring is interchangeable.
func TestDataGrid_ContextMenuKey_SortedGridMatchesPointerIDForm(t *testing.T) {
	data := []map[string]any{
		{"name": "Charlie"},
		{"name": "Alice"},
		{"name": "Bob"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "name"}})
	g.SelectionMode = SelectionSingleCell
	layOut(t, g)
	g.API().SetSort(SortItem{ColID: "name", Direction: SortAscending})
	// Display order: Alice(src 1), Bob(src 2), Charlie(src 0).

	var pointerID, keyboardID CellID
	g.OnCellSecondaryTapped = func(id CellID, e *event.PointEvent) { pointerID = id }
	g.OnCellContextMenu = func(id CellID, anchor geo.Position) { keyboardID = id }

	// Pointer path on display row 2 (Charlie).
	headerH := g.HeaderHeight()
	rowPitch := g.rowHeight() + g.cellPadding()
	g.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, headerH+rowPitch*2+rowPitch/2)})

	// Keyboard path on the same display cell.
	g.focused = true
	g.currentFocus = CellID{Row: 2, Col: 0}
	g.TypedKey(&event.KeyEvent{Name: desktop.KeyMenu})

	if pointerID != keyboardID {
		t.Errorf("keyboard id %+v differs from pointer id %+v", keyboardID, pointerID)
	}
	if keyboardID.Row != 2 {
		t.Errorf("keyboard id row = %d, want display row 2", keyboardID.Row)
	}
	if src := g.SourceRow(keyboardID.Row); src != 0 {
		t.Errorf("SourceRow(keyboard id) = %d, want 0 (Charlie)", src)
	}
}
