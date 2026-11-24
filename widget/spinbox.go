package widget

import (
	"fmt"
	"math"
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget             = (*SpinBox)(nil)
	_ event.Focusable         = (*SpinBox)(nil)
	_ core.Disableable        = (*SpinBox)(nil)
	_ core.Accessible         = (*SpinBox)(nil)
	_ core.AccessibleValuer   = (*SpinBox)(nil)
	_ core.AccessibleActioner = (*SpinBox)(nil)
)

// SpinBox is a numeric input with increment and decrement buttons.
//
// Its number field is as wide as the wider of Min and Max as the box formats
// them, so every value it can hold shows whole (#1420). A huge finite end
// makes a correspondingly wide field; an end that is meant to be unbounded
// should be math.Inf(-1) or math.Inf(1), which takes no width.
//
// After writing Value, Min, Max or Step directly, call Refresh: it clamps
// Value into [Min, Max], shows it formatted for Step and updates the buttons,
// without calling OnChanged (#1433). SetValue is the reporting setter.
//
// Since: 1.0
type SpinBox struct {
	DisableableWidget

	// Value is the current value of the spin box.
	Value float64
	// Min is the minimum value the spin box may hold.
	Min float64
	// Max is the maximum value the spin box may hold.
	Max float64
	// Step is the amount added or subtracted on each increment or decrement.
	Step float64
	// OnChanged is called whenever the value changes.
	OnChanged func(float64) `json:"-"`

	entry        *NumericEntry
	incBtn       *Button
	decBtn       *Button
	content      *bellui.Container
	settingValue bool
	// shownText is the entry text the box itself last wrote. Text that
	// differs from it was typed by the user, and Refresh keeps it while it
	// still stands for Value (#1433).
	shownText string
}

// NewSpinBox creates a new spin box with the given range and step.
//
// Since: 1.0
func NewSpinBox(min, max, step float64) *SpinBox {
	s := &SpinBox{
		Min:  min,
		Max:  max,
		Step: step,
	}
	s.Value = min
	s.ExtendBaseWidget(s)
	s.setupUI()
	return s
}

// AccessibleRole returns [core.RoleSpinButton].
func (s *SpinBox) AccessibleRole() core.AccessibleRole { return core.RoleSpinButton }

// AccessibleLabel returns an empty string; spin boxes have no intrinsic label.
func (s *SpinBox) AccessibleLabel() string { return "" }

// AccessibleValue returns a human-readable representation of the current value.
func (s *SpinBox) AccessibleValue() string {
	// The field's own text while it stands for Value, so a screen reader reads
	// what is shown; otherwise Value as the field formats it (#1440). It used
	// its own format, which read 2.25 at step 0.25 as "2.2" and 1.5 at step
	// 1.5 as "1".
	if s.entry != nil {
		if v, err := strconv.ParseFloat(s.entry.Text, 64); err == nil && v == s.Value {
			return s.entry.Text
		}
	}
	return s.formatValue()
}

// AccessibleRange returns the minimum, maximum, and current spin box values.
func (s *SpinBox) AccessibleRange() (min, max, current float64) {
	return s.Min, s.Max, s.Value
}

// AccessibleActions returns the "increment" and "decrement" actions.
func (s *SpinBox) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{
		{Name: core.ActionIncrement, Description: "Increase the value"},
		{Name: core.ActionDecrement, Description: "Decrease the value"},
	}
}

// PerformAccessibleAction adjusts the spin box value by one step.
func (s *SpinBox) PerformAccessibleAction(name string) {
	if s.Disabled() {
		return
	}
	switch name {
	case core.ActionIncrement:
		s.SetValue(s.Value + s.Step)
	case core.ActionDecrement:
		s.SetValue(s.Value - s.Step)
	}
}

// SetValue sets the spin box value, clamping to [Min, Max].
func (s *SpinBox) SetValue(val float64) {
	val = s.clamp(val)
	if val == s.Value {
		return
	}
	s.Value = val
	s.showValue()
	s.updateButtonState()
	if f := s.OnChanged; f != nil {
		f(s.Value)
	}
}

// Refresh brings the box in line with its fields after they were written
// directly: Value is clamped into [Min, Max], the field shows it formatted
// for Step, and the buttons follow the range. It is silent: OnChanged is not
// called, as a slider's Refresh clamps its value silently (#1433). Text the
// user is typing is kept while it still stands for Value, so a Refresh in the
// middle of typing "1.5" does not turn "1." into "1.00".
func (s *SpinBox) Refresh() {
	s.ExtendBaseWidget(s)
	if s.entry != nil {
		s.syncFields()
	}
	s.DisableableWidget.Refresh()
}

