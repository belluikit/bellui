package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cursorGridCases are the gutters the cursor tests run over: none (the
// control), a 1-digit gutter ("2|") and a 2-digit gutter ("12|").
var cursorGridCases = []struct {
	lineNumbers bool
	lines       int
	gutter      int // drawn cells before the text
}{
	{lineNumbers: false, lines: 2, gutter: 0},
	{lineNumbers: true, lines: 2, gutter: 2},
	{lineNumbers: true, lines: 12, gutter: 3},
}

// newCursorGrid shows a grid of `lines` rows of "abcdef" in a 300×100 window.
func newCursorGrid(t *testing.T, lineNumbers bool, lines int, scroll bellui.ScrollDirection, row string) *TextGrid {
	t.Helper()
	test.NewTempApp(t)
	g := NewTextGrid()
	g.Scroll = scroll
	g.ShowLineNumbers = lineNumbers
	g.SetText(strings.TrimSuffix(strings.Repeat(row+"\n", lines), "\n"))
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 100))
	return g
}

// drawnCellPos is where row widget `row` draws cell `cell` (counting the
// gutter's cells), in the grid's own coordinates: what the user points at.
func drawnCellPos(t *testing.T, g *TextGrid, row, cell int) geo.Position {
	t.Helper()
	// A scrolled-to row can be in the pool before anything has laid it out;
	// lay it out as the driver's next frame would.
	cache.Renderer(g.content).Layout(g.content.Size())
	r := gutterTestRow(t, g, row)
	cache.Renderer(r).Layout(r.Size())
	require.Less(t, cell*3, len(r.objects), "row %d has no cell %d", row, cell)
	pos := r.Position().Add(r.objects[cell*3].Position())
	if g.scroll != nil && g.scroll.Visible() {
		pos = pos.Subtract(g.scroll.Offset)
	}
	return pos
}

// TestTextGrid_CursorColumnIsTextColumn: col is a text column, the index into
// Rows[row].Cells that SetCell, SetStyle and SetRune take, so the
// line-number gutter is not counted in it (#1434). A point over a row's first
// text cell is col 0, and so is a point over the gutter; col 0's position is
// that first text cell.
func TestTextGrid_CursorColumnIsTextColumn(t *testing.T) {
	for _, tc := range cursorGridCases {
		t.Run(fmt.Sprintf("gutter=%d", tc.gutter), func(t *testing.T) {
			g := newCursorGrid(t, tc.lineNumbers, tc.lines, bellui.ScrollNone, "abcdef")
			require.Equal(t, tc.gutter, g.content.gutterCells(), "drawn cells before the text")
			cell := g.content.cellSize
			half := geo.NewPos(cell.Width/2, cell.Height/2)

			first := drawnCellPos(t, g, 1, tc.gutter)
			row, col := g.CursorLocationForPosition(first.Add(half))
			assert.Equal(t, [2]int{1, 0}, [2]int{row, col}, "a point over the first text cell")

			row, col = g.CursorLocationForPosition(drawnCellPos(t, g, 1, tc.gutter+2).Add(half))
			assert.Equal(t, [2]int{1, 2}, [2]int{row, col}, "a point over the third text cell")

			for k := 0; k < tc.gutter; k++ {
				row, col = g.CursorLocationForPosition(drawnCellPos(t, g, 1, k).Add(half))
				assert.Equal(t, [2]int{1, 0}, [2]int{row, col}, "a point over gutter cell %d goes to the line's start", k)
			}

			assert.Equal(t, first, g.PositionForCursorLocation(1, 0), "col 0 is the first text cell")
			assert.Equal(t, drawnCellPos(t, g, 1, tc.gutter+3), g.PositionForCursorLocation(1, 3))
		})
	}
}

