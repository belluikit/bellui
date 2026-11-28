package widget

import (
	"fmt"
	"sort"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rtlTable is a table wider and taller than its window, with a header row
// and column and one sticky column, so it scrolls both ways.
func rtlTable() *Table {
	tb := NewTableWithHeaders(
		func() (int, int) { return 40, 12 },
		func() core.CanvasObject { return NewLabel("cell") },
		func(id TableCellID, o core.CanvasObject) { o.(*Label).SetText(fmt.Sprintf("%d,%d", id.Row, id.Col)) })
	tb.StickyColumnCount = 1
	return tb
}

// tableDirections runs f left-to-right, right-to-left app-wide, and
// right-to-left as the window's own direction in a left-to-right app.
func tableDirections(t *testing.T, f func(t *testing.T, rtl bool, w bellui.Window, tb *Table)) {
	for _, c := range []struct {
		name   string
		global bellui.Direction
		window bool
	}{
		{"ltr", bellui.DirectionLTR, false},
		{"rtl", bellui.DirectionRTL, false},
		{"window", bellui.DirectionLTR, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(c.global)
			defer bellui.SetLayoutDirection(bellui.DirectionAuto)
			tb := rtlTable()
			w := test.NewTempWindow(t, tb)
			if c.window {
				w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
			}
			w.Resize(geo.NewSize(420, 300))
			f(t, c.name != "ltr", w, tb)
		})
	}
}

// drawn is where o is drawn on the table: its left edge and width.
func drawn(tb *Table, o core.CanvasObject) (x, width float32) {
	d := bellui.CurrentApp().Driver()
	return d.AbsolutePositionForObject(o).X - d.AbsolutePositionForObject(tb).X, o.Size().Width
}

// leading is how far o is drawn from the table's leading edge: its left
// edge left-to-right, its right edge right-to-left.
func leading(tb *Table, rtl bool, o core.CanvasObject) float32 {
	x, w := drawn(tb, o)
	if rtl {
		return tb.Size().Width - x - w
	}
	return x
}

// tableLayout records how far from the leading edge every realized cell,
// header, visible divider and the selection marker are drawn.
func tableLayout(t *testing.T, tb *Table, rtl bool) map[string]float32 {
	t.Helper()
	r := test.TempWidgetRenderer(t, tb.cells).(*tableCellsRenderer)
	got := map[string]float32{}
	for id, c := range r.visible {
		got[fmt.Sprintf("cell %d,%d", id.Row, id.Col)] = leading(tb, rtl, c)
	}
	for id, h := range r.headers {
		got[fmt.Sprintf("header %d,%d", id.Row, id.Col)] = leading(tb, rtl, h)
	}
	var divs []float32
	for _, d := range r.dividers {
		if d.Visible() && d.Size().Width < tb.Size().Width {
			divs = append(divs, leading(tb, rtl, d))
		}
	}
	sort.Slice(divs, func(i, j int) bool { return divs[i] < divs[j] })
	for i, x := range divs {
		got[fmt.Sprintf("divider %d", i)] = x
	}
	if r.marker.Visible() {
		got["marker"] = leading(tb, rtl, r.marker)
	}
	// The header row's background spans the whole table from its strip's
	// start, so where it starts says nothing; TestTable_RTLKeepsScrollBarStrip
	// checks that it covers the strip.
	for name, bg := range map[string]core.CanvasObject{
		"header column background": r.headColBG,
		"corner column background": r.headColStickyBG, "corner row background": r.headRowStickyBG,
	} {
		if bg.Visible() {
			got[name] = leading(tb, rtl, bg)
		}
	}
	return got
}

