package widget

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var gutterTestRed = color.NRGBA{R: 255, A: 255}

// gutterTestRow is the row widget showing document row `row`.
func gutterTestRow(t *testing.T, g *TextGrid, row int) *textGridRow {
	t.Helper()
	for _, o := range g.content.visible {
		if r := o.(*textGridRow); r.row == row {
			return r
		}
	}
	t.Fatalf("row %d has no visible row widget", row)
	return nil
}

// filledCells is the left and right edges, in the grid's own coordinates, of
// the cells in document row `row` that have a background; left is -1 when
// none do.
func filledCells(t *testing.T, g *TextGrid, row int) (left, right float32) {
	t.Helper()
	r := gutterTestRow(t, g, row)
	left, right = -1, -1
	for i := 0; i+2 < len(r.objects); i += 3 {
		rect := r.objects[i].(*canvas.Rectangle)
		if rect.FillColor == nil {
			continue
		}
		if _, _, _, a := rect.FillColor.RGBA(); a == 0 {
			continue
		}
		x := r.Position().X + rect.Position().X
		if left < 0 || x < left {
			left = x
		}
		right = max(right, x+rect.Size().Width)
	}
	return left, right
}

// rowText is what row `row` draws, one rune per cell.
func rowText(t *testing.T, g *TextGrid, row int) string {
	t.Helper()
	r := gutterTestRow(t, g, row)
	var b strings.Builder
	for i := 1; i < len(r.objects); i += 3 {
		b.WriteString(r.objects[i].(*canvas.Text).Text)
	}
	return b.String()
}

// TestTextGrid_RowStyleEndsAtGridEdge: a row style's background fills its row
// up to the grid's right edge and no further, with or without line numbers
// (1- and 2-digit) and the whitespace marker. Line numbers used to add the
// gutter's cells on top of the cells that fit, so the background ran past
// the edge by about the gutter's width (#1422); ShowWhitespace added one more.
func TestTextGrid_RowStyleEndsAtGridEdge(t *testing.T) {
	for _, tc := range []struct {
		lines  int
		height float32
		digits int
	}{
		{lines: 2, height: 100, digits: 1},
		{lines: 12, height: 400, digits: 2},
	} {
		for _, ln := range []bool{false, true} {
			for _, ws := range []bool{false, true} {
				for _, sc := range []bellui.ScrollDirection{bellui.ScrollNone, bellui.ScrollBoth} {
					name := fmt.Sprintf("lines=%d/lineNumbers=%v/whitespace=%v/scroll=%v", tc.lines, ln, ws, sc)
					t.Run(name, func(t *testing.T) {
						test.NewTempApp(t)
						g := NewTextGrid()
						g.Scroll = sc
						g.ShowLineNumbers = ln
						g.ShowWhitespace = ws
						g.SetText(strings.TrimSuffix(strings.Repeat("ab\n", tc.lines), "\n"))
						w := test.NewWindow(g)
						defer w.Close()
						w.SetPadded(false)
						w.Resize(geo.NewSize(300, tc.height))
						g.SetRowStyle(1, &CustomTextGridStyle{BGColor: gutterTestRed})
						g.Refresh()

						if ln {
							require.Equal(t, tc.digits, g.content.lineNumberWidth(), "line-number digits")
							assert.True(t, strings.HasPrefix(rowText(t, g, 1), strings.Repeat(" ", tc.digits-1)+"2|"),
								"row 1 starts with its line number: %q", rowText(t, g, 1))
						}
						cell := g.content.cellSize.Width
						left, right := filledCells(t, g, 1)
						assert.Equal(t, float32(0), left, "the background starts at the grid's left edge")
						assert.LessOrEqual(t, right, g.Size().Width, "the background ends inside the grid")
						assert.Greater(t, right, g.Size().Width-cell, "the background reaches the grid's last whole cell")
					})
				}
			}
		}
	}
}

