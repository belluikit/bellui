package widget

import (
	"fmt"
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// DatePicker is an MD3-compliant date picker with a calendar grid.
//
// Since: 1.0
type DatePicker struct {
	BaseWidget
	Selected  time.Time
	OnChanged func(time.Time) `json:"-"`

	viewing time.Time // month being displayed

	focused bool
	cursor  time.Time // keyboard focus within the day grid (pre-rebrand tracker #534)
}

var _ event.Focusable = (*DatePicker)(nil)

// FocusGained is called when the picker gains keyboard focus; the day
// cursor starts on the selected date (or today) in the viewing month.
func (d *DatePicker) FocusGained() {
	d.focused = true
	if d.cursor.IsZero() {
		d.cursor = d.Selected
		if d.cursor.IsZero() {
			d.cursor = time.Now()
		}
	}
	if d.cursor.Month() != d.viewing.Month() || d.cursor.Year() != d.viewing.Year() {
		d.cursor = time.Date(d.viewing.Year(), d.viewing.Month(), 1, 0, 0, 0, 0, d.viewing.Location())
	}
	d.Refresh()
}

// FocusLost is called when the picker loses keyboard focus.
func (d *DatePicker) FocusLost() {
	d.focused = false
	d.Refresh()
}

// TypedRune is called if a text event happens while the picker is focused.
func (d *DatePicker) TypedRune(rune) {}

// TypedKey navigates the day grid (ARIA grid pattern): arrows move the day
// cursor (crossing a month boundary flips the viewing month), Home/End jump
// to the week edges, PageUp/PageDown move a month, Enter/Space select.
func (d *DatePicker) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeyLeft:
		d.moveCursor(d.cursor.AddDate(0, 0, -1))
	case event.KeyRight:
		d.moveCursor(d.cursor.AddDate(0, 0, 1))
	case event.KeyUp:
		d.moveCursor(d.cursor.AddDate(0, 0, -7))
	case event.KeyDown:
		d.moveCursor(d.cursor.AddDate(0, 0, 7))
	case event.KeyHome:
		d.moveCursor(d.cursor.AddDate(0, 0, -lang.WeekdayOffset(d.cursor)))
	case event.KeyEnd:
		d.moveCursor(d.cursor.AddDate(0, 0, 6-lang.WeekdayOffset(d.cursor)))
	case event.KeyPageUp:
		d.moveCursor(d.cursor.AddDate(0, -1, 0))
	case event.KeyPageDown:
		d.moveCursor(d.cursor.AddDate(0, 1, 0))
	case event.KeyReturn, event.KeyEnter, event.KeySpace:
		if d.cursor.IsZero() {
			return
		}
		d.Selected = time.Date(d.cursor.Year(), d.cursor.Month(), d.cursor.Day(), 0, 0, 0, 0, d.viewing.Location())
		if d.OnChanged != nil {
			d.OnChanged(d.Selected)
		}
		d.Refresh()
	}
}

// moveCursor moves the keyboard day cursor, flipping the viewing month when
// the cursor crosses a month boundary.
func (d *DatePicker) moveCursor(to time.Time) {
	d.cursor = to
	if to.Month() != d.viewing.Month() || to.Year() != d.viewing.Year() {
		d.viewing = to
	}
	d.Refresh()
}

// NewDatePicker creates a new MD3 date picker.
//
// Since: 1.0
func NewDatePicker(selected time.Time, changed func(time.Time)) *DatePicker {
	d := &DatePicker{Selected: selected, OnChanged: changed, viewing: selected}
	if d.viewing.IsZero() {
		d.viewing = time.Now()
	}
	d.ExtendBaseWidget(d)
	return d
}

// AccessibleRole returns [core.RoleGroup]; the month navigation buttons
// inside provide their own semantics.
func (d *DatePicker) AccessibleRole() core.AccessibleRole { return core.RoleGroup }

// AccessibleLabel returns a localized "Select date".
func (d *DatePicker) AccessibleLabel() string { return lang.L("Select date") }

