package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// resizeGrid shows a one-row grid of the given columns in a window of the
// given width.
func resizeGrid(t *testing.T, width float32, cols []*ColumnDef) *DataGrid {
	t.Helper()
	return resizeGridRows(t, width, 1, cols)
}

// resizeGridRows is resizeGrid with the given number of rows.
func resizeGridRows(t *testing.T, width float32, rows int, cols []*ColumnDef) *DataGrid {
	t.Helper()
	data := make([]map[string]any, rows)
	for i := range data {
		data[i] = map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}
	}
	g := newTestGrid(t, data, cols)
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(width, 300))
	return g
}

// dragBorder drags column col's header border by dx and returns its width.
func dragBorder(g *DataGrid, col int, dx float32) float32 {
	const startX = 300
	g.OnResizeDragStart(col, startX)
	g.OnResizeDrag(startX + dx)
	g.OnResizeDragEnd()
	return g.visibleCol(col).width
}

func fourColumns() []*ColumnDef {
	return []*ColumnDef{
		{Field: "a", Width: 150},
		{Field: "b", Width: 150},
		{Field: "c", Width: 100},
		{Field: "d", Width: 120},
	}
}

// TestDataGrid_ResizeInOverflowingGrid pins #1499: in a grid whose columns
// overflow it, so the body scrolls sideways, every border drag collapsed the
// column to its minimum, because the drag capped the width at the room left
// beside the other columns, which was negative.
func TestDataGrid_ResizeInOverflowingGrid(t *testing.T) {
	g := resizeGrid(t, 400, fourColumns())

	if got := dragBorder(g, 1, 10); got != 160 {
		t.Errorf("width after widening column 1 by 10 = %v, want 160", got)
	}
	if got := dragBorder(g, 1, -30); got != 130 {
		t.Errorf("width after narrowing column 1 by 30 = %v, want 130", got)
	}
	// The floors still hold.
	if got := dragBorder(g, 1, -1000); got != 40 {
		t.Errorf("width after narrowing column 1 past its minimum = %v, want 40", got)
	}
}

// TestDataGrid_ResizeHonoursMaxWidth checks that an overflowing grid still
// stops a drag at the column's MaxWidth.
func TestDataGrid_ResizeHonoursMaxWidth(t *testing.T) {
	cols := fourColumns()
	cols[1].MaxWidth = 170
	g := resizeGrid(t, 400, cols)

	if got := dragBorder(g, 1, 100); got != 170 {
		t.Errorf("width after widening column 1 past its MaxWidth = %v, want 170", got)
	}
}

// TestDataGrid_ResizeGrowsPastGridEdge checks that a main column may grow past
// the grid's right edge, as the body then scrolls sideways to its border. The
// drag used to stop where the columns filled the grid.
func TestDataGrid_ResizeGrowsPastGridEdge(t *testing.T) {
	g := resizeGrid(t, 600, fourColumns())

	if got := dragBorder(g, 1, 10); got != 160 {
		t.Errorf("width after widening column 1 by 10 = %v, want 160", got)
	}
	if got := dragBorder(g, 1, 300); got != 460 {
		t.Errorf("width after widening column 1 past the grid's edge = %v, want 460", got)
	}
	if got := dragBorder(g, 1, 1000); got != 1460 {
		t.Errorf("width after widening column 1 past the grid's whole width = %v, want 1460", got)
	}
	if w, content := g.Size().Width, g.viewport.ContentSize().Width; content <= w {
		t.Errorf("content width %v after the drag, want wider than the grid (%v)", content, w)
	}
}

// TestDataGrid_ResizePinnedInOverflowingGrid checks a pinned column: it
// resizes by the drag in an overflowing grid too, and stops where the two
// pinned bands fill the grid, since it does not scroll.
func TestDataGrid_ResizePinnedInOverflowingGrid(t *testing.T) {
	g := resizeGrid(t, 400, []*ColumnDef{
		{Field: "a", Width: 100, Pinned: PinLeft},
		{Field: "b", Width: 200},
		{Field: "c", Width: 200},
		{Field: "d", Width: 100, Pinned: PinRight},
	})

	if got := dragBorder(g, 0, 10); got != 110 {
		t.Errorf("width after widening the pinned-left column by 10 = %v, want 110", got)
	}
	// Both bands end in a gap; the body has no vertical bar (one row).
	want := g.Size().Width - 100 - 2*g.cellPadding()
	if got := dragBorder(g, 0, 1000); got != want {
		t.Errorf("width after widening the pinned-left column past the grid = %v, want %v", got, want)
	}
}

// TestDataGrid_ResizePinnedKeepsFloor checks that when the other pinned
// columns leave less room than a pinned column's minimum, a drag leaves it at
// that minimum rather than narrower.
func TestDataGrid_ResizePinnedKeepsFloor(t *testing.T) {
	g := resizeGrid(t, 200, []*ColumnDef{
		{Field: "a", Width: 100, Pinned: PinLeft},
		{Field: "b", Width: 200},
		{Field: "d", Width: 150, Pinned: PinRight},
	})
	if room := g.Size().Width - 150 - 2*g.cellPadding(); room >= 40 {
		t.Fatalf("the test needs less room than the 40 px floor, have %v", room)
	}
	if got := dragBorder(g, 0, 10); got != 40 {
		t.Errorf("width after widening a pinned column with no room = %v, want 40", got)
	}
}

// TestDataGrid_ResizePinnedRightInOverflowingGrid is the pinned-right case. The
// grid has enough rows for a vertical bar, whose gutter pinned-right keeps
// clear (#1257), so the cap counts it too.
func TestDataGrid_ResizePinnedRightInOverflowingGrid(t *testing.T) {
	g := resizeGridRows(t, 400, 50, []*ColumnDef{
		{Field: "a", Width: 100, Pinned: PinLeft},
		{Field: "b", Width: 200},
		{Field: "c", Width: 200},
		{Field: "d", Width: 100, Pinned: PinRight},
	})
	gutter := g.Theme().Size(bellui.ThemeSizeName("scrollBar"))
	if g.vScrollGutter(g.Size()) != gutter || gutter <= 0 {
		t.Fatalf("the test needs the vertical bar's gutter, have %v", g.vScrollGutter(g.Size()))
	}

	if got := dragBorder(g, 3, 10); got != 110 {
		t.Errorf("width after widening the pinned-right column by 10 = %v, want 110", got)
	}
	want := g.Size().Width - 100 - 2*g.cellPadding() - gutter
	if got := dragBorder(g, 3, 1000); got != want {
		t.Errorf("width after widening the pinned-right column past the grid = %v, want %v", got, want)
	}
}
