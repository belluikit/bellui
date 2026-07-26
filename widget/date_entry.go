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

// DateEntry is an Entry that accepts locale-formatted dates with an optional
// calendar popup triggered by an action button.
//
// Since: 2.7
type DateEntry struct {
	Entry

	// Selected is the currently selected date. Updated when a valid date is
	// typed or chosen from the popup.
	Selected time.Time
	// OnChanged is called when the selected date changes.
	OnChanged func(time.Time) `json:"-"`

	popUp *PopUp
}

// NewDateEntry creates a new DateEntry with a calendar action button and
// locale-aware date validation. Use the DateEntry.OnChanged field (not
// Entry.OnChanged) to receive parsed date values.
//
// Since: 2.7
func NewDateEntry() *DateEntry {
	e := &DateEntry{}
	e.PlaceHolder = lang.DatePatternHint()
	e.ValidateWithBorder = true
	e.Validator = e.validate
	e.ExtendBaseWidget(e)
	return e
}

// CreateRenderer returns a new renderer for this date entry.
func (e *DateEntry) CreateRenderer() core.WidgetRenderer {
	e.ExtendBaseWidget(e)
	if e.ActionItem == nil {
		e.ActionItem = e.setupCalendarButton()
	}
	return e.Entry.CreateRenderer()
}

// Enable enables the entry and its action button.
func (e *DateEntry) Enable() {
	if e.ActionItem != nil {
		e.ActionItem.(core.Disableable).Enable()
	}
	e.Entry.Enable()
}

// Disable disables the entry and its action button.
func (e *DateEntry) Disable() {
	if e.ActionItem != nil {
		e.ActionItem.(core.Disableable).Disable()
	}
	e.Entry.Disable()
}

// Move repositions the entry and any open popup.
func (e *DateEntry) Move(pos geo.Position) {
	e.Entry.Move(pos)
	if e.popUp != nil {
		e.popUp.Move(e.popUpPos())
	}
}

// Resize resizes the entry and repositions any open popup.
func (e *DateEntry) Resize(size geo.Size) {
	e.Entry.Resize(size)
	if e.popUp != nil {
		e.popUp.Move(e.popUpPos())
	}
}

// SetDate sets the entry text to the locale-formatted representation of t.
func (e *DateEntry) SetDate(t time.Time) {
	e.Selected = t
	e.SetText(lang.FormatDate(t))
}

// Date parses the current text and returns the date, or an error if invalid.
func (e *DateEntry) Date() (time.Time, error) {
	if e.Text == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	return lang.ParseDate(e.Text)
}

// MinSize returns the minimum size.
func (e *DateEntry) MinSize() geo.Size {
	e.ExtendBaseWidget(e)
	return e.Entry.MinSize()
}

// TypedRune filters characters that cannot appear in a date string.
func (e *DateEntry) TypedRune(r rune) {
	if unicode.IsDigit(r) || isDateSeparator(r) {
		e.Entry.TypedRune(r)
		return
	}
	// Reject silently.
}

// TypedShortcut intercepts paste to filter invalid characters.
func (e *DateEntry) TypedShortcut(s core.Shortcut) {
	if paste, ok := s.(*bellui.ShortcutPaste); ok {
		content := paste.Clipboard.Content()
		filtered := filterDateChars(content)
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

func (e *DateEntry) validate(s string) error {
	if s == "" {
		return nil
	}
	t, err := lang.ParseDate(s)
	if err != nil {
		return fmt.Errorf("invalid date")
	}
	if !t.Equal(e.Selected) {
		e.Selected = t
		if e.OnChanged != nil {
			e.OnChanged(t)
		}
	}
	return nil
}

func (e *DateEntry) setupCalendarButton() *Button {
	btn := NewButton("", func() {
		c := bellui.CurrentApp().Driver().CanvasForObject(e.super())
		if c == nil {
			return
		}
		e.showCalendarPopup(c)
	})
	btn.Importance = LowImportance
	btn.SetIcon(e.Theme().Icon(theme.IconNameCalendar))
	return btn
}

func (e *DateEntry) popUpPos() geo.Position {
	entryPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super())
	return entryPos.Add(geo.NewPos(0, e.Size().Height))
}

func (e *DateEntry) showCalendarPopup(c core.Canvas) {
	if e.popUp != nil {
		e.popUp.Hide()
		e.popUp = nil
	}

	sel := e.Selected
	if sel.IsZero() {
		sel = time.Now()
	}

	picker := NewDatePicker(sel, nil)

	var popUp *PopUp
	okBtn := &Button{Text: lang.L("OK"), Style: ButtonStyleFilled, OnTapped: func() {
		e.SetDate(picker.Selected)
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

// isDateSeparator returns true for characters commonly used in date formats.
func isDateSeparator(r rune) bool {
	return r == '/' || r == '.' || r == '-' || r == ' '
}

// filterDateChars keeps only digits and date separator characters.
func filterDateChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) || isDateSeparator(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
