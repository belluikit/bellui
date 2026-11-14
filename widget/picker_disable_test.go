package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// The date and time pickers are visible but inert while disabled (#1213): they
// keep their value, draw it in the disabled colours, refuse every input, and
// Tab skips them and the parts they build. Each test disables the picker
// before it is drawn, as Player does while building its tree, and drives each
// input once enabled too, so a refusal is not mistaken for an input that never
// arrived.

func themeColor(name bellui.ThemeColorName) any {
	return theme.Current().Color(name, bellui.CurrentApp().Settings().ThemeVariant())
}

// absPos is obj's position on its canvas.
func absPos(obj core.CanvasObject) geo.Position {
	return bellui.CurrentApp().Driver().AbsolutePositionForObject(obj)
}

func TestDatePicker_Disable(t *testing.T) {
	test.NewTempApp(t)
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
	changed := 0
	dp := NewDatePicker(sel, func(time.Time) { changed++ })
	dp.Disable() // before the first draw: no window, no renderer
	other := NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{dp, other}})
	c := w.Canvas()
	r := test.TempWidgetRenderer(t, dp).(*datePickerRenderer)

	require.True(t, dp.Disabled())
	assert.True(t, r.prevBtn.Disabled(), "the previous-month button stayed enabled")
	assert.True(t, r.nextBtn.Disabled(), "the next-month button stayed enabled")
	assert.Equal(t, themeColor(theme.ColorNameDisabled), r.monthLabel.Color, "month label is not drawn disabled")
	day20 := dayCellFor(t, dp, 20)
	assert.Equal(t, themeColor(theme.ColorNameDisabled), day20.label.Color, "a day is not drawn disabled")
	assert.Equal(t, themeColor(theme.ColorNameDisabledButton), dayCellFor(t, dp, 15).bg.FillColor,
		"the selected day is not filled with the disabled container colour")
	assert.True(t, day20.AccessibleState()&core.StateDisabled != 0, "a day does not report disabled")

	// Tap: a day and the month buttons.
	test.Tap(day20)
	day20.PerformAccessibleAction(core.ActionPress)
	test.Tap(r.nextBtn)
	r.prevBtn.PerformAccessibleAction(core.ActionPress)
	assert.Equal(t, 0, changed, "a tap on a day of a disabled picker fired OnChanged")
	assert.Equal(t, sel, dp.Selected)
	assert.Equal(t, time.June, dp.viewing.Month(), "a month button of a disabled picker moved the month")
	// Drag and scroll: the picker has neither gesture. Both are sent at a day
	// to show nothing there takes them.
	test.Drag(c, absPos(day20).Add(geo.NewPos(4, 4)), 30, 0)
	test.Scroll(c, absPos(day20).Add(geo.NewPos(4, 4)), 0, -30)
	// Keys: move the day cursor and select.
	dp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	dp.TypedKey(&event.KeyEvent{Name: event.KeyPageDown})
	dp.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, 0, changed, "a key on a disabled picker fired OnChanged")
	assert.Equal(t, sel, dp.Selected)
	assert.Equal(t, time.June, dp.viewing.Month(), "PageDown on a disabled picker moved the month")

	// Tab skips the picker and its month buttons.
	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab stopped on the disabled picker or a part of it")

	dp.Enable()
	assert.False(t, r.nextBtn.Disabled(), "Enable left the next-month button disabled")
	assert.Equal(t, themeColor(theme.ColorNameOnSurface), r.monthLabel.Color, "Enable left the month label disabled")
	assert.Equal(t, themeColor(theme.ColorNamePrimary), dayCellFor(t, dp, 15).bg.FillColor, "Enable left the selection disabled")
	test.Tap(dayCellFor(t, dp, 20))
	assert.Equal(t, 1, changed, "a tap on a day of an enabled picker did not fire OnChanged")
	test.Tap(r.nextBtn)
	assert.Equal(t, time.July, dp.viewing.Month(), "the next-month button of an enabled picker did nothing")
	dp.FocusGained() // puts the day cursor in the grid
	dp.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, 2, changed, "Return on an enabled picker did not select")
	dp.FocusLost()

	// Disabled after the first draw, with the focus on the picker: the focus
	// goes. (The month buttons are IconButtons, which take no focus.)
	c.Focus(dp)
	require.Equal(t, dp, c.Focused())
	dp.Disable()
	assert.Nil(t, c.Focused(), "the disabled picker kept the focus")
	assert.Equal(t, themeColor(theme.ColorNameDisabled), r.monthLabel.Color)
}

