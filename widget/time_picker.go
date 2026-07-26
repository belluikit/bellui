package widget

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// TimePicker is an MD3-compliant time picker with a clock dial.
//
// Since: 2.7
type TimePicker struct {
	BaseWidget
	Hour, Minute    int
	Use24Hour       bool
	OnChanged       func(hour, minute int) `json:"-"`
	selectingMinute bool

	focused    bool
	focusField int // keyboard cursor: which header field is focused (#543)

	// Direct digit-entry buffer (#549): the pending tens digit of a two-digit
	// value being typed into the focused hour/minute field. hasPartial gates
	// it so a buffered 0 is distinguishable from "no partial".
	partial    int
	hasPartial bool
}

// Keyboard focus fields, in left-to-right header order.
const (
	tpFieldHour = iota
	tpFieldMinute
	tpFieldAM
	tpFieldPM
)

var _ event.Focusable = (*TimePicker)(nil)

// FocusGained is called when the picker gains keyboard focus; the field
// cursor starts on whichever of hour/minute the dial is selecting.
func (t *TimePicker) FocusGained() {
	t.focused = true
	if t.selectingMinute {
		t.focusField = tpFieldMinute
	} else {
		t.focusField = tpFieldHour
	}
	t.Refresh()
}

// FocusLost is called when the picker loses keyboard focus.
func (t *TimePicker) FocusLost() {
	t.focused = false
	t.Refresh()
}

// TypedRune drives direct digit entry (#549): typed digits accumulate into the
// focused hour or minute field. A first digit that could still be the tens place
// of a valid two-digit value (hour "1"/"2" in 24h, "0"/"1" in 12h; minute "0".."5")
// is buffered to await a second digit; any other digit commits immediately as a
// leading-zero value and auto-advances (hour to minute). A second digit forms the
// two-digit number, clamps to the valid range and commits. In 12h mode, "A"/"P"
// (case-insensitive) set the meridiem. The advance and commit are deterministic
// (no wall-clock timeout), so behaviour is fully reproducible.
func (t *TimePicker) TypedRune(r rune) {
	if !t.Use24Hour {
		switch r {
		case 'a', 'A':
			t.clearPartial()
			t.setMeridiem(false)
			return
		case 'p', 'P':
			t.clearPartial()
			t.setMeridiem(true)
			return
		}
	}
	if r < '0' || r > '9' {
		return
	}
	d := int(r - '0')
	switch t.focusField {
	case tpFieldHour:
		t.typeHourDigit(d)
	case tpFieldMinute:
		t.typeMinuteDigit(d)
	}
}

// typeHourDigit feeds one digit into the hour field.
func (t *TimePicker) typeHourDigit(d int) {
	if t.hasPartial {
		t.commitHour(t.partial*10 + d)
		return
	}
	maxTens := 1 // 12h hours 01-12 have tens digit 0 or 1.
	if t.Use24Hour {
		maxTens = 2 // 24h hours 00-23 have tens digit 0, 1 or 2.
	}
	if d <= maxTens {
		// Could still be the tens digit; wait for a second digit.
		t.partial = d
		t.hasPartial = true
		t.Refresh()
		return
	}
	t.commitHour(d)
}

// commitHour clamps and stores a fully-typed hour value, then auto-advances to
// the minute field.
func (t *TimePicker) commitHour(v int) {
	if t.Use24Hour {
		if v > 23 {
			v = 23
		}
		t.Hour = v
	} else {
		t.setHour12(v)
	}
	t.clearPartial()
	t.fireChanged()
	// Auto-advance to the minute field so "1437" flows straight through.
	t.focusField = tpFieldMinute
	t.selectingMinute = true
	t.Refresh()
}

// setHour12 stores a 1-12 clock hour into the 24h-based Hour field, preserving
// the current AM/PM meridiem.
func (t *TimePicker) setHour12(h12 int) {
	if h12 < 1 {
		h12 = 1
	}
	if h12 > 12 {
		h12 = 12
	}
	pm := t.Hour >= 12
	h := h12 % 12 // 12 o'clock maps to 0.
	if pm {
		h += 12
	}
	t.Hour = h
}

