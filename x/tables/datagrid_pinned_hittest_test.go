package tables

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// pinnedHitGrid builds a windowed grid with one pinned-left column and three main
// columns, narrow enough that the main band can scroll horizontally.
func pinnedHitGrid(t *testing.T) *DataGrid {
	t.Helper()
	data := make([]map[string]any, 20)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft, Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Width: 80},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(220, 300))
	return g
}

// A pinned-left column is drawn fixed at the left edge, so a tap on it must
// resolve to that column regardless of the main band's horizontal scroll —
// previously the scroll offset was wrongly added, leaking the tap into a main
// column (#54).
func TestCellAtPosition_PinnedLeftFixedUnderHScroll(t *testing.T) {
	g := pinnedHitGrid(t)
	y := g.HeaderHeight() + 5

	// Baseline: unscrolled, a tap inside the pinned-left column resolves to col 0.
	if id := g.cellAtPosition(geo.NewPos(10, y)); id.Col != 0 {
		t.Fatalf("pinned-left tap (no scroll) = col %d, want 0", id.Col)
	}

	// Scroll the main content far right; the fixed pinned-left tap is unchanged.
	g.offset = geo.NewPos(200, 0)
	if id := g.cellAtPosition(geo.NewPos(10, y)); id.Col != 0 {
		t.Errorf("pinned-left tap (scrolled) = col %d, want 0", id.Col)
	}
}

// A tap in the main band still tracks the horizontal scroll (regression guard for
// the columnAtX rewrite): scrolling by exactly one column pitch shifts the mapped
// column one place right.
func TestCellAtPosition_MainBandTracksHScroll(t *testing.T) {
	g := pinnedHitGrid(t)
	y := g.HeaderHeight() + 5
	pitch := 80 + g.cellPadding() // column width + inter-column padding
	x := float32(90)              // a point just inside the main band (col "b")

	unscrolled := g.cellAtPosition(geo.NewPos(x, y))
	g.offset = geo.NewPos(pitch, 0)
	scrolled := g.cellAtPosition(geo.NewPos(x, y))

	if unscrolled.Col < 0 || scrolled.Col < 0 {
		t.Fatalf("main-band taps should hit a column: %+v / %+v", unscrolled, scrolled)
	}
	if scrolled.Col != unscrolled.Col+1 {
		t.Errorf("scrolling one pitch should shift the column by 1: unscrolled=%d scrolled=%d",
			unscrolled.Col, scrolled.Col)
	}
}

// cellAtPosition returns an absolute display-column index, so it must be bounded
// by the total visible column count, not the main-only ColumnCount() — otherwise
// pinned-right cells and the tail main columns (when pinned-left exists) are
// wrongly rejected as no-hits (#59).
func TestCellAtPosition_PinnedCellsNotRejected(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i}
	}
	// Layout.all = [a(pinned-left), b, c, d(pinned-right)] -> absolute 0..3;
	// main = [b, c] so ColumnCount()==2 would reject absolute 2 (c) and 3 (d).
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft, Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Pinned: PinRight, Width: 80},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(600, 300)) // wide enough that all four columns fit
	y := g.HeaderHeight() + 5

	mainX, pinnedRightX, _ := g.hitRegions()
	pad := g.cellPadding()

	cases := []struct {
		name string
		x    float32
		want int
	}{
		{"pinned-left a", 40, 0},
		{"first main b", mainX + 40, 1},
		{"tail main c", mainX + (80 + pad) + 40, 2},
		{"pinned-right d", pinnedRightX + 40, 3},
	}
	for _, c := range cases {
		if id := g.cellAtPosition(geo.NewPos(c.x, y)); id.Row != 0 || id.Col != c.want {
			t.Errorf("%s: cellAtPosition = %+v, want {Row:0, Col:%d}", c.name, id, c.want)
		}
	}
}

// The tree disclosure on a pinned-left column is fixed, so its chevron hit-test
// ignores the main band's horizontal scroll (#54).
func TestTreeData_PinnedChevronIgnoresHScroll(t *testing.T) {
	g := newPinnedTreeGrid(t, treeFixture())
	y := g.HeaderHeight() + 5

	g.offset = geo.NewPos(200, 0)
	if row := g.treeToggleAt(geo.NewPos(4, y)); row != 0 {
		t.Errorf("pinned tree chevron under h-scroll = %d, want 0", row)
	}
}

// Master-detail disclosure on a pinned-left column: the chevron sits at the very
// left (grid-x 0), a tap toggles it, and horizontal scroll does not move it (#53
// + #54).
func TestMasterDetail_PinnedLeftChevron(t *testing.T) {
	data := []map[string]any{{"id": 1, "name": "A"}, {"id": 2, "name": "B"}}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "id", Pinned: PinLeft, Width: 80},
		{Field: "name", Width: 120},
	})
	g.MasterDetail = defaultDetailCfg(40)
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 300))
	y := g.HeaderHeight() + 5

	// Chevron zone starts at grid-x 0 (pinned-left), not past the pinned width.
	if row := g.detailToggleAt(geo.NewPos(4, y)); row != 0 {
		t.Fatalf("pinned master chevron hit = %d, want 0", row)
	}
	// A tap toggles the detail.
	g.Tapped(&event.PointEvent{Position: geo.NewPos(4, y)})
	if !g.IsDetailExpanded(0) {
		t.Fatal("tap on pinned master chevron did not expand the detail")
	}

	// Fixed under horizontal scroll.
	g.CollapseAllDetails()
	g.offset = geo.NewPos(200, 0)
	if row := g.detailToggleAt(geo.NewPos(4, y)); row != 0 {
		t.Errorf("pinned master chevron under h-scroll = %d, want 0", row)
	}
}