// syncFields clamps Value, re-shows it and updates the buttons, silently.
func (s *SpinBox) syncFields() {
	clamped := s.clamp(s.Value)
	moved := clamped != s.Value
	s.Value = clamped
	if s.entry.Text != s.shownText && !moved {
		// The user has typed since the box last wrote the field, and the
		// range still holds Value: keep the text if it is what they typed
		// for this value, read as parseAndUpdate reads it. A Value the new
		// range moved is shown, since the typed text no longer stands for it.
		if val, err := strconv.ParseFloat(s.entry.Text, 64); err == nil && s.clamp(val) == s.Value {
			s.updateButtonState()
			return
		}
	}
	if s.entry.Text != s.formatValue() {
		s.showValue()
	}
	s.updateButtonState()
}

// commitText makes the field show Value once the user is done typing (Enter
// or focus loss, #1440). Value already follows what was typed, clamped to
// [Min, Max]; text that was out of range, did not parse, or was written
// loosely ("3.") is replaced by Value formatted for Step. Text that is
// exactly Value is kept when Step's format would round it away, so a 2.5
// typed at step 1 is not shown as "2" while Value is 2.5. It is silent:
// Value does not change here, so OnChanged is not called.
func (s *SpinBox) commitText() {
	text := s.entry.Text
	formatted := s.formatValue()
	if text == formatted {
		return
	}
	if v, err := strconv.ParseFloat(text, 64); err == nil && v == s.Value {
		if f, err := strconv.ParseFloat(formatted, 64); err != nil || f != s.Value {
			return
		}
	}
	s.showValue()
}

// clamp limits v to [Min, Max].
func (s *SpinBox) clamp(v float64) float64 {
	return math.Max(s.Min, math.Min(s.Max, v))
}

// showValue writes the formatted Value into the field without reporting it.
func (s *SpinBox) showValue() {
	text := s.formatValue()
	s.settingValue = true
	s.entry.SetText(text)
	s.settingValue = false
	s.shownText = text
}

// FocusGained delegates focus to the internal entry.
func (s *SpinBox) FocusGained() {
	s.entry.FocusGained()
}

// FocusLost delegates focus loss to the internal entry.
func (s *SpinBox) FocusLost() {
	s.entry.FocusLost()
}

// TypedRune delegates to the internal entry.
func (s *SpinBox) TypedRune(r rune) {
	s.entry.TypedRune(r)
}

// TypedKey handles arrow up/down for increment/decrement.
func (s *SpinBox) TypedKey(key *event.KeyEvent) {
	switch key.Name {
	case event.KeyUp:
		s.increment()
	case event.KeyDown:
		s.decrement()
	default:
		s.entry.TypedKey(key)
	}
}

// Enable enables the spin box and its children.
func (s *SpinBox) Enable() {
	s.DisableableWidget.Enable()
	if s.entry != nil {
		s.entry.Enable()
		s.updateButtonState()
	}
}

// Disable disables the spin box and its children.
func (s *SpinBox) Disable() {
	s.DisableableWidget.Disable()
	if s.entry != nil {
		s.entry.Disable()
		s.incBtn.Disable()
		s.decBtn.Disable()
	}
}

// CreateRenderer returns the renderer for the spin box.
func (s *SpinBox) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	if s.content == nil {
		s.setupUI()
	}
	s.syncFields() // a literal, or fields written before the first render (#1433)
	return NewSimpleRenderer(s.content)
}

// MinSize returns the minimum size of the spin box.
func (s *SpinBox) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

func (s *SpinBox) setupUI() {
	s.decBtn = NewButton("", s.decrement)
	s.decBtn.SetIcon(theme.NewThemedResource(theme.ContentRemoveIcon()))
	s.decBtn.Importance = LowImportance

	s.incBtn = NewButton("", s.increment)
	s.incBtn.SetIcon(theme.NewThemedResource(theme.ContentAddIcon()))
	s.incBtn.Importance = LowImportance

	s.entry = NewNumericEntry()
	s.entry.ValidateWithBorder = true
	s.entry.Entry.OnChanged = func(text string) {
		s.parseAndUpdate(text)
	}
	// Typed text is corrected when the user is done with it, on Enter or
	// when the field loses focus: "12" typed into 0–10 already set Value to
	// 10, and now the field shows it (#1440). Correcting on every keystroke
	// would fight typing: in 10–20 the "1" of "15" is below Min.
	s.entry.OnSubmitted = func(string) { s.commitText() }
	s.entry.onFocusChanged = func(focused bool) {
		if !focused {
			s.commitText()
		}
	}
	s.entry.OnTypedKey = func(key *event.KeyEvent) bool {
		switch key.Name {
		case event.KeyUp:
			s.increment()
			return true
		case event.KeyDown:
			s.decrement()
			return true
		}
		return false
	}
	s.showValue()

	s.content = &bellui.Container{Layout: &spinBoxLayout{spin: s}, Objects: []core.CanvasObject{s.decBtn, s.entry, s.incBtn}}
	s.updateButtonState()
}

