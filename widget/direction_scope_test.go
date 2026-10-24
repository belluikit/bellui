package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// inRTLScope shows obj inside a right-to-left container.DirectionScope in a
// left-to-right app, the setup of #696: layout mirrors through the scope, and
// these tests check that event handlers and Refresh-driven layout follow it.
func inRTLScope(t *testing.T, obj core.CanvasObject) bellui.Window {
	t.Helper()
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	require.False(t, lang.IsRTLDirection(), "the app itself is left to right")

	w := test.NewTempWindow(t, container.NewDirectionScope(obj, bellui.DirectionRTL))
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 200))
	return w
}

func TestCarousel_SwipeInRTLScope(t *testing.T) {
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"), widget.NewLabel("3"))
	w := inRTLScope(t, c)

	test.Drag(w.Canvas(), geo.NewPos(150, 80), 150, 0) // a right swipe advances under RTL
	assert.Equal(t, 1, c.CurrentItem)
}

func TestCarousel_ControlsStayMirroredAfterNextInRTLScope(t *testing.T) {
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"), widget.NewLabel("3"))
	w := inRTLScope(t, c)
	objs := cache.Renderer(c).Objects()
	next := objs[len(objs)-1]
	require.Less(t, next.Position().X, float32(150), "laid out mirrored: next on the left")

	c.Next() // Refresh -> Layout outside any layout walk
	assert.Less(t, next.Position().X, float32(150), "still mirrored after Next")

	w.Canvas().Refresh(c)
	assert.Less(t, next.Position().X, float32(150), "still mirrored after a canvas refresh")
}

func TestInputChip_DeleteRegionInRTLScope(t *testing.T) {
	var tapped, deleted bool
	c := widget.NewChip("Tag", func() { tapped = true })
	c.Style = widget.ChipStyleInput
	c.OnDeleted = func() { deleted = true }
	inRTLScope(t, container.NewHBox(c))

	// Under RTL the X sits on the leading (left) edge.
	test.TapAt(c, geo.NewPos(3, c.Size().Height/2))
	assert.True(t, deleted, "tapping the mirrored X fires OnDeleted")
	assert.False(t, tapped)
}

func TestSegmentedButton_ArrowKeysInRTLScope(t *testing.T) {
	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, nil)
	sb.Selected = 1
	inRTLScope(t, sb)

	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 0, sb.Selected, "RTL KeyRight retreats")
	sb.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 1, sb.Selected, "RTL KeyLeft advances")
}

func TestSelect_ArrowKeysInRTLScope(t *testing.T) {
	s := widget.NewSelect([]string{"A", "B", "C"}, nil)
	s.SetSelected("B")
	inRTLScope(t, s)

	s.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, "A", s.Selected, "RTL KeyRight steps to the previous option")
	s.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, "B", s.Selected, "RTL KeyLeft steps to the next option")
}

func TestGridWrap_ArrowKeysInRTLScope(t *testing.T) {
	selected := -1
	g := widget.NewGridWrap(
		func() int { return 6 },
		func() core.CanvasObject { return widget.NewLabel("item") },
		func(widget.GridWrapItemID, core.CanvasObject) {},
	)
	g.OnSelected = func(id widget.GridWrapItemID) { selected = id }
	inRTLScope(t, g)

	g.TypedKey(&event.KeyEvent{Name: event.KeyLeft}) // RTL: the left arrow advances
	g.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.Equal(t, 1, selected)
}

// Each fixed reader outside a scope still reads the global direction.
func TestDirectionScope_ReadersOutsideScopeFollowApp(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, nil)
	sb.Selected = 1
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"))
	scoped := widget.NewLabel("scoped")
	w := test.NewTempWindow(t, container.NewVBox(sb, c, container.NewDirectionScope(scoped, bellui.DirectionRTL)))
	w.Resize(geo.NewSize(300, 300))

	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 2, sb.Selected, "LTR KeyRight advances outside the scope")
	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(sb))
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(scoped))
}

// Scrolling places the newly visible rows outside a layout walk, once a frame;
// in an RTL scope their first column must still sit on the right.
func TestGridWrap_ScrollPlacementInRTLScope(t *testing.T) {
	labels := map[widget.GridWrapItemID]*widget.Label{}
	g := widget.NewGridWrap(
		func() int { return 200 },
		func() core.CanvasObject { return widget.NewLabel("item") },
		func(id widget.GridWrapItemID, o core.CanvasObject) { labels[id] = o.(*widget.Label) },
	)
	inRTLScope(t, g)
	cols := g.ColumnCount()
	require.Greater(t, cols, 1)

	g.ScrollToOffset(1000) // well past the rows laid out by the walk
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject
	var row int
	for id := range labels {
		if r := int(id) / cols; r > row {
			row = r
		}
	}
	first, second := labels[widget.GridWrapItemID(row*cols)], labels[widget.GridWrapItemID(row*cols+1)]
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.Greater(t, pos(first).X, pos(second).X, "the first column leads on the right after a scroll")
}

