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
	if s.Step >= 1 {
		return fmt.Sprintf("%d", int64(s.Value))
	}
	return fmt.Sprintf("%.1f", s.Value)
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
	val = math.Max(s.Min, math.Min(s.Max, val))
	if val == s.Value {
		return
	}
	s.Value = val
	s.settingValue = true
	s.entry.SetText(s.formatValue())
	s.settingValue = false
	s.updateButtonState()
	if f := s.OnChanged; f != nil {
		f(s.Value)
	}
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
	s.entry.SetText(s.formatValue())

	s.content = &bellui.Container{Layout: &spinBoxLayout{}, Objects: []core.CanvasObject{s.decBtn, s.entry, s.incBtn}}
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
	if s.Step >= 1 && s.Step == math.Floor(s.Step) {
		return fmt.Sprintf("%d", int64(s.Value))
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
	return strconv.FormatFloat(s.Value, 'f', decimals, 64)
}

func (s *SpinBox) parseAndUpdate(text string) {
	if s.settingValue {
		return // avoid double-firing OnChanged from SetValue → SetText → OnChanged
	}
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return
	}
	val = math.Max(s.Min, math.Min(s.Max, val))
	s.Value = val
	s.updateButtonState()
	if f := s.OnChanged; f != nil {
		f(s.Value)
	}
}

// --- spinBoxLayout: lays out [decBtn, entry, incBtn] ---

type spinBoxLayout struct{}

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
	w := btnMin.Height*2 + entryMin.Width // square buttons + entry
	return geo.NewSize(w, h)
}

var _ core.Layout = (*spinBoxLayout)(nil)
