package tables

import (
	"strconv"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/render"
)

// RowSwipeAction is a button a DataGrid row reveals when a finger swipes it
// sideways (#1483), as a mail app's rows reveal Archive or Delete. The grid
// asks for a row's actions through DataGrid.LeadingSwipeActions and
// DataGrid.TrailingSwipeActions.
//
// The same actions are reachable without a swipe: a screen reader offers them
// as actions on each of the row's cells, and a secondary tap or the Menu key
// on the row lists them in a menu when the app shows no context menu of its
// own (DataGrid.OnCellSecondaryTapped, DataGrid.OnCellContextMenu). An app
// that does can add them to its menu with DataGrid.RowSwipeMenuItems.
//
// Since: 1.7
type RowSwipeAction struct {
	// Label names the action: on its button, to a screen reader and in the
	// menu.
	Label string

	// Icon, when set, is drawn on the button above the label.
	Icon bellui.Resource

	// Importance colours the button: DangerImportance in the error colour,
	// WarningImportance and SuccessImportance in theirs, HighImportance in
	// the primary colour, and the others in the colour of a plain button.
	Importance widget.Importance

	// FullSwipe lets a swipe across most of the row choose the action when
	// the finger lifts, without a tap: the row slides off, then the action
	// runs. Only the first action of a side, the outermost, can have it.
	FullSwipe bool

	// OnTriggered runs with the row's source row (see DataGrid.SourceRow)
	// when the action is chosen. After a full swipe the row is back at rest
	// when it runs; after a tap on the button it is sliding back.
	OnTriggered func(sourceRow int)
}

// swipeGesture is what the touch drag under way on the grid does.
type swipeGesture uint8

const (
	// gestureUndecided: no drag, or one whose first move has not arrived.
	gestureUndecided swipeGesture = iota
	// gestureScroll: the drag scrolls the body.
	gestureScroll
	// gestureSwipe: the drag moves the swiped row.
	gestureSwipe
)

const (
	// swipeActionMinWidth is the narrowest an action's button is drawn, and
	// swipeActionPadding the room each side of a label wider than it allows.
	swipeActionMinWidth float32 = 72
	swipeActionPadding  float32 = 16

	// swipeMinTravel is how far a finger must move for a quick release to
	// count as a flick: Carousel's distance for a swipe.
	swipeMinTravel float32 = 24

	// swipeFlickSpeed is how fast, in points per second, a release must be
	// to open or close the row whatever its position, as on a Carousel.
	swipeFlickSpeed float32 = 400

	// swipeFullShare is the share of the row's width a full swipe must reach.
	swipeFullShare float32 = 0.6

	// swipeOverdrag is how far, as a share of the finger's travel, a row
	// whose side has no full swipe follows the finger past its actions.
	swipeOverdrag float32 = 0.3

	// The flick speed is measured over the last swipeVelocityWindow of the
	// drag, and is zero when the finger had stopped for swipeStoppedAfter,
	// as Carousel, WheelPicker and SwipeArea measure it.
	swipeVelocityWindow = 100 * time.Millisecond
	swipeStoppedAfter   = 40 * time.Millisecond
)

// swipeNow is the clock a swipe's speed is measured with; tests replace it.
var swipeNow = time.Now

// rowSwipe is the state of the one row that is swiped or open.
type rowSwipe struct {
	// src is the source row of the swiped row, -1 for none. The row is kept
	// by its record, so a sort or a filter that keeps it in view keeps it
	// open (followSwipedRow).
	src int
	// offset is how far the row sits from rest: below 0 moved left,
	// revealing the actions on its right edge.
	offset float32
	// left and right are the actions on the row's left and right edges, read
	// when the swipe began.
	left, right []RowSwipeAction

	gesture swipeGesture
	from    float32 // offset when the drag began
	travel  float32 // the finger's sideways travel in this drag
	speed   swipeSpeed
	moving  *bellui.Animation
}

// swipeSpeed measures how fast a drag was moving sideways when the finger
// lifted, over the last swipeVelocityWindow, and zero once it had stopped
// for swipeStoppedAfter.
type swipeSpeed struct {
	samples []swipeSample
}

