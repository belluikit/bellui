package tables

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// newPinnedRightGrid is a grid of rows rows with three 200 px main columns and
// a 100 px pinned-right column, shown in a window of the given size.
func newPinnedRightGrid(t *testing.T, rows int, size geo.Size) *DataGrid {
	t.Helper()
	var data []map[string]any
	for i := range rows {
		data = append(data, map[string]any{"a": i, "b": i, "c": fmt.Sprintf("c%d", i), "d": i})
	}
	g := newTestGrid(t, data, []*ColumnDef{
		{Field: "a", Width: 200},
		{Field: "b", Width: 200},
		{Field: "c", Width: 200},
		{Field: "d", Width: 100, Pinned: PinRight},
	})
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(size)
	return g
}

func scrollBarWidth(t *testing.T, g *DataGrid) float32 {
	t.Helper()
	barW := g.Theme().Size(bellui.ThemeSizeName("scrollBar"))
	if barW <= 0 {
		t.Fatalf("theme scrollBar size = %v; the test needs a real bar width", barW)
	}
	return barW
}

// With the body overflowing vertically, pinned-right ends one bar width short of
// the grid's right edge, so the scroll's vertical bar draws in that gutter and
// not under the pinned cells (#1257). Header, body and hit-testing share the
// anchor.
func TestDataGrid_PinnedRightKeepsClearOfVerticalBar(t *testing.T) {
	g := newPinnedRightGrid(t, 40, geo.NewSize(500, 300))
	barW := scrollBarWidth(t, g)

	_, prX, prW := g.hitRegions()
	if got, want := prX+prW, g.Size().Width-barW; got != want {
		t.Errorf("pinned-right ends at %v, want %v (grid width %v less the %v bar)", got, want, g.Size().Width, barW)
	}
	if pos, size := g.pinnedRightCells.Position(), g.pinnedRightCells.Size(); pos.X != prX || size.Width != prW {
		t.Errorf("pinned-right band at x=%v w=%v, want the anchor x=%v w=%v", pos.X, size.Width, prX, prW)
	}
	if hx, hpr, _ := g.HeaderRegions(); hpr != prX {
		t.Errorf("header pinned-right at %v (main at %v), want the body's %v", hpr, hx, prX)
	}
	// A pointer in the gutter is over no column; one just inside pinned-right
	// is over it.
	if col := g.columnAtX(g.Size().Width - barW/2); col != -1 {
		t.Errorf("columnAtX in the gutter = %d, want -1", col)
	}
	if col := g.columnAtX(prX + 1); col != 3 {
		t.Errorf("columnAtX just inside pinned-right = %d, want 3", col)
	}

	// Rows that fit leave no vertical bar, and no gutter.
	g2 := newPinnedRightGrid(t, 2, geo.NewSize(500, 300))
	if _, prX, prW := g2.hitRegions(); prX+prW != g2.Size().Width {
		t.Errorf("no vertical bar: pinned-right ends at %v, want the grid's right edge %v", prX+prW, g2.Size().Width)
	}
}

// Scrolled fully right, the last main column is entirely left of pinned-right:
// the scroll content has trailing room for the band and the gutter. It used to
// end under pinned-right, so the column's last 100 px could never be seen.
func TestDataGrid_LastMainColumnScrollsClearOfPinnedRight(t *testing.T) {
	for _, rows := range []int{2, 40} { // without and with the vertical bar
		t.Run(fmt.Sprintf("%d rows", rows), func(t *testing.T) {
			g := newPinnedRightGrid(t, rows, geo.NewSize(500, 300))
			g.scrollContent.ScrollTo(geo.NewPos(1e6, 0))
			g.Refresh()

			mainX, prX, _ := g.hitRegions()
			x, w := g.viewport.ColX(2)
			right := mainX + x + w - g.scrollContent.Offset().X
			if right > prX {
				t.Errorf("last main column ends at %v, past pinned-right at %v (offset %v)", right, prX, g.scrollContent.Offset().X)
			}
			if gap := prX - right; gap > g.cellPadding()+1 {
				t.Errorf("last main column ends %v px short of pinned-right; the scroll overshoots", gap)
			}
		})
	}
}

// The main band covers everything past its visible right edge — under
// pinned-right and in the gutter — so a main column passing under pinned-right
// cannot show its text in the gutter.
func TestDataGrid_MainBandCoversTheGutter(t *testing.T) {
	g := newPinnedRightGrid(t, 40, geo.NewSize(500, 300))
	contentW := g.cellsWidget.Size().Width
	viewRight := g.viewport.ViewRight

	covers := 0
	for _, o := range test.WidgetRenderer(g.cellsWidget).Objects() {
		r, ok := o.(*canvas.Rectangle)
		if !ok || !r.Visible() || r.Position().X != viewRight {
			continue
		}
		covers++
		if r.Position().X+r.Size().Width != contentW {
			t.Errorf("cover spans [%v, %v), want to the content's right edge %v", r.Position().X, r.Position().X+r.Size().Width, contentW)
		}
	}
	if first, last := g.viewport.VisibleRows.Min, g.viewport.VisibleRows.Max; covers != last-first+1 {
		t.Errorf("%d covers for visible rows %d..%d, want one per row", covers, first, last)
	}
}

// With the body overflowing horizontally, pinned-right stops above the
// horizontal bar, so the bar's thumb is not hidden under it at the far right.
func TestDataGrid_PinnedRightKeepsClearOfHorizontalBar(t *testing.T) {
	g := newPinnedRightGrid(t, 40, geo.NewSize(500, 300))
	barW := scrollBarWidth(t, g)
	if got, want := g.pinnedRightCells.Size().Height, g.scrollContent.Scroll.Size().Height-barW; got != want {
		t.Errorf("pinned-right height = %v, want %v (the body less the %v bar)", got, want, barW)
	}
}

// SizeColumnsToFit leaves room for the gutter, so fitted columns do not
// overflow by the bar's width and bring up a horizontal bar.
func TestDataGrid_SizeColumnsToFitLeavesTheGutter(t *testing.T) {
	g := newPinnedRightGrid(t, 40, geo.NewSize(500, 300))
	g.API().SizeColumnsToFit()
	if content, scroll := g.cellsWidget.MinSize().Width, g.scrollContent.Scroll.Size().Width; content > scroll {
		t.Errorf("after SizeColumnsToFit the content is %v wide in a %v scroll: a horizontal bar shows", content, scroll)
	}
	if _, prX, prW := g.hitRegions(); prX+prW > g.Size().Width-scrollBarWidth(t, g) {
		t.Errorf("after SizeColumnsToFit pinned-right ends at %v, inside the gutter", prX+prW)
	}
}
