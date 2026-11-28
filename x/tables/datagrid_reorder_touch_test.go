//go:build (ci || android || ios || mobile) && !wasm

// These drive row reordering through the touch handler the mobile, embedded
// and desktop-touch drivers share, where the body Scroll is Draggable as on a
// phone (bellui's scroller_mobile.go is built for ci too).

package tables

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// handleGrid is a 300x400 window filled by a grid of 100 rows with a grip
// column h before the value column n, that can move rows, the moves it
// reports, and the touch handler that reaches it.
type handleGrid struct {
	g     *DataGrid
	h     *common.TouchHandler
	moved [][2]int
	menus int
	// later holds what a cancelled hold scheduled (afterHoldCancel), for the
	// test to run when it chooses.
	later []func()
}

func newHandleGrid(t *testing.T, handle bool) *handleGrid {
	t.Helper()
	cols := []*ColumnDef{{ColID: "h", RowDragHandle: handle, Width: 40}, {ColID: "n", Field: "n", Width: 120}}
	hg := &handleGrid{g: newTestGrid(t, manyRows(100), cols)}
	hg.g.Options.RowDragEnabled = true
	hg.g.OnRowMoved = func(from, to int) { hg.moved = append(hg.moved, [2]int{from, to}) }
	hg.g.OnCellSecondaryTapped = func(CellID, *event.PointEvent) { hg.menus++ }
	w := test.NewTempWindow(t, hg.g)
	w.Resize(geo.NewSize(300, 400))
	hg.h = &common.TouchHandler{}
	hg.h.InitTouchHandler(&pageFinder{content: hg.g})
	old := afterHoldCancel
	afterHoldCancel = func(f func()) { hg.later = append(hg.later, f) }
	t.Cleanup(func() { afterHoldCancel = old })
	return hg
}

// hold presses contact id at p and waits for the hold to fire. The handler
// fires it from its timer's goroutine; the channel orders that before the
// test goes on.
func (hg *handleGrid) hold(t *testing.T, id int, p geo.Position) {
	t.Helper()
	fired := make(chan struct{}, 1)
	hg.h.LongPressCallback = func(w event.LongPressable, ev *event.PointEvent) {
		w.LongPressed(ev)
		fired <- struct{}{}
	}
	hg.h.TapDown(p, id)
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("the hold never fired")
	}
}

// at is a point in display row row's cells, frac of the way down, at x.
func (hg *handleGrid) at(row int, x, frac float32) geo.Position {
	g := hg.g
	y, h := g.viewport.RowY(row)
	return geo.NewPos(x, g.scrollContent.Scroll.Position().Y+y-g.offset.Y+h*frac)
}

func (hg *handleGrid) release(id int, p geo.Position) {
	hg.h.TapUp(p, id, func(event.Tappable, *event.PointEvent) {},
		func(w event.SecondaryTappable, ev *event.PointEvent) { w.TappedSecondary(ev) },
		func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {},
		func(d event.Draggable, _ *event.DragEvent) { d.DragEnd() })
}

func (hg *handleGrid) drag(id int, from, to geo.Position) {
	drag := func(d event.Draggable, ev *event.DragEvent) { d.Dragged(ev) }
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		hg.h.TapMove(geo.NewPos(from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f), id, drag)
	}
}

// A finger dragging a grip moves its row at once (#1512): the grip, inside
// the body's Scroll, takes the drag before the Scroll can scroll.
func TestDataGrid_TouchHandleMovesTheRow(t *testing.T) {
	hg := newHandleGrid(t, true)
	from, to := hg.at(1, 20, 0.5), hg.at(4, 20, 0.25)
	hg.h.TapDown(from, 1)
	hg.drag(1, from, to)
	require.True(t, hg.g.lift.active, "the grip lifted the row")
	assert.False(t, hg.g.focused, "a touch lift does not focus the grid, which would raise the keyboard")
	hg.release(1, to)
	assert.Equal(t, [][2]int{{1, 3}}, hg.moved, "row 1 went before row 4")
	assert.Zero(t, hg.g.scrollContent.Offset().Y, "the body did not scroll")
	assert.False(t, hg.g.lift.active)
}

