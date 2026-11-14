package widget

import (
	"context"
	"image/color"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/auth"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// The disabled tests of #1212: Chip, ButtonGroup, SegmentedButton,
// ThemeSelector and AuthButton are visible but inert while disabled. Each
// test disables its widget before it is drawn, and again after, and checks
// input, Tab, focus and the look, then that Enable restores them.
//
// None of these widgets is Draggable or Scrollable, so a drag or a scroll on
// them reaches nothing of theirs; the tests still send one through the canvas
// (test.Drag, test.Scroll) and check that no callback fired, so a widget
// gaining such a handler without a disabled guard shows here.

// disableTestCentre is the canvas position of the centre of o.
func disableTestCentre(o core.CanvasObject) geo.Position {
	p := bellui.CurrentApp().Driver().AbsolutePositionForObject(o)
	return p.Add(geo.NewPos(o.Size().Width/2, o.Size().Height/2))
}

// disableTestDragScroll sends a drag and a scroll to the canvas at o.
func disableTestDragScroll(c core.Canvas, o core.CanvasObject) {
	at := disableTestCentre(o)
	test.Drag(c, at, 20, 0)
	test.Scroll(c, at, 0, -20)
}

// disableTestColor is the theme colour name in the current variant.
func disableTestColor(th bellui.Theme, name bellui.ThemeColorName) color.Color {
	return th.Color(name, bellui.CurrentApp().Settings().ThemeVariant())
}

func TestChip_Disable(t *testing.T) {
	test.NewTempApp(t)
	tapped, deleted := 0, 0
	chip := &Chip{Text: "Tag", Style: ChipStyleInput, Icon: theme.HomeIcon(), OnTapped: func() { tapped++ }, OnDeleted: func() { deleted++ }}
	chip.ExtendBaseWidget(chip)

	chip.Disable() // before any window or renderer
	assert.True(t, chip.Disabled())
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{chip}})
	c := w.Canvas()
	r := test.TempWidgetRenderer(t, chip).(*chipRenderer)
	th := chip.Theme()

	assertDisabledLook := func() {
		t.Helper()
		assert.Equal(t, disableTestColor(th, theme.ColorNameDisabled), r.label.Color)
		assert.Equal(t, disableTestColor(th, theme.ColorNameDisabledButton), r.bg.StrokeColor)
		assert.IsType(t, &theme.DisabledResource{}, r.icon.Resource)
		assert.IsType(t, &theme.DisabledResource{}, r.deleteBtn.Resource)
	}
	assertDisabledLook()

	// A tap on the chip and on its delete X, through the canvas hit test.
	test.TapCanvas(c, disableTestCentre(chip))
	xAt := bellui.CurrentApp().Driver().AbsolutePositionForObject(r.deleteBtn).Add(geo.NewPos(2, 2))
	test.TapCanvas(c, xAt)
	disableTestDragScroll(c, chip)
	chip.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(4, 4)}, Button: desktop.MouseButtonPrimary})
	assert.False(t, internalWidget.PressFeedbackOf(&chip.pressTarget).Pressed(), "a disabled chip showed a press")
	chip.MouseUp(&desktop.MouseEvent{})
	chip.HoverIn(&event.HoverEvent{})
	assert.False(t, chip.hovered, "a disabled chip showed a hover")
	assert.Zero(t, tapped, "OnTapped fired while disabled")
	assert.Zero(t, deleted, "OnDeleted fired while disabled")
	assert.Equal(t, color.Transparent, r.bg.FillColor, "a hover state layer was drawn while disabled")
	// The chip takes no focus or key, so Tab and typing have nothing to skip.

	chip.Enable()
	assert.Equal(t, disableTestColor(th, theme.ColorNameOnSurface), r.label.Color)
	assert.Equal(t, disableTestColor(th, theme.ColorNameOutline), r.bg.StrokeColor)
	assert.IsType(t, &theme.ThemedResource{}, r.icon.Resource)
	test.TapCanvas(c, disableTestCentre(chip))
	test.TapCanvas(c, xAt)
	assert.Equal(t, 1, tapped, "an enabled chip did not take the tap")
	assert.Equal(t, 1, deleted, "an enabled chip's X did not take the tap")

	chip.HoverIn(&event.HoverEvent{})
	chip.Disable() // after the first draw, while hovered
	assert.False(t, chip.hovered)
	assertDisabledLook()
	test.TapCanvas(c, disableTestCentre(chip))
	assert.Equal(t, 1, tapped)

	// A selected filter chip's fill turns to the disabled container colour.
	filter := NewFilterChip("F", nil)
	filter.Selected = true
	filter.Disable()
	fr := test.TempWidgetRenderer(t, filter).(*chipRenderer)
	assert.Equal(t, disableTestColor(th, theme.ColorNameDisabledButton), fr.bg.FillColor)
	test.Tap(filter)
	assert.True(t, filter.Selected, "a disabled filter chip toggled")
}

