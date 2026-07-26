package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

func TestDateTimeEntry_SetDateTime(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	dt := time.Date(2025, time.March, 5, 14, 30, 0, 0, time.UTC)
	e.SetDateTime(dt)

	assert.Equal(t, "03/05/2025 2:30 PM", e.Text)
	assert.Equal(t, dt.Year(), e.Selected.Year())
	assert.Equal(t, time.March, e.Selected.Month())
	assert.Equal(t, 5, e.Selected.Day())
	assert.Equal(t, 14, e.Selected.Hour())
	assert.Equal(t, 30, e.Selected.Minute())
}

func TestDateTimeEntry_DateTime(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	e.SetText("03/05/2025 2:30 PM")

	dt, err := e.DateTime()
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
	assert.Equal(t, 14, dt.Hour())
	assert.Equal(t, 30, dt.Minute())
}

func TestDateTimeEntry_DateTime_Empty(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_, err := e.DateTime()
	assert.Error(t, err)
}

func TestDateTimeEntry_Validation(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	assert.NoError(t, e.Validate())

	e.SetText("03/05/2025 2:30 PM")
	assert.NoError(t, e.Validate())

	e.SetText("garbage")
	assert.Error(t, e.Validate())
}

func TestDateTimeEntry_OnChanged(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	var called time.Time
	e := NewDateTimeEntry()
	e.OnChanged = func(dt time.Time) {
		called = dt
	}

	e.SetText("07/14/2025 9:00 AM")
	assert.Equal(t, 2025, called.Year())
	assert.Equal(t, time.July, called.Month())
	assert.Equal(t, 14, called.Day())
	assert.Equal(t, 9, called.Hour())
}

func TestDateTimeEntry_ActionItem(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)
	assert.NotNil(t, e.ActionItem, "picker button should be set")
}

func TestDateTimeEntry_Use24Hour(t *testing.T) {
	test.NewTempApp(t)

	lang.SetLocaleForTests(t, "en-US")
	e := NewDateTimeEntry()
	assert.False(t, e.Use24Hour, "US should default to 12h")

	lang.SetLocaleForTests(t, "de-DE")
	e2 := NewDateTimeEntry()
	assert.True(t, e2.Use24Hour, "Germany should default to 24h")
}

func TestDateTimeEntry_GermanLocale(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "de-DE")

	e := NewDateTimeEntry()
	dt := time.Date(2025, time.March, 5, 14, 30, 0, 0, time.UTC)
	e.SetDateTime(dt)
	assert.Equal(t, "05.03.2025 14:30", e.Text)

	parsed, err := e.DateTime()
	assert.NoError(t, err)
	assert.Equal(t, dt.Year(), parsed.Year())
	assert.Equal(t, dt.Month(), parsed.Month())
	assert.Equal(t, 14, parsed.Hour())
}

func TestDateTimeEntry_Placeholder(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e := NewDateTimeEntry()
	assert.Equal(t, "MM/DD/YYYY h:mm AM/PM", e.PlaceHolder)

	lang.SetLocaleForTests(t, "de-DE")
	e2 := NewDateTimeEntry()
	assert.Equal(t, "DD.MM.YYYY hh:mm", e2.PlaceHolder)
}

func TestDateTimeEntry_TypedRune_Digits(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)

	e.TypedRune('1')
	e.TypedRune('2')
	assert.Equal(t, "12", e.Text)
}

func TestDateTimeEntry_TypedRune_Separators(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)

	e.TypedRune('/')
	e.TypedRune(':')
	e.TypedRune(' ')
	assert.Equal(t, "/: ", e.Text)
}

func TestDateTimeEntry_TypedRune_AMPMChars(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)

	e.TypedRune('A')
	e.TypedRune('M')
	e.TypedRune('P')
	assert.Equal(t, "AMP", e.Text)
}

func TestDateTimeEntry_TypedRune_RejectsLetters(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)

	e.TypedRune('x')
	e.TypedRune('Z')
	e.TypedRune('!')
	assert.Equal(t, "", e.Text, "non-datetime characters should be rejected")
}

func TestDateTimeEntry_TypedShortcut_Paste(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)

	clipboard := test.NewClipboard()
	clipboard.SetContent("03/05/2025 2:30 PM#$%")
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clipboard})
	assert.Equal(t, "03/05/2025 2:30 PM", e.Text, "non-datetime characters should be filtered")
}

func TestDateTimeEntry_RoundTrip_MultiLocale(t *testing.T) {
	test.NewTempApp(t)
	dt := time.Date(2025, time.November, 23, 16, 45, 0, 0, time.UTC)

	for _, loc := range []string{"en-US", "de-DE", "ja-JP", "en-GB"} {
		t.Run(loc, func(t *testing.T) {
			lang.SetLocaleForTests(t, loc)
			e := NewDateTimeEntry()
			e.SetDateTime(dt)
			parsed, err := e.DateTime()
			assert.NoError(t, err)
			assert.Equal(t, dt.Year(), parsed.Year())
			assert.Equal(t, dt.Month(), parsed.Month())
			assert.Equal(t, dt.Day(), parsed.Day())
			assert.Equal(t, dt.Hour(), parsed.Hour())
			assert.Equal(t, dt.Minute(), parsed.Minute())
		})
	}
}

func TestDateTimeEntry_Disable(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateTimeEntry()
	_ = test.TempWidgetRenderer(t, e)
	assert.NotNil(t, e.ActionItem)

	e.Disable()
	assert.True(t, e.Disabled())
	assert.True(t, e.ActionItem.(core.Disableable).Disabled())

	e.Enable()
	assert.False(t, e.Disabled())
	assert.False(t, e.ActionItem.(core.Disableable).Disabled())
}

func TestDateTimeEntry_OnChanged_NotCalledForInvalid(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	called := false
	e := NewDateTimeEntry()
	e.OnChanged = func(_ time.Time) {
		called = true
	}

	e.SetText("garbage")
	assert.False(t, called, "OnChanged should not fire for invalid input")
}
