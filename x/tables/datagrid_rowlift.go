package tables

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/tables/render"
)

// A row being moved by a drag (#1512). A mouse press arms the drag (dragRow,
// in MouseDown); the press's first move lifts the row. A copy of it then
// follows the pointer, drawn whole across the bands by a rowSwipeLayer in its
// lift mode, and a line between two rows shows where it will land: over the
// upper half of a row means before it, the lower half after it (owner
// decision 4). The body scrolls while the pointer is near its top or bottom
// edge, so a row can move past the rows on screen. Escape puts it back.

// rowLift is the state of a lifted row.
type rowLift struct {
	active bool
	// grab is how far below the lifted row's top the pointer took it, so the
	// copy keeps that point under the pointer however the body scrolls.
	grab float32
	// pointerY is the pointer's height in the body.
	pointerY float32
	// insert is the display row the lifted row goes before, from 0 to
	// RowCount (after the last row).
	insert int
	// ticking is set while the edge auto-scroll's ticker runs; stop ends it.
	ticking bool
	stop    chan struct{}
	// held marks a row lifted by a hold that no drag has moved yet. Its
	// touch can still end without one (released, or cancelled by the
	// system), and then the row goes back (datagrid_rowhandle.go).
	held bool
	// cancelPending marks a held row whose touch was cancelled, waiting for
	// the drag that may have caused it (DataGrid.TouchCancel).
	cancelPending bool
}

// reorderEdge is how close to the body's top or bottom edge the pointer has
// to be to scroll it, and reorderEdgeStep how far a step scrolls with the
// pointer at the very edge. Steps come every reorderTick.
const (
	reorderEdge     = 32
	reorderEdgeStep = 12
	reorderTick     = 16 * time.Millisecond
)

// liftRow lifts display row dragRow, for a press that went down at height
// pressY in the grid's coordinates and has started to drag.
func (g *DataGrid) liftRow(pressY float32) {
	l := &g.lift
	if l.active || g.dragRow < 0 {
		return
	}
	rowY, _ := g.viewport.RowY(g.dragRow)
	g.liftGen++
	l.active = true
	l.grab = (pressY - g.bodyTop()) - (rowY - g.offset.Y)
	l.pointerY = pressY - g.bodyTop()
	l.insert = g.dragRow
	// Escape has to reach the grid to put a mouse-dragged row back. Not on
	// touch: there a focused grid raises the on-screen keyboard (seen on the
	// pixel143 emulator), and a finger has no Escape to press.
	if g.mousePressed {
		if c := bellui.CurrentApp().Driver().CanvasForObject(g); c != nil {
			c.Focus(g)
		}
	}
	g.rowLiftLayer.Show()
	g.Refresh()
}

// moveLift follows the pointer, at y in the grid's coordinates.
func (g *DataGrid) moveLift(y float32) {
	l := &g.lift
	if !l.active {
		return
	}
	l.pointerY = y - g.bodyTop()
	l.insert = g.insertionAt(l.pointerY)
	if g.edgeScrollSpeed() != 0 {
		g.startEdgeScroll()
	}
	g.rowLiftLayer.Refresh()
	g.Refresh()
}

// insertionAt is the display row a row dropped at height y in the body goes
// before: the row under y when y is in its upper half, the next one when it
// is in its lower half. Above the first row it is 0, below the last
// RowCount.
func (g *DataGrid) insertionAt(y float32) int {
	n := g.RowCount()
	if n == 0 {
		return 0
	}
	contentY := y + g.offset.Y
	for r := max(g.viewport.RowAtY(contentY)-1, 0); r < n; r++ {
		rowY, rowH := g.viewport.RowY(r)
		if contentY < rowY+(rowH-g.DetailBandHeight(r))/2 {
			return r
		}
	}
	return n
}

// liftTarget is where the lifted row lands, as MoveRow's toDisplayRow: the
// row it goes before, less one when that is below where it came from, since
// it leaves its own place empty.
func (g *DataGrid) liftTarget() int {
	to := g.lift.insert
	if to > g.dragRow {
		to--
	}
	return to
}