func TestButtonGroup_Disable(t *testing.T) {
	test.NewTempApp(t)
	taps := 0
	a := NewButton("A", func() { taps++ })
	b := NewButton("B", func() { taps++ })
	own := NewButton("Own", func() { taps++ })
	own.Disable() // the app's own choice, which the group must keep
	g := NewButtonGroup(a, b, own)

	g.Disable() // before any window or renderer
	assert.True(t, g.Disabled())
	assert.True(t, a.Disabled() && b.Disabled(), "the group's buttons stayed enabled")
	other := NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{g, other}})
	c := w.Canvas()

	for _, btn := range g.Buttons {
		test.TapCanvas(c, disableTestCentre(btn))
		disableTestDragScroll(c, btn)
		btn.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	}
	assert.Zero(t, taps, "a button of a disabled group fired")
	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab stopped in a disabled group")
	// The look is the disabled Button's own.
	ar := test.TempWidgetRenderer(t, a).(*buttonRenderer)
	assert.Equal(t, disableTestColor(a.Theme(), theme.ColorNameDisabledButton), ar.background.FillColor)

	// A button added while the group is disabled is disabled when drawn.
	late := NewButton("Late", func() { taps++ })
	g.Buttons = append(g.Buttons, late)
	g.Refresh()
	assert.True(t, late.Disabled(), "a button added to a disabled group stayed enabled")

	g.Enable()
	assert.False(t, a.Disabled() || b.Disabled() || late.Disabled(), "Enable left a group button disabled")
	assert.True(t, own.Disabled(), "Enable re-enabled a button the app had disabled")
	assert.NotEqual(t, disableTestColor(a.Theme(), theme.ColorNameDisabledButton), ar.background.FillColor)
	test.TapCanvas(c, disableTestCentre(a))
	assert.Equal(t, 1, taps)

	// After the first draw, with a button focused: the focus is dropped.
	c.Focus(b)
	require.Equal(t, b, c.Focused())
	g.Disable()
	assert.Nil(t, c.Focused(), "the disabled group kept the focus")
	test.TapCanvas(c, disableTestCentre(a))
	assert.Equal(t, 1, taps)
}

func TestSegmentedButton_Disable(t *testing.T) {
	test.NewTempApp(t)
	changed := 0
	sb := NewSegmentedButton([]string{"A", "B", "C"}, func(int) { changed++ })

	sb.Disable() // before any window or renderer
	assert.True(t, sb.Disabled())
	other := NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{sb, other}})
	c := w.Canvas()
	r := test.TempWidgetRenderer(t, sb).(*segButtonRenderer)
	require.Len(t, r.segments, 3)
	th := sb.Theme()

	assertDisabledLook := func() {
		t.Helper()
		for i, seg := range r.segments {
			sr := test.TempWidgetRenderer(t, seg).(*segmentRenderer)
			assert.Equal(t, disableTestColor(th, theme.ColorNameDisabled), sr.label.Color, "segment %d label", i)
			assert.Equal(t, disableTestColor(th, theme.ColorNameDisabled), sr.bg.StrokeColor, "segment %d outline", i)
			assert.True(t, seg.AccessibleState().Has(core.StateDisabled), "segment %d not announced disabled", i)
		}
		sel := test.TempWidgetRenderer(t, r.segments[0]).(*segmentRenderer)
		assert.Equal(t, disableTestColor(th, theme.ColorNameDisabledButton), sel.bg.FillColor, "selected segment fill")
	}
	assertDisabledLook()

	refuse := func() {
		t.Helper()
		test.TapCanvas(c, disableTestCentre(r.segments[2]))
		r.segments[1].PerformAccessibleAction(core.ActionSelect)
		r.segments[1].MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(4, 4)}, Button: desktop.MouseButtonPrimary})
		assert.False(t, internalWidget.PressFeedbackOf(&r.segments[1].pressTarget).Pressed(), "a disabled segment showed a press")
		r.segments[1].MouseUp(&desktop.MouseEvent{})
		disableTestDragScroll(c, sb)
		sb.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		assert.Zero(t, changed, "OnChanged fired while disabled")
		assert.Equal(t, 0, sb.Selected, "the selection moved while disabled")
	}
	refuse()
	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab stopped on a disabled SegmentedButton")

	sb.Enable()
	for i, seg := range r.segments {
		sr := test.TempWidgetRenderer(t, seg).(*segmentRenderer)
		assert.NotEqual(t, disableTestColor(th, theme.ColorNameDisabled), sr.label.Color, "segment %d label", i)
		assert.False(t, seg.AccessibleState().Has(core.StateDisabled))
	}
	test.TapCanvas(c, disableTestCentre(r.segments[2]))
	assert.Equal(t, 1, changed, "an enabled segment did not take the tap")
	assert.Equal(t, 2, sb.Selected)
	sb.Selected = 0
	sb.Refresh()
	changed = 0

	// After the first draw, while focused.
	c.Focus(sb)
	require.Equal(t, sb, c.Focused())
	sb.Disable()
	assert.Nil(t, c.Focused(), "the disabled SegmentedButton kept the focus")
	assertDisabledLook()
	refuse()
}