// typeMinuteDigit feeds one digit into the minute field.
func (t *TimePicker) typeMinuteDigit(d int) {
	if t.hasPartial {
		v := t.partial*10 + d
		if v > 59 {
			v = 59
		}
		t.Minute = v
		t.clearPartial()
		t.fireChanged()
		t.Refresh()
		return
	}
	if d <= 5 {
		// Could still be the tens digit (00-59); wait for a second digit.
		t.partial = d
		t.hasPartial = true
		t.Refresh()
		return
	}
	// 6-9 cannot be a tens digit; commit immediately as 0d.
	t.Minute = d
	t.clearPartial()
	t.fireChanged()
	t.Refresh()
}

// clearPartial discards any pending tens digit.
func (t *TimePicker) clearPartial() {
	t.partial = 0
	t.hasPartial = false
}

// TypedKey drives the picker from the keyboard (#543): Left/Right move
// between the hour, minute and (12h mode) AM/PM fields, Up/Down adjust the
// focused field — hour and minute wrap, AM/PM toggles — and Enter/Space
// activates it like a tap (switching the dial or setting the meridiem).
func (t *TimePicker) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeyLeft:
		t.moveFocusField(-1)
	case event.KeyRight:
		t.moveFocusField(1)
	case event.KeyUp:
		t.adjustFocusField(1)
	case event.KeyDown:
		t.adjustFocusField(-1)
	case event.KeyReturn, event.KeyEnter, event.KeySpace:
		switch t.focusField {
		case tpFieldHour:
			t.selectingMinute = false
			t.Refresh()
		case tpFieldMinute:
			t.selectingMinute = true
			t.Refresh()
		case tpFieldAM:
			t.setMeridiem(false)
		case tpFieldPM:
			t.setMeridiem(true)
		}
	case event.KeyEscape:
		// Discard a partial digit entry (#549); the committed value and the
		// dial that follows it are left untouched.
		t.clearPartial()
		t.Refresh()
	}
}

func (t *TimePicker) moveFocusField(dir int) {
	t.clearPartial() // moving the cursor discards a pending digit (#549)
	last := tpFieldMinute
	if !t.Use24Hour {
		last = tpFieldPM
	}
	f := t.focusField + dir
	if f < tpFieldHour {
		f = tpFieldHour
	}
	if f > last {
		f = last
	}
	t.focusField = f
	// Keep the dial on the field the cursor is over, so Up/Down feedback
	// is visible on the clock face too.
	switch f {
	case tpFieldHour:
		t.selectingMinute = false
	case tpFieldMinute:
		t.selectingMinute = true
	}
	t.Refresh()
}

func (t *TimePicker) adjustFocusField(delta int) {
	switch t.focusField {
	case tpFieldHour:
		t.Hour = (t.Hour + delta + 24) % 24
	case tpFieldMinute:
		t.Minute = (t.Minute + delta + 60) % 60
	case tpFieldAM, tpFieldPM:
		t.setMeridiem(t.Hour < 12) // toggle
		return
	}
	t.Refresh()
	t.fireChanged()
}

// setMeridiem forces the hour into AM (pm false) or PM (pm true).
func (t *TimePicker) setMeridiem(pm bool) {
	switch {
	case pm && t.Hour < 12:
		t.Hour += 12
	case !pm && t.Hour >= 12:
		t.Hour -= 12
	default:
		t.Refresh()
		return
	}
	t.Refresh()
	t.fireChanged()
}

// NewTimePicker creates a new MD3 time picker.
//
// Since: 2.7
func NewTimePicker(hour, minute int, use24h bool, changed func(hour, minute int)) *TimePicker {
	t := &TimePicker{Hour: hour, Minute: minute, Use24Hour: use24h, OnChanged: changed}
	t.ExtendBaseWidget(t)
	return t
}

// AccessibleRole returns [core.RoleGroup]; the hour/minute controls inside
// provide their own semantics.
func (t *TimePicker) AccessibleRole() core.AccessibleRole { return core.RoleGroup }

// AccessibleLabel returns a localized "Select time".
func (t *TimePicker) AccessibleLabel() string { return lang.L("Select time") }

// AccessibleValue returns the chosen time in the locale's format.
func (t *TimePicker) AccessibleValue() string {
	return lang.FormatTime(t.Hour, t.Minute)
}

// AccessibleRange is not applicable to time pickers; it returns zeros.
func (t *TimePicker) AccessibleRange() (min, max, current float64) { return 0, 0, 0 }

// MinSize returns the minimum size for the time picker.
func (t *TimePicker) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

