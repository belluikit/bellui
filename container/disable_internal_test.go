package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// disableContainers builds each container of #1218 around content.
var disableContainers = map[string]func(content core.CanvasObject) core.Disableable{
	"AppTabs": func(content core.CanvasObject) core.Disableable {
		return NewAppTabs(NewTabItem("A", content), NewTabItem("B", widget.NewLabel("b")))
	},
	"DocTabs": func(content core.CanvasObject) core.Disableable {
		return NewDocTabs(NewTabItem("A", content), NewTabItem("B", widget.NewLabel("b")))
	},
	"InnerWindow": func(content core.CanvasObject) core.Disableable {
		return NewInnerWindow("Title", content)
	},
}

// Disable before the first draw and after it, then Enable, leave the
// container and the widget inside it in the right state (#1218). Player
// disables while it builds the tree, before anything is drawn.
func TestDisableContainers_BeforeAndAfterDraw(t *testing.T) {
	for name, build := range disableContainers {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			child := widget.NewButton("child", func() {})
			c := build(child)

			c.Disable()
			assert.True(t, c.Disabled())
			assert.True(t, child.Disabled(), "the child was not disabled before the first draw")

			test.NewTempWindow(t, c.(core.CanvasObject)).Resize(geo.NewSize(400, 300))
			assert.True(t, c.Disabled())
			assert.True(t, child.Disabled(), "drawing the container enabled the child")

			c.Enable()
			assert.False(t, c.Disabled())
			assert.False(t, child.Disabled())

			c.Disable()
			assert.True(t, child.Disabled(), "the child was not disabled after the first draw")
			c.Enable()
			assert.False(t, child.Disabled())
		})
	}
}

// While disabled a container gives up a focus inside it, Tab skips what is
// inside, and keys typed at it change nothing (#1218). The subject is a
// Slider: a disabled Entry may take a focus to select its text (#1251).
func TestDisableContainers_FocusAndKeys(t *testing.T) {
	for name, build := range disableContainers {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			entry := widget.NewSlider(0, 10)
			c := build(entry)
			other := widget.NewEntry()
			win := test.NewTempWindow(t, NewVBox(c.(core.CanvasObject), other))
			win.Resize(geo.NewSize(400, 400))
			canvas := win.Canvas()

			canvas.Focus(entry)
			require.True(t, entry == canvas.Focused(), "the entry did not take the focus, so the check would prove nothing")
			c.Disable()
			assert.Nil(t, canvas.Focused(), "the disabled container kept a focus inside it")

			canvas.FocusNext()
			assert.True(t, other == canvas.Focused(), "Tab did not skip the disabled container's content")
			canvas.Focus(entry)
			assert.True(t, other == canvas.Focused(), "the slider in a disabled container took the focus")

			entry.TypedKey(&event.KeyEvent{Name: event.KeyRight})
			entry.TypedRune('+')
			assert.Zero(t, entry.Value, "a key reached the slider in a disabled container")

			c.Enable()
			canvas.Focus(entry)
			assert.True(t, entry == canvas.Focused(), "the slider took no focus once the container was enabled")
			entry.TypedKey(&event.KeyEvent{Name: event.KeyRight})
			assert.NotZero(t, entry.Value, "the key the check relies on does not move the slider")
		})
	}
}