// A plain swipe off the grips scrolls the grid and moves no row, and with
// the rows sorted the grips are drawn disabled and a swipe on one scrolls.
func TestDataGrid_TouchHandleOnlyFromTheGrip(t *testing.T) {
	hg := newHandleGrid(t, true)
	from := hg.at(4, 100, 0.5)
	touchSwipe(hg.h, 1, from, from.SubtractXY(0, 80))
	assert.Equal(t, float32(80), hg.g.scrollContent.Offset().Y, "a swipe off the grips scrolled")
	assert.Empty(t, hg.moved)

	hg.g.scrollContent.ScrollTo(geo.Position{})
	hg.g.API().SetSort(SortItem{ColID: "n", Direction: SortAscending})
	grip := hg.at(4, 20, 0.5)
	touchSwipe(hg.h, 2, grip, grip.SubtractXY(0, 80))
	assert.Equal(t, float32(80), hg.g.scrollContent.Offset().Y, "a sorted grid's grip leaves the swipe to the scroll")
	assert.Empty(t, hg.moved)
	h := hg.handleIn(t, 4)
	_, disabled := h.icon.Resource.(*theme.DisabledResource)
	assert.True(t, disabled, "a sorted grid draws its grips disabled")
}

// handleIn finds display row row's grip.
func (hg *handleGrid) handleIn(t *testing.T, row int) *rowHandle {
	t.Helper()
	o, _, _ := hg.h.Finder.FindObjectAtPositionMatching(hg.at(row, 20, 0.5), func(o core.CanvasObject) bool {
		_, ok := o.(*rowHandle)
		return ok
	})
	require.IsType(t, &rowHandle{}, o)
	return o.(*rowHandle)
}

// With a grip column the mouse moves rows only from a grip (owner decision
// 2): a press elsewhere arms nothing, and a mouse drag on a grip moves.
func TestDataGrid_MouseWithAHandle(t *testing.T) {
	hg := newHandleGrid(t, true)
	g := hg.g
	g.MouseDown(pressAt(hg.at(1, 100, 0.5), 0))
	assert.Equal(t, -1, g.dragRow, "a press off the grips armed no row drag")
	g.MouseUp(pressAt(hg.at(1, 100, 0.5), 0))

	h := hg.handleIn(t, 1)
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(g)
	for _, y := range []float32{0.6, 0.8} {
		p := hg.at(3, 20, y)
		h.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: abs.Add(p)}, Dragged: geo.NewDelta(0, 10)})
	}
	h.DragEnd()
	assert.Equal(t, [][2]int{{1, 3}}, hg.moved, "row 1 went after row 3")
}

// Without a grip column, a held finger lifts the row under it, and the drag
// that follows moves it (owner decision 1, hold-then-drag).
func TestDataGrid_TouchHoldThenDragMovesTheRow(t *testing.T) {
	hg := newHandleGrid(t, false)
	from, to := hg.at(1, 100, 0.5), hg.at(5, 100, 0.25)
	hg.hold(t, 3, from)
	require.True(t, hg.g.lift.active, "the hold lifted the row")
	hg.drag(3, from, to)
	// The drag's latch cancelled the hold's touch on the grid; the wait it
	// started must find the drag under way and leave the row up.
	require.Len(t, hg.later, 1, "the latch cancelled the grid's touch")
	hg.later[0]()
	require.True(t, hg.g.lift.active, "the drag kept its row")
	hg.release(3, to)
	assert.Equal(t, [][2]int{{1, 4}}, hg.moved, "row 1 went before row 5")
	assert.Zero(t, hg.menus, "a hold that dragged opens no menu")
	assert.Zero(t, hg.g.scrollContent.Offset().Y)
}