// CreateRenderer returns a new renderer for the time picker.
func (t *TimePicker) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)

	bg := canvas.NewRectangle(color.Transparent)
	titleLabel := canvas.NewText(lang.L("Select time"), color.White)

	// Hour/minute display buttons (#543): real widgets, so they are
	// accessible nodes and receive their own taps.
	hourBtn := newTimePickerButton(core.RoleTab, lang.L("Hour"), func() {
		t.selectingMinute = false
		t.focusField = tpFieldHour
		t.Refresh()
	})
	// Screen-reader adjust (TalkBack / VoiceOver swipe up-down) on the hour
	// field, mirroring the keyboard Up/Down path (#796). The hour wraps through
	// the full 0–23 range in both 12h and 24h modes, matching adjustFocusField.
	hourBtn.onAdjust = func(delta int) {
		t.Hour = (t.Hour + delta + 24) % 24
		t.Refresh()
		t.fireChanged()
	}
	minuteBtn := newTimePickerButton(core.RoleTab, lang.L("Minute"), func() {
		t.selectingMinute = true
		t.focusField = tpFieldMinute
		t.Refresh()
	})
	minuteBtn.onAdjust = func(delta int) {
		t.Minute = (t.Minute + delta + 60) % 60
		t.Refresh()
		t.fireChanged()
	}

	colonText := canvas.NewText(":", color.White)
	colonText.TextStyle.Bold = true

	// AM/PM buttons (only for 12-hour mode).
	amBtn := newTimePickerButton(core.RoleRadioButton, lang.L("AM"), func() {
		t.focusField = tpFieldAM
		t.setMeridiem(false)
	})
	amBtn.label.Text = lang.L("AM")
	pmBtn := newTimePickerButton(core.RoleRadioButton, lang.L("PM"), func() {
		t.focusField = tpFieldPM
		t.setMeridiem(true)
	})
	pmBtn.label.Text = lang.L("PM")
	ampmBorder := canvas.NewRectangle(color.Transparent)

	// Clock dial.
	clockFace := &canvas.Circle{}
	selectorArm := canvas.NewLine(color.White)
	selectorArm.StrokeWidth = 2
	selectorDot := &canvas.Circle{}
	centerDot := &canvas.Circle{}

	// Number labels on the clock. 24 slots so the 24-hour dial can render two
	// concentric rings (outer 1–12, inner 00 and 13–23, #804); the 12h and
	// minute dials use only the first 12 and hide the rest.
	clockLabels := make([]*canvas.Text, 24)
	for i := range clockLabels {
		l := canvas.NewText("", color.White)
		l.Alignment = bellui.TextAlignCenter
		clockLabels[i] = l
	}

	// Selected number background circle.
	selectedBg := &canvas.Circle{}

	var objects []core.CanvasObject
	objects = append(objects, bg, titleLabel, hourBtn, colonText, minuteBtn)
	objects = append(objects, ampmBorder, amBtn, pmBtn)
	objects = append(objects, clockFace, selectorArm, centerDot, selectedBg, selectorDot)
	for _, l := range clockLabels {
		objects = append(objects, l)
	}

	r := &timePickerRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		titleLabel:   titleLabel,
		colonText:    colonText,
		hourBtn:      hourBtn,
		minuteBtn:    minuteBtn,
		amBtn:        amBtn,
		pmBtn:        pmBtn,
		ampmBorder:   ampmBorder,
		clockFace:    clockFace,
		selectorArm:  selectorArm,
		selectorDot:  selectorDot,
		centerDot:    centerDot,
		clockLabels:  clockLabels,
		selectedBg:   selectedBg,
		picker:       t,
	}
	r.Refresh()
	return r
}

// Tapped handles taps on the clock face; the header fields (hour, minute,
// AM/PM) are widgets that receive their own taps (#543).
func (t *TimePicker) Tapped(e *event.PointEvent) {
	if e == nil {
		return
	}

	tr := t.getRenderer()
	if tr == nil {
		return
	}

	t.handleClockInteraction(e.Position, tr, true)
}

// Dragged handles drag events on the clock dial.
func (t *TimePicker) Dragged(e *event.DragEvent) {
	if e == nil {
		return
	}
	tr := t.getRenderer()
	if tr == nil {
		return
	}
	t.handleClockInteraction(e.Position, tr, false)
}

// DragEnd is called when the drag finishes.
func (t *TimePicker) DragEnd() {}

func (t *TimePicker) getRenderer() *timePickerRenderer {
	r := cache.Renderer(t.super())
	if r == nil {
		return nil
	}
	tr, _ := r.(*timePickerRenderer)
	return tr
}

