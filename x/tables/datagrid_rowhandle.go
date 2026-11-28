package tables

import (
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
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

// reorderOffReason is the hint a grey grip gives when it is touched or
// clicked (owner, 2026-11-28): why the grid has turned row moving off, and
// how to turn it back on. It is "" while rows can move, and when the app
// turned it off itself (RowDragEnabled unset, or the grid disabled), which
// the app explains if it wants to.
func (g *DataGrid) reorderOffReason() string {
	switch {
	case !g.Options.RowDragEnabled || g.disabled:
		return ""
	case len(g.sortModel) > 0:
		return "Clear the sort to reorder rows"
	case g.groupTransform != nil:
		return "Remove the grouping to reorder rows"
	case g.pivotTransform != nil:
		return "Turn off the pivot to reorder rows"
	case g.treeTransform != nil:
		return "Rows in a tree can't be reordered"
	}
	return ""
}

// showReorderHint shows reorderOffReason as a toast, one at a time: a second
// touch replaces the first rather than stacking another. A toast is a status
// message, so screen readers announce it.
func (g *DataGrid) showReorderHint() {
	msg := g.reorderOffReason()
	if msg == "" {
		return
	}
	c := bellui.CurrentApp().Driver().CanvasForObject(g)
	if c == nil {
		return
	}
	if g.reorderHint != nil {
		g.reorderHint.Dismiss()
	}
	g.reorderHint = widget.ShowToast(msg, c)
}

// onRowDragHandle reports whether pos, in the grid's coordinates, is in a
// handle column's cell.
func (g *DataGrid) onRowDragHandle(pos geo.Position) bool {
	id := g.cellAtPosition(pos)
	if id.Row < 0 || id.Col < 0 {
		return false
	}
	rc := g.visibleCol(id.Col)
	return rc != nil && rc.def.RowDragHandle
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

// A grip is deliberately not Tappable (or Mouseable). The desktop driver
// gives a mouse press to the innermost object that is either, and a
// Tappable grip took the press from the grid: its MouseDown never ran, so a
// grip drag did not focus the grid and Escape could not put the row back
// (#1512). The grid's Tapped gives a grey grip's hint instead.

func (h *rowHandle) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(h.icon)
}

// ClaimsDrag takes every drag while rows can be moved, and none otherwise, so
// a swipe on a disabled grip scrolls the grid.
func (h *rowHandle) ClaimsDrag(geo.Delta) bool { return h.grid.rowsReorderable() }

// Dragged lifts the row on the drag's first move and moves it after.
func (h *rowHandle) Dragged(e *event.DragEvent) {
	g := h.grid
	if !g.rowsReorderable() || h.row < 0 || g.liftCancelled {
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
	g.liftCancelled = false
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
	if !g.rowsReorderable() && g.onRowDragHandle(e.Position) {
		g.showReorderHint() // a held grey grip says why, as a tapped one does
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
	g.lift.held = true
	g.rowLiftLayer.holdCover.Show()
	g.rowLiftLayer.Refresh()
}

// putHeldRowBack ends a lift that a hold started and no drag moved: its
// touch ended or was cancelled without one, a new touch landed, or the body
// scrolled under it. Before, the line and the lifted copy stayed on screen
// after a cancelled touch (#1512, seen on an iPad).
func (g *DataGrid) putHeldRowBack() {
	if g.lift.active && g.lift.held {
		g.cancelLift()
	}
}

// holdCancelWait is how long a cancelled hold's touch waits for the drag
// that may have caused the cancel before the row goes back, and
// afterHoldCancel runs f on the main goroutine after it; tests replace it.
const holdCancelWait = 150 * time.Millisecond

var afterHoldCancel = func(f func()) {
	time.AfterFunc(holdCancelWait, func() { bellui.Do(f) })
}

// TouchDown starts a new touch: a hold-lifted row from an earlier one goes
// back.
func (g *DataGrid) TouchDown(*mobile.TouchEvent) {
	g.putHeldRowBack()
	g.liftCancelled = false
}

// TouchUp ends the touch: a row its hold lifted and no drag moved goes back.
// The context menu still opens, through TappedSecondary.
func (g *DataGrid) TouchUp(*mobile.TouchEvent) {
	g.putHeldRowBack()
}

// TouchCancel ends the touch without a release. Either the system took it,
// and the row goes back, or a drag is latching the lifted row's cover: the
// touch handler cancels the touch on the grid at the drag's first move, just
// before it picks the cover. The row waits briefly for that drag, which
// clears the wait (liftCover.Dragged), before it goes back.
func (g *DataGrid) TouchCancel(*mobile.TouchEvent) {
	if !g.lift.active || !g.lift.held {
		return
	}
	g.lift.cancelPending = true
	gen := g.liftGen
	afterHoldCancel(func() {
		if g.liftGen == gen && g.lift.cancelPending {
			g.cancelLift()
		}
	})
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
	g := c.grid
	g.lift.held, g.lift.cancelPending = false, false // a drag moves it now
	g.moveLift(g.gridY(e.AbsolutePosition))
}

func (c *liftCover) DragEnd() {
	g := c.grid
	c.Hide()
	g.dropLift()
	g.dragRow = -1
	g.liftCancelled = false
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