type swipeSample struct {
	at time.Time
	d  float32 // the drag's travel at that time
}

func (s *swipeSpeed) reset() { s.samples = s.samples[:0] }

func (s *swipeSpeed) add(now time.Time, d float32) {
	s.samples = append(s.samples, swipeSample{at: now, d: d})
	for len(s.samples) > 2 && now.Sub(s.samples[0].at) > swipeVelocityWindow {
		s.samples = s.samples[1:]
	}
}

func (s *swipeSpeed) at(now time.Time) float32 {
	n := len(s.samples)
	if n < 2 {
		return 0
	}
	last := s.samples[n-1]
	if now.Sub(last.at) > swipeStoppedAfter {
		return 0
	}
	first := s.samples[0]
	for _, p := range s.samples[:n-1] {
		if now.Sub(p.at) <= swipeVelocityWindow {
			first = p
			break
		}
	}
	dt := last.at.Sub(first.at).Seconds()
	if dt <= 0 {
		return 0
	}
	return (last.d - first.d) / float32(dt)
}

// sideways reports whether a drag starting with d runs mostly across, by the
// test scrollCanMove picks the axis with.
func sideways(d geo.Delta) bool { return abs32(d.DX) > abs32(d.DY) }

// swipeSideFuncs returns the callbacks for the actions on a row's left and
// right edges: leading on the left, or on the right in a right-to-left layout.
func (g *DataGrid) swipeSideFuncs() (left, right func(int) []RowSwipeAction) {
	if g.rtl {
		return g.TrailingSwipeActions, g.LeadingSwipeActions
	}
	return g.LeadingSwipeActions, g.TrailingSwipeActions
}

// swipeActionsFor returns the actions on source row src's left and right
// edges.
func (g *DataGrid) swipeActionsFor(src int) (left, right []RowSwipeAction) {
	leftFn, rightFn := g.swipeSideFuncs()
	if leftFn != nil {
		left = leftFn(src)
	}
	if rightFn != nil {
		right = rightFn(src)
	}
	return left, right
}

// swipeClaims reports whether a touch drag whose first move is d swipes a
// row, without knowing which row it started on: it runs sideways, toward an
// edge rows may have actions on (a finger moving right reveals the left
// edge), and the body cannot scroll that way. The last is the edge rule: a
// sideways drag scrolls while it can (#1483).
func (g *DataGrid) swipeClaims(d geo.Delta) bool {
	if !sideways(d) {
		return false
	}
	leftFn, rightFn := g.swipeSideFuncs()
	revealed := rightFn
	if d.DX > 0 {
		revealed = leftFn
	}
	return revealed != nil && !scrollCanMove(g.scrollContent.Scroll, d)
}

// swipeRowAt returns the source row of the data row under pos, in grid
// coordinates, or -1 where no row can be swiped: outside the body, in a detail
// band, or on a row with no source, which a group header is.
func (g *DataGrid) swipeRowAt(pos geo.Position) int {
	top := g.bodyTop()
	if pos.Y < top || pos.Y >= top+g.bodyHeight(g.Size().Height) {
		return -1
	}
	y := pos.Y - top + g.offset.Y
	row := g.viewport.RowAtY(y)
	if row < 0 || row >= g.RowCount() {
		return -1
	}
	ry, rh := g.viewport.RowY(row)
	if band := g.DetailBandHeight(row); band > 0 && y >= ry+rh-band {
		return -1
	}
	return g.sourceRow(row)
}

// swipeDragged handles a touch drag on the grid's body while a row swipe may
// be under way, and reports whether it did; the drag scrolls the body when it
// did not. The first move decides: a sideways one on a row that has actions
// that way, with the body unable to scroll that way, swipes it. While a row is
// open, a drag on another row, or one up or down, closes it.
func (g *DataGrid) swipeDragged(e *event.DragEvent) bool {
	s := &g.swipe
	if s.gesture == gestureUndecided {
		s.gesture = g.startSwipeGesture(e)
	}
	if s.gesture != gestureSwipe {
		return false
	}
	g.moveSwipe(e.Dragged.DX)
	return true
}