// handleClockInteraction processes a position on the clock dial.
// autoAdvance controls whether to switch from hour→minute mode after selection.
func (t *TimePicker) handleClockInteraction(pos geo.Position, tr *timePickerRenderer, autoAdvance bool) {
	cx := tr.clockCenterX
	cy := tr.clockCenterY
	radius := tr.clockRadius

	dx := pos.X - cx
	dy := pos.Y - cy
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if dist > radius+30 || dist < 10 {
		return // outside clock face or too close to center
	}

	// Calculate angle: 0 = 12 o'clock, clockwise.
	angle := math.Atan2(float64(dx), float64(-dy))
	if angle < 0 {
		angle += 2 * math.Pi
	}

	if t.selectingMinute {
		minute := int(math.Round(angle/(2*math.Pi)*60)) % 60
		t.Minute = minute
		t.fireChanged()
	} else {
		hourVal := int(math.Round(angle/(2*math.Pi)*12)) % 12
		if hourVal == 0 {
			hourVal = 12
		}
		if t.Use24Hour {
			if dist < radius*0.65 {
				if hourVal == 12 {
					hourVal = 0
				} else {
					hourVal += 12
				}
			}
		} else {
			isPM := t.Hour >= 12
			if isPM {
				if hourVal != 12 {
					hourVal += 12
				}
			} else {
				if hourVal == 12 {
					hourVal = 0
				}
			}
		}
		t.Hour = hourVal
		if autoAdvance {
			t.selectingMinute = true
		}
		t.fireChanged()
	}

	t.Refresh()
}

// selected24HourLabelIndex maps an hour (0–23) to its clock-label slot in the
// two-ring 24h dial: the outer ring (hours 1–12) uses indices 0–11, and the
// inner ring (13–23, then 00) uses indices 12–23 (00 sits at the top, index 23).
func selected24HourLabelIndex(hour int) int {
	if hour >= 1 && hour <= 12 {
		return hour - 1 // outer ring
	}
	if hour == 0 {
		return 23 // 00 at the 12-o'clock inner position
	}
	return hour - 1 // 13→12 … 23→22
}

func (t *TimePicker) fireChanged() {
	if t.OnChanged != nil {
		t.OnChanged(t.Hour, t.Minute)
	}
}

// NewAutoTimePicker creates a time picker that auto-detects 12h/24h mode
// from the current locale.
//
// Since: 2.7
func NewAutoTimePicker(hour, minute int, changed func(hour, minute int)) *TimePicker {
	return NewTimePicker(hour, minute, lang.Prefers24HourClock(), changed)
}

// ShowAutoTimePicker shows a time picker modal that auto-detects 12h/24h mode.
//
// Since: 2.7
func ShowAutoTimePicker(hour, minute int, changed func(hour, minute int), c core.Canvas) {
	ShowTimePicker(hour, minute, lang.Prefers24HourClock(), changed, c)
}