// A hold released without a drag puts the row back and opens the context
// menu, as a hold did before (the iOS rule).
func TestDataGrid_TouchHoldReleaseOpensTheMenu(t *testing.T) {
	hg := newHandleGrid(t, false)
	p := hg.at(2, 100, 0.5)
	hg.hold(t, 4, p)
	require.True(t, hg.g.lift.active)
	hg.release(4, p)
	assert.Equal(t, 1, hg.menus, "the release opened the menu")
	assert.Empty(t, hg.moved)
	assert.False(t, hg.g.lift.active, "the row went back")
}

// The desktop fires a long-press for a mouse held still and sends no tap for
// its release: the grid answers with the tap, and lifts nothing.
func TestDataGrid_MouseHeldStillIsAClick(t *testing.T) {
	hg := newHandleGrid(t, false)
	g := hg.g
	clicked := 0
	g.OnCellClicked = func(CellID) { clicked++ }
	p := hg.at(2, 100, 0.5)
	g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: p}, Button: desktop.MouseButtonPrimary})
	g.LongPressed(&event.PointEvent{Position: p})
	assert.Equal(t, 1, clicked, "the held click clicked")
	assert.False(t, g.lift.active, "a mouse hold lifts nothing")
	g.MouseUp(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: p}, Button: desktop.MouseButtonPrimary})
}

// A hidden grip column is no handle: the mouse moves rows from anywhere.
func TestDataGrid_HiddenHandleColumn(t *testing.T) {
	hg := newHandleGrid(t, true)
	hg.g.ColumnDefs[0].Hidden = true
	hg.g.ResolveColumns()
	require.False(t, hg.g.hasRowDragHandle())
	p := hg.at(1, 60, 0.5)
	hg.g.MouseDown(pressAt(p, 0))
	assert.Equal(t, 1, hg.g.dragRow, "without a visible grip, a press on a row arms its drag")
	hg.g.MouseUp(pressAt(p, 0))
}

// While the rows are sorted nothing lifts: not a hold, and not a mouse drag
// on a grip, which the desktop gives the grip without asking ClaimsDrag.
func TestDataGrid_NothingLiftsWhileSorted(t *testing.T) {
	hg := newHandleGrid(t, false)
	hg.g.API().SetSort(SortItem{ColID: "n", Direction: SortAscending})
	hg.hold(t, 5, hg.at(2, 100, 0.5))
	assert.False(t, hg.g.lift.active, "a hold on a sorted grid lifted a row")
	hg.release(5, hg.at(2, 100, 0.5))

	hg = newHandleGrid(t, true)
	hg.g.API().SetSort(SortItem{ColID: "n", Direction: SortAscending})
	h := hg.handleIn(t, 1)
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(hg.g)
	h.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: abs.Add(hg.at(3, 20, 0.8))}, Dragged: geo.NewDelta(0, 10)})
	assert.False(t, hg.g.lift.active, "a mouse drag on a sorted grid's grip lifted a row")
	h.DragEnd()
	assert.Empty(t, hg.moved)
}

// A hold whose touch the system cancels (iOS touchesCancelled, Android
// ACTION_CANCEL) puts the row back: the line and the lifted copy used to stay
// on screen with nothing left to end them (seen on an iPad).
func TestDataGrid_TouchHoldCancelledPutsTheRowBack(t *testing.T) {
	hg := newHandleGrid(t, false)
	p := hg.at(2, 100, 0.5)
	hg.hold(t, 6, p)
	require.True(t, hg.g.lift.active)
	hg.h.TapCancel(p, 6)
	require.Len(t, hg.later, 1, "the cancel waits for a drag that may follow")
	assert.True(t, hg.g.lift.active, "until then the row stays up")
	hg.later[0]()
	assert.False(t, hg.g.lift.active, "the cancelled hold left the row lifted")
	assert.False(t, hg.g.rowLiftLayer.Visible())
	assert.False(t, hg.g.dropLine.Visible())
	assert.Empty(t, hg.moved)
	assert.Zero(t, hg.menus, "a cancelled touch opens no menu")
}

