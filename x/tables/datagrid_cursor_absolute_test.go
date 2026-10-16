package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// pinnedCursorGrid builds a grid whose display order (pinned-left, main,
// pinned-right) differs from the column-definition order, so index-space bugs
// surface. Absolute display columns: a=0 (pinned-left), b=1, c=2, d=3 (main),
// e=4 (pinned-right).
func pinnedCursorGrid(t *testing.T) *DataGrid {
	t.Helper()
	data := make([]map[string]any, 5)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i, "e": i}
	}
	return newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft, Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Width: 80},
		{Field: "e", Pinned: PinRight, Width: 80},
	})
}

// The keyboard cursor moves in absolute display-column space, so arrow keys
// traverse pinned-left → main → pinned-right and are bounded by the total
// visible column count (not the main-only ColumnCount) (pre-rebrand tracker #62).
func TestKeyboardCursor_MovesInAbsoluteSpace(t *testing.T) {
	g := pinnedCursorGrid(t)

	// Cursor starts at the origin {0,0}. Right walks the absolute display order.
	for want := 1; want <= 4; want++ {
		g.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		if g.currentFocus.Col != want {
			t.Fatalf("after %d× Right, currentFocus.Col = %d, want %d", want, g.currentFocus.Col, want)
		}
	}
	// Bounded by HeaderColumnCount()==5 (absolute 4 is the last column).
	g.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	if g.currentFocus.Col != 4 {
		t.Errorf("Right past the last column moved to %d, want 4 (clamped)", g.currentFocus.Col)
	}
}

// Keyboard selection records the cursor's ABSOLUTE column, matching the cell
// hosts' selection reporting (IsSelected(row, absoluteCol)); previously the
// keyboard path stored a main index, mis-highlighting with pinned columns (pre-rebrand tracker #62).
func TestKeyboardSelect_SelCellColIsAbsolute(t *testing.T) {
	g := pinnedCursorGrid(t)

	// Walk all the way to the pinned-right column (absolute 4) and select it —
	// a column the old main-bounded cursor (max index 2) could never reach.
	for i := 0; i < 4; i++ {
		g.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	}
	g.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	if g.selCellCol != 4 {
		t.Fatalf("selCellCol = %d, want 4 (absolute display index of pinned-right)", g.selCellCol)
	}
	if !g.IsSelected(0, 4) {
		t.Errorf("cell (0, abs 4) not reported selected")
	}
	if g.IsSelected(0, 2) {
		t.Errorf("cell (0, abs 2) wrongly reported selected")
	}
}

// scrollToCell horizontally scrolls only for main columns; pinned columns are
// always visible, so focusing one leaves the horizontal offset untouched (pre-rebrand tracker #62).
func TestScrollToCell_PinnedColumnSkipsHScroll(t *testing.T) {
	g := pinnedCursorGrid(t)
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	// Narrow enough that the tail main column is off-screen to the right.
	w.Resize(geo.NewSize(300, 300))

	// Focusing the pinned-right column (absolute 4) must not scroll horizontally.
	g.scrollContent.ScrollTo(geo.NewPos(0, 0))
	g.scrollToCell(CellID{Row: 0, Col: 4})
	if x := g.scrollContent.Scroll.Offset.X; x != 0 {
		t.Errorf("pinned-right focus scrolled horizontally to %v, want 0", x)
	}

	// Focusing the tail main column (absolute 3, off-screen) does scroll.
	g.scrollToCell(CellID{Row: 0, Col: 3})
	if x := g.scrollContent.Scroll.Offset.X; x <= 0 {
		t.Errorf("tail main focus did not scroll into view (offset.X = %v)", x)
	}
}

// Editing addresses the cell by its absolute display index and resolves it
// through visibleCol, so a commit writes the column actually shown there — even
// when pinning reorders the display relative to the column-definition order
// (previously resolvedCols[id.Col] wrote the wrong column) (pre-rebrand tracker #62).
func TestEdit_WritesDisplayedColumnUnderPinReorder(t *testing.T) {
	// Definition order [a, b]; b is pinned-left, so display order is [b, a] —
	// absolute 0 == b, absolute 1 == a (the reverse of resolvedCols).
	data := []map[string]any{{"a": "a0", "b": "b0"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Width: 80},
		{Field: "b", Pinned: PinLeft, Width: 80},
	})

	// Edit absolute column 0, which is the pinned "b" column.
	g.StartEditing(CellID{Row: 0, Col: 0})
	if g.editingCell == nil {
		t.Fatal("StartEditing did not begin editing absolute column 0")
	}
	g.commitEdit("edited")

	if got := data[0]["b"]; got != "edited" {
		t.Errorf("displayed column b = %v, want \"edited\"", got)
	}
	if got := data[0]["a"]; got != "a0" {
		t.Errorf("column a = %v, want unchanged \"a0\" (edit hit the wrong column)", got)
	}
}