// ShowTimePicker shows the time picker as a modal dialog.
//
// Since: 2.7
func ShowTimePicker(hour, minute int, use24h bool, changed func(hour, minute int), c core.Canvas) {
	picker := NewTimePicker(hour, minute, use24h, nil)

	var popUp *PopUp
	okBtn := &Button{Text: lang.L("OK"), Style: ButtonStyleFilled, OnTapped: func() {
		if changed != nil {
			changed(picker.Hour, picker.Minute)
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

// --- internal header buttons ---

// timePickerButton is one tap target in the time picker header — the hour
// and minute displays and the AM/PM toggles. Promoting them to widgets
// makes each an accessible semantic node and a real hit-test target
// instead of picker-level coordinate math (#543).
type timePickerButton struct {
	BaseWidget
	bg    *canvas.Rectangle
	label *canvas.Text

	role     core.AccessibleRole
	name     string
	selected bool
	focused  bool // the picker's keyboard field cursor is on this button
	onTap    func()
	onAdjust func(delta int) // set for value fields (hour/minute); enables a11y increment/decrement
}

func newTimePickerButton(role core.AccessibleRole, name string, onTap func()) *timePickerButton {
	b := &timePickerButton{role: role, name: name, onTap: onTap}
	b.bg = canvas.NewRectangle(color.Transparent)
	b.label = canvas.NewText("", color.White)
	b.label.Alignment = bellui.TextAlignCenter
	b.label.TextStyle.Bold = true
	b.ExtendBaseWidget(b)
	return b
}

func (b *timePickerButton) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	return &timePickerButtonRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{b.bg, b.label}),
		btn:          b,
	}
}

func (b *timePickerButton) Tapped(*event.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

// AccessibleRole returns the role given at construction: [core.RoleTab] for
// the hour/minute pair (one of two dial modes), [core.RoleRadioButton] for
// AM/PM (one-of-two meridiem).
func (b *timePickerButton) AccessibleRole() core.AccessibleRole { return b.role }

// AccessibleLabel returns the localized field name plus the displayed value.
func (b *timePickerButton) AccessibleLabel() string {
	if b.label.Text == "" || b.name == b.label.Text {
		return b.name
	}
	return b.name + " " + b.label.Text
}

// AccessibleState reports selection (as checked for the AM/PM radio pair)
// and the picker's keyboard focus.
func (b *timePickerButton) AccessibleState() core.AccessibleState {
	var state core.AccessibleState
	if b.selected {
		if b.role == core.RoleRadioButton {
			state |= core.StateChecked
		} else {
			state |= core.StateSelected
		}
	}
	if b.focused {
		state |= core.StateFocused
	}
	return state
}

// AccessibleActions returns the "select" action, plus "increment"/"decrement"
// for value fields (hour/minute) so a screen reader can adjust the time without
// the clock dial or a physical keyboard (#796).
func (b *timePickerButton) AccessibleActions() []core.AccessibleAction {
	actions := []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this field"}}
	if b.onAdjust != nil {
		actions = append(
			actions,
			core.AccessibleAction{Name: core.ActionIncrement, Description: "Increase the value"},
			core.AccessibleAction{Name: core.ActionDecrement, Description: "Decrease the value"},
		)
	}
	return actions
}

// PerformAccessibleAction activates the button like a tap would, or adjusts the
// field value for the increment/decrement actions.
func (b *timePickerButton) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionSelect:
		b.Tapped(nil)
	case core.ActionIncrement:
		if b.onAdjust != nil {
			b.onAdjust(1)
		}
	case core.ActionDecrement:
		if b.onAdjust != nil {
			b.onAdjust(-1)
		}
	}
}

type timePickerButtonRenderer struct {
	internalWidget.BaseRenderer
	btn *timePickerButton
}

func (r *timePickerButtonRenderer) MinSize() geo.Size { return geo.Size{} }

func (r *timePickerButtonRenderer) Layout(size geo.Size) {
	r.btn.bg.Resize(size)
	labelH := r.btn.label.MinSize().Height
	r.btn.label.Move(geo.NewPos(0, (size.Height-labelH)/2))
	r.btn.label.Resize(geo.NewSize(size.Width, labelH))
}

func (r *timePickerButtonRenderer) Refresh() {
	r.btn.bg.Refresh()
	r.btn.label.Refresh()
	r.Layout(r.btn.Size())
}

// --- renderer ---

// sizes returns theme-derived layout dimensions for the time picker.
func (r *timePickerRenderer) sizes() (dialSize, padding, headerH, timeBoxW, timeBoxH, ampmW, ampmH, bubbleSize float32) {
	th := r.picker.Theme()
	bubbleSize = th.Size(theme.SizeNameIconButtonSize)
	dialSize = bubbleSize * 6.4
	padding = th.Size(theme.SizeNamePadding) * 4
	timeBoxW = bubbleSize * 1.4
	timeBoxH = bubbleSize * 1.8
	ampmW = bubbleSize * 1.3
	ampmH = bubbleSize * 0.9
	// Header must span: title (20px offset) + time boxes + gap,
	// regardless of how small padding is (e.g. dense theme).
	titleOffset := float32(20)
	headerH = titleOffset + timeBoxH + 8
	return
}

type timePickerRenderer struct {
	internalWidget.BaseRenderer
	bg          *canvas.Rectangle
	titleLabel  *canvas.Text
	colonText   *canvas.Text
	hourBtn     *timePickerButton
	minuteBtn   *timePickerButton
	amBtn       *timePickerButton
	pmBtn       *timePickerButton
	ampmBorder  *canvas.Rectangle
	clockFace   *canvas.Circle
	selectorArm *canvas.Line
	selectorDot *canvas.Circle
	centerDot   *canvas.Circle
	clockLabels []*canvas.Text
	selectedBg  *canvas.Circle
	picker      *TimePicker

	// Cached layout values for hit testing.
	clockCenterX float32
	clockCenterY float32
	clockRadius  float32
}

