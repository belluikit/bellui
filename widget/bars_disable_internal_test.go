package widget

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// disableHarness shows bar between two entries, so a Tab from the first one
// must cross the bar to reach the second.
func disableHarness(t *testing.T, bar core.CanvasObject) (c core.Canvas, before, after *Entry) {
	t.Helper()
	before, after = NewEntry(), NewEntry()
	box := &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{before, bar, after}}
	w := test.NewTempWindow(t, box)
	w.Resize(geo.NewSize(480, 400))
	return w.Canvas(), before, after
}

// centreOf is the canvas position of the centre of o.
func centreOf(o core.CanvasObject) geo.Position {
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(o)
	return abs.AddXY(o.Size().Width/2, o.Size().Height/2)
}

// pokeAt sends what a user can send to the object under pos: a hover, a
// tap, a drag and a scroll through the canvas, and a key to whatever has the
// focus. None of the bars takes a drag, a scroll or a key; they are sent so
// that one added later, unguarded, is caught.
func pokeAt(c core.Canvas, pos geo.Position) {
	test.MoveMouse(c, pos)
	test.TapCanvas(c, pos)
	test.Drag(c, pos, 30, 0)
	test.Scroll(c, pos, 0, -30)
	if f := c.Focused(); f != nil {
		f.TypedKey(&event.KeyEvent{Name: event.KeySpace})
		f.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	}
	test.MoveMouse(c, geo.NewPos(0, 0))
}

// assertTabCrosses checks that Tab from before goes straight to after, past
// the bar and its parts. No part of these bars takes the focus at all, so
// there is no focus inside the bar for Disable to drop: what is pinned is that
// Tab still crosses the disabled bar.
func assertTabCrosses(t *testing.T, c core.Canvas, before, after *Entry) {
	t.Helper()
	c.Focus(before)
	c.FocusNext()
	assert.Equal(t, after, c.Focused(), "Tab did not cross the disabled bar")
	c.Unfocus()
}

func navItemLook(t *testing.T, indicator *canvas.Rectangle, icon *canvas.Image, label *canvas.Text, selected, disabled bool) {
	t.Helper()
	th := test.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	iconName, labelName := theme.ColorNameOnSurfaceVariant, theme.ColorNameOnSurfaceVariant
	var fill color.Color = color.Transparent
	if selected {
		iconName, labelName = theme.ColorNameOnSecondaryContainer, theme.ColorNameOnSurface
		fill = th.Color(theme.ColorNameSecondaryContainer, v)
	}
	if disabled {
		iconName, labelName = theme.ColorNameDisabled, theme.ColorNameDisabled
		if selected {
			fill = th.Color(theme.ColorNameDisabledButton, v)
		}
	}
	res, ok := icon.Resource.(*theme.ThemedResource)
	require.True(t, ok, "the icon is not a themed resource")
	assert.Equal(t, iconName, res.ColorName, "icon colour (selected %v, disabled %v)", selected, disabled)
	assert.Equal(t, th.Color(labelName, v), label.Color, "label colour (selected %v, disabled %v)", selected, disabled)
	assert.Equal(t, fill, indicator.FillColor, "indicator fill (selected %v, disabled %v)", selected, disabled)
}

func TestNavigationBar_Disable(t *testing.T) {
	test.NewTempApp(t)
	changed := 0
	bar := NewNavigationBar([]*NavigationBarItem{{Label: "Home", Icon: theme.HomeIcon()}, {Label: "Mail", Icon: theme.MailComposeIcon()}}, func(int) { changed++ })
	bar.Selected = 1

	// Before the first draw: no window, no renderer.
	bar.Disable()
	assert.True(t, bar.Disabled())
	c, before, after := disableHarness(t, bar)

	items := cache.Renderer(bar).(*navBarRenderer).items
	require.Len(t, items, 2)
	look := func(disabled bool) {
		t.Helper()
		for i, item := range items {
			r := cache.Renderer(item).(*navBarItemRenderer)
			navItemLook(t, r.indicator, r.icon, r.label, i == bar.Selected, disabled)
			assert.Equal(t, disabled, item.Disabled(), "item %d does not report the bar's state", i)
		}
	}
	refused := func() {
		t.Helper()
		look(true)
		for _, item := range items {
			pokeAt(c, centreOf(item))
			item.PerformAccessibleAction(core.ActionSelect)
			_, ok := item.pressShape()
			assert.False(t, ok, "a disabled item shows a pressed state")
		}
		assert.Equal(t, 0, changed, "a disabled bar fired OnChanged")
		assert.Equal(t, 1, bar.Selected, "a disabled bar changed its selection")
		assertTabCrosses(t, c, before, after)
	}
	refused()

	bar.Enable()
	look(false)
	test.TapCanvas(c, centreOf(items[0]))
	assert.Equal(t, 1, changed, "an enabled bar refused a tap")
	assert.Equal(t, 0, bar.Selected)
	bar.Selected = 1
	bar.Refresh()
	changed = 0

	// After the first draw.
	bar.Disable()
	refused()
	bar.Enable()
	look(false)
}

