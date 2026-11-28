package tables

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// A mouse press arms a gesture: a row drag on a row (with RowDragEnabled), a
// column resize on a header border, a column reorder on a header. A click, a
// press with no drag, used to leave it armed, and the next drag finished it.
// On a screen with a mouse and a touchscreen that drag can be a finger
// scrolling the grid: it did not scroll, and it moved the clicked row or
// resized the column (#1511). These tests click, then drag as a finger does,
// with no press; and drag with the mouse, to show the gestures still work.
//
// The events come in the glfw driver's order: on the release, MouseUp, then
// DragEnd if the press dragged.

// pressGrid is a 300x300 grid of 30 rows and two 100-wide columns, a and b,
// that can drag rows, and the rows it reports moved.
func pressGrid(t *testing.T) (*DataGrid, *[][2]int) {
	t.Helper()
	data := make([]map[string]any, 30)
	for i := range data {
		data[i] = map[string]any{"a": fmt.Sprintf("a%d", i), "b": fmt.Sprintf("b%d", i)}
	}
	g := newTestGrid(t, data, []*ColumnDef{{ColID: "a", Field: "a"}, {ColID: "b", Field: "b"}})
	g.Options.RowDragEnabled = true
	moved := &[][2]int{}
	g.OnRowMoved = func(from, to int) { *moved = append(*moved, [2]int{from, to}) }
	layOut(t, g)
	require.Equal(t, float32(100), g.visibleCol(0).width)
	return g, moved
}

// rowAt is a point in display row r of column a.
func rowAt(g *DataGrid, r int) geo.Position {
	y, h := g.viewport.RowY(r)
	return geo.NewPos(40, g.bodyTop()+y+h/2)
}

// borderAt is a point on column a's border in the header.
func borderAt(t *testing.T, g *DataGrid) geo.Position {
	t.Helper()
	for x := float32(1); x < 300; x++ {
		if g.headerBorderCol(x) == 0 {
			return geo.NewPos(x, g.HeaderHeight()/2)
		}
	}
	t.Fatal("no border for column a in the header")
	return geo.Position{}
}

func pressAt(p geo.Position, mod event.KeyModifier) *desktop.MouseEvent {
	return &desktop.MouseEvent{PointEvent: event.PointEvent{Position: p}, Button: desktop.MouseButtonPrimary, Modifier: mod}
}

func dragTo(p geo.Position, d geo.Delta) *event.DragEvent {
	return &event.DragEvent{PointEvent: event.PointEvent{Position: p}, Dragged: d}
}

// fingerScroll drags a finger 60 up the body from display row 4: the grid
// claims it, as the touch handler asks before the first move, then gets the
// moves and the end. It reports whether the grid claimed it.
func fingerScroll(g *DataGrid) bool {
	from := rowAt(g, 4)
	claimed := g.ClaimsDrag(geo.Delta{DY: -20})
	for i := 1; i <= 3; i++ {
		g.Dragged(dragTo(from.AddXY(0, float32(-20*i)), geo.Delta{DY: -20}))
	}
	g.DragEnd()
	return claimed
}

func TestDataGrid_ClickedRowThenFingerScrolls(t *testing.T) {
	g, moved := pressGrid(t)
	g.MouseDown(pressAt(rowAt(g, 1), 0))
	g.MouseUp(pressAt(rowAt(g, 1), 0))
	assert.Equal(t, -1, g.dragRow, "the click left no row drag armed")

	assert.True(t, fingerScroll(g), "the grid claimed the finger's drag")
	assert.Equal(t, float32(60), g.scrollContent.Offset().Y, "the finger scrolled the body")
	assert.Empty(t, *moved, "no row moved")
}

func TestDataGrid_MouseDragStillMovesARow(t *testing.T) {
	g, moved := pressGrid(t)
	g.MouseDown(pressAt(rowAt(g, 1), 0))
	g.Dragged(dragTo(rowAt(g, 2), geo.Delta{DY: g.rowHeight()}))
	g.Dragged(dragTo(rowAt(g, 3), geo.Delta{DY: g.rowHeight()}))
	g.MouseUp(pressAt(rowAt(g, 3), 0))
	g.DragEnd()
	assert.Equal(t, [][2]int{{1, 3}}, *moved, "the row dragged from 1 went to 3")
	assert.Zero(t, g.scrollContent.Offset().Y, "a mouse drag does not scroll the body")
	assert.Equal(t, -1, g.dragRow)
}