func (r *timePickerRenderer) MinSize() geo.Size {
	dialSize, padding, headerH, _, _, ampmW, _, _ := r.sizes()
	w := dialSize + padding*2
	if !r.picker.Use24Hour {
		w += ampmW + 8 // space for AM/PM
	}
	h := padding + headerH + dialSize + padding
	return geo.NewSize(w, h)
}

func (r *timePickerRenderer) Layout(size geo.Size) {
	dialSize, padding, headerH, timeBoxW, timeBoxH, ampmW, ampmH, _ := r.sizes()

	r.bg.Resize(size)

	// Title.
	r.titleLabel.Move(geo.NewPos(padding, padding))

	// Time display buttons.
	timeY := padding + 20
	hx := padding
	r.hourBtn.Move(geo.NewPos(hx, timeY))
	r.hourBtn.Resize(geo.NewSize(timeBoxW, timeBoxH))

	colonX := hx + timeBoxW + 2
	r.colonText.Move(geo.NewPos(colonX, timeY+8))

	mx := colonX + 16
	r.minuteBtn.Move(geo.NewPos(mx, timeY))
	r.minuteBtn.Resize(geo.NewSize(timeBoxW, timeBoxH))

	// AM/PM — vertically centered alongside the time buttons.
	if !r.picker.Use24Hour {
		ampmX := mx + timeBoxW + 12
		ampmTotalH := ampmH * 2
		ampmY := timeY + (timeBoxH-ampmTotalH)/2
		r.ampmBorder.Move(geo.NewPos(ampmX, ampmY))
		r.ampmBorder.Resize(geo.NewSize(ampmW, ampmTotalH))
		r.amBtn.Move(geo.NewPos(ampmX, ampmY))
		r.amBtn.Resize(geo.NewSize(ampmW, ampmH))
		r.pmBtn.Move(geo.NewPos(ampmX, ampmY+ampmH))
		r.pmBtn.Resize(geo.NewSize(ampmW, ampmH))
		r.ampmBorder.Show()
		r.amBtn.Show()
		r.pmBtn.Show()
	} else {
		r.ampmBorder.Hide()
		r.amBtn.Hide()
		r.pmBtn.Hide()
	}

	// Clock dial.
	dialY := padding + headerH
	dialX := (size.Width - dialSize) / 2
	r.clockFace.Move(geo.NewPos(dialX, dialY))
	r.clockFace.Resize(geo.NewSize(dialSize, dialSize))

	cx := dialX + dialSize/2
	cy := dialY + dialSize/2
	r.clockCenterX = cx
	r.clockCenterY = cy

	radius := dialSize/2 - 20
	r.clockRadius = radius

	// Center dot.
	r.centerDot.Move(geo.NewPos(cx-4, cy-4))
	r.centerDot.Resize(geo.NewSize(8, 8))

	// Layout clock labels and selector.
	r.layoutClockNumbers(cx, cy, radius)
}

func (r *timePickerRenderer) placeClockLabel(i int, text string, nx, ny, bubbleSize float32) {
	r.clockLabels[i].Text = text
	// Use full bubble width for horizontal centering (TextAlignCenter handles it).
	// Vertically center the text's actual height at the angular position.
	textH := r.clockLabels[i].MinSize().Height
	r.clockLabels[i].Move(geo.NewPos(nx-bubbleSize/2, ny-textH/2))
	r.clockLabels[i].Resize(geo.NewSize(bubbleSize, textH))
}