// Disabling a container disables the widgets inside every part of it, the
// tabs not shown too, and a button inside a widget that is not disableable,
// like HTML's <fieldset disabled>. Enabling it brings back only what it
// disabled: a widget the app disabled stays disabled (owner,
// belluikit/player#641, 2026-11-15).
func TestAppTabs_DisableCascade(t *testing.T) {
	test.NewTempApp(t)
	own := widget.NewButton("own", func() {})
	own.Disable()
	inCard := widget.NewButton("in card", func() {})
	hidden := widget.NewCheck("hidden tab", nil)
	nested := widget.NewButton("nested tabs", func() {})
	inner := NewAppTabs(NewTabItem("X", nested))
	tabs := NewAppTabs(
		NewTabItem("A", NewVBox(own, widget.NewCard("card", "", inCard))),
		NewTabItem("B", hidden),
		NewTabItem("C", inner),
	)

	tabs.Disable()
	assert.True(t, inCard.Disabled(), "a button inside a card")
	assert.True(t, hidden.Disabled(), "a widget in a tab not shown")
	assert.True(t, inner.Disabled(), "tabs inside the tabs")
	assert.True(t, nested.Disabled(), "a widget inside the inner tabs")

	win := test.NewTempWindow(t, tabs)
	win.Resize(geo.NewSize(400, 300))

	// The app cannot enable a child while the tabs are disabled: the next
	// refresh disables it again.
	inCard.Enable()
	tabs.Refresh()
	assert.True(t, inCard.Disabled(), "a child the app enabled stayed enabled in disabled tabs")

	// Content arriving while disabled is disabled; content leaving is
	// released.
	added := widget.NewButton("added", func() {})
	tabs.Append(NewTabItem("D", added))
	assert.True(t, added.Disabled(), "a tab appended while disabled")
	tabs.RemoveIndex(1)
	assert.False(t, hidden.Disabled(), "a tab removed while disabled kept its widget disabled")
	replaced := widget.NewButton("replaced", func() {})
	tabs.SetItems(append(tabs.Items, NewTabItem("E", replaced)))
	assert.True(t, replaced.Disabled(), "items set while disabled")

	tabs.Enable()
	assert.False(t, inCard.Disabled())
	assert.False(t, inner.Disabled())
	assert.False(t, nested.Disabled())
	assert.False(t, added.Disabled())
	assert.False(t, replaced.Disabled())
	assert.True(t, own.Disabled(), "enabling the tabs enabled a button the app disabled")
}

func TestDocTabs_DisableCascade(t *testing.T) {
	test.NewTempApp(t)
	own := widget.NewButton("own", func() {})
	own.Disable()
	ours := widget.NewButton("ours", func() {})
	gone := widget.NewButton("gone", func() {})
	tabs := NewDocTabs(NewTabItem("A", NewVBox(own, ours)), NewTabItem("B", gone))
	test.NewTempWindow(t, tabs).Resize(geo.NewSize(400, 300))

	tabs.Disable()
	assert.True(t, ours.Disabled())
	assert.True(t, gone.Disabled())

	added := widget.NewButton("added", func() {})
	tabs.SetItems([]*TabItem{tabs.Items[0], NewTabItem("C", added)})
	assert.True(t, added.Disabled(), "items set while disabled")
	assert.False(t, gone.Disabled(), "a tab removed while disabled kept its widget disabled")

	tabs.Enable()
	assert.False(t, ours.Disabled())
	assert.False(t, added.Disabled())
	assert.True(t, own.Disabled(), "enabling the tabs enabled a button the app disabled")
}

func TestInnerWindow_DisableCascade(t *testing.T) {
	test.NewTempApp(t)
	own := widget.NewButton("own", func() {})
	own.Disable()
	ours := widget.NewButton("ours", func() {})
	w := NewInnerWindow("Title", NewVBox(own, ours))
	test.NewTempWindow(t, w).Resize(geo.NewSize(400, 300))

	w.Disable()
	assert.True(t, ours.Disabled())

	next := widget.NewButton("next", func() {})
	w.SetContent(next)
	assert.True(t, next.Disabled(), "content set while disabled")
	assert.False(t, ours.Disabled(), "content replaced while disabled kept its widget disabled")
	w.SetContent(NewVBox(own, ours))
	assert.True(t, ours.Disabled())
	assert.False(t, next.Disabled())

	w.Enable()
	assert.False(t, ours.Disabled())
	assert.True(t, own.Disabled(), "enabling the window enabled a button the app disabled")
}