// startSwipeGesture decides what a touch drag whose first move is e does.
// The finger went down where e started: e.Dragged is the move from there.
func (g *DataGrid) startSwipeGesture(e *event.DragEvent) swipeGesture {
	s := &g.swipe
	d := e.Dragged
	start := geo.NewPos(e.Position.X-d.DX, e.Position.Y-d.DY)
	if s.src >= 0 {
		if sideways(d) && g.swipeRowAt(start) == s.src {
			g.continueSwipe()
			return gestureSwipe
		}
		g.closeSwipe(true)
		return gestureScroll
	}
	if !g.swipeClaims(d) {
		return gestureScroll
	}
	src := g.swipeRowAt(start)
	if src < 0 {
		return gestureScroll
	}
	left, right := g.swipeActionsFor(src)
	if len(left) == 0 && len(right) == 0 {
		return gestureScroll
	}
	s.src, s.left, s.right = src, left, right
	s.offset = 0
	g.continueSwipe()
	return gestureSwipe
}

// continueSwipe starts a drag of the swiped row from where it sits, stopping
// any animation it was in.
func (g *DataGrid) continueSwipe() {
	s := &g.swipe
	g.stopSwipeAnimation()
	s.from, s.travel = s.offset, 0
	s.speed.reset()
}

// moveSwipe moves the swiped row with a finger that moved dx sideways.
func (g *DataGrid) moveSwipe(dx float32) {
	s := &g.swipe
	s.travel += dx
	s.speed.add(swipeNow(), s.travel)
	g.placeSwipe(g.followFinger(s.from + s.travel))
}

// endSwipeGesture ends the touch drag under way: a row swipe is released.
func (g *DataGrid) endSwipeGesture() {
	s := &g.swipe
	gesture := s.gesture
	s.gesture = gestureUndecided
	if gesture == gestureSwipe {
		g.releaseSwipe()
	}
}

// actionsAt returns the actions an offset reveals: those on the right edge
// for a row moved left, on the left edge for one moved right.
func (s *rowSwipe) actionsAt(offset float32) []RowSwipeAction {
	if offset > 0 {
		return s.left
	}
	return s.right
}

// swipeActionWidth is the width of an action's button at rest.
func (g *DataGrid) swipeActionWidth(a RowSwipeAction) float32 {
	w := bellui.MeasureText(a.Label, g.swipeActionTextSize(), bellui.TextStyle{}).Width + 2*swipeActionPadding
	return geo.Max(swipeActionMinWidth, w)
}

// swipeActionTextSize is the size of an action's label: the theme's caption
// size, as the grid's theme gives it.
func (g *DataGrid) swipeActionTextSize() float32 {
	return g.Theme().Size(theme.SizeNameCaptionText)
}

// swipeActionsWidth is the width of a side's buttons at rest: how far an
// open row sits from rest.
func (g *DataGrid) swipeActionsWidth(acts []RowSwipeAction) float32 {
	var w float32
	for _, a := range acts {
		w += g.swipeActionWidth(a)
	}
	return w
}

// followFinger returns where the swiped row sits for a finger that would put
// it at offset: nowhere a side has no actions, at most the row's width, and,
// past its actions on a side with no full swipe, only part of the way.
func (g *DataGrid) followFinger(offset float32) float32 {
	acts := g.swipe.actionsAt(offset)
	if len(acts) == 0 || offset == 0 {
		return 0
	}
	a := abs32(offset)
	if w := g.swipeActionsWidth(acts); a > w && !acts[0].FullSwipe {
		a = w + (a-w)*swipeOverdrag
	}
	a = geo.Min(a, g.Size().Width)
	if offset < 0 {
		return -a
	}
	return a
}

// fullSwipeAt reports whether a row released at offset is fully swiped: its
// side's first action allows it and the row went past the threshold.
func (g *DataGrid) fullSwipeAt(offset float32) bool {
	acts := g.swipe.actionsAt(offset)
	if offset == 0 || len(acts) == 0 || !acts[0].FullSwipe {
		return false
	}
	threshold := geo.Max(g.swipeActionsWidth(acts)+swipeMinTravel, g.Size().Width*swipeFullShare)
	return abs32(offset) >= threshold
}