// A row a hold lifted goes back when the body scrolls under it, whatever
// took the touch, and when a new touch lands on the grid.
func TestDataGrid_TouchHoldLeftoversGoBack(t *testing.T) {
	hg := newHandleGrid(t, false)
	g := hg.g
	g.dragRow = 2
	g.liftRow(hg.at(2, 100, 0.5).Y)
	g.lift.held = true
	g.scrollContent.ScrollTo(geo.NewPos(0, 40))
	g.onScrolled(geo.NewPos(0, 40))
	assert.False(t, g.lift.active, "a scroll under a held row put it back")

	g.dragRow = 2
	g.liftRow(hg.at(2, 100, 0.5).Y)
	g.lift.held = true
	g.TouchDown(nil)
	assert.False(t, g.lift.active, "a new touch put the held row back")

	// A row a drag is moving is not a leftover: neither a scroll nor a
	// second finger landing drops it.
	g.dragRow = 2
	g.liftRow(hg.at(2, 100, 0.5).Y)
	g.onScrolled(geo.NewPos(0, 80))
	assert.True(t, g.lift.active)
	g.TouchDown(nil)
	assert.True(t, g.lift.active, "a second finger dropped a dragged row")
	g.cancelLift()
}

// A cancelled hold's wait that ends during a later lift leaves that lift
// alone: it belongs to the hold it came from.
func TestDataGrid_TouchHoldOldWaitLeavesANewLift(t *testing.T) {
	hg := newHandleGrid(t, false)
	p := hg.at(2, 100, 0.5)
	hg.hold(t, 7, p)
	hg.h.TapCancel(p, 7)
	require.Len(t, hg.later, 1)
	hg.g.TouchDown(nil) // the next touch puts the first row back
	require.False(t, hg.g.lift.active)

	hg.hold(t, 8, hg.at(4, 100, 0.5))
	require.True(t, hg.g.lift.active)
	hg.g.TouchCancel(nil)
	hg.later[0]() // the first hold's wait
	assert.True(t, hg.g.lift.active, "an old wait put a newer held row back")
	hg.later[1]()
	assert.False(t, hg.g.lift.active, "its own wait did")
}

// Escape during a grip drag puts the row back, and the rest of that drag
// lifts nothing: the pointer keeps moving after the key, and a grip's next
// move used to lift the row again (seen on a Mac). The next drag works.
func TestDataGrid_EscapeOnAGripDragStaysCancelled(t *testing.T) {
	hg := newHandleGrid(t, true)
	g := hg.g
	h := hg.handleIn(t, 1)
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(g)
	drag := func(row int, frac float32) {
		h.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: abs.Add(hg.at(row, 20, frac))}, Dragged: geo.NewDelta(0, 10)})
	}
	press := hg.at(1, 20, 0.5)
	g.MouseDown(pressAt(press, 0))
	drag(2, 0.5)
	drag(3, 0.5)
	require.True(t, g.lift.active)
	g.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	require.False(t, g.lift.active)
	drag(3, 0.8)
	drag(4, 0.3)
	assert.False(t, g.lift.active, "the drag lifted the row again after Escape")
	g.MouseUp(pressAt(hg.at(4, 20, 0.3), 0))
	h.DragEnd()
	assert.Empty(t, hg.moved, "a drag cancelled by Escape moved a row")

	// The next drag lifts and moves as usual.
	g.MouseDown(pressAt(press, 0))
	drag(2, 0.5)
	drag(3, 0.8)
	require.True(t, g.lift.active, "the drag after a cancelled one lifts")
	g.MouseUp(pressAt(hg.at(3, 20, 0.8), 0))
	h.DragEnd()
	assert.Equal(t, [][2]int{{1, 3}}, hg.moved)
}
