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

func TestDateEntry_SetDate(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	dt := time.Date(2025, time.March, 5, 0, 0, 0, 0, time.UTC)
	e.SetDate(dt)

	assert.Equal(t, "03/05/2025", e.Text)
	assert.Equal(t, dt.Year(), e.Selected.Year())
	assert.Equal(t, dt.Month(), e.Selected.Month())
	assert.Equal(t, dt.Day(), e.Selected.Day())
}

func TestDateEntry_Date(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	e.SetText("03/05/2025")

	dt, err := e.Date()
	assert.NoError(t, err)
	assert.Equal(t, 2025, dt.Year())
	assert.Equal(t, time.March, dt.Month())
	assert.Equal(t, 5, dt.Day())
}

func TestDateEntry_Date_Empty(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	_, err := e.Date()
	assert.Error(t, err)
}

func TestDateEntry_Validation(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()

	assert.NoError(t, e.Validate())

	e.SetText("03/05/2025")
	assert.NoError(t, e.Validate())

	e.SetText("not-a-date")
	assert.Error(t, e.Validate())

	e.SetText("13/32/2025")
	assert.Error(t, e.Validate())
}

func TestDateEntry_OnChanged(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	var called time.Time
	e := NewDateEntry()
	e.OnChanged = func(t time.Time) {
		called = t
	}

	e.SetText("07/14/2025")
	assert.Equal(t, 2025, called.Year())
	assert.Equal(t, time.July, called.Month())
	assert.Equal(t, 14, called.Day())
}

func TestDateEntry_ActionItem(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	_ = test.TempWidgetRenderer(t, e)
	assert.NotNil(t, e.ActionItem, "calendar button should be set")
}

func TestDateEntry_Placeholder(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e := NewDateEntry()
	assert.Equal(t, "MM/DD/YYYY", e.PlaceHolder)

	lang.SetLocaleForTests(t, "de-DE")
	e2 := NewDateEntry()
	assert.Equal(t, "DD.MM.YYYY", e2.PlaceHolder)
}

func TestDateEntry_GermanLocale(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "de-DE")

	e := NewDateEntry()
	dt := time.Date(2025, time.March, 5, 0, 0, 0, 0, time.UTC)
	e.SetDate(dt)
	assert.Equal(t, "05.03.2025", e.Text)

	parsed, err := e.Date()
	assert.NoError(t, err)
	assert.Equal(t, dt.Year(), parsed.Year())
	assert.Equal(t, dt.Month(), parsed.Month())
	assert.Equal(t, dt.Day(), parsed.Day())
}

func TestDateEntry_TypedRune_Digits(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	w := test.TempWidgetRenderer(t, e)
	_ = w

	// Digits should be accepted.
	e.TypedRune('1')
	e.TypedRune('2')
	assert.Equal(t, "12", e.Text)
}

func TestDateEntry_TypedRune_Separators(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	_ = test.TempWidgetRenderer(t, e)

	e.TypedRune('/')
	e.TypedRune('.')
	e.TypedRune('-')
	assert.Equal(t, "/.-", e.Text)
}

func TestDateEntry_TypedRune_RejectsLetters(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	_ = test.TempWidgetRenderer(t, e)

	e.TypedRune('a')
	e.TypedRune('Z')
	e.TypedRune('!')
	assert.Equal(t, "", e.Text, "letters and symbols should be rejected")
}

func TestDateEntry_TypedShortcut_Paste(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	_ = test.TempWidgetRenderer(t, e)

	clipboard := test.NewClipboard()
	clipboard.SetContent("03/05/2025abc!")
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: clipboard})
	assert.Equal(t, "03/05/2025", e.Text, "non-date characters should be filtered from paste")
}

func TestDateEntry_RoundTrip_MultiLocale(t *testing.T) {
	test.NewTempApp(t)
	dt := time.Date(2025, time.November, 23, 0, 0, 0, 0, time.UTC)

	for _, loc := range []string{"en-US", "de-DE", "ja-JP", "en-GB"} {
		t.Run(loc, func(t *testing.T) {
			lang.SetLocaleForTests(t, loc)
			e := NewDateEntry()
			e.SetDate(dt)
			parsed, err := e.Date()
			assert.NoError(t, err)
			assert.Equal(t, dt.Year(), parsed.Year())
			assert.Equal(t, dt.Month(), parsed.Month())
			assert.Equal(t, dt.Day(), parsed.Day())
		})
	}
}

func TestDateEntry_Disable(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	e := NewDateEntry()
	_ = test.TempWidgetRenderer(t, e)
	assert.NotNil(t, e.ActionItem)

	e.Disable()
	assert.True(t, e.Disabled())
	assert.True(t, e.ActionItem.(core.Disableable).Disabled(), "ActionItem should be disabled")

	e.Enable()
	assert.False(t, e.Disabled())
	assert.False(t, e.ActionItem.(core.Disableable).Disabled(), "ActionItem should be enabled")
}

func TestDateEntry_EntryOnChanged_NotHijacked(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	var rawText string
	e := NewDateEntry()
	e.Entry.OnChanged = func(s string) {
		rawText = s
	}

	var parsedDate time.Time
	e.OnChanged = func(t time.Time) {
		parsedDate = t
	}

	e.SetText("03/05/2025")
	assert.Equal(t, "03/05/2025", rawText, "Entry.OnChanged should receive raw text")
	assert.Equal(t, 2025, parsedDate.Year(), "DateEntry.OnChanged should receive parsed date")
}

func TestDateEntry_OnChanged_NotCalledForInvalid(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	called := false
	e := NewDateEntry()
	e.OnChanged = func(_ time.Time) {
		called = true
	}

	e.SetText("not-valid")
	assert.False(t, called, "OnChanged should not fire for invalid input")
}
