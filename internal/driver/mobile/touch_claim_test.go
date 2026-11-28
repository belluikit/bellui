//go:build (ci || android || ios || mobile) && (!windows || !ci) && !wasm

// The page Scroll is Draggable only where internal/widget/scroller_mobile.go
// is built, and these tests are about which Draggable a touch drag reaches.

package mobile

import (
	"image/color"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// A touch drag goes to the innermost Draggable at its start that claims it
// (event.DragClaimer, #662): a widget that only moves one way, or a scroll
// that cannot move that way, leaves the drag to the scroll around it.

func claimDrag(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }

// swipe drags contact id from one point to another in ten steps and lifts it
// after holding still, so that nothing coasts on after the test has moved on.
func swipe(c *canvas, id int, from, to geo.Position) {
	now := time.Unix(0, 0)
	c.SetClock(func() time.Time { return now })
	defer c.SetClock(nil)
	c.TapDown(from, id)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		now = now.Add(16 * time.Millisecond)
		c.TapMove(geo.NewPos(from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f), id, claimDrag)
	}
	now = now.Add(time.Second)
	c.TapUp(to, id, func(event.Tappable, *event.PointEvent) {}, func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {},
		func(wid event.Draggable, _ *event.DragEvent) { wid.DragEnd() })
}

func filler(w, h float32) *belluicanvas.Rectangle {
	r := belluicanvas.NewRectangle(color.Transparent)
	r.SetMinSize(geo.NewSize(w, h))
	return r
}

func claimCanvas(content core.CanvasObject, size geo.Size) *canvas {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(content)
	c.Resize(size)
	return c
}

