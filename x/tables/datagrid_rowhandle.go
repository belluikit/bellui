package tables

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
)

// Touch reordering (#1512, owner decision 1): a drag that starts on a
// ColumnDef.RowDragHandle column's grip moves its row at once, and a row held
// still and then dragged moves from anywhere. A plain vertical swipe can't be
// the gesture, because it scrolls the grid.
//
// A grip is a widget in the body's cells. On a phone the body's Scroll is a
// Draggable too, and a drag goes to the innermost Draggable that claims it,
// so the grip, inside the Scroll, gets its drag first. The mouse drags it the
// same way.
//
// A hold goes through LongPressed: the touch handler fires it while the finger
// is still down, and then gives the drag that follows to the topmost Draggable
// under the finger without asking ClaimsDrag. The lifted row's cover
// (liftCover) is that Draggable. A hold released without a drag ends in
// TappedSecondary, as before, which puts the row back and opens the context
// menu: the iOS rule.

// hasRowDragHandle reports whether a visible column is the rows' drag handle.
func (g *DataGrid) hasRowDragHandle() bool {
	for _, rc := range g.resolvedCols {
		if rc.def.RowDragHandle && !rc.def.Hidden {
			return true
		}
	}
	return false
}

// rowHandleRenderer makes a handle column's cells.
type rowHandleRenderer struct{ grid *DataGrid }

var _ cell.RendererFactory = (*rowHandleRenderer)(nil)

func (r *rowHandleRenderer) CreateCell() core.CanvasObject {
	h := &rowHandle{grid: r.grid, row: -1}
	h.icon = widget.NewIcon(theme.MenuIcon())
	h.ExtendBaseWidget(h)
	return h
}

func (r *rowHandleRenderer) UpdateCell(obj core.CanvasObject, p cell.Params) {
	h, ok := obj.(*rowHandle)
	if !ok {
		return
	}
	h.row = p.RowIndex
	res := theme.MenuIcon()
	if !r.grid.rowsReorderable() {
		res = theme.NewDisabledResource(res)
	}
	h.icon.SetResource(res)
}

// rowHandle is a grip in a handle column's cell. Dragging it moves its row.
type rowHandle struct {
	widget.BaseWidget
	grid *DataGrid
	icon *widget.Icon
	row  int // the display row it is in, -1 before its first update
}

var (
	_ event.Draggable   = (*rowHandle)(nil)
	_ event.DragClaimer = (*rowHandle)(nil)
)

func (h *rowHandle) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(h.icon)
}

// ClaimsDrag takes every drag while rows can be moved, and none otherwise, so
// a swipe on a disabled grip scrolls the grid.
func (h *rowHandle) ClaimsDrag(geo.Delta) bool { return h.grid.rowsReorderable() }

// Dragged lifts the row on the drag's first move and moves it after.
func (h *rowHandle) Dragged(e *event.DragEvent) {
	g := h.grid
	if !g.rowsReorderable() || h.row < 0 {
		return
	}
	y := g.gridY(e.AbsolutePosition)
	if !g.lift.active {
		g.dragRow = h.row
		g.liftRow(y - e.Dragged.DY)
	}
	g.moveLift(y)
}

// DragEnd drops the row where the line shows.
func (h *rowHandle) DragEnd() {
	g := h.grid
	g.dropLift()
	g.dragRow = -1
}

// gridY is the height in the grid of a point on the canvas.
func (g *DataGrid) gridY(abs geo.Position) float32 {
	return abs.Y - bellui.CurrentApp().Driver().AbsolutePositionForObject(g).Y
}

// LongPressed lifts the row under a held finger (#1512), so the drag that
// follows moves it.
//
// The desktop fires a long-press for a mouse held still too, and then sends
// no tap for its release. A held click is a click: the grid answers it with
// the tap it would have been, and the mouse moves rows from its press as
// before.
func (g *DataGrid) LongPressed(e *event.PointEvent) {
	if g.disabled {
		return
	}
	if g.mousePressed {
		g.Tapped(e)
		return
	}
	if !g.rowsReorderable() {
		return
	}
	id := g.cellAtPosition(e.Position)
	if id.Row < 0 {
		return
	}
	g.closeSwipe(true)
	g.dragRow = id.Row
	g.liftRow(e.Position.Y)
	g.rowLiftLayer.holdCover.Show()
	g.rowLiftLayer.Refresh()
}

// liftCover lies over a row lifted by a hold, and takes the drag that
// follows (a drag after a hold-fired long-press goes to the topmost
// Draggable under the finger).
type liftCover struct {
	widget.BaseWidget
	grid *DataGrid
}

var (
	_ event.Draggable         = (*liftCover)(nil)
	_ event.SecondaryTappable = (*liftCover)(nil)
)

func newLiftCover(g *DataGrid) *liftCover {
	c := &liftCover{grid: g}
	c.ExtendBaseWidget(c)
	c.Hide()
	return c
}

func (c *liftCover) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (c *liftCover) Dragged(e *event.DragEvent) {
	c.grid.moveLift(c.grid.gridY(e.AbsolutePosition))
}

func (c *liftCover) DragEnd() {
	g := c.grid
	c.Hide()
	g.dropLift()
	g.dragRow = -1
}

// TappedSecondary is the release of a hold that did not drag: the row goes
// back, and the grid opens its context menu as for any hold.
func (c *liftCover) TappedSecondary(e *event.PointEvent) {
	g := c.grid
	c.Hide()
	g.cancelLift()
	pos := e.AbsolutePosition.Subtract(bellui.CurrentApp().Driver().AbsolutePositionForObject(g))
	g.TappedSecondary(&event.PointEvent{Position: pos, AbsolutePosition: e.AbsolutePosition})
}