func (r *timePickerRenderer) layoutClockNumbers(cx, cy, radius float32) {
	_, _, _, _, _, _, _, bubbleSize := r.sizes()
	picker := r.picker

	innerRadius := radius * 0.6

	var selectedAngle float64
	var selectedRadius float32
	active := 12 // number of clock labels used by the current mode

	if picker.selectingMinute {
		for i := 0; i < 12; i++ {
			angle := float64(i) / 12.0 * 2 * math.Pi
			nx := cx + radius*float32(math.Sin(angle))
			ny := cy - radius*float32(math.Cos(angle))
			r.placeClockLabel(i, fmt.Sprintf("%02d", i*5), nx, ny, bubbleSize)
		}
		selectedAngle = float64(picker.Minute) / 60.0 * 2 * math.Pi
		selectedRadius = radius
	} else if picker.Use24Hour {
		// MD3 24h dial: two concentric rings. Outer ring 1–12 at full radius
		// (label indices 0–11); inner ring 13–23 and 00 at 0.6× radius (label
		// indices 12–23), placed at o'clock positions 1..12 with 00 at the top.
		active = 24
		for i := 0; i < 12; i++ {
			angle := float64(i+1) / 12.0 * 2 * math.Pi
			nx := cx + radius*float32(math.Sin(angle))
			ny := cy - radius*float32(math.Cos(angle))
			r.placeClockLabel(i, fmt.Sprintf("%d", i+1), nx, ny, bubbleSize)
		}
		for p := 1; p <= 12; p++ {
			hourVal := p + 12
			if hourVal == 24 {
				hourVal = 0
			}
			angle := float64(p) / 12.0 * 2 * math.Pi
			nx := cx + innerRadius*float32(math.Sin(angle))
			ny := cy - innerRadius*float32(math.Cos(angle))
			r.placeClockLabel(11+p, fmt.Sprintf("%02d", hourVal), nx, ny, bubbleSize)
		}
		h := picker.Hour % 12
		if h == 0 {
			h = 12
		}
		selectedAngle = float64(h) / 12.0 * 2 * math.Pi
		if picker.Hour == 0 || picker.Hour > 12 {
			selectedRadius = innerRadius
		} else {
			selectedRadius = radius
		}
	} else {
		for i := 0; i < 12; i++ {
			hourVal := i + 1
			angle := float64(hourVal) / 12.0 * 2 * math.Pi
			nx := cx + radius*float32(math.Sin(angle))
			ny := cy - radius*float32(math.Cos(angle))
			r.placeClockLabel(i, fmt.Sprintf("%d", hourVal), nx, ny, bubbleSize)
		}
		h := picker.Hour % 12
		if h == 0 {
			h = 12
		}
		selectedAngle = float64(h) / 12.0 * 2 * math.Pi
		selectedRadius = radius
	}

	// Show the labels this mode uses; hide the rest (the inner 24h ring).
	for i := 0; i < active; i++ {
		r.clockLabels[i].Show()
	}
	for i := active; i < len(r.clockLabels); i++ {
		r.clockLabels[i].Text = ""
		r.clockLabels[i].Hide()
	}

	// Selector arm + dot + selected bg — all centered at the angular position.
	selX := cx + selectedRadius*float32(math.Sin(selectedAngle))
	selY := cy - selectedRadius*float32(math.Cos(selectedAngle))
	r.selectorArm.Position1 = geo.NewPos(cx, cy)
	r.selectorArm.Position2 = geo.NewPos(selX, selY)

	r.selectorDot.Move(geo.NewPos(selX-bubbleSize/2, selY-bubbleSize/2))
	r.selectorDot.Resize(geo.NewSize(bubbleSize, bubbleSize))

	r.selectedBg.Move(geo.NewPos(selX-bubbleSize/2, selY-bubbleSize/2))
	r.selectedBg.Resize(geo.NewSize(bubbleSize, bubbleSize))
}