// Disabled tabs take no tap on a tab, no drag along them, no accessible
// select and open no overflow menu; OnSelected does not fire. Select from
// code still works (#1218).
func TestAppTabs_DisabledRefusesInput(t *testing.T) {
	test.NewTempApp(t)
	a := NewTabItem("A", widget.NewLabel("a"))
	b := NewTabItem("B", widget.NewLabel("b"))
	tabs := NewAppTabs(a, b)
	tabs.DragToSelect = true
	var selected []string
	tabs.OnSelected = func(i *TabItem) { selected = append(selected, i.Text) }
	win := test.NewTempWindow(t, tabs)
	win.Resize(geo.NewSize(400, 300))
	r := cache.Renderer(tabs).(*appTabsRenderer)

	tabs.Disable()
	test.Tap(b.button)
	b.button.PerformAccessibleAction(core.ActionSelect)
	assert.False(t, r.dragSurface.ClaimsDrag(geo.Delta{DX: 10}), "a disabled tab bar claimed a drag")
	r.dragSurface.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: b.button.Position().AddXY(5, 5)},
		Dragged:    geo.Delta{DX: 100},
	})
	r.dragSurface.DragEnd()
	test.Tap(r.action)
	assert.Equal(t, a, tabs.Selected())
	assert.Empty(t, selected, "OnSelected fired from input on disabled tabs")
	assert.Nil(t, win.Canvas().Overlays().Top(), "the overflow menu opened")
	assert.True(t, b.button.AccessibleState()&core.StateDisabled != 0, "a tab of disabled tabs reports it is enabled")

	tabs.Select(b)
	assert.Equal(t, b, tabs.Selected(), "Select from code is refused")
	assert.Equal(t, []string{"B"}, selected)

	tabs.Enable()
	test.Tap(a.button)
	assert.Equal(t, a, tabs.Selected(), "a tap after Enable did not select")
}

// Disabled DocTabs close no tab, create none, open no menu and do not
// scroll their bar (#1218).
func TestDocTabs_DisabledRefusesInput(t *testing.T) {
	test.NewTempApp(t)
	items := make([]*TabItem, 12)
	for i := range items {
		items[i] = NewTabItem("Document tab", widget.NewLabel("content"))
	}
	tabs := NewDocTabs(items...)
	var closed, created, selected int
	tabs.OnClosed = func(*TabItem) { closed++ }
	tabs.OnSelected = func(*TabItem) { selected++ }
	tabs.CreateTab = func() *TabItem {
		created++
		return NewTabItem("new", widget.NewLabel("new"))
	}
	win := test.NewTempWindow(t, tabs)
	win.Resize(geo.NewSize(300, 200))
	r := cache.Renderer(tabs).(*docTabsRenderer)

	tabs.Disable()
	test.Tap(items[1].button)
	test.Tap(cache.Renderer(items[0].button).(*tabButtonRenderer).close)
	test.Tap(r.create)
	test.Tap(r.action)
	offset := r.scroller.Offset
	r.scroller.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DX: -100}})

	assert.Len(t, tabs.Items, 12, "a tab was closed")
	assert.Equal(t, items[0], tabs.Selected())
	assert.Zero(t, closed, "OnClosed")
	assert.Zero(t, created, "CreateTab")
	assert.Zero(t, selected, "OnSelected")
	assert.Nil(t, win.Canvas().Overlays().Top(), "the all-tabs menu opened")
	assert.Equal(t, offset, r.scroller.Offset, "the tab bar scrolled")

	tabs.Enable()
	test.Tap(cache.Renderer(items[0].button).(*tabButtonRenderer).close)
	assert.Equal(t, 1, closed, "a close after Enable was refused")
}

// A disabled InnerWindow closes, minimises and maximises on no tap, and
// neither moves nor resizes; none of its callbacks fire (#1218).
func TestInnerWindow_DisabledRefusesInput(t *testing.T) {
	test.NewTempApp(t)
	w := NewInnerWindow("Title", widget.NewLabel("content"))
	w.Icon = theme.HomeIcon()
	fired := map[string]int{}
	count := func(name string) func() { return func() { fired[name]++ } }
	w.CloseIntercept = count("close")
	w.OnMinimized = count("minimise")
	w.OnMaximized = count("maximise")
	w.OnTappedBar = count("bar")
	w.OnTappedIcon = count("icon")
	w.OnDragged = func(*event.DragEvent) { fired["drag"]++ }
	w.OnResized = func(*event.DragEvent) { fired["resize"]++ }
	test.NewTempWindow(t, w).Resize(geo.NewSize(400, 300))
	r := cache.Renderer(w).(*innerWindowRenderer)
	title := r.bar.Objects[2].(*bellui.Container).Objects[0].(*draggableLabel)
	drag := &event.DragEvent{Dragged: geo.Delta{DX: 10, DY: 10}}

	w.Disable()
	for _, b := range r.buttons {
		test.Tap(b)
	}
	test.Tap(r.icon)
	title.Tapped(&event.PointEvent{})
	title.Dragged(drag)
	r.corner.(*draggableCorner).Dragged(drag)
	assert.Empty(t, fired)
	assert.True(t, w.Visible())

	w.Enable()
	test.Tap(r.buttons[0])
	title.Dragged(drag)
	assert.Equal(t, map[string]int{"close": 1, "drag": 1}, fired, "the chrome refused input after Enable")
}