// TestTextGrid_CursorLocationRoundTrip: a cell's position maps back to the
// same row and column, unscrolled and scrolled both ways.
func TestTextGrid_CursorLocationRoundTrip(t *testing.T) {
	for _, tc := range cursorGridCases {
		for _, scrolled := range []bool{false, true} {
			t.Run(fmt.Sprintf("gutter=%d/scrolled=%v", tc.gutter, scrolled), func(t *testing.T) {
				// 20 rows of 80 cells overflow the 300×100 view both ways.
				lines := max(tc.lines, 20)
				if tc.gutter == 2 {
					lines = 8 // keep the gutter at one digit
				}
				g := newCursorGrid(t, tc.lineNumbers, lines, bellui.ScrollBoth, strings.Repeat("x", 80))
				require.Equal(t, tc.gutter, g.content.gutterCells())
				cell := g.content.cellSize
				if scrolled {
					g.scroll.ScrollToOffset(geo.NewPos(5*cell.Width+3, 2*cell.Height+4))
					g.Refresh()
					require.NotEqual(t, float32(0), g.scroll.Offset.X, "scrolled across")
					require.NotEqual(t, float32(0), g.scroll.Offset.Y, "scrolled down")
				}
				half := geo.NewPos(cell.Width/2, cell.Height/2)
				for r := 2; r < 5; r++ {
					for c := 4; c < 12; c++ {
						p := g.PositionForCursorLocation(r, c)
						assert.Equal(t, drawnCellPos(t, g, r, tc.gutter+c), p, "row %d col %d is drawn there", r, c)
						row, col := g.CursorLocationForPosition(p.Add(half))
						assert.Equal(t, [2]int{r, c}, [2]int{row, col}, "row %d col %d round-trips", r, c)
					}
				}
			})
		}
	}
}

// TestTextGrid_ClickToStyle: styling the location under a point styles the
// cell the user pointed at, not one a gutter's width further right.
func TestTextGrid_ClickToStyle(t *testing.T) {
	for _, tc := range cursorGridCases {
		t.Run(fmt.Sprintf("gutter=%d", tc.gutter), func(t *testing.T) {
			g := newCursorGrid(t, tc.lineNumbers, tc.lines, bellui.ScrollNone, "abcdef")
			cell := g.content.cellSize
			under := drawnCellPos(t, g, 1, tc.gutter+2)

			row, col := g.CursorLocationForPosition(under.Add(geo.NewPos(cell.Width/2, cell.Height/2)))
			g.SetStyle(row, col, &CustomTextGridStyle{BGColor: gutterTestRed})

			left, right := filledCells(t, g, 1)
			assert.Equal(t, under.X, left, "the styled cell is the one under the point")
			assert.Equal(t, under.X+cell.Width, right, "and only that one")
		})
	}
}

// TestTextGrid_SetTextScrollsToLineStart: shortening a scrolled grid's text
// scrolls it back to its drawn left edge, line numbers in view, as it did
// before col became a text column. SetText takes the offset from the cursor
// position of a column; col 0's position is now the first text cell, so
// taken as it is the offset would scroll the gutter out of view.
func TestTextGrid_SetTextScrollsToLineStart(t *testing.T) {
	for _, tc := range cursorGridCases {
		for _, cellsAcross := range []float32{0, 1, 6} {
			t.Run(fmt.Sprintf("gutter=%d/scrolledCells=%v", tc.gutter, cellsAcross), func(t *testing.T) {
				lines := 30
				if tc.gutter == 2 {
					lines = 8
				}
				g := newCursorGrid(t, tc.lineNumbers, lines, bellui.ScrollBoth, strings.Repeat("x", 80))
				cell := g.content.cellSize
				g.scroll.ScrollToOffset(geo.NewPos(cellsAcross*cell.Width, 2*cell.Height))
				g.Refresh()
				require.Equal(t, cellsAcross*cell.Width, g.scroll.Offset.X)

				g.SetText(strings.TrimSuffix(strings.Repeat(strings.Repeat("y", 80)+"\n", lines-1), "\n"))
				assert.Equal(t, float32(0), g.scroll.Offset.X, "scrolled back to the drawn left edge")
			})
		}
	}
}