// Right-to-left, a Table is its left-to-right layout mirrored: the header
// column and sticky column at the right, column 1 next to them, and every
// cell, header, divider and the selection marker as far from the right edge
// as it is from the left one left-to-right. That holds at the start, after
// scrolling both ways, after a column is widened, and after scrolling back
// to the leading edge (#779). It used to keep the left-to-right shape, and
// an overflowing table opened scrolled to its last columns.
func TestTable_RTLMirrorsLayout(t *testing.T) {
	steps := []struct {
		name string
		do   func(tb *Table)
	}{
		{"start", func(tb *Table) { tb.Select(TableCellID{Row: 1, Col: 2}) }},
		{"scrolled", func(tb *Table) { tb.ScrollTo(TableCellID{Row: 20, Col: 9}) }},
		{"widened", func(tb *Table) {
			tb.SetColumnWidth(8, 140)
			// The cells' MinSize grew; a driver's next frame lays the
			// scroll out again for it, which resizes the cells.
			tb.content.Refresh()
		}},
		{"leading", func(tb *Table) { tb.ScrollToLeading(); tb.ScrollToTop() }},
	}
	layouts := map[string][]map[string]float32{}
	tableDirections(t, func(t *testing.T, rtl bool, w bellui.Window, tb *Table) {
		for _, s := range steps {
			s.do(tb)
			layouts[t.Name()] = append(layouts[t.Name()], tableLayout(t, tb, rtl))
		}
	})
	ltr := layouts["TestTable_RTLMirrorsLayout/ltr"]
	require.Len(t, ltr, len(steps))
	for _, name := range []string{"rtl", "window"} {
		got := layouts["TestTable_RTLMirrorsLayout/"+name]
		require.Len(t, got, len(steps), name)
		for i, s := range steps {
			require.NotEmpty(t, ltr[i], "%s: something is drawn", s.name)
			assert.Contains(t, ltr[i], "marker", "%s: the selection is drawn", s.name)
			for k, want := range ltr[i] {
				if assert.Contains(t, got[i], k, "%s %s: %s is drawn", name, s.name, k) {
					assert.InDelta(t, want, got[i][k], 0.5, "%s %s: %s's distance from the leading edge", name, s.name, k)
				}
			}
			assert.Len(t, got[i], len(ltr[i]), "%s %s: the same parts are drawn", name, s.name)
		}
	}
}

// A tap selects the cell drawn under it, a header divider drawn between two
// columns resizes the one on its leading side when dragged away from it,
// and the arrow keys move the focus the way they point: right-to-left, Left
// goes toward the last column (#779).
func TestTable_RTLInput(t *testing.T) {
	tableDirections(t, func(t *testing.T, rtl bool, w bellui.Window, tb *Table) {
		r := test.TempWidgetRenderer(t, tb.cells).(*tableCellsRenderer)
		pad := tb.Theme().Size(theme.SizeNamePadding)

		for _, id := range []TableCellID{{Row: 2, Col: 3}, {Row: 1, Col: 0}} {
			c := r.visible[id]
			require.NotNil(t, c, "cell %v is realized", id)
			x, width := drawn(tb, c)
			y := bellui.CurrentApp().Driver().AbsolutePositionForObject(c).Y - bellui.CurrentApp().Driver().AbsolutePositionForObject(tb).Y
			tb.Tapped(&event.PointEvent{Position: geo.NewPos(x+width/2, y+c.Size().Height/2)})
			require.NotNil(t, tb.selectedCell)
			assert.Equal(t, id, *tb.selectedCell, "a tap on cell %v selects it", id)
		}

		// The divider after column 2: right of it left-to-right, left of it
		// right-to-left.
		head := r.headers[TableCellID{Row: -1, Col: 2}]
		require.NotNil(t, head)
		x, width := drawn(tb, head)
		gap, out := x+width+pad/2, float32(20)
		if rtl {
			gap, out = x-pad/2, -20
		}
		at := geo.NewPos(gap, head.Size().Height/2)
		tb.hoverAt(at)
		require.Equal(t, 2, tb.hoverHeaderCol, "the divider after column 2 is under the pointer")
		tb.tapped(at)
		before := head.Size().Width
		tb.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: at.AddXY(out, 0)}, Dragged: geo.NewDelta(out, 0)})
		tb.DragEnd()
		assert.InDelta(t, before+20, tb.columnWidths[2], 0.5, "dragging the divider away from column 2 widens it")

		lastward, firstward := event.KeyRight, event.KeyLeft
		if rtl {
			lastward, firstward = event.KeyLeft, event.KeyRight
		}
		tb.currentFocus = TableCellID{Row: 0, Col: 1}
		tb.TypedKey(&event.KeyEvent{Name: lastward})
		assert.Equal(t, 2, tb.currentFocus.Col, "the arrow pointing toward the last column moves to it")
		tb.TypedKey(&event.KeyEvent{Name: firstward})
		tb.TypedKey(&event.KeyEvent{Name: firstward})
		assert.Equal(t, 0, tb.currentFocus.Col, "the other arrow moves toward column 0")
	})
}

// Right-to-left, an overflowing table opens at its first columns, on the
// right, not scrolled to its last ones (#779).
func TestTable_RTLOpensAtFirstColumn(t *testing.T) {
	tableDirections(t, func(t *testing.T, rtl bool, w bellui.Window, tb *Table) {
		r := test.TempWidgetRenderer(t, tb.cells).(*tableCellsRenderer)
		assert.Contains(t, r.visible, TableCellID{Row: 0, Col: 1}, "column 1 is realized")
		assert.NotContains(t, r.visible, TableCellID{Row: 0, Col: 11}, "the last column is not")
		assert.Zero(t, tb.offset.X, "the scroll is at the leading edge")
		var cells int
		driver.WalkVisibleObjectTree(tb, func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			if _, ok := o.(*tableCellHost); ok {
				cells++
			}
			return false
		}, nil)
		assert.NotZero(t, cells)
	})
}