// TestTextGrid_LongestRowFitsWithGutter: the longest row's last character is
// drawn, and the grid is wide enough for a scroller to reach it, when the
// line-number gutter and the newline marker share the row with the text. The
// row used to have room for the text plus the digits only, so with line
// numbers its last character (or the newline marker) was dropped, and the
// minimum width ignored both, so the last cells were past the scroll range.
//
// The 400-high view adds blank rows below the two-line document. Those must not
// widen the gutter: the minimum width cannot depend on the height, so a gutter
// that did would again put the row's last cells past the scroll range.
func TestTextGrid_LongestRowFitsWithGutter(t *testing.T) {
	for _, height := range []float32{100, 400} {
		for _, ln := range []bool{false, true} {
			for _, ws := range []bool{false, true} {
				t.Run(fmt.Sprintf("height=%v/lineNumbers=%v/whitespace=%v", height, ln, ws), func(t *testing.T) {
					longestRowTest(t, height, ln, ws)
				})
			}
		}
	}
}

func longestRowTest(t *testing.T, height float32, ln, ws bool) {
	test.NewTempApp(t)
	g := NewTextGrid()
	g.Scroll = bellui.ScrollBoth
	g.ShowLineNumbers = ln
	g.ShowWhitespace = ws
	g.SetText(strings.Repeat("x", 39) + "Z\nab")
	w := test.NewWindow(g)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, height))

	want := strings.Repeat("x", 39) + "Z"
	if ln {
		want = "1|" + want
	}
	if ws {
		want += string(textAreaNewLineSymbol)
	}
	assert.Equal(t, want, rowText(t, g, 0), "the longest row draws all of its cells")
	assert.Equal(t, float32(len([]rune(want)))*g.content.cellSize.Width, g.content.MinSize().Width,
		"the grid's minimum width covers every cell of the longest row")
}

// TestTextGrid_CellUpdatesPaintAtTheirColumn: SetStyle and SetRune on a grid
// that is showing paint the cell straight away, at its column after the
// line-number gutter. The per-cell refresh used to offset the cell by the
// row's whole cell count, which is always past the row's last cell, so it
// drew nothing until something refreshed the whole grid.
func TestTextGrid_CellUpdatesPaintAtTheirColumn(t *testing.T) {
	for _, ln := range []bool{false, true} {
		t.Run(fmt.Sprintf("lineNumbers=%v", ln), func(t *testing.T) {
			test.NewTempApp(t)
			g := NewTextGrid()
			g.ShowLineNumbers = ln
			g.SetText("ab\ncd")
			w := test.NewWindow(g)
			defer w.Close()
			w.SetPadded(false)
			w.Resize(geo.NewSize(300, 100))

			g.SetStyle(1, 1, &CustomTextGridStyle{BGColor: gutterTestRed})
			g.SetRune(1, 0, 'X')

			gutter := 0
			if ln {
				gutter = 2 // "2|"
			}
			cell := g.content.cellSize.Width
			left, right := filledCells(t, g, 1)
			assert.Equal(t, float32(gutter+1)*cell, left, "the styled cell is column 1 of the text")
			assert.Equal(t, float32(gutter+2)*cell, right, "only that cell is styled")
			assert.Equal(t, "Xd", rowText(t, g, 1)[gutter:gutter+2], "SetRune drew its rune")
		})
	}
}

// TestTextGrid_CellUpdateShowsWhitespaceSymbol: a per-cell update to a space
// draws the whitespace symbol when ShowWhitespace is on, as a full refresh
// does, rather than a bare space.
func TestTextGrid_CellUpdateShowsWhitespaceSymbol(t *testing.T) {
	test.NewTempApp(t)
	g := NewTextGrid()
	g.ShowWhitespace = true
	g.SetText("ab\ncd")
	w := test.NewWindow(g)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 100))

	g.SetRune(1, 0, ' ')
	assert.Equal(t, textAreaSpaceSymbol, []rune(rowText(t, g, 1))[0])
}
