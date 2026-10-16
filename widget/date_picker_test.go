package widget

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

func TestDatePicker_Create(t *testing.T) {
	test.NewTempApp(t)
	sel := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	dp := NewDatePicker(sel, nil)
	assert.Equal(t, sel, dp.Selected)
	assert.False(t, dp.viewing.IsZero())
}

func TestDatePicker_ZeroTime(t *testing.T) {
	test.NewTempApp(t)
	dp := NewDatePicker(time.Time{}, nil)
	assert.False(t, dp.viewing.IsZero(), "viewing should default to now when zero time given")
}

func TestDatePicker_MinSize(t *testing.T) {
	test.NewTempApp(t)
	dp := NewDatePicker(time.Now(), nil)
	min := dp.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestDatePicker_Renderer(t *testing.T) {
	test.NewTempApp(t)
	dp := NewDatePicker(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	r := test.TempWidgetRenderer(t, dp)
	assert.NotNil(t, r)
	assert.Greater(t, len(r.Objects()), 0)
}

func TestDatePicker_MonthNavigation(t *testing.T) {
	test.NewTempApp(t)
	dp := NewDatePicker(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), nil)
	assert.Equal(t, time.June, dp.viewing.Month())

	// Navigate forward.
	dp.viewing = dp.viewing.AddDate(0, 1, 0)
	dp.Refresh()
	assert.Equal(t, time.July, dp.viewing.Month())

	// Navigate backward.
	dp.viewing = dp.viewing.AddDate(0, -2, 0)
	dp.Refresh()
	assert.Equal(t, time.May, dp.viewing.Month())
}

// dayCellFor returns the realized cell widget for the given day of the
// picker's viewing month.
func dayCellFor(t *testing.T, dp *DatePicker, day int) *dateDayCell {
	t.Helper()
	r, ok := test.TempWidgetRenderer(t, dp).(*datePickerRenderer)
	if !ok {
		t.Fatal("unexpected renderer type")
	}
	dp.Refresh()

	firstOfMonth := time.Date(dp.viewing.Year(), dp.viewing.Month(), 1, 0, 0, 0, 0, dp.viewing.Location())
	idx := lang.WeekdayOffset(firstOfMonth) + day - 1
	if idx < 0 || idx >= len(r.dayCells) {
		t.Fatalf("day %d out of grid range", day)
	}
	return r.dayCells[idx]
}

func TestDatePicker_DaySelection(t *testing.T) {
	test.NewTempApp(t)
	var received time.Time
	dp := NewDatePicker(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), func(d time.Time) {
		received = d
	})
	dp.Resize(dp.MinSize())

	// Day cells are real tappable widgets (pre-rebrand tracker #521); tap June 20 directly.
	cell := dayCellFor(t, dp, 20)
	assert.Equal(t, 20, cell.day)
	cell.Tapped(&event.PointEvent{})

	assert.Equal(t, 20, received.Day())
	assert.Equal(t, time.June, received.Month())
	assert.Equal(t, 2025, received.Year())
}

func TestDatePicker_BlankCellsInert(t *testing.T) {
	test.NewTempApp(t)
	called := false
	dp := NewDatePicker(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), func(d time.Time) {
		called = true
	})
	dp.Resize(dp.MinSize())

	r, _ := test.TempWidgetRenderer(t, dp).(*datePickerRenderer)
	dp.Refresh()

	// The last grid cell is past June's end: hidden and inert.
	blank := r.dayCells[len(r.dayCells)-1]
	assert.Equal(t, 0, blank.day)
	assert.False(t, blank.Visible())
	blank.Tapped(&event.PointEvent{})
	assert.False(t, called)
}

func TestDateDayCell_Accessible(t *testing.T) {
	test.NewTempApp(t)
	dp := NewDatePicker(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), nil)
	dp.Resize(dp.MinSize())

	sel := dayCellFor(t, dp, 15)
	assert.Equal(t, core.RoleButton, sel.AccessibleRole())
	assert.NotEmpty(t, sel.AccessibleLabel())
	assert.True(t, sel.AccessibleState().Has(core.StateSelected))

	other := dayCellFor(t, dp, 20)
	assert.False(t, other.AccessibleState().Has(core.StateSelected))

	// Activating via the accessibility action selects the day.
	other.PerformAccessibleAction(core.ActionPress)
	assert.Equal(t, 20, dp.Selected.Day())
	assert.True(t, other.AccessibleState().Has(core.StateSelected))
	assert.False(t, sel.AccessibleState().Has(core.StateSelected))
}

func TestDatePicker_KeyboardNavigation(t *testing.T) {
	test.NewTempApp(t)
	var received time.Time
	dp := NewDatePicker(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), func(d time.Time) {
		received = d
	})
	dp.Resize(dp.MinSize())

	dp.FocusGained()
	assert.True(t, dp.focused)
	assert.Equal(t, 15, dp.cursor.Day(), "cursor starts on the selected day")

	dp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.Equal(t, 16, dp.cursor.Day())

	dp.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 23, dp.cursor.Day())

	dp.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	dp.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 15, dp.cursor.Day())

	// Crossing the month boundary flips the viewing month.
	dp.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	dp.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	dp.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, time.July, dp.cursor.Month())
	assert.Equal(t, time.July, dp.viewing.Month())

	// PageUp returns to June; Enter selects the cursor day.
	dp.TypedKey(&event.KeyEvent{Name: event.KeyPageUp})
	assert.Equal(t, time.June, dp.viewing.Month())
	dp.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, dp.cursor.Day(), received.Day())
	assert.Equal(t, time.June, received.Month())
	assert.Equal(t, dp.Selected.Day(), received.Day())
}

func TestDatePicker_KeyboardHomeEnd(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	dp := NewDatePicker(time.Date(2025, 6, 18, 0, 0, 0, 0, time.UTC), nil) // a Wednesday
	dp.FocusGained()

	dp.TypedKey(&event.KeyEvent{Name: event.KeyHome})
	assert.Equal(t, 0, lang.WeekdayOffset(dp.cursor), "Home moves to the first day of the week")

	dp.TypedKey(&event.KeyEvent{Name: event.KeyEnd})
	assert.Equal(t, 6, lang.WeekdayOffset(dp.cursor), "End moves to the last day of the week")
}

func TestDatePicker_FocusStateOnCell(t *testing.T) {
	test.NewTempApp(t)
	dp := NewDatePicker(time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), nil)
	dp.Resize(dp.MinSize())

	dp.FocusGained()
	cell := dayCellFor(t, dp, 15)
	assert.True(t, cell.focused)
	assert.True(t, cell.AccessibleState().Has(core.StateFocused))

	dp.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.False(t, dayCellFor(t, dp, 15).focused)
	assert.True(t, dayCellFor(t, dp, 16).AccessibleState().Has(core.StateFocused))

	dp.FocusLost()
	assert.False(t, dayCellFor(t, dp, 16).focused)
}

func TestDatePicker_DaysInMonth(t *testing.T) {
	assert.Equal(t, 28, daysIn(time.February, 2025))
	assert.Equal(t, 29, daysIn(time.February, 2024)) // leap year
	assert.Equal(t, 31, daysIn(time.January, 2025))
	assert.Equal(t, 30, daysIn(time.April, 2025))
}
