package widget

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// DateTimeEntry is an Entry that accepts locale-formatted date+time values
// with an optional picker popup triggered by an action button.
//
// Since: 1.0
type DateTimeEntry struct {
	Entry

	// Selected is the currently selected date and time.
	Selected time.Time
	// OnChanged is called when the selected datetime changes.
	OnChanged func(time.Time) `json:"-"`
	// Use24Hour controls 12h/24h mode for the time picker popup.
	// Set automatically from the locale by NewDateTimeEntry.
	Use24Hour bool

	popUp *PopUp
}

// NewDateTimeEntry creates a new DateTimeEntry with a picker action button.
// The 12h/24h mode is auto-detected from the current locale.
// NewDateTimeEntry creates a new DateTimeEntry with a picker action button.
// The 12h/24h mode is auto-detected from the current locale. Use the
// DateTimeEntry.OnChanged field (not Entry.OnChanged) to receive parsed values.
//
// Since: 1.0
func NewDateTimeEntry() *DateTimeEntry {
	e := &DateTimeEntry{
		Use24Hour: lang.Prefers24HourClock(),
	}
	e.PlaceHolder = lang.DateTimePatternHint()
	e.ValidateWithBorder = true
	e.Validator = e.validate
	e.ExtendBaseWidget(e)
	return e
}

// CreateRenderer returns a new renderer for this datetime entry.
func (e *DateTimeEntry) CreateRenderer() core.WidgetRenderer {
	e.ExtendBaseWidget(e)
	if e.ActionItem == nil {
		e.ActionItem = e.setupPickerButton()
	}
	return e.Entry.CreateRenderer()
}

// Enable enables the entry and its action button.
func (e *DateTimeEntry) Enable() {
	if e.ActionItem != nil {
		e.ActionItem.(core.Disableable).Enable()
	}
	e.Entry.Enable()
}

// Disable disables the entry and its action button.
func (e *DateTimeEntry) Disable() {
	if e.ActionItem != nil {
		e.ActionItem.(core.Disableable).Disable()
	}
	e.Entry.Disable()
}

// Move repositions the entry and any open popup.
func (e *DateTimeEntry) Move(pos geo.Position) {
	e.Entry.Move(pos)
	if e.popUp != nil {
		e.popUp.Move(e.popUpPos())
	}
}

// Resize resizes the entry and repositions any open popup.
func (e *DateTimeEntry) Resize(size geo.Size) {
	e.Entry.Resize(size)
	if e.popUp != nil {
		e.popUp.Move(e.popUpPos())
	}
}

// SetDateTime sets the entry text to the locale-formatted representation of t.
func (e *DateTimeEntry) SetDateTime(t time.Time) {
	e.Selected = t
	e.SetText(lang.FormatDateTime(t))
}

// DateTime parses the current text and returns the datetime, or an error.
func (e *DateTimeEntry) DateTime() (time.Time, error) {
	if e.Text == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	return lang.ParseDateTime(e.Text)
}

// MinSize returns the minimum size.
func (e *DateTimeEntry) MinSize() geo.Size {
	e.ExtendBaseWidget(e)
	return e.Entry.MinSize()
}

// TypedRune filters characters that cannot appear in a datetime string.
// Lowercase AM/PM letters are accepted and uppercased during validation.
func (e *DateTimeEntry) TypedRune(r rune) {
	if unicode.IsDigit(r) || isDateTimeSeparator(r) {
		e.Entry.TypedRune(r)
		return
	}
	// Accept AM/PM letters — uppercase them for Go's time.Parse.
	if r == 'A' || r == 'P' || r == 'M' || r == 'a' || r == 'p' || r == 'm' {
		e.Entry.TypedRune(unicode.ToUpper(r))
		return
	}
}

// TypedShortcut intercepts paste to filter invalid characters.
func (e *DateTimeEntry) TypedShortcut(s core.Shortcut) {
	if paste, ok := s.(*bellui.ShortcutPaste); ok {
		content := paste.Clipboard.Content()
		filtered := filterDateTimeChars(content)
		if filtered == "" {
			return
		}
		paste.Clipboard.SetContent(filtered)
		e.Entry.TypedShortcut(paste)
		paste.Clipboard.SetContent(content)
		return
	}
	e.Entry.TypedShortcut(s)
}

func (e *DateTimeEntry) validate(s string) error {
	if s == "" {
		return nil
	}
	t, err := lang.ParseDateTime(s)
	if err != nil {
		return fmt.Errorf("invalid date/time")
	}
	if !t.Equal(e.Selected) {
		e.Selected = t
		if e.OnChanged != nil {
			e.OnChanged(t)
		}
	}
	return nil
}

func (e *DateTimeEntry) setupPickerButton() *Button {
	btn := NewButton("", func() {
		c := bellui.CurrentApp().Driver().CanvasForObject(e.super())
		if c == nil {
			return
		}
		e.showPickerPopup(c)
	})
	btn.Importance = LowImportance
	btn.SetIcon(e.Theme().Icon(theme.IconNameCalendar))
	return btn
}

func (e *DateTimeEntry) popUpPos() geo.Position {
	entryPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super())
	return entryPos.Add(geo.NewPos(0, e.Size().Height))
}

func (e *DateTimeEntry) showPickerPopup(c core.Canvas) {
	if e.popUp != nil {
		e.popUp.Hide()
		e.popUp = nil
	}

	sel := e.Selected
	if sel.IsZero() {
		sel = time.Now()
	}

	picker := NewDateTimePicker(sel, e.Use24Hour, nil)

	var popUp *PopUp
	okBtn := &Button{Text: lang.L("OK"), Style: ButtonStyleFilled, OnTapped: func() {
		e.SetDateTime(picker.Selected)
		if popUp != nil {
			popUp.Hide()
		}
		e.popUp = nil
	}}
	cancelBtn := &Button{Text: lang.L("Cancel"), Style: ButtonStyleText, OnTapped: func() {
		if popUp != nil {
			popUp.Hide()
		}
		e.popUp = nil
	}}

	buttons := &bellui.Container{
		Layout:  layout.NewHBoxLayout(),
		Objects: []core.CanvasObject{layout.NewSpacer(), cancelBtn, okBtn},
	}
	content := &bellui.Container{
		Layout:  layout.NewVBoxLayout(),
		Objects: []core.CanvasObject{picker, buttons},
	}

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bgColor := th.Color(theme.ColorNameSurfaceContainerHigh, v)

	popUp = NewPopUp(content, c)
	popUp.bgColor = bgColor
	popUp.ShowAtPosition(e.popUpPos())
	e.popUp = popUp
}

// isDateTimeSeparator returns true for characters used in date/time formats.
func isDateTimeSeparator(r rune) bool {
	return r == '/' || r == '.' || r == '-' || r == ':' || r == ' '
}

// filterDateTimeChars keeps digits, date/time separators, and AM/PM letters
// (uppercased for compatibility with Go's time.Parse).
func filterDateTimeChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) || isDateTimeSeparator(r) {
			b.WriteRune(r)
		} else if r == 'A' || r == 'P' || r == 'M' || r == 'a' || r == 'p' || r == 'm' {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}