// AccessibleValue returns the selected date in the locale's format.
func (d *DatePicker) AccessibleValue() string {
	if d.Selected.IsZero() {
		return ""
	}
	return lang.FormatDate(d.Selected)
}

// AccessibleRange is not applicable to date pickers; it returns zeros.
func (d *DatePicker) AccessibleRange() (min, max, current float64) { return 0, 0, 0 }

// MinSize returns the minimum size for the date picker.
func (d *DatePicker) MinSize() geo.Size {
	d.ExtendBaseWidget(d)
	return d.BaseWidget.MinSize()
}

// CreateRenderer returns a new renderer for the date picker.
func (d *DatePicker) CreateRenderer() core.WidgetRenderer {
	d.ExtendBaseWidget(d)

	bg := canvas.NewRectangle(color.Transparent)
	monthLabel := canvas.NewText("", color.White)
	monthLabel.TextStyle.Bold = true

	prevBtn := NewIconButton(theme.NavigateBackIcon(), func() {
		d.viewing = d.viewing.AddDate(0, -1, 0)
		d.Refresh()
	})
	prevBtn.SetAccessibilityLabel(lang.L("Previous month"))
	nextBtn := NewIconButton(theme.NavigateNextIcon(), func() {
		d.viewing = d.viewing.AddDate(0, 1, 0)
		d.Refresh()
	})
	nextBtn.SetAccessibilityLabel(lang.L("Next month"))

	// Day of week headers.
	dayHeaders := make([]*canvas.Text, 7)
	days := lang.DayHeaders()
	for i, dayName := range days {
		t := canvas.NewText(dayName, color.White)
		t.Alignment = bellui.TextAlignCenter
		dayHeaders[i] = t
	}

	// Day cells: 6 rows × 7 columns = 42 cells.
	dayCells := make([]*dateDayCell, 42)
	for i := range dayCells {
		dayCells[i] = newDateDayCell(d.selectDay)
	}

	r := &datePickerRenderer{
		bg:         bg,
		monthLabel: monthLabel,
		prevBtn:    prevBtn,
		nextBtn:    nextBtn,
		dayHeaders: dayHeaders,
		dayCells:   dayCells,
		picker:     d,
	}

	// Build objects list.
	var objects []core.CanvasObject
	objects = append(objects, bg, monthLabel, prevBtn, nextBtn)
	for _, h := range dayHeaders {
		objects = append(objects, h)
	}
	for _, c := range dayCells {
		objects = append(objects, c)
	}

	r.BaseRenderer = internalWidget.NewBaseRenderer(objects)
	r.Refresh()
	return r
}

// ShowDatePicker shows the date picker as a modal dialog.
//
// Since: 1.0
func ShowDatePicker(selected time.Time, changed func(time.Time), c core.Canvas) {
	picker := NewDatePicker(selected, nil)

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

// --- internal types ---

// dateDayCell is one day in the calendar grid: a tappable, accessible
// widget so screen readers can reach and activate individual days (pre-rebrand tracker #521).
type dateDayCell struct {
	BaseWidget
	bg    *canvas.Circle
	label *canvas.Text

	day, month, year int
	selected         bool
	focused          bool // the picker's keyboard day cursor is on this cell
	onTap            func(*dateDayCell)
}

func newDateDayCell(onTap func(*dateDayCell)) *dateDayCell {
	c := &dateDayCell{onTap: onTap}
	c.bg = &canvas.Circle{}
	c.label = canvas.NewText("", color.White)
	c.label.Alignment = bellui.TextAlignCenter
	c.ExtendBaseWidget(c)
	return c
}

func (c *dateDayCell) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	return &dateDayCellRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{c.bg, c.label}),
		cell:         c,
	}
}

func (c *dateDayCell) Tapped(*event.PointEvent) {
	if c.day == 0 || c.onTap == nil {
		return
	}
	c.onTap(c)
}

// date returns the cell's day as a time; only meaningful when day != 0.
func (c *dateDayCell) date() time.Time {
	return time.Date(c.year, time.Month(c.month), c.day, 0, 0, 0, 0, time.Local)
}

