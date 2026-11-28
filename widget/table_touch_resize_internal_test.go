package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// headerTable shows a table with a header row and a header column, so the
// header row's strip starts right of the table's origin and the header
// column's below it.
func headerTable(t *testing.T) *Table {
	t.Helper()
	return headerTableWith(t, nil)
}

// headerTableWith is headerTable with header cells made by createHeader, or
// the default ones when it is nil.
func headerTableWith(t *testing.T, createHeader func() core.CanvasObject) *Table {
	t.Helper()
	table := NewTable(
		func() (int, int) { return 10, 4 },
		func() core.CanvasObject {
			r := canvas.NewRectangle(color.Black)
			r.SetMinSize(geo.NewSize(60, 30))
			return r
		},
		func(TableCellID, core.CanvasObject) {},
	)
	table.ShowHeaderRow = true
	table.ShowHeaderColumn = true
	if createHeader != nil {
		table.CreateHeader = createHeader
		table.UpdateHeader = func(TableCellID, core.CanvasObject) {}
	}
	w := test.NewTempWindow(t, table)
	w.Resize(geo.NewSize(400, 400))
	return table
}

// dividerAt scans from from along step for the first point where a pointer
// hovering would find a header divider, as hoverAt decides it.
func dividerAt(t *testing.T, table *Table, from, step geo.Position) geo.Position {
	t.Helper()
	for p := from; p.X < 400 && p.Y < 400; p = p.Add(step) {
		table.hoverAt(p)
		found := table.hoverHeaderCol != noCellMatch || table.hoverHeaderRow != noCellMatch
		table.hoverHeaderCol, table.hoverHeaderRow = noCellMatch, noCellMatch
		if found {
			return p
		}
	}
	t.Fatal("no divider found")
	return geo.Position{}
}

// A long-press near a column divider in the header row, then a drag, widens
// that column by the drag (#1293), measured from where the finger pressed in
// the header strip's own coordinates, which its drags arrive in.
func TestTable_LongPressHeaderResizesColumn(t *testing.T) {
	table := headerTable(t)
	h := table.headerSize.Height / 2
	gap := dividerAt(t, table, geo.NewPos(table.headerSize.Width, h), geo.NewPos(1, 0))
	press := gap.SubtractXY(3, 0) // in column 0, a little short of the divider
	inStrip := press.Subtract(table.top.Position())
	require.NotZero(t, table.top.Position().X, "the header row's strip starts right of the header column")

	table.top.LongPressed(&event.PointEvent{Position: inStrip})
	require.Equal(t, 0, table.dragCol, "the long-press armed column 0's resize")
	assert.Equal(t, noCellMatch, table.dragRow)
	assert.Equal(t, noCellMatch, table.hoverHeaderCol, "a finger leaves no hover behind")
	assert.True(t, table.ClaimsDrag(geo.NewDelta(10, 0)), "the table keeps the held finger's drag")

	table.top.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: inStrip.AddXY(40, 0)}, Dragged: geo.NewDelta(40, 0)})
	table.top.DragEnd()
	assert.InDelta(t, table.cellSize.Width+40, table.columnWidths[0], 0.01)
	assert.Equal(t, noCellMatch, table.dragCol, "the resize ended with the drag")
}

// The same in the header column resizes a row.
func TestTable_LongPressHeaderResizesRow(t *testing.T) {
	table := headerTable(t)
	w := table.headerSize.Width / 2
	gap := dividerAt(t, table, geo.NewPos(w, table.headerSize.Height), geo.NewPos(0, 1))
	press := gap.SubtractXY(0, 3) // in row 0, a little above the divider
	inStrip := press.Subtract(table.left.Position())

	table.left.LongPressed(&event.PointEvent{Position: inStrip})
	require.Equal(t, 0, table.dragRow, "the long-press armed row 0's resize")
	assert.Equal(t, noCellMatch, table.dragCol)

	table.left.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: inStrip.AddXY(0, 25)}, Dragged: geo.NewDelta(0, 25)})
	table.left.DragEnd()
	assert.InDelta(t, table.cellSize.Height+25, table.rowHeights[0], 0.01)
}

// A long-press out of a finger's reach of every divider arms nothing, so its
// drag resizes nothing.
func TestTable_LongPressHeaderAwayFromDivider(t *testing.T) {
	table := headerTable(t)
	h := table.headerSize.Height / 2
	gap := dividerAt(t, table, geo.NewPos(table.headerSize.Width, h), geo.NewPos(1, 0))
	reach := table.Theme().Size(theme.SizeNameIconButtonSize) / 4
	press := gap.SubtractXY(reach+2, 0)

	table.top.LongPressed(&event.PointEvent{Position: press.Subtract(table.top.Position())})
	assert.Equal(t, noCellMatch, table.dragCol)
	assert.Equal(t, noCellMatch, table.dragRow)
	assert.False(t, table.ClaimsDrag(geo.NewDelta(10, 0)))
}

// A long-press on a header away from its dividers answers as the tap it would
// have been (#1293): a held click on a button in a header, such as a sort
// button, still presses it. The desktop fires a long-press for a mouse held
// still and sends no tap for its release.
func TestTable_LongPressHeaderTapsWhatIsUnderIt(t *testing.T) {
	taps := 0
	table := headerTableWith(t, func() core.CanvasObject {
		return NewButton("sort", func() { taps++ })
	})
	h := table.headerSize.Height / 2
	gap := dividerAt(t, table, geo.NewPos(table.headerSize.Width, h), geo.NewPos(1, 0))
	reach := table.Theme().Size(theme.SizeNameIconButtonSize) / 4
	press := gap.SubtractXY(reach+4, 0)

	table.top.LongPressed(&event.PointEvent{Position: press.Subtract(table.top.Position())})
	assert.Equal(t, 1, taps, "the held press tapped the header's button")
	assert.Equal(t, noCellMatch, table.dragCol)

	// Within reach of a divider, even over the button, the long-press grabs
	// the divider and taps nothing.
	table.top.LongPressed(&event.PointEvent{Position: gap.SubtractXY(3, 0).Subtract(table.top.Position())})
	assert.Equal(t, 1, taps, "a long-press that grabbed a divider tapped the button too")
	assert.Equal(t, 0, table.dragCol)
}
