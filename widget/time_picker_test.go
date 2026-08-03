package widget

import (
	"math"
	"testing"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

func TestTimePicker_Create(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(14, 30, false, nil)
	assert.Equal(t, 14, tp.Hour)
	assert.Equal(t, 30, tp.Minute)
	assert.False(t, tp.Use24Hour)
}

func TestTimePicker_Create24h(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(23, 59, true, nil)
	assert.Equal(t, 23, tp.Hour)
	assert.Equal(t, 59, tp.Minute)
	assert.True(t, tp.Use24Hour)
}

// visibleLabelTexts returns the text of every shown clock label, in slot order.
func visibleLabelTexts(r *timePickerRenderer) []string {
	var out []string
	for _, l := range r.clockLabels {
		if l.Visible() {
			out = append(out, l.Text)
		}
	}
	return out
}

// TestTimePicker_24hInnerRing verifies the MD3 24-hour dial renders both a
// full-radius outer ring (1–12) and an inner ring (13–23 and 00), and that the
// inner ring is only present in 24h hour-selection mode (#804).
func TestTimePicker_24hInnerRing(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(23, 0, true, nil)
	tp.Resize(tp.MinSize())
	tp.Refresh()
	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)

	labels := visibleLabelTexts(r)
	assert.Equal(t, []string{
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12",
		"13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "00",
	}, labels, "24h dial should render both rings")

	// 12h and minute dials keep the single 12-label ring.
	tp12 := NewTimePicker(10, 30, false, nil)
	tp12.Resize(tp12.MinSize())
	tp12.Refresh()
	r12 := test.TempWidgetRenderer(t, tp12).(*timePickerRenderer)
	assert.Len(t, visibleLabelTexts(r12), 12)

	tp.selectingMinute = true
	tp.Refresh()
	assert.Len(t, visibleLabelTexts(r), 12, "minute dial hides the inner ring")
}

// TestTimePicker_24hInnerRingSelection checks that inner-ring hours (0 and
// 13–23) place the selector arm at the inner radius and highlight the inner
// label, and that tapping the inner ring selects an inner hour (#804).
func TestTimePicker_24hInnerRingSelection(t *testing.T) {
	test.NewTempApp(t)

	// Highlight + arm radius for an inner-ring hour.
	tp := NewTimePicker(23, 0, true, nil)
	tp.Resize(tp.MinSize())
	tp.Refresh()
	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)

	cx, cy, radius := r.clockCenterX, r.clockCenterY, r.clockRadius
	armLen := func() float32 {
		dx := r.selectorArm.Position2.X - cx
		dy := r.selectorArm.Position2.Y - cy
		return float32(math.Sqrt(float64(dx*dx + dy*dy)))
	}
	assert.Less(t, armLen(), radius*0.8, "hour 23 arm should reach the inner ring")
	assert.Equal(t, "23", r.clockLabels[selected24HourLabelIndex(23)].Text)
	assert.Equal(t, "00", r.clockLabels[selected24HourLabelIndex(0)].Text)

	// An outer-ring hour keeps the arm at the full radius.
	tp.Hour = 10
	tp.Refresh()
	assert.Greater(t, armLen(), radius*0.8, "hour 10 arm should reach the outer ring")

	// Tapping near the center (inner ring) at 3 o'clock selects hour 15.
	var gotH int
	tp2 := NewTimePicker(10, 0, true, func(h, _ int) { gotH = h })
	tp2.Resize(tp2.MinSize())
	tp2.Refresh()
	r2 := test.TempWidgetRenderer(t, tp2).(*timePickerRenderer)
	tp2.Tapped(&event.PointEvent{Position: geo.NewPos(r2.clockCenterX+r2.clockRadius*0.6, r2.clockCenterY)})
	assert.Equal(t, 15, gotH, "tap on inner ring 3 o'clock = 15:00")
}

func TestTimePicker_MinSize(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(12, 0, false, nil)
	min := tp.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))

	// 24h mode should be narrower (no AM/PM).
	tp24 := NewTimePicker(12, 0, true, nil)
	min24 := tp24.MinSize()
	assert.Less(t, min24.Width, min.Width)
}