func TestTimePicker_Disable(t *testing.T) {
	test.NewTempApp(t)
	changed := 0
	tp := NewTimePicker(10, 30, false, func(int, int) { changed++ })
	tp.Disable() // before the first draw
	other := NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{tp, other}})
	c := w.Canvas()
	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)

	require.True(t, tp.Disabled())
	assert.Equal(t, themeColor(theme.ColorNameDisabled), r.hourBtn.label.Color, "hour is not drawn disabled")
	assert.Equal(t, themeColor(theme.ColorNameDisabled), r.minuteBtn.label.Color, "minute is not drawn disabled")
	assert.Equal(t, themeColor(theme.ColorNameDisabledButton), r.hourBtn.bg.FillColor, "the selected field is not filled disabled")
	assert.Equal(t, themeColor(theme.ColorNameDisabledButton), r.selectedBg.FillColor, "the dial selection is not filled disabled")
	assert.Equal(t, themeColor(theme.ColorNameDisabled), r.selectorArm.StrokeColor, "the dial arm is not drawn disabled")
	assert.True(t, r.hourBtn.AccessibleState()&core.StateDisabled != 0, "the hour field does not report disabled")

	// The dial at 3 o'clock, in the picker's and the canvas's coordinates.
	dial3 := geo.NewPos(r.clockCenterX+r.clockRadius, r.clockCenterY)
	refuse := func(what string) {
		t.Helper()
		assert.Equal(t, 0, changed, "%s on a disabled picker fired OnChanged", what)
		assert.Equal(t, 10, tp.Hour, "%s on a disabled picker changed the hour", what)
		assert.Equal(t, 30, tp.Minute, "%s on a disabled picker changed the minute", what)
		assert.False(t, tp.selectingMinute, "%s on a disabled picker switched the dial", what)
	}
	test.TapAt(tp, dial3)
	refuse("a dial tap")
	test.Drag(c, absPos(tp).Add(dial3), 0, 5)
	refuse("a dial drag")
	// Scroll: the picker has no scroll gesture; one sent at the dial lands
	// on nothing.
	test.Scroll(c, absPos(tp).Add(dial3), 0, -30)
	refuse("a scroll")
	test.Tap(r.minuteBtn)
	test.Tap(r.pmBtn)
	r.hourBtn.PerformAccessibleAction(core.ActionIncrement)
	r.minuteBtn.PerformAccessibleAction(core.ActionSelect)
	refuse("a field tap or accessible action")
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	tp.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	test.Type(tp, "0715p")
	refuse("a key")
	assert.False(t, tp.HandlesEscape())

	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab stopped on the disabled picker")

	tp.Enable()
	assert.Equal(t, themeColor(theme.ColorNameOnPrimaryContainer), r.hourBtn.label.Color, "Enable left the hour disabled")
	assert.Equal(t, themeColor(theme.ColorNamePrimary), r.selectedBg.FillColor, "Enable left the dial selection disabled")
	test.TapAt(tp, dial3)
	assert.Equal(t, 1, changed, "a dial tap on an enabled picker did not fire OnChanged")
	assert.Equal(t, 3, tp.Hour)
	tp.selectingMinute = false
	test.Drag(c, absPos(tp).Add(dial3), 0, 5)
	assert.Equal(t, 2, changed, "a dial drag on an enabled picker did not fire OnChanged")
	test.Tap(r.pmBtn)
	assert.Equal(t, 15, tp.Hour, "the PM field of an enabled picker did nothing")
	test.Tap(r.hourBtn)
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 16, tp.Hour, "Up on an enabled picker did nothing")

	// A half-typed value is discarded by Disable; the focus goes with it.
	c.Focus(tp)
	require.Equal(t, tp, c.Focused())
	tp.TypedRune('1')
	require.True(t, tp.hasPartial)
	tp.Disable()
	assert.Nil(t, c.Focused(), "the disabled picker kept the focus")
	assert.False(t, tp.hasPartial, "Disable kept a half-typed digit")
	assert.Equal(t, themeColor(theme.ColorNameDisabled), r.hourBtn.label.Color)
}