// dropLift ends the drag: the row moves to where the line shows.
func (g *DataGrid) dropLift() {
	if !g.lift.active {
		return
	}
	from, to := g.dragRow, g.liftTarget()
	g.endLift()
	if to != from && g.rowsReorderable() {
		g.MoveRow(from, to)
		g.currentFocus.Row = to
		g.focusSrcRow = g.sourceRow(to)
	}
}

// cancelLift puts the row back: nothing moves (Escape).
func (g *DataGrid) cancelLift() bool {
	if !g.lift.active {
		return false
	}
	g.endLift()
	g.dragRow = -1
	g.Refresh()
	return true
}

func (g *DataGrid) endLift() {
	g.stopEdgeScroll()
	g.lift = rowLift{}
	g.rowLiftLayer.holdCover.Hide()
	g.rowLiftLayer.Hide()
	g.dropLine.Hide()
	g.Refresh()
}

// edgeScrollSpeed is how far a step of the edge auto-scroll moves the body
// now: up (negative) near its top, down near its bottom, faster the closer
// the pointer is to the edge, and 0 away from both or where the body cannot
// move further.
func (g *DataGrid) edgeScrollSpeed() float32 {
	l := &g.lift
	if !l.active {
		return 0
	}
	h := g.bodyHeight(g.Size().Height)
	s := g.scrollContent.Scroll
	limit := s.Content.MinSize().Height - s.Size().Height
	switch {
	case l.pointerY < reorderEdge && s.Offset.Y > 0:
		return -reorderEdgeStep * min(1, (reorderEdge-l.pointerY)/reorderEdge)
	case l.pointerY > h-reorderEdge && s.Offset.Y < limit:
		return reorderEdgeStep * min(1, (l.pointerY-(h-reorderEdge))/reorderEdge)
	}
	return 0
}

// edgeScrollStep scrolls the body one step, and reports whether it should go
// on. The pointer has not moved, but the rows under it have, so the line
// moves with them.
func (g *DataGrid) edgeScrollStep() bool {
	d := g.edgeScrollSpeed()
	if d == 0 {
		return false
	}
	g.scrollContent.Scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -d)})
	g.lift.insert = g.insertionAt(g.lift.pointerY)
	g.rowLiftLayer.Refresh()
	g.Refresh()
	return true
}

// startEdgeScroll runs edgeScrollStep every reorderTick until it stops, the
// drag ends, or the pointer leaves the edge.
func (g *DataGrid) startEdgeScroll() {
	l := &g.lift
	if l.ticking {
		return
	}
	l.ticking = true
	stop := make(chan struct{})
	l.stop = stop
	go func() {
		t := time.NewTicker(reorderTick)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				done := make(chan bool, 1)
				bellui.Do(func() {
					select {
					case <-stop:
						done <- false
						return
					default:
					}
					more := g.edgeScrollStep()
					if !more {
						g.lift.ticking = false
						g.lift.stop = nil
					}
					done <- more
				})
				if !<-done {
					return
				}
			}
		}
	}()
}

func (g *DataGrid) stopEdgeScroll() {
	l := &g.lift
	if l.stop != nil {
		close(l.stop)
	}
	l.ticking, l.stop = false, nil
}

// layoutDropLine places the line that shows where a lifted row will land,
// between two rows, across the body, or hides it.
func (g *DataGrid) layoutDropLine(line *canvas.Rectangle, width float32) {
	l := &g.lift
	if !l.active {
		line.Hide()
		return
	}
	y := g.insertionY(l.insert) - g.offset.Y
	h := g.bodyHeight(g.Size().Height)
	if y < 0 || y > h {
		line.Hide()
		return
	}
	const thick = 2
	line.FillColor = render.ResolveThemeColors().Primary
	line.Move(geo.NewPos(0, g.bodyTop()+y-thick/2))
	line.Resize(geo.NewSize(width, thick))
	line.Show()
	line.Refresh()
}

// insertionY is the height in the body's content between display row ins-1
// and ins: the middle of the gap above row ins, or just below the last row.
func (g *DataGrid) insertionY(ins int) float32 {
	n := g.RowCount()
	if n == 0 {
		return 0
	}
	if ins >= n {
		y, h := g.viewport.RowY(n - 1)
		return y + h
	}
	y, _ := g.viewport.RowY(ins)
	if ins == 0 {
		return y
	}
	py, ph := g.viewport.RowY(ins - 1)
	return (py + ph + y) / 2
}