func TestNavigationRail_Disable(t *testing.T) {
	test.NewTempApp(t)
	changed := 0
	rail := NewNavigationRail([]*NavigationRailItem{{Label: "Home", Icon: theme.HomeIcon()}, {Label: "Mail", Icon: theme.MailComposeIcon()}}, func(int) { changed++ })
	rail.Selected = 1

	rail.Disable()
	assert.True(t, rail.Disabled())
	c, before, after := disableHarness(t, rail)

	items := cache.Renderer(rail).(*navRailRenderer).items
	require.Len(t, items, 2)
	look := func(disabled bool) {
		t.Helper()
		for i, item := range items {
			r := cache.Renderer(item).(*navRailItemRenderer)
			navItemLook(t, r.indicator, r.icon, r.label, i == rail.Selected, disabled)
			assert.Equal(t, disabled, item.Disabled(), "item %d does not report the rail's state", i)
		}
	}
	refused := func() {
		t.Helper()
		look(true)
		for _, item := range items {
			pokeAt(c, centreOf(item))
			item.PerformAccessibleAction(core.ActionSelect)
			_, ok := item.pressShape()
			assert.False(t, ok, "a disabled item shows a pressed state")
		}
		assert.Equal(t, 0, changed, "a disabled rail fired OnChanged")
		assert.Equal(t, 1, rail.Selected, "a disabled rail changed its selection")
		assertTabCrosses(t, c, before, after)
	}
	refused()

	rail.Enable()
	look(false)
	test.TapCanvas(c, centreOf(items[0]))
	assert.Equal(t, 1, changed, "an enabled rail refused a tap")
	assert.Equal(t, 0, rail.Selected)
	rail.Selected = 1
	rail.Refresh()
	changed = 0

	rail.Disable()
	refused()
	rail.Enable()
	look(false)
}

// iconButtonLook checks an icon button's icon colour and that it shows no
// hover or fill while disabled.
func iconButtonLook(t *testing.T, btn *IconButton, disabled bool) {
	t.Helper()
	r := cache.Renderer(btn).(*iconButtonRenderer)
	res, ok := r.icon.Resource.(*theme.ThemedResource)
	require.True(t, ok)
	if disabled {
		assert.Equal(t, theme.ColorNameDisabled, res.ColorName, "a disabled bar's button keeps its colour")
		assert.Equal(t, color.Transparent, r.bg.FillColor, "a disabled bar's button keeps a fill or hover layer")
		assert.True(t, btn.Disabled())
	} else {
		assert.Equal(t, theme.ColorNameOnSurfaceVariant, res.ColorName)
		assert.False(t, btn.Disabled())
	}
}