func TestDateTimePicker_Disable(t *testing.T) {
	test.NewTempApp(t)
	sel := time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local)
	changed := 0
	dt := NewDateTimePicker(sel, true, func(time.Time) { changed++ })
	dt.Disable() // before the first draw: the tabs do not exist yet
	other := NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{dt, other}})
	c := w.Canvas()
	test.TempWidgetRenderer(t, dt)
	require.NotNil(t, dt.tabs)

	require.True(t, dt.Disabled())
	assert.True(t, dt.datePicker.Disabled(), "the date picker stayed enabled")
	assert.True(t, dt.timePicker.Disabled(), "the time picker stayed enabled")
	assert.True(t, dt.tabs.Disabled(), "the tabs built after Disable stayed enabled")
	segs := test.TempWidgetRenderer(t, dt.tabs).(*segButtonRenderer).segments
	require.Len(t, segs, 2)
	segLabel := func(i int) any { return test.TempWidgetRenderer(t, segs[i]).(*segmentRenderer).label.Color }
	assert.Equal(t, themeColor(theme.ColorNameDisabled), segLabel(1), "a tab is not drawn disabled")
	dr := test.TempWidgetRenderer(t, dt.datePicker).(*datePickerRenderer)
	assert.Equal(t, themeColor(theme.ColorNameDisabled), dr.monthLabel.Color, "the date page is not drawn disabled")

	// Tap: the Time tab, and a day on the date page.
	test.Tap(segs[1])
	segs[1].PerformAccessibleAction(core.ActionSelect)
	assert.Equal(t, 0, dt.tabs.Selected, "a tap on a tab of a disabled picker switched the page")
	assert.True(t, dt.datePicker.Visible())
	assert.False(t, dt.timePicker.Visible(), "a tap on a tab of a disabled picker showed the time page")
	test.Tap(dayCellFor(t, dt.datePicker, 20))
	assert.Equal(t, 0, changed, "a tap on a day of a disabled picker fired OnChanged")
	// Drag and scroll: neither the tabs nor the date page has either
	// gesture; the time page's drag is covered by TestTimePicker_Disable.
	test.Drag(c, absPos(segs[1]).Add(geo.NewPos(4, 4)), 30, 0)
	test.Scroll(c, absPos(segs[1]).Add(geo.NewPos(4, 4)), 0, -30)
	// Keys on each part.
	dt.tabs.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	dt.datePicker.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	dt.timePicker.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 0, dt.tabs.Selected, "a key on the tabs of a disabled picker switched the page")
	assert.Equal(t, 0, changed, "a key on a disabled picker fired OnChanged")
	assert.Equal(t, sel, dt.Selected)

	c.FocusNext()
	assert.Equal(t, other, c.Focused(), "Tab stopped on the disabled picker or a part of it")

	dt.Enable()
	assert.False(t, dt.tabs.Disabled() || dt.datePicker.Disabled() || dt.timePicker.Disabled(), "Enable left a part disabled")
	assert.Equal(t, themeColor(theme.ColorNameOnSurface), segLabel(1), "Enable left a tab disabled")
	test.Tap(dayCellFor(t, dt.datePicker, 20))
	assert.Equal(t, 1, changed, "a tap on a day of an enabled picker did not fire OnChanged")
	test.Tap(segs[1])
	assert.True(t, dt.timePicker.Visible(), "a tap on the Time tab of an enabled picker did not show the time page")

	// Disabled after the first draw with the focus on its tabs: the focus goes.
	c.Focus(dt.tabs)
	require.Equal(t, dt.tabs, c.Focused())
	dt.Disable()
	assert.Nil(t, c.Focused(), "the tabs of the disabled picker kept the focus")
	assert.True(t, dt.tabs.Disabled())
}

// A renderer built after Disable, as when a window closes and the picker is
// shown again, builds new tabs and month buttons: they come up disabled.
func TestDateTimePicker_DisabledRendererRebuilt(t *testing.T) {
	test.NewTempApp(t)
	dt := NewDateTimePicker(time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local), true, nil)
	dt.Disable()
	cache.DestroyRenderer(dt)
	cache.DestroyRenderer(dt.datePicker)
	test.TempWidgetRenderer(t, dt)
	require.NotNil(t, dt.tabs)
	assert.True(t, dt.tabs.Disabled(), "the rebuilt tabs are enabled")
	dr := test.TempWidgetRenderer(t, dt.datePicker).(*datePickerRenderer)
	assert.True(t, dr.nextBtn.Disabled(), "the rebuilt month button is enabled")
}