// The containers draw their own disabled look, so the paint walk does not
// dim them, and their bar or chrome shows the disabled colours (#1223).
func TestDisableContainers_Look(t *testing.T) {
	test.NewTempApp(t)
	for name, build := range disableContainers {
		look, ok := build(widget.NewLabel("x")).(core.DisabledLook)
		require.True(t, ok, name)
		assert.True(t, look.DrawsDisabledLook(), name)
	}

	v := bellui.CurrentApp().Settings().ThemeVariant()
	th := test.Theme()
	t.Run("AppTabs", func(t *testing.T) {
		a := NewTabItemWithIcon("A", theme.HomeIcon(), widget.NewLabel("a"))
		b := NewTabItemWithIcon("B", theme.HomeIcon(), widget.NewLabel("b"))
		tabs := NewAppTabs(a, b)
		test.NewTempWindow(t, tabs).Resize(geo.NewSize(400, 300))
		r := cache.Renderer(tabs).(*appTabsRenderer)
		ra := cache.Renderer(a.button).(*tabButtonRenderer)
		rb := cache.Renderer(b.button).(*tabButtonRenderer)

		tabs.Disable()
		assert.Equal(t, th.Color(theme.ColorNameDisabled, v), ra.label.Color, "selected tab's label")
		assert.Equal(t, th.Color(theme.ColorNameDisabled, v), rb.label.Color, "other tab's label")
		assert.IsType(t, &theme.DisabledResource{}, ra.icon.Resource, "selected tab's icon")
		assert.IsType(t, &theme.DisabledResource{}, rb.icon.Resource, "other tab's icon")
		assert.Equal(t, th.Color(theme.ColorNameDisabled, v), r.indicator.FillColor, "indicator")
		assert.True(t, r.action.Disabled(), "overflow button")

		tabs.Enable()
		assert.Equal(t, th.Color(theme.ColorNamePrimary, v), ra.label.Color)
		assert.Equal(t, th.Color(theme.ColorNameForeground, v), rb.label.Color)
		assert.IsType(t, &theme.PrimaryThemedResource{}, ra.icon.Resource)
		assert.IsType(t, &theme.ThemedResource{}, rb.icon.Resource)
		assert.Equal(t, th.Color(theme.ColorNamePrimary, v), r.indicator.FillColor)
		assert.False(t, r.action.Disabled())
	})
	t.Run("InnerWindow", func(t *testing.T) {
		w := NewInnerWindow("Title", widget.NewLabel("content"))
		w.OnMaximized = func() {}
		test.NewTempWindow(t, w).Resize(geo.NewSize(400, 300))
		r := cache.Renderer(w).(*innerWindowRenderer)
		corner := r.corner.(*draggableCorner)

		w.Disable()
		for i, b := range r.buttons {
			assert.True(t, b.b.Disabled(), "frame button %d", i)
		}
		assert.IsType(t, &theme.DisabledResource{}, corner.prop.Resource, "resize corner")

		w.Enable()
		assert.False(t, r.buttons[0].b.Disabled(), "close")
		assert.True(t, r.buttons[1].b.Disabled(), "minimise has no action, so stays disabled")
		assert.False(t, r.buttons[2].b.Disabled(), "maximise")
		assert.IsType(t, &theme.ThemedResource{}, corner.prop.Resource)
	})
}
