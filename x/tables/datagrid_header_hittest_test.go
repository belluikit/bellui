package tables

import (
	"testing"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func headerHitGrid(t *testing.T) *DataGrid {
	t.Helper()
	data := make([]map[string]any, 5)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i}
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Width: 80},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(200, 300))
	return g
}

// The header now scrolls horizontally in sync with the body (all columns are
// non-pinned here), so header column identification is region-aware and tracks
// the scroll offset — a tap resolves to the column currently drawn under it
// (#58). This supersedes the static-header mapping from #56.
func TestHeaderHitTest_TracksHScroll(t *testing.T) {
	g := headerHitGrid(t)
	yHead := g.HeaderHeight() / 2 // inside the header row

	// A tap near the left resolves to column 0 while unscrolled.
	if id := g.cellAtPosition(geo.NewPos(8, yHead)); id.Row != -1 || id.Col != 0 {
		t.Fatalf("unscrolled header tap = %+v, want {Row:-1, Col:0}", id)
	}

	// After scrolling one column pitch, the same widget-X now sits over column 1,
	// matching where the header label is drawn (header scrolls with the body).
	pitch := 80 + g.cellPadding()
	g.offset = geo.NewPos(pitch, 0)
	if id := g.cellAtPosition(geo.NewPos(8, yHead)); id.Row != -1 || id.Col != 1 {
		t.Errorf("scrolled header tap = %+v, want {Row:-1, Col:1} (header tracks scroll)", id)
	}
}

// Starting a column reorder from the header picks the column currently drawn
// under the cursor, using the same region-aware mapping the header renderer uses
// (#58): under horizontal scroll that is columnAtX, not a raw-X mapping.
func TestHeaderReorder_PicksColumnUnderHScroll(t *testing.T) {
	g := headerHitGrid(t)
	yHead := g.HeaderHeight() / 2
	x := float32(90)

	pitch := 80 + g.cellPadding()
	g.offset = geo.NewPos(pitch, 0)
	g.MouseDown(&desktop.MouseEvent{
		PointEvent: event.PointEvent{Position: geo.NewPos(x, yHead)},
	})

	if g.reorderFromCol != g.columnAtX(x) {
		t.Errorf("reorderFromCol = %d, want %d (region-aware column under cursor)",
			g.reorderFromCol, g.columnAtX(x))
	}
	// x=90 unscrolled is col 1; scrolled one pitch it is col 2.
	if g.reorderFromCol != 2 {
		t.Errorf("reorderFromCol = %d, want 2 (scrolled column under cursor)", g.reorderFromCol)
	}
}

// With pinned-left and pinned-right bands, header taps resolve to the same
// absolute columns the body does: the pinned bands are fixed (unaffected by
// horizontal scroll) while the main band tracks the scroll offset (#58). A
// header tap shares the body's region model, so cellAtPosition in the header
// row (Row == -1) mirrors hitRegions.
func TestHeaderHitTest_RegionsUnderHScroll(t *testing.T) {
	data := make([]map[string]any, 10)
	for i := range data {
		data[i] = map[string]any{"a": i, "b": i, "c": i, "d": i}
	}
	// layout.all = [a(pinned-left), b, c, d(pinned-right)] -> absolute 0..3.
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Pinned: PinLeft, Width: 80},
		{Field: "b", Width: 80},
		{Field: "c", Width: 80},
		{Field: "d", Pinned: PinRight, Width: 80},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(600, 300)) // wide enough that all four columns fit
	yHead := g.HeaderHeight() / 2

	mainX, pinnedRightX, _ := g.hitRegions()
	pad := g.cellPadding()

	assertCol := func(name string, x float32, want int) {
		t.Helper()
		if id := g.cellAtPosition(geo.NewPos(x, yHead)); id.Row != -1 || id.Col != want {
			t.Errorf("%s: header tap = %+v, want {Row:-1, Col:%d}", name, id, want)
		}
	}

	// Unscrolled: each band's header sits over its own column.
	assertCol("pinned-left a", 40, 0)
	assertCol("main b", mainX+40, 1)
	assertCol("main c", mainX+(80+pad)+40, 2)
	assertCol("pinned-right d", pinnedRightX+40, 3)

	// Scroll the main band one pitch: the pinned bands are unchanged, but the
	// main header now shows column c where b was.
	g.offset = geo.NewPos(80+pad, 0)
	assertCol("pinned-left fixed", 40, 0)
	assertCol("pinned-right fixed", pinnedRightX+40, 3)
	assertCol("main shifted", mainX+40, 2)
}
