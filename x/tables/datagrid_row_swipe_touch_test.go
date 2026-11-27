//go:build (ci || android || ios || mobile) && !wasm

// These drive swipes through the touch handler the mobile, embedded and
// desktop-touch drivers share, where the body Scroll is Draggable as on a
// phone (bellui's scroller_mobile.go is built for ci too).

package tables

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// swipeClock makes swipeNow advance by step at each reading, so a drag's
// speed is set by the test: a slow step leaves a release with no speed, as a
// finger that stopped before lifting.
func swipeClock(t *testing.T, step time.Duration) {
	t.Helper()
	now := time.Unix(0, 0)
	old := swipeNow
	swipeNow = func() time.Time { now = now.Add(step); return now }
	t.Cleanup(func() { swipeNow = old })
}

// swipeGrid is a grid of 100 rows with one action on each edge, laid out
// alone in a 300x400 window, and the touch handler that reaches it. widths
// sets the columns: one of 120 fits, wider ones scroll sideways.
type swipeGrid struct {
	g       *DataGrid
	w       bellui.Window
	h       *common.TouchHandler
	chosen  []string
	touchID int
}

func newSwipeGrid(t *testing.T, widths ...float32) *swipeGrid {
	t.Helper()
	if len(widths) == 0 {
		widths = []float32{120}
	}
	data := manyRows(100)
	var cols []*ColumnDef
	for i, w := range widths {
		id := "n"
		if i > 0 {
			id = "c" + string(rune('a'+i))
		}
		cols = append(cols, &ColumnDef{ColID: id, Field: "n", Width: w})
	}
	sg := &swipeGrid{g: newTestGrid(t, data, cols)}
	sg.g.LeadingSwipeActions = func(src int) []RowSwipeAction {
		return []RowSwipeAction{{Label: "Flag", OnTriggered: func(r int) { sg.choose("Flag", r) }}}
	}
	sg.g.TrailingSwipeActions = func(src int) []RowSwipeAction {
		return []RowSwipeAction{{Label: "Delete", Importance: widget.DangerImportance, OnTriggered: func(r int) { sg.choose("Delete", r) }}}
	}
	sg.w = test.NewTempWindow(t, sg.g)
	sg.w.Resize(geo.NewSize(300, 400))
	sg.h = &common.TouchHandler{}
	sg.h.InitTouchHandler(&pageFinder{content: sg.g})
	swipeClock(t, 50*time.Millisecond)
	return sg
}