func (s *SpinBox) increment() {
	s.SetValue(s.Value + s.Step)
}

func (s *SpinBox) decrement() {
	s.SetValue(s.Value - s.Step)
}

func (s *SpinBox) updateButtonState() {
	if s.Disabled() {
		s.incBtn.Disable()
		s.decBtn.Disable()
		return
	}
	if s.Value >= s.Max {
		s.incBtn.Disable()
	} else {
		s.incBtn.Enable()
	}
	if s.Value <= s.Min {
		s.decBtn.Disable()
	} else {
		s.decBtn.Enable()
	}
}

func (s *SpinBox) formatValue() string {
	return s.format(s.Value)
}

// format renders v the way the box displays its value: whole numbers for a
// whole step, otherwise as many decimals as the step has (two at least).
func (s *SpinBox) format(v float64) string {
	if s.Step >= 1 && s.Step == math.Floor(s.Step) {
		return fmt.Sprintf("%d", int64(v))
	}
	// Determine decimal places from step
	text := strconv.FormatFloat(s.Step, 'f', -1, 64)
	decimals := 0
	if dot := len(text) - 1 - len(strconv.FormatFloat(math.Floor(s.Step), 'f', -1, 64)); dot > 0 {
		decimals = dot
	}
	// Fallback: count digits after decimal point in step
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '.' {
			decimals = len(text) - 1 - i
			break
		}
	}
	if decimals == 0 {
		decimals = 2
	}
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// fieldMinWidth is the width the number field needs to show the widest value
// the box can hold, Min or Max as the box formats them, without scrolling: the
// text plus the entry's inner padding on both sides and its border (#1420).
// The box clamps every value to [Min, Max], so nothing wider is ever displayed
// by it. An infinite or NaN end has no width to fit and is skipped.
func (s *SpinBox) fieldMinWidth() float32 {
	th := s.entry.Theme()
	textSize := th.Size(theme.SizeNameText)
	widest := float32(0)
	for _, v := range [...]float64{s.Min, s.Max} {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			continue
		}
		widest = geo.Max(widest, bellui.MeasureText(s.format(v), textSize, s.entry.TextStyle).Width)
	}
	if widest == 0 {
		return 0
	}
	return widest + th.Size(theme.SizeNameInnerPadding)*2 + th.Size(theme.SizeNameInputBorder)*2
}

func (s *SpinBox) parseAndUpdate(text string) {
	if s.settingValue {
		return // avoid double-firing OnChanged from SetValue → SetText → OnChanged
	}
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return
	}
	s.Value = s.clamp(val)
	s.updateButtonState()
	if f := s.OnChanged; f != nil {
		f(s.Value)
	}
}

// --- spinBoxLayout: lays out [decBtn, entry, incBtn] ---

type spinBoxLayout struct {
	spin *SpinBox // sizes the field to the widest value the range allows
}

func (l *spinBoxLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	if len(objects) < 3 {
		return
	}
	btnSize := objects[0].MinSize()
	btnW := btnSize.Height // square buttons
	entryW := size.Width - btnW*2

	objects[0].Resize(geo.NewSize(btnW, size.Height))
	objects[0].Move(geo.NewPos(0, 0))

	objects[1].Resize(geo.NewSize(entryW, size.Height))
	objects[1].Move(geo.NewPos(btnW, 0))

	objects[2].Resize(geo.NewSize(btnW, size.Height))
	objects[2].Move(geo.NewPos(btnW+entryW, 0))

	// Under RTL reflect the row so the increment button is on the left and the
	// decrement button on the right; the entry stays centred (symmetric).
	mirrorIfRTL(objects, size.Width)
}

func (l *spinBoxLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if len(objects) < 3 {
		return geo.NewSize(0, 0)
	}
	btnMin := objects[0].MinSize()
	entryMin := objects[1].MinSize()
	h := geo.Max(btnMin.Height, entryMin.Height)
	entryW := entryMin.Width
	if l.spin != nil {
		entryW = geo.Max(entryW, l.spin.fieldMinWidth())
	}
	w := btnMin.Height*2 + entryW // square buttons + entry
	return geo.NewSize(w, h)
}

var _ core.Layout = (*spinBoxLayout)(nil)