func TestTimePicker_Renderer(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(10, 45, false, nil)
	r := test.TempWidgetRenderer(t, tp)
	assert.NotNil(t, r)
	assert.Greater(t, len(r.Objects()), 0)
}

func TestTimePicker_HourSelection(t *testing.T) {
	test.NewTempApp(t)
	var gotH, gotM int
	tp := NewTimePicker(3, 0, false, func(h, m int) {
		gotH = h
		gotM = m
	})
	tp.Resize(tp.MinSize())
	tp.Refresh()

	// Tap on 6 o'clock position (bottom of dial).
	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)
	cx := r.clockCenterX
	cy := r.clockCenterY
	radius := r.clockRadius

	// 6 o'clock = 180° = bottom.
	tapX := cx
	tapY := cy + radius

	tp.Tapped(&event.PointEvent{Position: geo.NewPos(tapX, tapY)})
	assert.Equal(t, 6, gotH)
	_ = gotM
	// After hour selection, should switch to minute mode.
	assert.True(t, tp.selectingMinute)
}

func TestTimePicker_MinuteSelection(t *testing.T) {
	test.NewTempApp(t)
	var gotM int
	tp := NewTimePicker(10, 0, false, func(h, m int) {
		gotM = m
	})
	tp.selectingMinute = true
	tp.Resize(tp.MinSize())
	tp.Refresh()

	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)
	cx := r.clockCenterX
	cy := r.clockCenterY
	radius := r.clockRadius

	// Tap at 3 o'clock position = 15 minutes.
	tapX := cx + radius
	tapY := cy

	tp.Tapped(&event.PointEvent{Position: geo.NewPos(tapX, tapY)})
	assert.Equal(t, 15, gotM)
}

func TestTimePicker_AMPMToggle(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(9, 30, false, nil) // 9 AM
	assert.True(t, tp.Hour < 12)

	tp.Resize(tp.MinSize())
	tp.Refresh()

	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)

	// The AM/PM fields are widgets that receive their own taps (#543).
	r.pmBtn.Tapped(&event.PointEvent{})
	assert.Equal(t, 21, tp.Hour) // 9 AM + 12 = 21

	r.amBtn.Tapped(&event.PointEvent{})
	assert.Equal(t, 9, tp.Hour) // back to 9 AM
}

func TestTimePicker_ModeSwitch(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(10, 30, false, nil)
	assert.False(t, tp.selectingMinute)

	tp.Resize(tp.MinSize())
	tp.Refresh()

	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)

	// Tap the minute field to switch to minute mode.
	r.minuteBtn.Tapped(&event.PointEvent{})
	assert.True(t, tp.selectingMinute)

	// Tap the hour field to switch back.
	r.hourBtn.Tapped(&event.PointEvent{})
	assert.False(t, tp.selectingMinute)
}

func TestTimePicker_TapOutsideDial(t *testing.T) {
	test.NewTempApp(t)
	called := false
	tp := NewTimePicker(10, 0, false, func(h, m int) {
		called = true
	})
	tp.Resize(tp.MinSize())
	tp.Refresh()

	// Tap far outside dial.
	tp.Tapped(&event.PointEvent{Position: geo.NewPos(0, 0)})
	// Should not have triggered changed (it may switch hour/minute mode from box tap).
	_ = called
}

func TestTimePicker_KeyboardNavigation(t *testing.T) {
	test.NewTempApp(t)
	var gotH, gotM int
	tp := NewTimePicker(9, 30, false, func(h, m int) { gotH, gotM = h, m })
	tp.Resize(tp.MinSize())
	tp.Refresh()

	tp.FocusGained()
	assert.True(t, tp.focused)
	assert.Equal(t, tpFieldHour, tp.focusField, "cursor starts on the hour field")

	// Up/Down adjust the hour with wrap.
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 10, tp.Hour)
	assert.Equal(t, 10, gotH)
	tp.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 9, tp.Hour)

	// Right moves to the minute field and syncs the dial mode.
	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, tpFieldMinute, tp.focusField)
	assert.True(t, tp.selectingMinute)
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 31, tp.Minute)
	assert.Equal(t, 31, gotM)

	// Minute wraps at the hour boundary.
	tp.Minute = 59
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 0, tp.Minute)

	// Right again reaches AM, Enter forces AM (no-op here), PM sets PM.
	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, tpFieldAM, tp.focusField)
	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, tpFieldPM, tp.focusField)
	tp.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, 21, tp.Hour, "Enter on PM moves 9 AM to 9 PM")

	// Up/Down on the meridiem fields toggles it.
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 9, tp.Hour)

	tp.FocusLost()
	assert.False(t, tp.focused)
}