func (sg *swipeGrid) choose(label string, src int) {
	sg.chosen = append(sg.chosen, label+":"+itoa(src))
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

// rowCenter is the centre of display row row's cells, in grid coordinates,
// at x.
func (sg *swipeGrid) rowCenter(row int, x float32) geo.Position {
	g := sg.g
	y, h := g.viewport.RowY(row)
	return geo.NewPos(x, g.scrollContent.Scroll.Position().Y+y-g.offset.Y+h/2)
}

// swipe drags a fresh contact from one point to another.
func (sg *swipeGrid) swipe(from, to geo.Position) {
	sg.touchID++
	touchSwipe(sg.h, sg.touchID, from, to)
}

// swipeRow drags sideways by dx along display row row from x.
func (sg *swipeGrid) swipeRow(row int, x, dx float32) {
	from := sg.rowCenter(row, x)
	sg.swipe(from, from.AddXY(dx, 0))
}

func (sg *swipeGrid) deleteWidth() float32 {
	return sg.g.swipeActionsWidth(sg.g.TrailingSwipeActions(0))
}

// A swipe left past half the trailing actions opens the row on them; a tap on
// the button runs the action for the row's source row and closes it.
func TestRowSwipe_OpensAndRunsTheTappedAction(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g

	sg.swipeRow(3, 200, -100)
	require.Equal(t, 3, g.swipe.src, "row 3 is open")
	assert.Equal(t, -sg.deleteWidth(), g.swipe.offset, "it rests open on its trailing actions")
	assert.True(t, g.rowSwipeLayer.Visible())

	test.TapCanvas(sg.w.Canvas(), sg.rowCenter(3, 300-sg.deleteWidth()/2))
	assert.Equal(t, []string{"Delete:3"}, sg.chosen)
	assert.Equal(t, -1, g.swipe.src, "the row closed")
	assert.False(t, g.rowSwipeLayer.Visible())
}

// A swipe right opens the leading actions, on the left.
func TestRowSwipe_RightOpensTheLeadingActions(t *testing.T) {
	sg := newSwipeGrid(t)
	sg.swipeRow(2, 40, 100)
	require.Equal(t, 2, sg.g.swipe.src)
	flagW := sg.g.swipeActionsWidth(sg.g.LeadingSwipeActions(0))
	assert.Equal(t, flagW, sg.g.swipe.offset)
	test.TapCanvas(sg.w.Canvas(), sg.rowCenter(2, flagW/2))
	assert.Equal(t, []string{"Flag:2"}, sg.chosen)
}

// A short slow swipe springs back; the same distance flicked opens.
func TestRowSwipe_ShortSwipeSpringsBackAFlickOpens(t *testing.T) {
	sg := newSwipeGrid(t)
	short := sg.deleteWidth() / 2 * 0.8

	sg.swipeRow(3, 200, -short)
	assert.Equal(t, -1, sg.g.swipe.src, "short of half its actions, the row slid back")

	swipeClock(t, 5*time.Millisecond)
	sg.swipeRow(3, 200, -short)
	assert.Equal(t, 3, sg.g.swipe.src, "flicked, the same distance opens it")
	assert.Equal(t, -sg.deleteWidth(), sg.g.swipe.offset)
}

// Past its actions a row with no full swipe follows the finger only part of
// the way, and released there it rests open.
func TestRowSwipe_OverdragResists(t *testing.T) {
	sg := newSwipeGrid(t)
	w := sg.deleteWidth()
	sg.g.swipe.right = sg.g.TrailingSwipeActions(3)
	assert.Equal(t, -(w + (200-w)*swipeOverdrag), sg.g.followFinger(-200))
	sg.g.swipe.right = nil
	sg.swipeRow(3, 250, -200)
	assert.Equal(t, -w, sg.g.swipe.offset)
	assert.Empty(t, sg.chosen, "no full swipe without FullSwipe")
}

// A full swipe runs the first action once the row has slid off, and leaves it
// at rest.
func TestRowSwipe_FullSwipeRunsTheFirstAction(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	g.TrailingSwipeActions = func(int) []RowSwipeAction {
		return []RowSwipeAction{
			{Label: "Archive", FullSwipe: true, OnTriggered: func(r int) { sg.choose("Archive", r) }},
			{Label: "More", OnTriggered: func(r int) { sg.choose("More", r) }},
		}
	}
	sg.swipeRow(5, 260, -250)
	assert.Equal(t, []string{"Archive:5"}, sg.chosen)
	assert.Equal(t, -1, g.swipe.src)
	assert.Zero(t, g.swipe.offset)

	// Short of the threshold it only opens.
	sg.chosen = nil
	sg.swipeRow(5, 260, -(g.swipeActionsWidth(g.TrailingSwipeActions(0)) + 4))
	assert.Empty(t, sg.chosen)
	assert.Equal(t, 5, g.swipe.src)
}

// The edge rule (#1483): a sideways drag on a grid that scrolls sideways
// scrolls it while it can; one that starts at the edge swipes; one that
// scrolls into the edge stays a scroll.
func TestRowSwipe_EdgeRule(t *testing.T) {
	sg := newSwipeGrid(t, 200, 200, 200)
	g := sg.g

	sg.swipeRow(3, 250, -60)
	assert.Equal(t, float32(60), g.scrollContent.Offset().X, "away from the edge, the drag scrolls")
	assert.Equal(t, -1, g.swipe.src)

	// Into the edge and well past it: still a scroll, stopped at the edge.
	maxX := g.cellsWidget.MinSize().Width - g.scrollContent.Scroll.Size().Width
	require.Greater(t, maxX, float32(60))
	sg.swipeRow(3, 250, -(maxX - 60 + 150))
	assert.Equal(t, maxX, g.scrollContent.Offset().X, "the drag scrolled to the right edge")
	assert.Equal(t, -1, g.swipe.src, "a drag that scrolled into the edge did not swipe")

	// A new drag from the edge swipes.
	sg.swipeRow(3, 250, -100)
	assert.Equal(t, 3, g.swipe.src, "a drag that starts at the edge swipes the row")
	assert.Equal(t, maxX, g.scrollContent.Offset().X, "and does not scroll")

	// At the left edge, only a swipe right swipes.
	g.closeSwipe(false)
	g.scrollContent.ScrollTo(geo.NewPos(0, 0))
	sg.swipeRow(4, 40, 100)
	assert.Equal(t, 4, g.swipe.src, "at the left edge a swipe right swipes")
}

// A drag up or down on a swipeable row scrolls the body.
func TestRowSwipe_VerticalDragScrolls(t *testing.T) {
	sg := newSwipeGrid(t)
	from := sg.rowCenter(5, 100)
	sg.swipe(from, from.SubtractXY(0, 80))
	assert.Equal(t, float32(80), sg.g.scrollContent.Offset().Y)
	assert.Equal(t, -1, sg.g.swipe.src)
}

// An open row closes on a tap anywhere on the grid, which does nothing else,
// and on a scroll.
func TestRowSwipe_TapOrScrollCloses(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	var clicked []CellID
	g.OnCellClicked = func(id CellID) { clicked = append(clicked, id) }

	sg.swipeRow(3, 200, -100)
	require.Equal(t, 3, g.swipe.src)
	test.TapCanvas(sg.w.Canvas(), sg.rowCenter(3, 60))
	assert.Equal(t, -1, g.swipe.src, "a tap on the open row closed it")
	sg.swipeRow(3, 200, -100)
	test.TapCanvas(sg.w.Canvas(), sg.rowCenter(6, 60))
	assert.Equal(t, -1, g.swipe.src, "a tap on another row closed it")
	assert.Empty(t, clicked, "the taps that closed it clicked nothing")
	assert.Empty(t, sg.chosen)

	sg.swipeRow(3, 200, -100)
	from := sg.rowCenter(6, 100)
	sg.swipe(from, from.SubtractXY(0, 80))
	assert.Equal(t, -1, g.swipe.src, "a scroll closed it")
	assert.Equal(t, float32(80), g.scrollContent.Offset().Y)
}

// A sideways drag on an open row moves the row even where the body could
// scroll that way: here back to rest, on a grid at its right edge.
func TestRowSwipe_DragOnTheOpenRowMovesIt(t *testing.T) {
	sg := newSwipeGrid(t, 200, 200, 200)
	g := sg.g
	g.scrollContent.ScrollTo(geo.NewPos(1e6, 0))
	g.scrollContent.Scroll.Refresh()
	maxX := g.scrollContent.Offset().X
	sg.swipeRow(3, 250, -100)
	require.Equal(t, 3, g.swipe.src)

	sg.swipeRow(3, 100, 100)
	assert.Equal(t, -1, g.swipe.src, "dragged back, the row closed")
	assert.Equal(t, maxX, g.scrollContent.Offset().X, "the body did not scroll")
}

// The open row follows its record through a sort, and closes when a filter
// takes it out of view, or new data arrives.
func TestRowSwipe_FollowsTheRecord(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	sg.swipeRow(3, 200, -100)
	require.Equal(t, 3, g.swipe.src)

	g.API().SetSort(SortItem{ColID: "n", Direction: SortDescending})
	assert.Equal(t, 3, g.swipe.src, "still open on source row 3")
	assert.NotEqual(t, 3, g.displayRowForSource(3), "which moved")
	assert.True(t, g.rowSwipeLayer.Visible())

	g.API().SetQuickFilter("row9")
	assert.Equal(t, -1, g.swipe.src, "filtered out, it closed")

	g.API().SetQuickFilter("")
	sg.swipeRow(3, 200, -100)
	require.GreaterOrEqual(t, g.swipe.src, 0)
	g.RefreshData()
	assert.Equal(t, -1, g.swipe.src, "new data closed it")
}

// A disabled grid closes its open row and swipes none.
func TestRowSwipe_DisabledDoesNotSwipe(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	sg.swipeRow(3, 200, -100)
	require.Equal(t, 3, g.swipe.src)
	g.Disable()
	assert.Equal(t, -1, g.swipe.src)
	sg.swipeRow(3, 200, -100)
	assert.Equal(t, -1, g.swipe.src)
	g.Enable()
	sg.swipeRow(3, 200, -100)
	assert.Equal(t, 3, g.swipe.src)
}

// Group headers do not swipe.
func TestRowSwipe_GroupHeaderDoesNotSwipe(t *testing.T) {
	data := []map[string]any{
		{"dept": "A", "name": "Alice"}, {"dept": "B", "name": "Bob"}, {"dept": "A", "name": "Cy"},
	}
	g := newTestGrid(t, data, []*ColumnDef{{Field: "dept", Width: 60}, {Field: "name", Width: 60}})
	g.API().SetRowGroupColumn("dept")
	g.TrailingSwipeActions = func(int) []RowSwipeAction { return []RowSwipeAction{{Label: "Delete"}} }
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(300, 400))
	h := &common.TouchHandler{}
	h.InitTouchHandler(&pageFinder{content: g})
	swipeClock(t, 50*time.Millisecond)
	sg := &swipeGrid{g: g, w: w, h: h}

	require.True(t, g.IsGroupRow(0))
	sg.swipeRow(0, 200, -100)
	assert.Equal(t, -1, g.swipe.src, "a group header did not swipe")
	sg.swipeRow(1, 200, -100)
	assert.Equal(t, g.sourceRow(1), g.swipe.src, "a data row under it did")
}

