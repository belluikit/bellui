package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/test"
)

// The combined picker merges date and time changes so that adjusting one
// component never discards the other (pre-rebrand tracker #291).

func TestDateTimePicker_DateChangePreservesTime(t *testing.T) {
	test.NewTempApp(t)
	base := time.Date(2024, time.March, 15, 9, 30, 0, 0, time.UTC)
	var got time.Time
	dt := NewDateTimePicker(base, true, func(tm time.Time) { got = tm })

	// The date child reports a new day; the time-of-day must be preserved.
	dt.datePicker.OnChanged(time.Date(2025, time.July, 20, 0, 0, 0, 0, time.UTC))

	assert.Equal(t, 2025, dt.Selected.Year())
	assert.Equal(t, time.July, dt.Selected.Month())
	assert.Equal(t, 20, dt.Selected.Day())
	assert.Equal(t, 9, dt.Selected.Hour(), "hour preserved across a date change")
	assert.Equal(t, 30, dt.Selected.Minute(), "minute preserved across a date change")
	assert.Equal(t, dt.Selected, got, "OnChanged receives the merged value")
}

func TestDateTimePicker_TimeChangePreservesDate(t *testing.T) {
	test.NewTempApp(t)
	base := time.Date(2024, time.March, 15, 9, 30, 0, 0, time.UTC)
	var got time.Time
	dt := NewDateTimePicker(base, true, func(tm time.Time) { got = tm })

	// The time child reports a new H:M; the calendar date must be preserved.
	dt.timePicker.OnChanged(14, 45)

	assert.Equal(t, 2024, dt.Selected.Year(), "year preserved across a time change")
	assert.Equal(t, time.March, dt.Selected.Month(), "month preserved across a time change")
	assert.Equal(t, 15, dt.Selected.Day(), "day preserved across a time change")
	assert.Equal(t, 14, dt.Selected.Hour())
	assert.Equal(t, 45, dt.Selected.Minute())
	assert.Equal(t, dt.Selected, got)
}

func TestDateTimePicker_RendererTabs(t *testing.T) {
	test.NewTempApp(t)
	dt := NewDateTimePicker(time.Now(), true, nil)
	// CreateRenderer wires the Date/Time tab toggle and hides the time pane
	// initially; exercise it so the render path is covered.
	assert.NotPanics(t, func() { test.TempWidgetRenderer(t, dt) })
	assert.True(t, dt.datePicker.Visible())
	assert.False(t, dt.timePicker.Visible(), "time pane starts hidden behind the Date tab")
}