// Each press starts afresh: a drag whose end never reached the grid (the
// grid disabled and enabled during it, say) leaves the next click armed with
// nothing.
func TestDataGrid_ClickAfterAnUnendedDragArmsNothing(t *testing.T) {
	g, moved := pressGrid(t)
	g.MouseDown(pressAt(rowAt(g, 1), 0))
	g.Dragged(dragTo(rowAt(g, 2), geo.Delta{DY: g.rowHeight()}))
	g.MouseUp(pressAt(rowAt(g, 2), 0))
	g.Disable()
	g.Enable()

	g.MouseDown(pressAt(rowAt(g, 1), 0))
	g.MouseUp(pressAt(rowAt(g, 1), 0))
	assert.Equal(t, -1, g.dragRow, "the click left no row drag armed")
	assert.True(t, fingerScroll(g))
	assert.Equal(t, float32(60), g.scrollContent.Offset().Y, "the finger scrolled the body")
	assert.Empty(t, *moved)
}

// The header gets the presses on its own area, and arms a resize on a border.
func TestDataGrid_ClickedBorderThenFingerScrolls(t *testing.T) {
	g, moved := pressGrid(t)
	h := g.headerWidget
	at := borderAt(t, g)
	h.MouseDown(pressAt(at, 0))
	h.MouseUp(pressAt(at, 0))
	h.Tapped(&event.PointEvent{Position: at})
	assert.Equal(t, -1, g.resizeCol, "the click left no resize armed")
	assert.False(t, h.ClaimsDrag(geo.Delta{DY: -20}), "the header claims no finger's drag")

	assert.True(t, fingerScroll(g))
	assert.Equal(t, float32(60), g.scrollContent.Offset().Y, "the finger scrolled the body")
	assert.Equal(t, float32(100), g.visibleCol(0).width, "column a kept its width")
	assert.Empty(t, *moved)
}

func TestDataGrid_MouseDragStillResizes(t *testing.T) {
	g, _ := pressGrid(t)
	h := g.headerWidget
	at := borderAt(t, g)
	h.MouseDown(pressAt(at, 0))
	h.Dragged(dragTo(at.AddXY(30, 0), geo.Delta{DX: 30}))
	h.MouseUp(pressAt(at.AddXY(30, 0), 0))
	assert.Equal(t, float32(130), g.visibleCol(0).width, "the drag widened column a")
	h.DragEnd()
	assert.Equal(t, -1, g.resizeCol, "the drag's end finished the resize")
	assert.False(t, h.ClaimsDrag(geo.Delta{DY: -20}))
}

// The grid's own press handler arms a reorder for a press on the header area
// it gets (the header widget usually takes those first).
func TestDataGrid_ClickedHeaderThenFingerScrolls(t *testing.T) {
	g, _ := pressGrid(t)
	at := geo.NewPos(40, g.HeaderHeight()/2)
	g.MouseDown(pressAt(at, 0))
	g.MouseUp(pressAt(at, 0))
	assert.Equal(t, -1, g.reorderFromCol, "the click left no reorder armed")

	assert.True(t, fingerScroll(g))
	assert.Equal(t, float32(60), g.scrollContent.Offset().Y, "the finger scrolled the body")
	assert.Equal(t, "a", g.visibleCol(0).def.ColID, "no column moved")
}

func TestDataGrid_MouseDragStillReordersColumns(t *testing.T) {
	g, _ := pressGrid(t)
	at := geo.NewPos(40, g.HeaderHeight()/2)
	g.MouseDown(pressAt(at, 0))
	g.Dragged(dragTo(at.AddXY(100, 0), geo.Delta{DX: 100}))
	g.MouseUp(pressAt(at.AddXY(100, 0), 0))
	g.DragEnd()
	assert.Equal(t, "b", g.visibleCol(0).def.ColID, "column a moved after b")
	assert.Equal(t, -1, g.reorderFromCol)
}

// A press that drags on a header is no tap. Its modifiers used to stay for the
// next tap, and a finger's tap, which sends no press of its own, sorted with
// the dragging press's Shift: it added a column to the sort instead of
// sorting by its own.
func TestDataGrid_HeaderDragLeavesNoModifierForTheNextTap(t *testing.T) {
	g, _ := pressGrid(t)
	h := g.headerWidget
	y := g.HeaderHeight() / 2
	h.Tapped(&event.PointEvent{Position: geo.NewPos(40, y)})
	require.Equal(t, fmt.Sprint([]SortItem{{ColID: "a", Direction: SortAscending}}), fmt.Sprint(g.API().GetSort()))

	h.MouseDown(pressAt(geo.NewPos(140, y), event.KeyModifierShift))
	h.Dragged(dragTo(geo.NewPos(160, y), geo.Delta{DX: 20}))
	h.MouseUp(pressAt(geo.NewPos(160, y), event.KeyModifierShift))
	h.DragEnd()

	h.Tapped(&event.PointEvent{Position: geo.NewPos(140, y)})
	assert.Equal(t, fmt.Sprint([]SortItem{{ColID: "b", Direction: SortAscending}}), fmt.Sprint(g.API().GetSort()),
		"a finger's tap on b sorts by b alone")
}