// In a right-to-left layout the leading actions are on the right, so a swipe
// left reveals them.
func TestRowSwipe_RightToLeftMirrorsTheSides(t *testing.T) {
	g := newTestGrid(t, manyRows(50), []*ColumnDef{{ColID: "n", Field: "n", Width: 120}})
	var chosen []string
	g.LeadingSwipeActions = func(int) []RowSwipeAction {
		return []RowSwipeAction{{Label: "Flag", OnTriggered: func(int) { chosen = append(chosen, "Flag") }}}
	}
	g.TrailingSwipeActions = func(int) []RowSwipeAction {
		return []RowSwipeAction{{Label: "Delete", OnTriggered: func(int) { chosen = append(chosen, "Delete") }}}
	}
	scope := container.NewDirectionScope(g, bellui.DirectionRTL)
	w := test.NewTempWindow(t, scope)
	w.Resize(geo.NewSize(300, 400))
	require.True(t, g.rtl, "the grid was laid out right to left")
	h := &common.TouchHandler{}
	h.InitTouchHandler(&pageFinder{content: scope})
	swipeClock(t, 50*time.Millisecond)
	sg := &swipeGrid{g: g, w: w, h: h}

	sg.swipeRow(3, 200, -100)
	require.Equal(t, 3, g.swipe.src)
	assert.Equal(t, "Flag", g.swipe.actionsAt(g.swipe.offset)[0].Label, "the leading action is on the right")
}

