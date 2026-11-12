package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// assertCircleShape checks that a press at at shows on circle, the control's
// indicator, spreading from its centre, in want (#1091).
func assertCircleShape(t *testing.T, target pressEvents, t0 *widget.PressTarget, at geo.Position, circle core.CanvasObject, want bellui.ThemeColorName) {
	t.Helper()
	target.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: at}})
	defer target.TouchCancel(nil)
	r := widget.PressFeedbackOf(t0).Ripple
	if !assert.True(t, widget.PressFeedbackOf(t0).Pressed(), "no pressed state") {
		return
	}
	assert.Equal(t, circle.Position(), r.Position(), "the pressed state is not on the control's circle")
	assert.Equal(t, circle.Size(), r.Size())
	assert.Equal(t, circle.Size().Width/2, r.CornerRadius, "the pressed state is not a circle")
	assert.Equal(t, geo.NewPos(circle.Size().Width/2, circle.Size().Height/2), r.Center, "the ripple does not spread from the circle's centre")
	assertSameRGB(t, test.Theme().Color(want, bellui.CurrentApp().Settings().ThemeVariant()), r.Color)
}

func TestCheck_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	c := NewCheck("Remember me", func(bool) {})
	c.Resize(geo.NewSize(300, c.MinSize().Height))
	assertPressCycle(t, c, &c.pressTarget, c.HoverOut)
	ind := cache.Renderer(c).(*checkRenderer).focusIndicator
	// A press on the label shows around the box.
	assertCircleShape(t, c, &c.pressTarget, geo.NewPos(c.MinSize().Width-5, 10), ind, theme.ColorNameOnSurface)
	c.SetChecked(true)
	assertCircleShape(t, c, &c.pressTarget, geo.NewPos(5, 10), ind, theme.ColorNamePrimary)

	// Beyond the box and label a tap does nothing, so a press shows nothing.
	c.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(290, 10)}})
	assert.False(t, widget.PressFeedbackOf(&c.pressTarget).Pressed(), "a press beyond the label showed")

	assertTapFromCode(t, &c.pressTarget, func() { c.TypedRune(' ') })

	c.Disable()
	c.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&c.pressTarget).Pressed(), "a disabled check showed a press")
}

func TestRadioItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	i := newRadioItem("One", func(*radioItem) {})
	i.Resize(geo.NewSize(200, i.MinSize().Height))
	assertPressCycle(t, i, &i.pressTarget, i.HoverOut)
	ind := &cache.Renderer(i).(*radioItemRenderer).focusIndicator
	assertCircleShape(t, i, &i.pressTarget, geo.NewPos(120, 10), ind, theme.ColorNameOnSurface)
	i.SetSelected(true)
	assertCircleShape(t, i, &i.pressTarget, geo.NewPos(5, 10), ind, theme.ColorNamePrimary)
	assertTapFromCode(t, &i.pressTarget, func() { i.TypedRune(' ') })

	i.Disable()
	i.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&i.pressTarget).Pressed(), "a disabled radio showed a press")
}

func TestSwitch_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	s := NewSwitch(func(bool) {})
	s.Resize(s.MinSize())
	assertPressCycle(t, s, &s.pressTarget, s.HoverOut)
	halo := cache.Renderer(s).(*switchRenderer).focusIndicator
	assertCircleShape(t, s, &s.pressTarget, geo.NewPos(40, 5), halo, theme.ColorNameOnSurface)
	s.On = true
	s.Refresh()
	assertCircleShape(t, s, &s.pressTarget, geo.NewPos(2, 5), halo, theme.ColorNamePrimary)
	assertTapFromCode(t, &s.pressTarget, func() { s.TypedKey(&event.KeyEvent{Name: event.KeySpace}) })

	s.Disable()
	s.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&s.pressTarget).Pressed(), "a disabled switch showed a press")
}

func TestDateDayCell_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	c := newDateDayCell(func(*dateDayCell) {})
	c.day, c.month, c.year = 3, 4, 2026
	c.label.Color = test.Theme().Color(theme.ColorNameOnSurface, bellui.CurrentApp().Settings().ThemeVariant())
	c.Resize(geo.NewSquareSize(40))
	assertPressCycle(t, c, &c.pressTarget, c.HoverOut)
	assertCircleShape(t, c, &c.pressTarget, geo.NewPos(1, 1), c.bg, theme.ColorNameOnSurface)
	assertTapFromCode(t, &c.pressTarget, func() { c.PerformAccessibleAction(core.ActionPress) })

	blank := newDateDayCell(func(*dateDayCell) {})
	blank.Resize(geo.NewSquareSize(40))
	cache.Renderer(blank)
	blank.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&blank.pressTarget).Pressed(), "a cell with no day showed a press")
}

func TestPasswordRevealer_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	e := NewPasswordEntry()
	r := newPasswordRevealer(e)
	r.Resize(r.MinSize())
	assertPressCycle(t, r, &r.pressTarget, r.HoverOut)
	whole := canvasBounds(r.Size())
	assertCircleShape(t, r, &r.pressTarget, geo.NewPos(2, 2), whole, theme.ColorNameOnSurfaceVariant)
	assertTapFromCode(t, &r.pressTarget, func() { r.PerformAccessibleAction(core.ActionPress) })

	e.Disable()
	r.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&r.pressTarget).Pressed(), "the revealer of a disabled entry showed a press")
}

func TestThemeSwatch_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	sel := NewThemeSelector(nil)
	s := &themeSwatch{name: "blue", owner: sel}
	s.ExtendBaseWidget(s)
	s.Resize(geo.NewSquareSize(32))
	assertPressCycle(t, s, &s.pressTarget, s.HoverOut)
	assertCircleShape(t, s, &s.pressTarget, geo.NewPos(2, 2), cache.Renderer(s).(*themeSwatchRenderer).ring, theme.ColorNameOnSurface)
	assertTapFromCode(t, &s.pressTarget, func() { s.Tapped(nil) })
}

// canvasBounds is a stand-in object covering size at the origin.
func canvasBounds(size geo.Size) core.CanvasObject {
	r := NewLabel("")
	r.Resize(size)
	return r
}