// releaseSwipe settles the swiped row when the finger lifts: a full swipe
// runs its action; a row past half its actions, or flicked open, opens; any
// other closes.
func (g *DataGrid) releaseSwipe() {
	s := &g.swipe
	v := s.speed.at(swipeNow())
	s.speed.reset()
	if s.offset == 0 {
		g.closeSwipe(false)
		return
	}
	if g.fullSwipeAt(s.offset) {
		g.fullSwipe()
		return
	}
	w := g.swipeActionsWidth(s.actionsAt(s.offset))
	open := abs32(s.offset) >= w/2
	if abs32(s.travel) >= swipeMinTravel && abs32(v) >= swipeFlickSpeed {
		open = (v > 0) == (s.offset > 0)
	}
	if !open {
		g.closeSwipe(true)
		return
	}
	if s.offset < 0 {
		w = -w
	}
	g.animateSwipe(w, canvas.DurationMedium2, bellui.AnimationEaseStandard, nil)
}

// fullSwipe slides the swiped row off the side it was swiped to, puts it back
// at rest, and runs the side's first action.
func (g *DataGrid) fullSwipe() {
	s := &g.swipe
	act, src := s.actionsAt(s.offset)[0], s.src
	to := g.Size().Width
	if s.offset < 0 {
		to = -to
	}
	// Linear: the row leaves at speed, and an easing that starts slow would
	// stop it at the release.
	g.animateSwipe(to, canvas.DurationShort4, bellui.AnimationLinear, func() {
		g.resetSwipe()
		if act.OnTriggered != nil {
			act.OnTriggered(src)
		}
	})
}

// chooseSwipeAction runs an action whose button was tapped, closing the row.
func (g *DataGrid) chooseSwipeAction(act RowSwipeAction, src int) {
	if g.disabled {
		return
	}
	g.closeSwipe(true)
	if act.OnTriggered != nil {
		act.OnTriggered(src)
	}
}

// closeSwipe puts the swiped row back at rest, sliding when animated is set
// and the app shows animations.
func (g *DataGrid) closeSwipe(animated bool) {
	s := &g.swipe
	if s.src < 0 {
		return
	}
	if !animated || s.offset == 0 {
		g.resetSwipe()
		return
	}
	g.animateSwipe(0, canvas.DurationMedium2, bellui.AnimationEaseStandard, g.resetSwipe)
}

// resetSwipe forgets the swiped row, leaving it at rest at once.
func (g *DataGrid) resetSwipe() {
	g.stopSwipeAnimation()
	s := &g.swipe
	s.src, s.offset = -1, 0
	s.left, s.right = nil, nil
	g.placeSwipe(0)
}

// placeSwipe puts the swiped row at offset and redraws it.
func (g *DataGrid) placeSwipe(offset float32) {
	g.swipe.offset = offset
	if g.rowSwipeLayer == nil {
		return
	}
	if g.swipe.src < 0 {
		g.rowSwipeLayer.Hide()
		return
	}
	g.rowSwipeLayer.Show()
	g.rowSwipeLayer.Refresh()
}

// animateSwipe moves the swiped row to offset to over d, then runs done. With
// animations off in the app settings it moves at once.
func (g *DataGrid) animateSwipe(to float32, d time.Duration, curve bellui.AnimationCurve, done func()) {
	g.stopSwipeAnimation()
	s := &g.swipe
	from := s.offset
	if !showAnimations() {
		g.placeSwipe(to)
		if done != nil {
			done()
		}
		return
	}
	var a *bellui.Animation
	a = bellui.NewAnimation(d, func(f float32) {
		if s.moving != a {
			return
		}
		g.placeSwipe(from + (to-from)*f)
		if f >= 1 {
			s.moving = nil
			if done != nil {
				done()
			}
		}
	})
	a.Curve = curve
	s.moving = a
	a.Start()
}

func (g *DataGrid) stopSwipeAnimation() {
	if a := g.swipe.moving; a != nil {
		g.swipe.moving = nil
		a.Stop()
	}
}