// A mouse drag never swipes.
func TestRowSwipe_MouseDragDoesNotSwipe(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	at := sg.rowCenter(3, 200)
	g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	g.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: at.AddXY(-100, 0)}, Dragged: geo.NewDelta(-100, 0)})
	g.DragEnd()
	assert.Equal(t, -1, g.swipe.src)
}

// While a row is swiped the layer draws it whole, pinned cells included, slid
// by the offset.
func TestRowSwipe_PinnedCellsSlideWithTheRow(t *testing.T) {
	sg := newSwipeGrid(t, 80, 80)
	g := sg.g
	g.SetColumnPinned("n", PinLeft)
	sg.w.Resize(geo.NewSize(300, 400))
	sg.swipeRow(3, 250, -100)
	require.Equal(t, 3, g.swipe.src)

	l := g.rowSwipeLayer
	require.True(t, l.pinnedLeft.Visible())
	d := bellui.CurrentApp().Driver()
	gridX := d.AbsolutePositionForObject(g).X
	assert.Equal(t, g.swipe.offset, d.AbsolutePositionForObject(l.pinnedLeft).X-gridX,
		"the pinned-left cells moved with the row")
	mainX, _, _ := g.hitRegions()
	assert.Equal(t, g.swipe.offset+mainX-g.offset.X, d.AbsolutePositionForObject(l.cells).X-gridX,
		"and so did the main ones")
}

// The swipe actions are each cell's accessible actions, run for the row's
// source row, and fill the context menu when the app shows none.
func TestRowSwipe_ReachableWithoutTouch(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	g.API().SetSort(SortItem{ColID: "n", Direction: SortDescending})
	row := g.displayRowForSource(7)

	acts := g.AccessibleRowActions(row)
	require.Len(t, acts, 2)
	assert.Equal(t, "Flag", acts[0].Description)
	assert.Equal(t, "Delete", acts[1].Description)
	g.PerformAccessibleRowAction(row, acts[1].Name)
	assert.Equal(t, []string{"Delete:7"}, sg.chosen)

	// A screen reader reads them from the row's cells.
	g.scrollToCell(CellID{Row: row, Col: 0})
	var cellActions []core.AccessibleAction
	var cell core.AccessibleActioner
	for _, o := range test.WidgetRenderer(g.cellsWidget).Objects() {
		if gi, ok := o.(core.AccessibleGridItem); ok {
			if r, _ := gi.AccessibleCellPosition(); r == row {
				cell = o.(core.AccessibleActioner)
				cellActions = cell.AccessibleActions()
			}
		}
	}
	require.NotNil(t, cell, "row %d has a realized cell", row)
	require.Len(t, cellActions, 3, "select, then the row's two actions")
	assert.Equal(t, "Flag", cellActions[1].Description)
	cell.PerformAccessibleAction(cellActions[1].Name)
	assert.Equal(t, []string{"Delete:7", "Flag:7"}, sg.chosen)

	sg.chosen = nil
	items := g.RowSwipeMenuItems(7)
	require.Len(t, items, 2)
	assert.Equal(t, "Flag", items[0].Label)
	assert.Equal(t, "Delete", items[1].Label)
	items[1].Action()
	assert.Equal(t, []string{"Delete:7"}, sg.chosen)

	// A secondary tap with no app menu shows them.
	c := sg.w.Canvas()
	test.TapSecondaryAt(g, sg.rowCenter(row, 60))
	require.Len(t, c.Overlays().List(), 1, "a menu opened")

	// With the app's own handler, only it runs.
	c.Overlays().Remove(c.Overlays().Top())
	var own int
	g.OnCellSecondaryTapped = func(CellID, *event.PointEvent) { own++ }
	test.TapSecondaryAt(g, sg.rowCenter(row, 60))
	assert.Equal(t, 1, own)
	assert.Empty(t, c.Overlays().List())
}