func TestTopAppBar_Disable(t *testing.T) {
	test.NewTempApp(t)
	taps := 0
	tap := func() { taps++ }
	bar := NewTopAppBar("Title", theme.MenuIcon(), tap)
	bar.Actions = []TopAppBarAction{{Icon: theme.SearchIcon(), OnTapped: tap}, {Icon: theme.SettingsIcon(), OnTapped: tap}}

	bar.Disable()
	assert.True(t, bar.Disabled())
	c, before, after := disableHarness(t, bar)

	r := cache.Renderer(bar).(*topAppBarRenderer)
	buttons := func() []*IconButton { return append([]*IconButton{r.navBtn}, r.actionBtns...) }
	require.Len(t, buttons(), 3)
	look := func(disabled bool) {
		t.Helper()
		name := theme.ColorNameOnSurface
		if disabled {
			name = theme.ColorNameDisabled
		}
		assert.Equal(t, test.Theme().Color(name, bellui.CurrentApp().Settings().ThemeVariant()), r.title.Color, "title colour")
		for _, btn := range buttons() {
			iconButtonLook(t, btn, disabled)
		}
	}
	refused := func() {
		t.Helper()
		look(true)
		for _, btn := range buttons() {
			pokeAt(c, centreOf(btn))
			btn.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
			btn.MouseUp(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
			btn.PerformAccessibleAction(core.ActionPress)
		}
		assert.Equal(t, 0, taps, "a disabled bar ran a button's handler")
		assertTabCrosses(t, c, before, after)
	}
	refused()

	bar.Enable()
	look(false)
	for _, btn := range buttons() {
		test.TapCanvas(c, centreOf(btn))
	}
	assert.Equal(t, 3, taps, "an enabled bar refused a tap")
	taps = 0

	bar.Disable()
	refused()

	// Buttons the bar builds while disabled are disabled from the start.
	bar.Actions = append(bar.Actions, TopAppBarAction{Icon: theme.DeleteIcon(), OnTapped: tap})
	bar.Refresh()
	require.Len(t, buttons(), 4)
	refused()

	bar.Enable()
	look(false)
}

func TestBottomAppBar_Disable(t *testing.T) {
	test.NewTempApp(t)
	taps, fabTaps := 0, 0
	tap := func() { taps++ }
	fab := NewFAB(theme.ContentAddIcon(), func() { fabTaps++ })
	bar := NewBottomAppBar([]*BottomAppBarAction{{Icon: theme.SearchIcon(), OnTapped: tap}, {Icon: theme.DeleteIcon(), OnTapped: tap}}, fab)

	bar.Disable()
	assert.True(t, bar.Disabled())
	assert.True(t, fab.Disabled(), "Disable before the first draw left the FAB enabled")
	c, before, after := disableHarness(t, bar)

	r := cache.Renderer(bar).(*bottomAppBarRenderer)
	require.Len(t, r.actionBtns, 2)
	fabLook := func(disabled bool) {
		t.Helper()
		fr := cache.Renderer(fab).(*fabRenderer)
		res, ok := fr.icon.Resource.(*theme.ThemedResource)
		require.True(t, ok)
		v := bellui.CurrentApp().Settings().ThemeVariant()
		if disabled {
			assert.Equal(t, theme.ColorNameDisabled, res.ColorName, "FAB icon colour")
			assert.Equal(t, test.Theme().Color(theme.ColorNameDisabledButton, v), fr.bg.FillColor, "FAB container colour")
		} else {
			assert.Equal(t, theme.ColorNameOnPrimaryContainer, res.ColorName, "FAB icon colour")
			assert.Equal(t, test.Theme().Color(theme.ColorNamePrimaryContainer, v), fr.bg.FillColor, "FAB container colour")
		}
	}
	look := func(disabled bool) {
		t.Helper()
		for _, btn := range r.actionBtns {
			iconButtonLook(t, btn, disabled)
		}
		assert.Equal(t, disabled, fab.Disabled())
		fabLook(disabled)
	}
	refused := func() {
		t.Helper()
		look(true)
		for _, btn := range r.actionBtns {
			pokeAt(c, centreOf(btn))
			btn.PerformAccessibleAction(core.ActionPress)
		}
		pokeAt(c, centreOf(fab))
		test.MoveMouse(c, centreOf(fab))
		fabLook(true) // no hover state layer on a disabled FAB
		test.MoveMouse(c, geo.NewPos(0, 0))
		fab.PerformAccessibleAction(core.ActionPress)
		_, ok := fab.pressShape()
		assert.False(t, ok, "a disabled FAB shows a pressed state")
		assert.Equal(t, 0, taps, "a disabled bar ran an action's handler")
		assert.Equal(t, 0, fabTaps, "a disabled bar's FAB ran its handler")
		assertTabCrosses(t, c, before, after)
	}
	refused()

	bar.Enable()
	look(false)
	for _, btn := range r.actionBtns {
		test.TapCanvas(c, centreOf(btn))
	}
	test.TapCanvas(c, centreOf(fab))
	assert.Equal(t, 2, taps, "an enabled bar refused a tap")
	assert.Equal(t, 1, fabTaps, "an enabled bar's FAB refused a tap")
	taps, fabTaps = 0, 0

	bar.Disable()
	refused()
	bar.Enable()
	look(false)
}

// The FAB is the app's widget: the bar's Enable undoes only the bar's own
// Disable, and a FAB that leaves a disabled bar leaves enabled.
func TestBottomAppBar_DisableKeepsAppFABState(t *testing.T) {
	test.NewTempApp(t)
	fab := NewFAB(theme.ContentAddIcon(), func() {})
	bar := NewBottomAppBar(nil, fab)
	test.NewTempWindow(t, bar)

	fab.Disable()
	bar.Disable()
	bar.Enable()
	assert.True(t, fab.Disabled(), "the bar's Enable undid the app's FAB.Disable")
	fab.Enable()

	bar.Disable()
	other := NewFAB(theme.ContentAddIcon(), func() {})
	bar.FAB = other
	bar.Refresh()
	assert.False(t, fab.Disabled(), "a FAB replaced in a disabled bar stays disabled")
	assert.True(t, other.Disabled(), "a FAB added to a disabled bar is enabled")
	bar.Enable()
	assert.False(t, other.Disabled())
}
