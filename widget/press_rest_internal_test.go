package widget

import (
	"image/color"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func pressedAt(t0 *widget.PressTarget) bool { return widget.PressFeedbackOf(t0).Pressed() }

func TestChip_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	c := NewChip("Tag", func() {})
	c.Resize(c.MinSize())
	assertPressCycle(t, c, &c.pressTarget, c.HoverOut)
	assertRowShape(t, c, &c.pressTarget, test.Theme().Size(theme.SizeNameShapeSmall))
	assertTapFromCode(t, &c.pressTarget, func() { c.Tapped(nil) })

	f := NewFilterChip("Filter", func(bool) {})
	f.Selected = true
	f.Resize(f.MinSize())
	cache.Renderer(f)
	f.TouchDown(&mobile.TouchEvent{})
	assertSameRGB(t, test.Theme().Color(theme.ColorNameOnSecondaryContainer, bellui.CurrentApp().Settings().ThemeVariant()),
		widget.PressFeedbackOf(&f.pressTarget).Ripple.Color)
}

// An input chip's delete X shows a circle of its own, and a press on it does
// not press the chip.
func TestChip_DeletePressFeedback(t *testing.T) {
	test.NewTempApp(t)
	c := &Chip{Text: "Tag", Style: ChipStyleInput, OnTapped: func() {}, OnDeleted: func() {}}
	c.ExtendBaseWidget(c)
	c.Resize(c.MinSize())
	x := cache.Renderer(c).(*chipRenderer).deleteBtn

	onX := x.Position().Add(geo.NewPos(1, 1))
	c.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: onX}})
	assert.True(t, pressedAt(&c.deletePress), "a press on the X showed no press")
	assert.False(t, pressedAt(&c.pressTarget), "a press on the X pressed the chip")
	r := widget.PressFeedbackOf(&c.deletePress).Ripple
	assert.Equal(t, r.Size().Width/2, r.CornerRadius, "the X's pressed state is not a circle")
	xc := x.Position().Add(geo.NewPos(x.Size().Width/2, x.Size().Height/2))
	assert.Equal(t, xc, r.Position().Add(geo.NewPos(r.Size().Width/2, r.Size().Height/2)), "the circle is not around the X")
	c.TouchUp(&mobile.TouchEvent{})
	assert.False(t, pressedAt(&c.deletePress))

	c.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(4, 4)}, Button: desktop.MouseButtonPrimary})
	assert.True(t, pressedAt(&c.pressTarget), "a press on the label showed no press")
	assert.False(t, pressedAt(&c.deletePress))
	c.HoverOut()
	assert.False(t, pressedAt(&c.pressTarget))

	c.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: onX}, Button: desktop.MouseButtonPrimary})
	c.TouchCancel(nil)
	assert.False(t, pressedAt(&c.deletePress), "a cancel left the X pressed")
}

func TestFAB_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	for style, want := range map[FABStyle]bellui.ThemeColorName{
		FABStylePrimary:   theme.ColorNameOnPrimaryContainer,
		FABStyleSurface:   theme.ColorNamePrimary,
		FABStyleSecondary: theme.ColorNameOnSecondaryContainer,
		FABStyleTertiary:  theme.ColorNameOnTertiaryContainer,
	} {
		f := NewFAB(theme.ContentAddIcon(), func() {})
		f.Style = style
		f.Resize(f.MinSize())
		assertPressCycle(t, f, &f.pressTarget, f.HoverOut)
		f.TouchDown(&mobile.TouchEvent{})
		r := widget.PressFeedbackOf(&f.pressTarget).Ripple
		assert.Equal(t, f.Size(), r.Size())
		assert.Equal(t, test.Theme().Size(theme.SizeNameShapeLarge), r.CornerRadius)
		assertSameRGB(t, test.Theme().Color(want, v), r.Color)
		f.TouchCancel(nil)
	}

	f := NewFAB(theme.ContentAddIcon(), func() {})
	f.Resize(f.MinSize())
	cache.Renderer(f)
	assertTapFromCode(t, &f.pressTarget, func() { f.PerformAccessibleAction(core.ActionPress) })
	f.Disable()
	f.TouchDown(&mobile.TouchEvent{})
	assert.False(t, pressedAt(&f.pressTarget), "a disabled FAB showed a press")
}

func TestAuthButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	b := NewAuthButton("github", "id", nil)
	b.Resize(b.MinSize())
	assertPressCycle(t, b, &b.pressTarget, b.HoverOut)
	b.TouchDown(&mobile.TouchEvent{})
	r := widget.PressFeedbackOf(&b.pressTarget).Ripple
	assert.Equal(t, b.Size(), r.Size())
	assertSameRGB(t, b.preset().TextColor, r.Color)
	b.TouchCancel(nil)

	// While a sign-in runs a tap does nothing, so a press shows nothing.
	b.cancel = func() {}
	b.TouchDown(&mobile.TouchEvent{})
	assert.False(t, pressedAt(&b.pressTarget), "a press showed while authenticating")
	b.cancel = nil
}

func TestHyperlink_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	u, _ := url.Parse("https://example.com")
	hl := NewHyperlink("link", u)
	hl.OnTapped = func() {}
	hl.Resize(geo.NewSize(200, hl.MinSize().Height))
	focus := cache.Renderer(hl).(*hyperlinkRenderer).focus
	on := focus.Position().Add(geo.NewPos(3, 3))

	hl.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: on}})
	assert.True(t, pressedAt(&hl.pressTarget), "a press on the link's text showed nothing")
	r := widget.PressFeedbackOf(&hl.pressTarget).Ripple
	assert.Equal(t, focus.Position(), r.Position(), "the pressed state is not the focus box")
	assert.Equal(t, focus.Size(), r.Size())
	assertSameRGB(t, test.Theme().Color(theme.ColorNamePrimary, bellui.CurrentApp().Settings().ThemeVariant()), r.Color)
	hl.TouchUp(&mobile.TouchEvent{})
	assert.False(t, pressedAt(&hl.pressTarget))

	hl.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: on}, Button: desktop.MouseButtonPrimary})
	hl.HoverOut()
	assert.False(t, pressedAt(&hl.pressTarget), "a press dragged off the link stayed")

	// Beyond the text a tap does nothing, so a press shows nothing.
	hl.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(195, 3)}})
	assert.False(t, pressedAt(&hl.pressTarget), "a press beyond the link's text showed")

	assertTapFromCode(t, &hl.pressTarget, func() { hl.TypedKey(&event.KeyEvent{Name: event.KeySpace}) })
}

func TestSearchSelect_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	s := NewSearchSelect([]string{"a", "b"}, nil)
	s.Resize(geo.NewSize(150, s.MinSize().Height))
	assertPressCycle(t, s, &s.pressTarget, s.HoverOut)
	assertRowShape(t, s, &s.pressTarget, test.Theme().Size(theme.SizeNameInputRadius))

	s.Disable()
	s.TouchDown(&mobile.TouchEvent{})
	assert.False(t, pressedAt(&s.pressTarget), "a disabled search select showed a press")
}

func TestTimePickerButton_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	b := newTimePickerButton(core.RoleTab, "Hour", func() {})
	b.bg.CornerRadius = 8
	b.label.Color = color.White
	b.Resize(geo.NewSize(80, 60))
	assertPressCycle(t, b, &b.pressTarget, b.HoverOut)
	b.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(7, 9)}})
	r := widget.PressFeedbackOf(&b.pressTarget).Ripple
	assert.Equal(t, b.Size(), r.Size())
	assert.Equal(t, float32(8), r.CornerRadius, "the pressed state does not take the button's corners")
	assert.Equal(t, geo.NewPos(7, 9), r.Center)
	assertSameRGB(t, color.White, r.Color)
	b.TouchCancel(nil)
	assertTapFromCode(t, &b.pressTarget, func() { b.PerformAccessibleAction(core.ActionSelect) })
}