// On a desktop touchscreen the body Scroll is not Draggable, so every touch
// drag reaches the grid, which applies the edge rule itself: away from the
// edge a sideways drag scrolls, from the edge it swipes.
func TestRowSwipe_GridAppliesTheEdgeRuleItself(t *testing.T) {
	sg := newSwipeGrid(t, 200, 200, 200)
	g := sg.g
	drag := func(row int, x, dx float32) {
		t.Helper()
		const steps = 5
		from := sg.rowCenter(row, x)
		step := dx / steps
		require.True(t, g.ClaimsDrag(geo.NewDelta(step, 0)), "the grid takes a sideways drag it can use")
		for i := 1; i <= steps; i++ {
			g.Dragged(&event.DragEvent{
				PointEvent: event.PointEvent{Position: from.AddXY(step*float32(i), 0)},
				Dragged:    geo.NewDelta(step, 0),
			})
		}
		g.DragEnd()
	}

	drag(3, 250, -60)
	assert.Equal(t, float32(60), g.scrollContent.Offset().X, "away from the edge the drag scrolled")
	assert.Equal(t, -1, g.swipe.src)

	g.scrollContent.ScrollTo(geo.NewPos(1e6, 0))
	g.scrollContent.Scroll.Refresh()
	maxX := g.scrollContent.Offset().X
	drag(3, 250, -100)
	assert.Equal(t, 3, g.swipe.src, "from the edge it swiped")
	assert.Equal(t, maxX, g.scrollContent.Offset().X)
}

// An expanded master row swipes its cells, and its detail band stays where
// the grid draws it: the layer draws the row again, but the detail content is
// one object, and it stays in the grid's own cells.
func TestRowSwipe_DetailContentStaysWithTheGrid(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	detail := canvas.NewText("detail", nil)
	g.MasterDetail = MasterDetailConfig{
		Enabled: true, DetailHeight: 60,
		DetailRenderer: func(int) core.CanvasObject { return detail },
	}
	g.Refresh()
	g.ToggleDetail(2)
	require.True(t, g.IsDetailExpanded(2))
	before := detail.Position()

	// The middle of the row's cells, above its detail band.
	y, h := g.viewport.RowY(2)
	from := geo.NewPos(200, g.scrollContent.Scroll.Position().Y+y-g.offset.Y+(h-g.DetailBandHeight(2))/2)
	sg.swipe(from, from.AddXY(-100, 0))
	require.Equal(t, 2, g.swipe.src)
	assert.Contains(t, test.WidgetRenderer(g.cellsWidget).Objects(), core.CanvasObject(detail),
		"the grid's cells still hold the detail")
	assert.NotContains(t, test.WidgetRenderer(g.rowSwipeLayer.cells).Objects(), core.CanvasObject(detail),
		"the swiped row's cells do not")
	assert.Equal(t, before, detail.Position())
}

// The layer draws the open row where its record now is: a filter that moves
// source row 1 from display row 1 to row 0 moves the open row up with it.
func TestRowSwipe_LayerDrawsTheRecordWhereItMoved(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	sg.swipeRow(1, 200, -100)
	require.Equal(t, 1, g.swipe.src)

	g.API().SetQuickFilter("row1")
	require.Equal(t, 0, g.displayRowForSource(1), "the filter moved source row 1 up")
	require.Equal(t, 1, g.swipe.src, "and it is still open")

	d := bellui.CurrentApp().Driver()
	y, _ := g.viewport.RowY(0)
	want := g.scrollContent.Scroll.Position().Y + y - g.offset.Y
	assert.Equal(t, want, d.AbsolutePositionForObject(g.rowSwipeLayer.cover).Y-d.AbsolutePositionForObject(g).Y,
		"the swiped row is drawn at display row 0")
}