func (r *timePickerRenderer) Refresh() {
	th := r.picker.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerHigh, v)
	r.bg.CornerRadius = 28
	r.bg.Refresh()

	// Update text sizes from theme.
	displaySmall := th.Size(theme.SizeNameDisplaySmall)
	bodyMedium := th.Size(theme.SizeNameBodyMedium)
	bodySmall := th.Size(theme.SizeNameBodySmall)

	r.titleLabel.Text = lang.L("Select time")
	r.titleLabel.TextSize = bodySmall
	r.titleLabel.Color = th.Color(theme.ColorNameOnSurfaceVariant, v)
	r.titleLabel.Refresh()

	r.hourBtn.label.TextSize = displaySmall
	r.colonText.TextSize = displaySmall
	r.minuteBtn.label.TextSize = displaySmall

	r.amBtn.label.TextSize = bodyMedium
	r.pmBtn.label.TextSize = bodyMedium

	for _, l := range r.clockLabels {
		l.TextSize = bodyMedium
	}

	r.colonText.Color = th.Color(theme.ColorNameOnSurface, v)
	r.colonText.Refresh()

	primary := th.Color(theme.ColorNamePrimary, v)
	onPrimary := th.Color(theme.ColorNameForegroundOnPrimary, v)
	onSurface := th.Color(theme.ColorNameOnSurface, v)
	surfContHigh := th.Color(theme.ColorNameSurfaceContainerHighest, v)
	tertiaryContainer := th.Color(theme.ColorNamePrimaryContainer, v)
	onTertiary := th.Color(theme.ColorNameOnPrimaryContainer, v)

	// Hour/minute buttons.
	if r.picker.selectingMinute {
		r.hourBtn.bg.FillColor = surfContHigh
		r.hourBtn.label.Color = onSurface
		r.minuteBtn.bg.FillColor = tertiaryContainer
		r.minuteBtn.label.Color = onTertiary
	} else {
		r.hourBtn.bg.FillColor = tertiaryContainer
		r.hourBtn.label.Color = onTertiary
		r.minuteBtn.bg.FillColor = surfContHigh
		r.minuteBtn.label.Color = onSurface
	}
	r.hourBtn.selected = !r.picker.selectingMinute
	r.minuteBtn.selected = r.picker.selectingMinute
	r.hourBtn.bg.CornerRadius = 8
	r.minuteBtn.bg.CornerRadius = 8

	// Format time.
	h := r.picker.Hour
	if !r.picker.Use24Hour {
		h = h % 12
		if h == 0 {
			h = 12
		}
	}
	r.hourBtn.label.Text = fmt.Sprintf("%02d", h)
	r.minuteBtn.label.Text = fmt.Sprintf("%02d", r.picker.Minute)

	// AM/PM.
	if !r.picker.Use24Hour {
		outline := th.Color(theme.ColorNameOutline, v)
		r.ampmBorder.StrokeColor = outline
		r.ampmBorder.StrokeWidth = 1
		r.ampmBorder.FillColor = color.Transparent
		r.ampmBorder.CornerRadius = 8
		r.ampmBorder.Refresh()

		isAM := r.picker.Hour < 12
		if isAM {
			r.amBtn.bg.FillColor = tertiaryContainer
			r.amBtn.label.Color = onTertiary
			r.pmBtn.bg.FillColor = color.Transparent
			r.pmBtn.label.Color = onSurface
		} else {
			r.amBtn.bg.FillColor = color.Transparent
			r.amBtn.label.Color = onSurface
			r.pmBtn.bg.FillColor = tertiaryContainer
			r.pmBtn.label.Color = onTertiary
		}
		r.amBtn.selected = isAM
		r.pmBtn.selected = !isAM
		r.amBtn.bg.CornerRadius = 8
		r.pmBtn.bg.CornerRadius = 8
	}

	// Keyboard field cursor: focus ring over whichever look the field has,
	// mirrored to the buttons' accessible focus state (#543).
	focusColor := th.Color(theme.ColorNameFocus, v)
	buttons := [...]*timePickerButton{r.hourBtn, r.minuteBtn, r.amBtn, r.pmBtn}
	for field, btn := range buttons {
		btn.focused = r.picker.focused && r.picker.focusField == field
		if btn.focused {
			btn.bg.StrokeWidth = 2
			btn.bg.StrokeColor = focusColor
		} else {
			btn.bg.StrokeWidth = 0
			btn.bg.StrokeColor = color.Transparent
		}
		btn.bg.Refresh()
		btn.label.Refresh()
	}

	// Clock face.
	r.clockFace.FillColor = surfContHigh
	r.clockFace.Refresh()

	// Selector.
	r.selectorArm.StrokeColor = primary
	r.selectorArm.Refresh()
	r.centerDot.FillColor = primary
	r.centerDot.Refresh()
	r.selectorDot.FillColor = color.Transparent
	r.selectorDot.Refresh()
	r.selectedBg.FillColor = primary
	r.selectedBg.Refresh()

	// Clock labels.
	selectedIdx := -1
	if r.picker.selectingMinute {
		for i := 0; i < 12; i++ {
			if r.picker.Minute == i*5 {
				selectedIdx = i
			}
		}
	} else if r.picker.Use24Hour {
		selectedIdx = selected24HourLabelIndex(r.picker.Hour)
	} else {
		h := r.picker.Hour % 12
		if h == 0 {
			h = 12
		}
		selectedIdx = h - 1
	}

	for i, l := range r.clockLabels {
		if i == selectedIdx {
			l.Color = onPrimary
		} else {
			l.Color = onSurface
		}
		l.Refresh()
	}

	r.Layout(r.picker.Size())
	canvas.Refresh(r.picker.super())
}