// A table that scrolls only vertically has a strip the vertical scroll bar
// reserves on desktop. Right-to-left, the Scroll puts that strip at its left
// edge, the table's outer edge (#1496), so the whole table is its
// left-to-right layout mirrored: every cell, header, divider and the
// selection marker as far from the right edge as it is from the left one
// left-to-right, the header column at the right, and column 0 beside it, not
// beside the bar. A tap on a cell selects it, and the header row's background
// covers the whole header strip, over the bar's strip too. A touch device
// reserves no strip, and the table mirrors the same. Before #1496 the strip
// stayed at the right of the cells, between them and the header column.
func TestTable_RTLScrollBarStripAtOuterEdge(t *testing.T) {
	layouts := map[string]map[string]float32{}
	tableDirections(t, func(t *testing.T, rtl bool, w bellui.Window, _ *Table) {
		tb := NewTableWithHeaders(
			func() (int, int) { return 40, 3 },
			func() core.CanvasObject { return NewLabel("cell") },
			func(id TableCellID, o core.CanvasObject) { o.(*Label).SetText(fmt.Sprintf("%d,%d", id.Row, id.Col)) })
		w.SetContent(tb)
		w.Resize(geo.NewSize(420, 300))
		r := test.TempWidgetRenderer(t, tb.cells).(*tableCellsRenderer)
		viewport := tb.content.Size().Width
		bar := viewport - tb.cells.Size().Width
		if bellui.CurrentDevice().IsMobile() {
			// A touch device's bar overlays the cells and reserves nothing.
			require.Zero(t, bar, "a mobile scroll reserves no strip for its bar")
		} else {
			require.Greater(t, bar, float32(0), "the scroll reserves a strip for its vertical bar")
		}

		tb.Select(TableCellID{Row: 1, Col: 1})
		layouts[t.Name()] = tableLayout(t, tb, rtl)

		for col := 0; col < 3; col++ {
			c := r.visible[TableCellID{Row: 0, Col: col}]
			require.NotNil(t, c)
			cx, cw := drawn(tb, c)
			if rtl {
				assert.GreaterOrEqual(t, cx, bar-0.5, "column %d is clear of the scroll bar's strip, at the left", col)
			} else {
				assert.LessOrEqual(t, cx+cw, tb.Size().Width-bar+0.5, "column %d is clear of the scroll bar's strip, at the right", col)
			}
		}

		// A tap on column 2, the one beside the strip right-to-left, selects
		// it: hit-testing maps the mirrored x back.
		c := r.visible[TableCellID{Row: 2, Col: 2}]
		require.NotNil(t, c)
		cx, cw := drawn(tb, c)
		cy := bellui.CurrentApp().Driver().AbsolutePositionForObject(c).Y - bellui.CurrentApp().Driver().AbsolutePositionForObject(tb).Y
		tb.Tapped(&event.PointEvent{Position: geo.NewPos(cx+cw/2, cy+c.Size().Height/2)})
		require.NotNil(t, tb.selectedCell)
		assert.Equal(t, TableCellID{Row: 2, Col: 2}, *tb.selectedCell, "a tap on column 2 selects it")

		// The header row's background covers the whole header strip, over
		// the bar's strip too, on every refresh.
		for i := 0; i < 2; i++ {
			tb.Refresh()
			bx, bw := drawn(tb, r.headRowBG)
			tx, tw := drawn(tb, tb.top)
			assert.LessOrEqual(t, bx, tx+0.5, "the header row's background starts at or before its strip")
			assert.GreaterOrEqual(t, bx+bw, tx+tw-0.5, "the header row's background reaches the end of its strip")
		}

		if rtl {
			rowHead := r.headers[TableCellID{Row: 0, Col: -1}]
			require.NotNil(t, rowHead)
			rx, rw := drawn(tb, rowHead)
			assert.InDelta(t, tb.Size().Width, rx+rw, 0.5, "the header column is at the right edge")
		}
	})
	ltr := layouts["TestTable_RTLScrollBarStripAtOuterEdge/ltr"]
	require.NotEmpty(t, ltr)
	assert.Contains(t, ltr, "marker", "the selection is drawn")
	for _, name := range []string{"rtl", "window"} {
		got := layouts["TestTable_RTLScrollBarStripAtOuterEdge/"+name]
		for k, want := range ltr {
			if assert.Contains(t, got, k, "%s: %s is drawn", name, k) {
				assert.InDelta(t, want, got[k], 0.5, "%s: %s's distance from the leading edge", name, k)
			}
		}
		assert.Len(t, got, len(ltr), "%s: the same parts are drawn", name)
	}
}