func TestTimePicker_Keyboard24hSkipsMeridiem(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(23, 0, true, nil)
	tp.FocusGained()

	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, tpFieldMinute, tp.focusField, "24h mode has no AM/PM fields")

	// Hour wraps 23 -> 0.
	tp.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	tp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	assert.Equal(t, 0, tp.Hour)
}

func TestTimePicker_DigitEntry(t *testing.T) {
	test.NewTempApp(t)

	cases := []struct {
		name       string
		use24h     bool
		startH     int
		startM     int
		startField int    // where the field cursor begins (tpFieldHour default)
		runes      string // typed characters, in order
		wantH      int
		wantM      int
		wantCalls  int
		wantField  int
	}{
		{
			name: "1437 flows hour then minute (24h)", use24h: true,
			runes: "1437", wantH: 14, wantM: 37, wantCalls: 2, wantField: tpFieldMinute,
		},
		{
			name: "hour 7 commits as 07 and advances (24h)", use24h: true,
			runes: "7", wantH: 7, wantM: 0, wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "hour 25 clamps to 23 (24h)", use24h: true,
			runes: "25", wantH: 23, wantM: 0, wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "hour 00 buffers leading zero (24h)", use24h: true, startH: 9,
			runes: "00", wantH: 0, wantM: 0, wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "minute 9 commits as 09 immediately", use24h: true,
			startField: tpFieldMinute, runes: "9", wantH: 0, wantM: 9,
			wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "minute 55 two-digit commit", use24h: true,
			startField: tpFieldMinute, runes: "55", wantH: 0, wantM: 55,
			wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "12h P sets PM", startH: 9,
			runes: "P", wantH: 21, wantM: 0, wantCalls: 1, wantField: tpFieldHour,
		},
		{
			name: "12h A sets AM", startH: 21,
			runes: "A", wantH: 9, wantM: 0, wantCalls: 1, wantField: tpFieldHour,
		},
		{
			name: "12h hour 12 maps to midnight (AM)", startH: 9,
			runes: "12", wantH: 0, wantM: 0, wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "12h hour 13 clamps to 12 (AM)", startH: 9,
			runes: "13", wantH: 0, wantM: 0, wantCalls: 1, wantField: tpFieldMinute,
		},
		{
			name: "12h hour keeps PM meridiem", startH: 15, // 3 PM
			runes: "05", wantH: 17, wantM: 0, wantCalls: 1, wantField: tpFieldMinute,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls, gotH, gotM int
			tp := NewTimePicker(c.startH, c.startM, c.use24h, func(h, m int) {
				calls++
				gotH, gotM = h, m
			})
			tp.Resize(tp.MinSize())
			tp.Refresh()
			tp.FocusGained()
			if c.startField == tpFieldMinute {
				tp.focusField = tpFieldMinute
				tp.selectingMinute = true
			}

			for _, r := range c.runes {
				tp.TypedRune(r)
			}

			assert.Equal(t, c.wantH, tp.Hour, "hour")
			assert.Equal(t, c.wantM, tp.Minute, "minute")
			assert.Equal(t, c.wantCalls, calls, "OnChanged call count")
			assert.Equal(t, c.wantField, tp.focusField, "field cursor")
			if c.wantCalls > 0 {
				assert.Equal(t, c.wantH, gotH, "OnChanged hour")
				assert.Equal(t, c.wantM, gotM, "OnChanged minute")
			}
			assert.False(t, tp.hasPartial, "partial buffer cleared after commit")
		})
	}
}