// followSwipedRow keeps the swiped row on its record after the rows were
// re-sorted, filtered or regrouped, and lets it go when the record left the
// view.
func (g *DataGrid) followSwipedRow() {
	if g.swipe.src < 0 {
		return
	}
	if g.displayRowForSource(g.swipe.src) < 0 {
		g.resetSwipe()
		return
	}
	g.placeSwipe(g.swipe.offset)
}

func showAnimations() bool {
	app := bellui.CurrentApp()
	return app != nil && app.Settings().ShowAnimations()
}

// RowSwipeMenuItems returns a row's swipe actions (RowSwipeAction) as menu
// items, leading ones first, for an app that shows its own context menu on
// the row and wants them in it. sourceRow is a source row (see SourceRow);
// the result is empty for a row with none.
//
// Since: 1.7
func (g *DataGrid) RowSwipeMenuItems(sourceRow int) []*bellui.MenuItem {
	if sourceRow < 0 {
		return nil
	}
	var items []*bellui.MenuItem
	add := func(acts []RowSwipeAction) {
		for _, a := range acts {
			item := bellui.NewMenuItem(a.Label, func() { g.chooseSwipeAction(a, sourceRow) })
			item.Icon = a.Icon
			items = append(items, item)
		}
	}
	if g.LeadingSwipeActions != nil {
		add(g.LeadingSwipeActions(sourceRow))
	}
	if g.TrailingSwipeActions != nil {
		add(g.TrailingSwipeActions(sourceRow))
	}
	return items
}

// showRowSwipeMenu lists display row row's swipe actions in a pop-up menu at
// pos, window-absolute, and reports whether it had any to list.
func (g *DataGrid) showRowSwipeMenu(row int, pos geo.Position) bool {
	items := g.RowSwipeMenuItems(g.sourceRow(row))
	if len(items) == 0 {
		return false
	}
	c := bellui.CurrentApp().Driver().CanvasForObject(g)
	if c == nil {
		return false
	}
	widget.ShowPopUpMenuAtPosition(bellui.NewMenu("", items...), c, pos)
	return true
}

var _ render.RowActioner = (*DataGrid)(nil)

// Accessible names of swipe actions: the side and the action's place in it,
// so the names a platform registers stay few however many rows there are.
// The label travels as the action's description, which screen readers show.
const (
	swipeLeadingName  = "swipe-leading-"
	swipeTrailingName = "swipe-trailing-"
)

// AccessibleRowActions returns display row row's swipe actions as actions a
// screen reader offers on each of its cells (render.RowActioner).
func (g *DataGrid) AccessibleRowActions(row int) []core.AccessibleAction {
	src := g.sourceRow(row)
	if src < 0 || g.disabled {
		return nil
	}
	var out []core.AccessibleAction
	add := func(prefix string, acts []RowSwipeAction) {
		for i, a := range acts {
			out = append(out, core.AccessibleAction{Name: prefix + strconv.Itoa(i), Description: a.Label})
		}
	}
	if g.LeadingSwipeActions != nil {
		add(swipeLeadingName, g.LeadingSwipeActions(src))
	}
	if g.TrailingSwipeActions != nil {
		add(swipeTrailingName, g.TrailingSwipeActions(src))
	}
	return out
}

// PerformAccessibleRowAction runs the swipe action of display row row that
// name, from AccessibleRowActions, names.
func (g *DataGrid) PerformAccessibleRowAction(row int, name string) {
	src := g.sourceRow(row)
	if src < 0 || g.disabled {
		return
	}
	fn, rest := g.LeadingSwipeActions, ""
	switch {
	case strings.HasPrefix(name, swipeLeadingName):
		rest = strings.TrimPrefix(name, swipeLeadingName)
	case strings.HasPrefix(name, swipeTrailingName):
		fn, rest = g.TrailingSwipeActions, strings.TrimPrefix(name, swipeTrailingName)
	default:
		return
	}
	i, err := strconv.Atoi(rest)
	if fn == nil || err != nil {
		return
	}
	if acts := fn(src); i >= 0 && i < len(acts) {
		g.chooseSwipeAction(acts[i], src)
	}
}