// AccessibleRole returns [core.RoleButton].
func (c *dateDayCell) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the cell's full date in the locale's format.
func (c *dateDayCell) AccessibleLabel() string {
	if c.day == 0 {
		return ""
	}
	return lang.FormatDate(c.date())
}

// AccessibleState reports whether this day is the selected date.
func (c *dateDayCell) AccessibleState() core.AccessibleState {
	var state core.AccessibleState
	if c.selected {
		state |= core.StateSelected
	}
	if c.focused {
		state |= core.StateFocused
	}
	return state
}

// AccessibleActions returns the "press" action.
func (c *dateDayCell) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Select this day"}}
}

// PerformAccessibleAction selects the day like a tap would.
func (c *dateDayCell) PerformAccessibleAction(name string) {
	if name == core.ActionPress {
		c.Tapped(nil)
	}
}

type dateDayCellRenderer struct {
	internalWidget.BaseRenderer
	cell *dateDayCell
}

func (r *dateDayCellRenderer) MinSize() geo.Size { return geo.Size{} }

func (r *dateDayCellRenderer) Layout(size geo.Size) {
	circleSize := size.Height - 8
	r.cell.bg.Move(geo.NewPos((size.Width-circleSize)/2, (size.Height-circleSize)/2))
	r.cell.bg.Resize(geo.NewSize(circleSize, circleSize))

	labelH := r.cell.label.MinSize().Height
	r.cell.label.Move(geo.NewPos(0, (size.Height-labelH)/2))
	r.cell.label.Resize(geo.NewSize(size.Width, labelH))
}

func (r *dateDayCellRenderer) Refresh() {
	r.cell.bg.Refresh()
	r.cell.label.Refresh()
}

type datePickerRenderer struct {
	internalWidget.BaseRenderer
	bg         *canvas.Rectangle
	monthLabel *canvas.Text
	prevBtn    *IconButton
	nextBtn    *IconButton
	dayHeaders []*canvas.Text
	dayCells   []*dateDayCell
	picker     *DatePicker
}

const (
	dpGridCols = 7
	dpGridRows = 6
)

// sizes returns theme-derived layout dimensions for the date picker.
func (r *datePickerRenderer) sizes() (cellSize, padding float32) {
	th := r.picker.Theme()
	cellSize = th.Size(theme.SizeNameIconButtonSize)
	padding = th.Size(theme.SizeNamePadding) * 4
	return
}

func (r *datePickerRenderer) MinSize() geo.Size {
	cellSize, padding := r.sizes()
	w := dpGridCols*cellSize + padding*2
	h := padding + cellSize + 8 + cellSize + dpGridRows*cellSize + padding
	return geo.NewSize(w, h)
}

func (r *datePickerRenderer) Layout(size geo.Size) {
	cellSize, padding := r.sizes()

	r.bg.Resize(size)

	// Header: prev | month label | next
	headerY := padding
	r.prevBtn.Move(geo.NewPos(padding, headerY))
	r.prevBtn.Resize(geo.NewSize(cellSize, cellSize))

	labelSize := r.monthLabel.MinSize()
	cx := size.Width / 2
	r.monthLabel.Move(geo.NewPos(cx-labelSize.Width/2, headerY+(cellSize-labelSize.Height)/2))

	r.nextBtn.Move(geo.NewPos(size.Width-padding-cellSize, headerY))
	r.nextBtn.Resize(geo.NewSize(cellSize, cellSize))

	// Day headers.
	y := headerY + cellSize + 8
	for i, h := range r.dayHeaders {
		x := padding + float32(i)*cellSize
		h.Move(geo.NewPos(x, y))
		h.Resize(geo.NewSize(cellSize, cellSize*0.6))
	}

	// Day cells.
	y += cellSize * 0.7
	for row := 0; row < dpGridRows; row++ {
		for col := 0; col < dpGridCols; col++ {
			idx := row*dpGridCols + col
			cell := r.dayCells[idx]
			cell.Move(geo.NewPos(padding+float32(col)*cellSize, y+float32(row)*cellSize))
			cell.Resize(geo.NewSize(cellSize, cellSize))
		}
	}
}

