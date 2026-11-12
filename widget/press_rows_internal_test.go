package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// pressEvents is what PressTarget adds to a widget.
type pressEvents interface {
	mobile.Touchable
	desktop.Mouseable
}

// assertPressCycle checks that a press target shows its pressed state from the
// press to the release, and ends it on a touch cancel and on hoverOut, the
// pointer leaving it (#1091).
func assertPressCycle(t *testing.T, target pressEvents, t0 *widget.PressTarget, hoverOut func()) {
	t.Helper()
	cache.Renderer(target.(core.Widget))
	press := widget.PressFeedbackOf(t0)
	require.NotNil(t, press, "the renderer attached no feedback")

	target.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(4, 4)}})
	assert.True(t, press.Pressed(), "no pressed state while the finger is down")
	assert.True(t, press.Ripple.Visible())
	target.TouchUp(&mobile.TouchEvent{})
	assert.False(t, press.Pressed(), "the release left it pressed")

	target.TouchDown(&mobile.TouchEvent{})
	target.TouchCancel(&mobile.TouchEvent{})
	assert.False(t, press.Pressed())
	assert.False(t, press.Ripple.Visible(), "a touch that became a scroll left it pressed")

	target.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	assert.True(t, press.Pressed())
	target.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	assert.False(t, press.Pressed())

	if hoverOut != nil {
		target.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
		hoverOut()
		assert.False(t, press.Pressed(), "a press dragged off it stayed")
		assert.False(t, press.Ripple.Visible())
	}

	target.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonSecondary})
	assert.False(t, press.Pressed(), "a secondary click pressed it")
}

// assertRowShape checks the pressed state covers the whole of obj, in
// OnSurface, with corners of radius.
func assertRowShape(t *testing.T, obj core.CanvasObject, t0 *widget.PressTarget, radius float32) {
	t.Helper()
	obj.(pressEvents).TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(6, 3)}})
	defer obj.(pressEvents).TouchCancel(nil)
	r := widget.PressFeedbackOf(t0).Ripple
	assert.Equal(t, geo.Position{}, r.Position())
	assert.Equal(t, obj.Size(), r.Size(), "the pressed state does not fill the row")
	assert.Equal(t, radius, r.CornerRadius)
	assert.Equal(t, geo.NewPos(6, 3), r.Center, "the ripple does not start at the press")
	assertSameRGB(t, test.Theme().Color(theme.ColorNameOnSurface, bellui.CurrentApp().Settings().ThemeVariant()), r.Color)
}

func assertSameRGB(t *testing.T, want, got color.Color) {
	t.Helper()
	wr, wg, wb, _ := want.RGBA()
	r, g, b, _ := got.RGBA()
	assert.Equal(t, [3]uint32{wr >> 8, wg >> 8, wb >> 8}, [3]uint32{r >> 8, g >> 8, b >> 8})
}

// assertTapFromCode checks that a tap with no press before it, from a key or
// an accessibility action, still shows one.
func assertTapFromCode(t *testing.T, t0 *widget.PressTarget, tap func()) {
	t.Helper()
	r := widget.PressFeedbackOf(t0).Ripple
	r.Radius = 0
	tap()
	assert.NotZero(t, r.Radius, "a tap from code showed no press")
}

func TestListItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	li := newListItem(NewLabel("row"), func() {})
	li.Resize(geo.NewSize(120, 40))
	assertPressCycle(t, li, &li.pressTarget, li.HoverOut)
	assertRowShape(t, li, &li.pressTarget, test.Theme().Size(theme.SizeNameSelectionRadius))
	assertTapFromCode(t, &li.pressTarget, func() { li.PerformAccessibleAction(core.ActionSelect) })

	off := newListItem(NewLabel("row"), nil)
	off.Resize(geo.NewSize(120, 40))
	cache.Renderer(off)
	off.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&off.pressTarget).Pressed(), "a row with no tap handler showed a press")
}

func TestGridWrapItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	gw := newGridWrapItem(NewLabel("cell"), func() {})
	gw.Resize(geo.NewSize(60, 60))
	assertPressCycle(t, gw, &gw.pressTarget, gw.HoverOut)
	assertRowShape(t, gw, &gw.pressTarget, test.Theme().Size(theme.SizeNameSelectionRadius))
	assertTapFromCode(t, &gw.pressTarget, func() { gw.PerformAccessibleAction(core.ActionSelect) })
}

func TestMenuItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	m := NewMenu(bellui.NewMenu("", bellui.NewMenuItem("Open", func() {}), &bellui.MenuItem{Label: "Off", Disabled: true}))
	m.Resize(m.MinSize())
	item := m.Items[0].(*menuItem)
	assertPressCycle(t, item, &item.pressTarget, item.HoverOut)
	assertRowShape(t, item, &item.pressTarget, test.Theme().Size(theme.SizeNameSelectionRadius))

	off := m.Items[1].(*menuItem)
	cache.Renderer(off)
	off.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&off.pressTarget).Pressed(), "a disabled item showed a press")
}

func TestTableSortHeader_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	h := newTableSortHeader()
	h.onTapped = func() {}
	h.set("Name", SortNone)
	h.Resize(geo.NewSize(100, 30))
	assertPressCycle(t, h, &h.pressTarget, h.HoverOut)
	assertRowShape(t, h, &h.pressTarget, 0)
	assertTapFromCode(t, &h.pressTarget, func() { h.Tapped(nil) })
}

func TestTreeNode_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	tree := NewTreeWithStrings(map[string][]string{"": {"a"}, "a": {"b"}})
	l := newLeaf(tree, NewLabel("leaf"))
	l.Resize(geo.NewSize(150, 36))
	assertPressCycle(t, l, &l.pressTarget, l.HoverOut)
	assertRowShape(t, l, &l.pressTarget, test.Theme().Size(theme.SizeNameSelectionRadius))

	// A branch's expand icon is a press target of its own: a circle centred on
	// the icon, wherever on it the press lands.
	b := newBranch(tree, NewLabel("branch"))
	b.Resize(geo.NewSize(150, 36))
	cache.Renderer(b)
	icon := b.icon.(*branchIcon)
	assertPressCycle(t, icon, &icon.pressTarget, nil)
	icon.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(1, 1)}})
	r := widget.PressFeedbackOf(&icon.pressTarget).Ripple
	assert.Equal(t, r.Size().Width, r.Size().Height)
	assert.Equal(t, r.Size().Width/2, r.CornerRadius, "the icon's pressed state is not a circle")
	centre := r.Position().Add(geo.NewPos(r.Size().Width/2, r.Size().Height/2))
	iconCentre := icon.Position().Add(geo.NewPos(icon.Size().Width/2, icon.Size().Height/2))
	assert.Equal(t, iconCentre, centre, "the circle is not around the icon")
	assert.Equal(t, geo.NewPos(r.Size().Width/2, r.Size().Height/2), r.Center, "the ripple does not spread from the icon's centre")
	assert.False(t, widget.PressFeedbackOf(&b.pressTarget).Pressed(), "a press on the icon pressed the row")

	// Released off the icon, on the row, the icon's press ends.
	b.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	assert.False(t, widget.PressFeedbackOf(&icon.pressTarget).Pressed(), "a press released off the icon stayed")
	icon.TouchDown(&mobile.TouchEvent{})
	b.HoverOut()
	assert.False(t, widget.PressFeedbackOf(&icon.pressTarget).Pressed(), "a press taken off the row stayed on the icon")
}

func TestTreeViewItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	tv := NewTreeView()
	tv.AllowReorder = true
	ti := newTreeViewItem(tv, NewLabel("node"), false)
	ti.Resize(geo.NewSize(150, 36))
	assertPressCycle(t, ti, &ti.pressTarget, ti.HoverOut)
	assertRowShape(t, ti, &ti.pressTarget, test.Theme().Size(theme.SizeNameShapeSmall))

	ti.TouchDown(&mobile.TouchEvent{})
	ti.Dragged(&event.DragEvent{Dragged: geo.NewDelta(0, 10)})
	ti.DragEnd()
	assert.False(t, widget.PressFeedbackOf(&ti.pressTarget).Pressed(), "a row being dragged showed a press")

	ti.renaming = true
	ti.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&ti.pressTarget).Pressed(), "a row being renamed showed a press")
}

// PressedListRowsForTest counts the rows of the list in c that show a pressed
// state, for the fling test, which drives the driver's touch handler and so
// lives outside this package.
func PressedListRowsForTest(c core.Canvas) (n int) {
	intdriver.WalkVisibleObjectTree(c.Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if li, ok := o.(*listItem); ok {
			if p := widget.PressFeedbackOf(&li.pressTarget); p != nil && (p.Pressed() || p.Ripple.Visible()) {
				n++
			}
		}
		return false
	}, nil)
	return n
}