// A Carousel at the top of a page: an upward swipe that starts on it scrolls
// the page, and a sideways one still moves the carousel, not the page.
func Test_canvas_SwipeOnCarouselInAPage(t *testing.T) {
	carousel := widget.NewCarousel(filler(100, 80), filler(100, 80), filler(100, 80))
	page := container.NewVScroll(container.NewVBox(carousel, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	on := geo.NewPos(100, 40)
	require.True(t, on.Y < carousel.Size().Height, "the swipe starts on the carousel")

	swipe(c, 1, on, geo.NewPos(100, 10))
	assert.Greater(t, page.Offset.Y, float32(20), "the page followed the upward swipe")

	scrolled := page.Offset
	at := on.SubtractXY(0, scrolled.Y-1) // still on the carousel, which moved up with the page
	swipe(c, 2, at, at.SubtractXY(90, 0))
	assert.Equal(t, 1, carousel.CurrentItem, "the sideways swipe paged the carousel")
	assert.Equal(t, scrolled, page.Offset, "and not the page")
}

// A scroll nested in a page (the gallery's tab content, #662): a swipe
// scrolls the inner one while it can move, and the page once it cannot.
func Test_canvas_SwipeOnNestedScroll(t *testing.T) {
	inner := container.NewVScroll(container.NewVBox(filler(100, 400)))
	inner.SetMinSize(geo.NewSize(100, 200))
	page := container.NewVScroll(container.NewVBox(inner, filler(100, 400)))
	c := claimCanvas(page, geo.NewSize(200, 300))

	swipe(c, 1, geo.NewPos(50, 150), geo.NewPos(50, 50))
	assert.Equal(t, float32(100), inner.Offset.Y, "the inner scroll took the swipe")
	assert.Zero(t, page.Offset.Y, "the page did not")

	// Pushed to its end, the inner scroll hands the next swipe to the page.
	inner.ScrollToBottom()
	swipe(c, 2, geo.NewPos(50, 150), geo.NewPos(50, 50))
	assert.Equal(t, float32(100), page.Offset.Y, "the page took the swipe the inner scroll could not follow")

	// Downward, the inner scroll can move again and takes it back.
	end := inner.Offset.Y
	swipe(c, 3, geo.NewPos(50, 100), geo.NewPos(50, 150))
	assert.Equal(t, end-50, inner.Offset.Y)
	assert.Equal(t, float32(100), page.Offset.Y)
}

// The gallery's original layout (#662): a two-axis page Scroll whose content
// fits holds a tab whose VScroll does not. A swipe over a gap in the tab
// scrolls the tab's VScroll.
func Test_canvas_SwipeInATabInsideAFittingPage(t *testing.T) {
	var rows []core.CanvasObject
	for range 6 {
		rows = append(rows, filler(100, 100), filler(100, 24))
	}
	inner := container.NewVScroll(container.NewVBox(rows...))
	page := container.NewScroll(container.NewAppTabs(container.NewTabItem("A", inner)))
	c := claimCanvas(page, geo.NewSize(200, 300))

	swipe(c, 1, geo.NewPos(50, 200), geo.NewPos(50, 120))
	assert.Equal(t, float32(80), inner.Offset.Y)
	assert.Equal(t, geo.Position{}, page.Offset)
}

// A horizontal Slider in a page: an upward swipe that starts on it scrolls the
// page and leaves the value, and a sideways one moves the value.
func Test_canvas_SwipeOnSliderInAPage(t *testing.T) {
	slider := widget.NewSlider(0, 100)
	page := container.NewVScroll(container.NewVBox(slider, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	on := geo.NewPos(100, slider.Size().Height/2)

	swipe(c, 1, on, on.SubtractXY(0, 100))
	assert.Equal(t, float32(100), page.Offset.Y)
	assert.Zero(t, slider.Value, "the slider ignored the swipe across it")

	page.ScrollToTop()
	swipe(c, 2, on, on.AddXY(60, 0))
	assert.Greater(t, slider.Value, float64(0))
	assert.Zero(t, page.Offset.Y)
}

// A multi-line Entry in a page (#801): a plain drag over its text scrolls the
// text and selects nothing; once the text cannot move that way, the drag
// scrolls the page; and a long-press still selects.
func Test_canvas_DragOverEntryTextScrolls(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(strings.Repeat("word word word\n", 40))
	entry.SetMinRowsVisible(4)
	page := container.NewVScroll(container.NewVBox(entry, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	on := geo.NewPos(60, entry.Size().Height/2)
	textScroll := entryScroll(t, entry)

	swipe(c, 1, on, on.SubtractXY(0, 30))
	assert.Equal(t, float32(30), textScroll.Offset.Y, "the entry's text scrolled")
	assert.Empty(t, entry.SelectedText(), "a plain drag selects nothing")
	assert.Zero(t, page.Offset.Y)

	textScroll.ScrollToTop()
	swipe(c, 2, on, on.AddXY(0, 30))
	assert.Zero(t, textScroll.Offset.Y)
	assert.Zero(t, page.Offset.Y, "nothing above the page's top either")

	swipe(c, 3, on.AddXY(0, 30), on)
	textScroll.ScrollToBottom()
	bottom := textScroll.Offset.Y
	swipe(c, 4, on, on.SubtractXY(0, 30))
	assert.Equal(t, bottom, textScroll.Offset.Y)
	assert.Equal(t, float32(30), page.Offset.Y, "the page took the drag the text could not follow")

	// A long-press selects the word under the finger, and a drag that follows
	// it extends the selection instead of scrolling.
	page.ScrollToTop()
	textScroll.ScrollToTop()
	press := geo.NewPos(20, 20)
	c.TapDown(press, 5)
	waitAndCheck(400, func() {
		assert.Equal(t, "word", entry.SelectedText(), "the long-press selected a word")
	})
	c.TapMove(press.AddXY(60, 0), 5, claimDrag)
	assert.Greater(t, len(entry.SelectedText()), len("word"), "the drag extended the selection")
	assert.Zero(t, textScroll.Offset.Y)
	lprTapUp(c, press.AddXY(60, 0))
}

// A selectable Label or RichText in a page (#986): a plain drag over its text
// scrolls the page and selects nothing, as over an Entry (#801); a long-press
// still selects, and a drag that follows it extends the selection. Before
// #999 asking the selectable whether it claimed the drag recursed forever.
func Test_canvas_DragOverSelectableTextScrolls(t *testing.T) {
	label := widget.NewLabel("word word word word")
	label.Selectable = true
	rich := widget.NewRichTextWithText("word word word word")
	rich.Selectable = true
	page := container.NewVScroll(container.NewVBox(label, rich, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))

	for i, tc := range []struct {
		name     string
		at       geo.Position
		selected func() string
	}{
		{"label", geo.NewPos(40, label.Size().Height/2), label.SelectedText},
		{"rich text", geo.NewPos(40, rich.Position().Y+rich.Size().Height/2), rich.SelectedText},
	} {
		id := 10 * (i + 1)
		on := tc.at
		page.ScrollToTop()
		swipe(c, id, on, on.SubtractXY(0, 30))
		assert.Equal(t, float32(30), page.Offset.Y, "%s: the page took the drag", tc.name)
		assert.Empty(t, tc.selected(), "%s: a plain drag selects nothing", tc.name)
		swipe(c, id+1, on, on.AddXY(80, 0))
		assert.Empty(t, tc.selected(), "%s: a sideways drag selects nothing either", tc.name)

		// Contact 0, the one lprTapUp lifts.
		page.ScrollToTop()
		c.TapDown(on, 0)
		waitAndCheck(400, func() {
			assert.Equal(t, "word", tc.selected(), "%s: the long-press selected a word", tc.name)
		})
		c.TapMove(on.AddXY(60, 0), 0, claimDrag)
		assert.Greater(t, len(tc.selected()), len("word"), "%s: the drag extended the selection", tc.name)
		assert.Zero(t, page.Offset.Y, tc.name)
		lprTapUp(c, on.AddXY(60, 0))
	}
}

// entryScroll finds the Scroll inside an Entry's renderer.
func entryScroll(t *testing.T, e *widget.Entry) *container.Scroll {
	t.Helper()
	var found *container.Scroll
	var walk func(o core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if s, ok := o.(*container.Scroll); ok && found == nil {
			found = s
			return
		}
		if w, ok := o.(core.Widget); ok {
			for _, child := range cache.Renderer(w).Objects() {
				walk(child)
			}
		} else if box, ok := o.(*bellui.Container); ok {
			for _, child := range box.Objects {
				walk(child)
			}
		}
	}
	walk(e)
	require.NotNil(t, found, "the entry has a scroll")
	return found
}

// A TreeView in a page (#1266): a row took every touch drag, so a swipe that
// started on a row scrolled neither the tree nor the page. A row now claims a
// drag only when the tree lets rows move; otherwise the tree scrolls, and the
// page once the tree cannot.
func Test_canvas_SwipeOnTreeViewRows(t *testing.T) {
	tree := widget.NewTreeView()
	for i := range 40 {
		require.NoError(t, tree.AddNode("", widget.TreeViewNode{ID: widget.TreeViewNodeID(strconv.Itoa(i)), Text: "row"}))
	}
	box := container.NewGridWrap(geo.NewSize(200, 200), tree)
	page := container.NewVScroll(container.NewVBox(box, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	on := geo.NewPos(60, 150)

	swipe(c, 1, on, on.SubtractXY(0, 60))
	assert.Equal(t, float32(60), tree.GetScrollOffset(), "the tree scrolled under a swipe that started on a row")
	assert.Zero(t, page.Offset.Y)

	tree.ScrollToOffset(1e6)
	bottom := tree.GetScrollOffset()
	swipe(c, 2, on, on.SubtractXY(0, 60))
	assert.Equal(t, bottom, tree.GetScrollOffset())
	assert.Equal(t, float32(60), page.Offset.Y, "the page took the swipe the tree could not follow")
}

// A Table in a page (#1266): the table resizes a header only once a pointer
// has found its divider, which a touch never does, so on touch it leaves a
// swipe its content cannot follow to the page. Before, a touch began resizing
// row 0 on TouchDown, and such a swipe made row 0 taller instead.
func Test_canvas_SwipeOnTableAtItsEnd(t *testing.T) {
	var lastRowHeight float32
	table := widget.NewTable(
		func() (int, int) { return 50, 2 },
		func() core.CanvasObject { return filler(60, 30) },
		func(widget.TableCellID, core.CanvasObject) {},
	)
	box := container.NewGridWrap(geo.NewSize(200, 200), table)
	page := container.NewVScroll(container.NewVBox(box, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	on := geo.NewPos(60, 150)

	table.ScrollToBottom()
	table.SetRowHeight(0, 30)
	swipe(c, 1, on, on.SubtractXY(0, 60))
	assert.Equal(t, float32(60), page.Offset.Y, "the page took the swipe the table could not follow")

	// A downward swipe the page cannot follow either: nothing moves, and no
	// row is resized by it.
	page.ScrollToTop()
	table.ScrollToTop()
	table.UpdateCell = func(id widget.TableCellID, o core.CanvasObject) {
		if id.Row == 0 {
			lastRowHeight = o.Size().Height
		}
	}
	swipe(c, 2, on, on.AddXY(0, 60))
	table.RefreshItem(widget.TableCellID{Row: 0, Col: 0})
	assert.Zero(t, page.Offset.Y)
	assert.Equal(t, float32(30), lastRowHeight, "the swipe resized row 0")
}

// An Interactive in a page (#1266) keeps a drag only when it has a drag
// callback to give it to.
func Test_canvas_SwipeOnInteractiveInAPage(t *testing.T) {
	target := widget.NewInteractive(filler(200, 100))
	page := container.NewVScroll(container.NewVBox(target, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	on := geo.NewPos(100, 50)

	swipe(c, 1, on, on.SubtractXY(0, 40))
	assert.Equal(t, float32(40), page.Offset.Y, "an Interactive with no drag callback left the swipe to the page")

	page.ScrollToTop()
	drags := 0
	target.OnDragged = func(*event.DragEvent) { drags++ }
	swipe(c, 2, on, on.SubtractXY(0, 40))
	assert.Zero(t, page.Offset.Y)
	assert.Positive(t, drags, "the Interactive took the swipe")
}

// A TimePicker in a page (#1266): the dial takes a drag in any direction, and
// a disabled picker leaves every drag to the page, as a disabled Slider does.
func Test_canvas_SwipeOnTimePickerInAPage(t *testing.T) {
	picker := widget.NewTimePicker(10, 30, true, nil)
	page := container.NewVScroll(container.NewVBox(picker, filler(100, 900)))
	c := claimCanvas(page, geo.NewSize(400, 400))
	on := geo.NewPos(picker.Size().Width/2, picker.Size().Height-60)

	swipe(c, 1, on, on.SubtractXY(0, 40))
	assert.Zero(t, page.Offset.Y, "the dial kept the swipe")

	picker.Disable()
	swipe(c, 2, on, on.SubtractXY(0, 40))
	assert.Equal(t, float32(40), page.Offset.Y, "a disabled picker left the swipe to the page")
}

// A reorderable TreeView in a page (#1296): a plain swipe that starts on a row
// scrolls the tree and moves no row, while a long-press then a drag moves the
// row, as Entry's long-press then drag selects (#801).
func Test_canvas_LongPressThenDragMovesTreeViewRow(t *testing.T) {
	tree := widget.NewTreeView()
	for i := range 40 {
		require.NoError(t, tree.AddNode("", widget.TreeViewNode{ID: widget.TreeViewNodeID(strconv.Itoa(i)), Text: "row " + strconv.Itoa(i)}))
	}
	tree.AllowReorder = true
	box := container.NewGridWrap(geo.NewSize(200, 200), tree)
	page := container.NewVScroll(container.NewVBox(box, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(200, 300))
	order := append([]widget.TreeViewNodeID(nil), tree.ChildIDs("")...)

	on := geo.NewPos(60, 150)
	swipe(c, 1, on, on.SubtractXY(0, 60))
	assert.Equal(t, float32(60), tree.GetScrollOffset(), "the tree scrolled under a plain swipe that started on a row")
	assert.Equal(t, order, tree.ChildIDs(""), "a plain swipe moved no row")
	assert.Zero(t, page.Offset.Y)

	// Contact 0, the one lprTapUp lifts. The press is on the first row on
	// screen once the tree is back at its top.
	tree.ScrollToOffset(0)
	press := geo.NewPos(60, 10)
	c.TapDown(press, 0)
	waitAndCheck(400, func() {
		assert.Equal(t, widget.TreeViewNodeID("0"), tree.Selected(), "the long-press selected the row it picked up")
	})
	for i := 1; i <= 10; i++ {
		c.TapMove(press.AddXY(0, float32(i)*10), 0, claimDrag)
	}
	lprTapUp(c, press.AddXY(0, 100))
	assert.Zero(t, tree.GetScrollOffset(), "the drag after the long-press did not scroll the tree")
	assert.Zero(t, page.Offset.Y)
	assert.NotEqual(t, widget.TreeViewNodeID("0"), tree.ChildIDs("")[0], "the long-press then drag moved the first row")
	assert.Contains(t, tree.ChildIDs("")[1:4], widget.TreeViewNodeID("0"), "to where the finger let go")
}

// A long-press on a Table's header divider, then a drag, resizes the column on
// touch (#1293): a finger never hovers, so before, nothing found the divider.
// A plain swipe over the header still resizes nothing, and a long-press away
// from any divider does not either.
func Test_canvas_LongPressThenDragResizesTableColumn(t *testing.T) {
	var width0 float32
	table := widget.NewTable(
		func() (int, int) { return 50, 4 },
		func() core.CanvasObject { return filler(60, 30) },
		func(id widget.TableCellID, o core.CanvasObject) {
			if id.Row == 0 && id.Col == 0 {
				width0 = o.Size().Width
			}
		},
	)
	table.ShowHeaderRow = true
	box := container.NewGridWrap(geo.NewSize(300, 200), table)
	page := container.NewVScroll(container.NewVBox(box, filler(100, 600)))
	c := claimCanvas(page, geo.NewSize(300, 300))
	readWidth := func() float32 {
		table.RefreshItem(widget.TableCellID{Row: 0, Col: 0})
		return width0
	}
	start := readWidth()
	require.Equal(t, float32(60), start)

	// The divider after column 0 is in the padding past its 60 points; the
	// finger lands a few points short of it.
	press := geo.NewPos(57, 10)
	swipe(c, 1, press, press.AddXY(40, 0))
	assert.Equal(t, start, readWidth(), "a plain swipe over the header resized nothing")

	longPressDrag := func(press geo.Position, dx float32) {
		c.TapDown(press, 0)
		waitAndCheck(400, func() {})
		for i := 1; i <= 10; i++ {
			c.TapMove(press.AddXY(dx*float32(i)/10, 0), 0, claimDrag)
		}
		lprTapUp(c, press.AddXY(dx, 0))
	}
	longPressDrag(geo.NewPos(30, 10), 40)
	assert.Equal(t, start, readWidth(), "a long-press inside a header cell resized nothing")

	longPressDrag(press, 40)
	assert.InDelta(t, start+40, readWidth(), 0.5, "the long-press then drag widened column 0 by the drag")
	assert.Zero(t, page.Offset.Y)

	// The resize ended with the finger: a later swipe resizes nothing more.
	swipe(c, 2, press, press.AddXY(40, 0))
	assert.InDelta(t, start+40, readWidth(), 0.5)
}
