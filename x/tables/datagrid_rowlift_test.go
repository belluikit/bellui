package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// rowEdge is a point in column a of display row r, frac of the way down its
// cells (0.25 is its upper half, 0.75 its lower half).
func rowEdge(g *DataGrid, r int, frac float32) geo.Position {
	y, h := g.viewport.RowY(r)
	return geo.NewPos(40, g.bodyTop()+y-g.offset.Y+h*frac)
}

// mouseDragRow presses on display row from and drags through the points,
// leaving the drag under way.
func mouseDragRow(g *DataGrid, from int, to ...geo.Position) {
	g.MouseDown(pressAt(rowAt(g, from), 0))
	for _, p := range to {
		g.Dragged(dragTo(p, geo.Delta{DY: 10}))
	}
}

func release(g *DataGrid, p geo.Position) {
	g.MouseUp(pressAt(p, 0))
	g.DragEnd()
}

// The drop rule (#1512, owner decision 4): over the upper half of a row the
// dragged row goes before it, over the lower half after it, and OnRowMoved
// gets the place it ends up at.
func TestDataGrid_LiftDropsBetweenRows(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from     int
		over     int
		frac     float32
		wantMove [][2]int
	}{
		{"down, upper half: before row 4", 1, 4, 0.25, [][2]int{{1, 3}}},
		{"down, lower half: after row 4", 1, 4, 0.75, [][2]int{{1, 4}}},
		{"up, upper half: before row 0", 3, 0, 0.25, [][2]int{{3, 0}}},
		{"up, lower half: after row 0", 3, 0, 0.75, [][2]int{{3, 1}}},
		{"back on itself: no move", 2, 2, 0.25, [][2]int{}},
		{"just below itself: no move", 2, 2, 0.75, [][2]int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, moved := pressGrid(t)
			p := rowEdge(g, tc.over, tc.frac)
			mouseDragRow(g, tc.from, p)
			require.True(t, g.lift.active, "the drag lifted the row")
			release(g, p)
			assert.Equal(t, tc.wantMove, *moved)
			assert.False(t, g.lift.active)
			assert.False(t, g.rowLiftLayer.Visible(), "the copy went away with the drag")
			assert.False(t, g.dropLine.Visible())
		})
	}
}

// While a row is lifted, a copy of it follows the pointer, keeping the point
// the press took it by under it, and a line shows where it will land.
func TestDataGrid_LiftFollowsThePointer(t *testing.T) {
	g, _ := pressGrid(t)
	press := rowAt(g, 1)
	y1, _ := g.viewport.RowY(1)
	grab := press.Y - g.bodyTop() - y1

	p := rowEdge(g, 4, 0.25)
	mouseDragRow(g, 1, p)
	require.True(t, g.rowLiftLayer.Visible())
	r, ok := test.TempWidgetRenderer(t, g.rowLiftLayer).(*rowSwipeLayerRenderer)
	require.True(t, ok)
	// The copy's top is grab above the pointer, in the body.
	assert.InDelta(t, p.Y-g.bodyTop()-grab, r.band.Position().Y+r.clip.Position().Y, 0.5)

	require.True(t, g.dropLine.Visible())
	assert.Equal(t, 4, g.lift.insert)
	lineMid := g.dropLine.Position().Y + g.dropLine.Size().Height/2
	y3, h3 := g.viewport.RowY(3)
	y4, _ := g.viewport.RowY(4)
	require.Greater(t, y4, y3+h3, "rows have a gap between them")
	assert.InDelta(t, g.bodyTop()+(y3+h3+y4)/2, lineMid, 0.5, "the line is in the middle of the gap between rows 3 and 4")
	release(g, p)
}

// Escape puts the lifted row back: the drag's end then moves nothing.
func TestDataGrid_LiftEscapeCancels(t *testing.T) {
	g, moved := pressGrid(t)
	p := rowEdge(g, 5, 0.25)
	mouseDragRow(g, 1, p)
	require.True(t, g.lift.active)

	g.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, g.lift.active)
	assert.False(t, g.rowLiftLayer.Visible())
	release(g, p)
	assert.Empty(t, *moved, "a cancelled drag moved nothing")
	assert.Equal(t, -1, g.dragRow)
}

// Near the body's bottom edge the body scrolls under a still pointer, faster
// nearer the edge, and the row lands where the line has moved to; at the end
// of the rows it stops. Near the top with the body at its start, nothing
// scrolls.
func TestDataGrid_LiftScrollsAtTheEdges(t *testing.T) {
	g, moved := pressGrid(t)
	bodyH := g.bodyHeight(g.Size().Height)
	near := geo.NewPos(40, g.bodyTop()+bodyH-reorderEdge/2)
	mouseDragRow(g, 1, near)
	g.stopEdgeScroll() // the test steps it by hand
	half := g.edgeScrollSpeed()
	require.Positive(t, half, "near the bottom it scrolls down")

	g.lift.pointerY = bodyH - 1
	assert.Greater(t, g.edgeScrollSpeed(), half, "faster at the edge itself")
	g.lift.pointerY = near.Y - g.bodyTop()

	insertBefore := g.lift.insert
	require.True(t, g.edgeScrollStep())
	assert.InDelta(t, half, g.scrollContent.Offset().Y, 0.01, "one step scrolled the body")
	for g.edgeScrollStep() {
	}
	limit := g.scrollContent.Scroll.Content.MinSize().Height - g.scrollContent.Scroll.Size().Height
	assert.InDelta(t, limit, g.scrollContent.Offset().Y, 0.01, "it stopped at the end")
	assert.Greater(t, g.lift.insert, insertBefore, "the line moved with the rows")
	ins := g.lift.insert

	release(g, near)
	require.Len(t, *moved, 1)
	assert.Equal(t, [2]int{1, ins - 1}, (*moved)[0], "the row landed where the line was")

	// Back at the top, the pointer near the top edge scrolls nothing.
	g.scrollContent.Scroll.ScrollToTop()
	g.Refresh()
	mouseDragRow(g, 3, geo.NewPos(40, g.bodyTop()+2))
	assert.Zero(t, g.edgeScrollSpeed(), "the body cannot scroll up past its start")
	assert.False(t, g.lift.ticking)
	release(g, geo.NewPos(40, g.bodyTop()+2))
}

// The edge scroll's ticker starts when the pointer reaches the edge and ends
// with the drag.
func TestDataGrid_LiftEdgeScrollTickerStops(t *testing.T) {
	g, _ := pressGrid(t)
	bodyH := g.bodyHeight(g.Size().Height)
	near := geo.NewPos(40, g.bodyTop()+bodyH-4)
	mouseDragRow(g, 1, near)
	assert.True(t, g.lift.ticking, "at the edge the ticker runs")
	stop := g.lift.stop
	require.NotNil(t, stop)
	release(g, near)
	assert.False(t, g.lift.ticking)
	assert.Nil(t, g.lift.stop)
	select {
	case <-stop:
	default:
		t.Error("the drag's end did not stop the ticker")
	}
}
