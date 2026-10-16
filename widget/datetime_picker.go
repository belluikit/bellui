package widget

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// DateTimePicker combines DatePicker and TimePicker in a tabbed interface.
//
// Since: 1.0
type DateTimePicker struct {
	BaseWidget
	Selected  time.Time
	OnChanged func(time.Time) `json:"-"`
	Use24Hour bool

	datePicker *DatePicker
	timePicker *TimePicker
}

// AccessibleRole returns [core.RoleGroup]; the embedded date and time
// pickers provide their own semantics.
func (dt *DateTimePicker) AccessibleRole() core.AccessibleRole { return core.RoleGroup }

// AccessibleLabel returns a localized "Select date & time".
func (dt *DateTimePicker) AccessibleLabel() string { return lang.L("Select date & time") }

// AccessibleValue returns the selected date and time in the locale's format.
func (dt *DateTimePicker) AccessibleValue() string {
	if dt.Selected.IsZero() {
		return ""
	}
	return lang.FormatDateTime(dt.Selected)
}

// AccessibleRange is not applicable to date+time pickers; it returns zeros.
func (dt *DateTimePicker) AccessibleRange() (min, max, current float64) { return 0, 0, 0 }

// NewDateTimePicker creates a new combined date+time picker.
//
// Since: 1.0
func NewDateTimePicker(selected time.Time, use24h bool, changed func(time.Time)) *DateTimePicker {
	dt := &DateTimePicker{
		Selected:  selected,
		OnChanged: changed,
		Use24Hour: use24h,
	}

	dt.datePicker = NewDatePicker(selected, func(d time.Time) {
		dt.Selected = time.Date(d.Year(), d.Month(), d.Day(),
			dt.Selected.Hour(), dt.Selected.Minute(), 0, 0, d.Location())
		if dt.OnChanged != nil {
			dt.OnChanged(dt.Selected)
		}
	})

	h, m := selected.Hour(), selected.Minute()
	dt.timePicker = NewTimePicker(h, m, use24h, func(hour, minute int) {
		dt.Selected = time.Date(dt.Selected.Year(), dt.Selected.Month(), dt.Selected.Day(),
			hour, minute, 0, 0, dt.Selected.Location())
		if dt.OnChanged != nil {
			dt.OnChanged(dt.Selected)
		}
	})

	dt.ExtendBaseWidget(dt)
	return dt
}

// MinSize returns the minimum size for the date+time picker.
func (dt *DateTimePicker) MinSize() geo.Size {
	dt.ExtendBaseWidget(dt)
	return dt.BaseWidget.MinSize()
}

// CreateRenderer returns a new renderer for the date+time picker.
func (dt *DateTimePicker) CreateRenderer() core.WidgetRenderer {
	dt.ExtendBaseWidget(dt)

	tabs := NewSegmentedButton([]string{lang.L("Date"), lang.L("Time")}, func(idx int) {
		dt.datePicker.Visible()
		dt.timePicker.Visible()
		if idx == 0 {
			dt.datePicker.Show()
			dt.timePicker.Hide()
		} else {
			dt.datePicker.Hide()
			dt.timePicker.Show()
		}
		dt.Refresh()
	})
	tabs.Selected = 0
	dt.timePicker.Hide()

	return &dateTimePickerRenderer{
		picker:     dt,
		tabs:       tabs,
		datePicker: dt.datePicker,
		timePicker: dt.timePicker,
	}
}

// NewAutoDateTimePicker creates a combined date+time picker that auto-detects
// 12h/24h mode from the current locale.
//
// Since: 1.0
func NewAutoDateTimePicker(selected time.Time, changed func(time.Time)) *DateTimePicker {
	return NewDateTimePicker(selected, lang.Prefers24HourClock(), changed)
}

// ShowAutoDateTimePicker shows a combined date+time picker modal that
// auto-detects 12h/24h mode from the current locale.
//
// Since: 1.0
func ShowAutoDateTimePicker(selected time.Time, changed func(time.Time), c core.Canvas) {
	ShowDateTimePicker(selected, lang.Prefers24HourClock(), changed, c)
}

// ShowDateTimePicker shows the combined picker as a modal dialog.
//
// Since: 1.0
func ShowDateTimePicker(selected time.Time, use24h bool, changed func(time.Time), c core.Canvas) {
	picker := NewDateTimePicker(selected, use24h, nil)

	var popUp *PopUp
	okBtn := &Button{Text: lang.L("OK"), Style: ButtonStyleFilled, OnTapped: func() {
		if changed != nil {
			changed(picker.Selected)
		}
		if popUp != nil {
			popUp.Hide()
		}
	}}
	cancelBtn := &Button{Text: lang.L("Cancel"), Style: ButtonStyleText, OnTapped: func() {
		if popUp != nil {
			popUp.Hide()
		}
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

	popUp = NewModalPopUp(content, c)
	popUp.bgColor = bgColor
	popUp.Show()
}

// --- renderer ---

type dateTimePickerRenderer struct {
	picker     *DateTimePicker
	tabs       *SegmentedButton
	datePicker *DatePicker
	timePicker *TimePicker
}

func (r *dateTimePickerRenderer) MinSize() geo.Size {
	tabsMin := r.tabs.MinSize()
	dateMin := r.datePicker.MinSize()
	timeMin := r.timePicker.MinSize()
	contentW := geo.Max(dateMin.Width, timeMin.Width)
	contentH := geo.Max(dateMin.Height, timeMin.Height)
	return geo.NewSize(
		geo.Max(contentW, tabsMin.Width)+16,
		tabsMin.Height+8+contentH+8,
	)
}

func (r *dateTimePickerRenderer) Layout(size geo.Size) {
	tabsMin := r.tabs.MinSize()
	tabsW := geo.Min(tabsMin.Width, size.Width-16)
	r.tabs.Move(geo.NewPos((size.Width-tabsW)/2, 4))
	r.tabs.Resize(geo.NewSize(tabsW, tabsMin.Height))

	contentY := tabsMin.Height + 8
	contentH := size.Height - contentY - 4
	r.datePicker.Move(geo.NewPos(8, contentY))
	r.datePicker.Resize(geo.NewSize(size.Width-16, contentH))
	r.timePicker.Move(geo.NewPos(8, contentY))
	r.timePicker.Resize(geo.NewSize(size.Width-16, contentH))
}

func (r *dateTimePickerRenderer) Refresh() {
	r.tabs.Refresh()
	r.datePicker.Refresh()
	r.timePicker.Refresh()
	canvas.Refresh(r.picker.super())
}

// Destroy has nothing to release: the child tabs and pickers are CanvasObjects
// returned by Objects and torn down by the framework's destroy walk. The modal
// popup that hosts the picker is caller-scoped (ShowDateTimePicker), not owned here.
func (r *dateTimePickerRenderer) Destroy() {}

func (r *dateTimePickerRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.tabs, r.datePicker, r.timePicker}
}