func (r *datePickerRenderer) Refresh() {
	th := r.picker.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerHigh, v)
	r.bg.CornerRadius = 28
	r.bg.Refresh()

	viewing := r.picker.viewing
	r.monthLabel.Text = lang.FormatMonthYear(viewing)
	r.monthLabel.TextSize = th.Size(theme.SizeNameHeadlineSmall)
	r.monthLabel.Color = th.Color(theme.ColorNameOnSurface, v)
	r.monthLabel.Refresh()

	// Day headers (refresh text in case locale changed).
	headerTextSize := th.Size(theme.SizeNameBodySmall)
	days := lang.DayHeaders()
	for i, h := range r.dayHeaders {
		h.Text = days[i]
		h.TextSize = headerTextSize
		h.Color = th.Color(theme.ColorNameOnSurfaceVariant, v)
		h.Refresh()
	}

	// Calculate calendar grid.
	firstOfMonth := time.Date(viewing.Year(), viewing.Month(), 1, 0, 0, 0, 0, viewing.Location())
	startDay := lang.WeekdayOffset(firstOfMonth)
	daysInMonth := daysIn(viewing.Month(), viewing.Year())

	today := time.Now()
	sel := r.picker.Selected

	cellTextSize := th.Size(theme.SizeNameBodyMedium)
	for i := 0; i < 42; i++ {
		cell := r.dayCells[i]
		cell.label.TextSize = cellTextSize
		dayNum := i - startDay + 1

		if dayNum < 1 || dayNum > daysInMonth {
			cell.day = 0
			cell.selected = false
			cell.focused = false
			cell.Hide()
			cell.label.Text = ""
			cell.bg.FillColor = color.Transparent
			cell.bg.StrokeWidth = 0
			cell.bg.StrokeColor = color.Transparent
		} else {
			cell.Show()
			cell.label.Text = fmt.Sprintf("%d", dayNum)
			cell.day = dayNum
			cell.month = int(viewing.Month())
			cell.year = viewing.Year()

			isToday := dayNum == today.Day() && viewing.Month() == today.Month() && viewing.Year() == today.Year()
			isSelected := dayNum == sel.Day() && viewing.Month() == sel.Month() && viewing.Year() == sel.Year()
			cursor := r.picker.cursor
			isFocused := r.picker.focused && dayNum == cursor.Day() &&
				viewing.Month() == cursor.Month() && viewing.Year() == cursor.Year()
			cell.selected = isSelected
			cell.focused = isFocused

			if isSelected {
				cell.bg.FillColor = th.Color(theme.ColorNamePrimary, v)
				cell.bg.StrokeWidth = 0
				cell.label.Color = th.Color(theme.ColorNameForegroundOnPrimary, v)
			} else if isToday {
				cell.bg.FillColor = color.Transparent
				cell.bg.StrokeWidth = 2
				cell.bg.StrokeColor = th.Color(theme.ColorNamePrimary, v)
				cell.label.Color = th.Color(theme.ColorNamePrimary, v)
			} else {
				cell.bg.FillColor = color.Transparent
				cell.bg.StrokeWidth = 0
				cell.label.Color = th.Color(theme.ColorNameOnSurface, v)
			}
			// The keyboard cursor draws a focus ring on top of any of the
			// three looks above (pre-rebrand tracker #534).
			if isFocused {
				cell.bg.StrokeWidth = 2
				cell.bg.StrokeColor = th.Color(theme.ColorNameFocus, v)
			}
		}
		cell.bg.Refresh()
		cell.label.Refresh()
	}

	r.Layout(r.picker.Size())
	canvas.Refresh(r.picker.super())
}

// selectDay commits a tapped day cell as the picker's selection.
func (d *DatePicker) selectDay(c *dateDayCell) {
	d.Selected = time.Date(c.year, time.Month(c.month), c.day, 0, 0, 0, 0, d.viewing.Location())
	if d.OnChanged != nil {
		d.OnChanged(d.Selected)
	}
	d.Refresh()
}

func daysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
