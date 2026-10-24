package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
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