func TestThemeSelector_Disable(t *testing.T) {
	test.NewTempApp(t)
	fired := 0
	sel := NewThemeSelector(func(ThemeSelection) { fired++ })

	sel.Disable() // before any window or renderer
	assert.True(t, sel.Disabled())
	other := NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{sel, other}})
	c := w.Canvas()
	r := test.TempWidgetRenderer(t, sel).(*themeSelectorRenderer)
	target := r.swatches[len(r.swatches)-1]
	require.NotEqual(t, sel.Selection.Primary, target.name)
	start := sel.Selection

	assertDisabledLook := func(want bool) {
		t.Helper()
		assert.Equal(t, want, r.variants.Disabled(), "the variant selector's state")
		sr := test.TempWidgetRenderer(t, target).(*themeSwatchRenderer)
		_, _, _, a := sr.dot.FillColor.RGBA()
		if want {
			assert.Less(t, a, uint32(0xffff), "a disabled swatch was drawn opaque")
		} else {
			assert.Equal(t, uint32(0xffff), a, "an enabled swatch was drawn translucent")
		}
		assert.Equal(t, want, target.AccessibleState().Has(core.StateDisabled))
	}
	assertDisabledLook(true)

	refuse := func() {
		t.Helper()
		test.TapCanvas(c, disableTestCentre(target))
		vr := test.TempWidgetRenderer(t, r.variants).(*segButtonRenderer)
		test.TapCanvas(c, disableTestCentre(vr.segments[2]))
		r.variants.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		disableTestDragScroll(c, sel)
		assert.Zero(t, fired, "OnChanged fired while disabled")
		assert.Equal(t, start, sel.Selection, "the selection changed while disabled")
	}
	refuse()
	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab stopped in a disabled ThemeSelector")

	sel.Enable()
	assertDisabledLook(false)
	test.TapCanvas(c, disableTestCentre(target))
	assert.Equal(t, 1, fired, "an enabled swatch did not take the tap")
	sel.SetSelection(start)
	fired = 0

	// After the first draw, with the variant selector focused.
	c.Focus(r.variants)
	require.Equal(t, r.variants, c.Focused())
	sel.Disable()
	assert.Nil(t, c.Focused(), "the disabled ThemeSelector kept the focus")
	assertDisabledLook(true)
	refuse()
}

func TestAuthButton_Disable(t *testing.T) {
	test.NewTempApp(t)
	var started atomic.Int32
	prevAuth := authenticate
	t.Cleanup(func() { authenticate = prevAuth })
	authenticate = func(auth.OIDCConfig) (*auth.OIDCResult, error) {
		started.Add(1)
		return nil, context.Canceled
	}

	b := NewAuthButton("google", "id", nil)
	b.Disable() // before any window or renderer
	assert.True(t, b.Disabled())
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{b}})
	c := w.Canvas()
	r := test.TempWidgetRenderer(t, b).(*authButtonRenderer)
	th := b.Theme()

	assertDisabledLook := func() {
		t.Helper()
		assert.Equal(t, disableTestColor(th, theme.ColorNameDisabledButton), r.bg.FillColor)
		assert.Equal(t, disableTestColor(th, theme.ColorNameDisabled), r.label.Color)
		assert.IsType(t, &theme.DisabledResource{}, r.logoImg.Resource)
	}
	assertDisabledLook()

	refuse := func() {
		t.Helper()
		test.TapCanvas(c, disableTestCentre(b))
		b.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(4, 4)}, Button: desktop.MouseButtonPrimary})
		assert.False(t, internalWidget.PressFeedbackOf(&b.pressTarget).Pressed(), "a disabled AuthButton showed a press")
		b.MouseUp(&desktop.MouseEvent{})
		disableTestDragScroll(c, b)
		b.mu.Lock()
		running := b.cancel != nil
		b.mu.Unlock()
		assert.False(t, running, "a disabled AuthButton started a sign-in")
		assert.Zero(t, started.Load(), "a disabled AuthButton called authenticate")
	}
	refuse()
	// The button takes no focus or key, so Tab and typing have nothing to skip.

	b.Enable()
	preset := b.preset()
	assert.Equal(t, preset.BgColor, r.bg.FillColor)
	assert.Equal(t, preset.TextColor, r.label.Color)
	assert.Equal(t, preset.Logo, r.logoImg.Resource)
	test.TapCanvas(c, disableTestCentre(b))
	assert.Eventually(t, func() bool { return started.Load() == 1 }, time.Second, 5*time.Millisecond, "an enabled AuthButton did not start a sign-in")
	assert.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.cancel == nil
	}, time.Second, 5*time.Millisecond)
	started.Store(0)

	b.Disable() // after the first draw
	assertDisabledLook()
	refuse()
}