// An object on no window gets the app's direction, even while a window with
// its own direction is open.
func TestDirectionOf_OffCanvasWithDirectedWindow(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	w := test.NewTempWindow(t, widget.NewLabel("shown"))
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)

	assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(widget.NewLabel("loose")))
}

// The lookup finds an object in its own window, not the one the test driver's
// CanvasForObject guesses (the last created).
func TestDirectionOf_ScopeInEarlierWindow(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	sb := widget.NewSegmentedButton([]string{"A", "B", "C"}, nil)
	sb.Selected = 1
	test.NewTempWindow(t, container.NewDirectionScope(sb, bellui.DirectionRTL))
	test.NewTempWindow(t, widget.NewLabel("opened later"))

	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(sb))
	sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 0, sb.Selected, "RTL KeyRight retreats")
}

// A renderer that lays itself out from its own Refresh — here the badge —
// keeps the scope's mirroring: BaseWidget.Refresh runs in the widget's own
// direction outside a layout walk (#749).
func TestBadge_RefreshKeepsMirrorInRTLScope(t *testing.T) {
	b := widget.NewBadgeWithCount(widget.NewIcon(nil), 3)
	b.Resize(geo.NewSize(100, 40))
	inRTLScope(t, container.NewWithoutLayout(b))
	b.Resize(geo.NewSize(100, 40))
	objs := cache.Renderer(b).Objects()
	var pill core.CanvasObject
	for _, o := range objs {
		if r, ok := o.(*canvas.Rectangle); ok && r.Visible() && r.Size().Width > 0 {
			pill = r
		}
	}
	require.NotNil(t, pill, "the count pill")
	require.Less(t, pill.Position().X, float32(50), "laid out mirrored: the pill leads on the left")

	b.Count = 12
	b.Refresh() // Refresh -> Layout outside any layout walk
	assert.Less(t, pill.Position().X, float32(50), "still on the left after Refresh")
}

// GridWrap resized by hand, outside a layout walk, with a column-count change,
// keeps its first column on the right in an RTL scope (#749 comment).
func TestGridWrap_ResizeByHandInRTLScope(t *testing.T) {
	labels := map[widget.GridWrapItemID]*widget.Label{}
	g := widget.NewGridWrap(
		func() int { return 30 },
		func() core.CanvasObject { return widget.NewLabel("item") },
		func(id widget.GridWrapItemID, o core.CanvasObject) { labels[id] = o.(*widget.Label) },
	)
	inRTLScope(t, container.NewWithoutLayout(g))
	g.Resize(geo.NewSize(150, 200))
	cols := g.ColumnCount()
	g.Resize(geo.NewSize(290, 200)) // by hand: no layout walk runs
	require.Greater(t, g.ColumnCount(), cols, "the column count changed")

	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject
	first, second := labels[0], labels[1]
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.Greater(t, pos(first).X, pos(second).X, "the first column leads on the right")
}

// A Select's popup sits in the window's overlays, outside the scope, but it
// takes the direction of the Select that opened it (#749).
func TestSelect_PopUpInRTLScope(t *testing.T) {
	s := widget.NewSelect([]string{"A", "B"}, nil)
	w := inRTLScope(t, container.NewVBox(s))

	test.Tap(s)
	oc, ok := w.Canvas().Overlays().Top().(interface{ core.Widget })
	require.True(t, ok, "the popup is open")
	var menu *widget.PopUpMenu
	for _, o := range cache.Renderer(oc).Objects() {
		if m, ok := o.(*widget.PopUpMenu); ok {
			menu = m
		}
	}
	require.NotNil(t, menu, "the popup menu")
	require.NotEmpty(t, menu.Items)
	assert.Equal(t, bellui.DirectionRTL, container.DirectionOf(menu.Items[0]), "the menu lays out right to left")

	outside := widget.NewSelect([]string{"A", "B"}, nil)
	w.SetContent(container.NewVBox(outside))
	test.Tap(outside)
	for _, o := range cache.Renderer(w.Canvas().Overlays().Top().(core.Widget)).Objects() {
		if m, ok := o.(*widget.PopUpMenu); ok {
			assert.Equal(t, bellui.DirectionLTR, container.DirectionOf(m.Items[0]), "outside a scope the menu follows the app")
		}
	}
}

// The DateEntry calendar popup's button row mirrors inside an RTL scope: OK,
// the trailing button, sits left of Cancel (#749).
func TestDateEntry_PopUpButtonsMirrorInRTLScope(t *testing.T) {
	e := widget.NewDateEntry()
	w := inRTLScope(t, container.NewVBox(e))
	test.Tap(e.ActionItem.(*widget.Button))

	pop, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	require.True(t, ok, "the calendar popup is open")
	content := pop.Content.(*bellui.Container)
	buttons := content.Objects[1].(*bellui.Container)
	cancel, okBtn := buttons.Objects[1], buttons.Objects[2]
	assert.Less(t, okBtn.Position().X, cancel.Position().X, "OK leads on the left under RTL")
}