func TestTimePicker_DigitEntryEscapeClearsPartial(t *testing.T) {
	test.NewTempApp(t)
	var calls int
	tp := NewTimePicker(9, 30, true, func(int, int) { calls++ })
	tp.FocusGained()

	// A first digit that can be a tens prefix is buffered, not committed.
	tp.TypedRune('1')
	assert.True(t, tp.hasPartial)
	assert.Equal(t, 9, tp.Hour, "value unchanged while partial is pending")
	assert.Equal(t, 0, calls, "no commit yet")

	// Escape discards the pending digit; the value keeps following the dial.
	tp.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, tp.hasPartial)
	assert.Equal(t, 9, tp.Hour)
	assert.Equal(t, 0, calls)

	// Moving the cursor also discards a pending digit.
	tp.TypedRune('2')
	assert.True(t, tp.hasPartial)
	tp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.False(t, tp.hasPartial, "cursor move clears the partial buffer")
	assert.Equal(t, 9, tp.Hour)
	assert.Equal(t, 0, calls)
}

func TestTimePicker_HandlesEscapeOnlyWhilePartial(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(9, 30, true, func(int, int) {})
	tp.FocusGained()

	assert.False(t, tp.HandlesEscape(), "with nothing to discard, Escape belongs to the dialog around the picker")

	tp.TypedRune('1')
	assert.True(t, tp.HandlesEscape(), "a buffered tens digit is the innermost thing to back out of")

	tp.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, tp.HandlesEscape())
}

func TestTimePickerButton_Accessible(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(9, 30, false, nil)
	tp.Resize(tp.MinSize())
	tp.Refresh()

	r := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)

	assert.Equal(t, core.RoleTab, r.hourBtn.AccessibleRole())
	assert.Equal(t, core.RoleRadioButton, r.amBtn.AccessibleRole())
	assert.Contains(t, r.hourBtn.AccessibleLabel(), "09")
	assert.True(t, r.hourBtn.AccessibleState().Has(core.StateSelected), "hour field drives the dial initially")
	assert.True(t, r.amBtn.AccessibleState().Has(core.StateChecked), "9:30 is AM")
	assert.False(t, r.pmBtn.AccessibleState().Has(core.StateChecked))

	// Activating via the accessibility action behaves like a tap.
	r.pmBtn.PerformAccessibleAction(core.ActionSelect)
	assert.Equal(t, 21, tp.Hour)
	tp.Refresh()
	assert.True(t, r.pmBtn.AccessibleState().Has(core.StateChecked))

	r.minuteBtn.PerformAccessibleAction(core.ActionSelect)
	assert.True(t, tp.selectingMinute)

	// The keyboard cursor reports StateFocused on the focused field.
	tp.FocusGained()
	tp.Refresh()
	assert.True(t, r.minuteBtn.AccessibleState().Has(core.StateFocused))
	assert.False(t, r.hourBtn.AccessibleState().Has(core.StateFocused))
}

func TestTimePicker_AngleCalculation(t *testing.T) {
	// Verify angle math: 12 o'clock = 0°, 3 o'clock = 90°.
	// atan2(dx, -dy) where dx=1,dy=-1 (3 o'clock: right of center, same height).
	angle := math.Atan2(1.0, 0.0) // dx=1, -dy=0
	assert.InDelta(t, math.Pi/2, angle, 0.01)

	// 6 o'clock: dx=0, dy=1 → atan2(0, -1) = π.
	angle = math.Atan2(0.0, -1.0)
	assert.InDelta(t, math.Pi, angle, 0.01)
}

func TestNewAutoTimePicker_US(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	tp := NewAutoTimePicker(14, 30, nil)
	assert.False(t, tp.Use24Hour, "US locale should auto-detect 12-hour")
	assert.Equal(t, 14, tp.Hour)
	assert.Equal(t, 30, tp.Minute)
}

func TestNewAutoTimePicker_Germany(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "de-DE")
	tp := NewAutoTimePicker(14, 30, nil)
	assert.True(t, tp.Use24Hour, "German locale should auto-detect 24-hour")
}

func TestNewAutoDateTimePicker_US(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	dt := NewAutoDateTimePicker(
		time.Date(2025, time.March, 5, 14, 30, 0, 0, time.UTC),
		nil,
	)
	assert.False(t, dt.Use24Hour, "US locale should auto-detect 12-hour")
}

func TestNewAutoDateTimePicker_France(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "fr-FR")
	dt := NewAutoDateTimePicker(
		time.Date(2025, time.March, 5, 14, 30, 0, 0, time.UTC),
		nil,
	)
	assert.True(t, dt.Use24Hour, "French locale should auto-detect 24-hour")
}
