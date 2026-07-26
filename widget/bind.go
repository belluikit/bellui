package widget

import (
	"fmt"

	"github.com/belluikit/bellui/flow"
)

// BindEntry creates a bidirectional binding between an Entry and a flow.Value[string].
// The Entry's text is set from the value, and typing in the Entry updates the value.
// Returns an unsubscribe function that removes the binding.
//
//	name := flow.New("Hello")
//	entry := widget.NewEntry()
//	unbind := widget.BindEntry(entry, name)
func BindEntry(entry *Entry, value *flow.Value[string]) func() {
	// Value → Entry
	entry.SetText(value.Get())
	cancel := value.OnChange(func() {
		v := value.Get()
		if entry.Text != v {
			entry.SetText(v)
		}
	})

	// Entry → Value
	prev := entry.OnChanged
	entry.OnChanged = func(s string) {
		value.Set(s)
		if prev != nil {
			prev(s)
		}
	}

	return func() {
		cancel()
		entry.OnChanged = prev
	}
}

// BindLabel creates a one-way binding from a flow.Source to a Label.
// The Label's text is updated whenever the source value changes.
// Works with both flow.Value[string] and flow.Derived[string].
// Returns an unsubscribe function.
//
//	name := flow.New("Hello")
//	label := widget.NewLabel("")
//	unbind := widget.BindLabel(label, name)
func BindLabel(label *Label, source StringSource) func() {
	label.SetText(source.GetString())
	return source.OnChange(func() {
		label.SetText(source.GetString())
	})
}

// BindCheck creates a bidirectional binding between a Check and a flow.Value[bool].
// Returns an unsubscribe function.
//
//	enabled := flow.New(true)
//	check := widget.NewCheck("Enable", nil)
//	unbind := widget.BindCheck(check, enabled)
func BindCheck(check *Check, value *flow.Value[bool]) func() {
	check.SetChecked(value.Get())
	cancel := value.OnChange(func() {
		v := value.Get()
		if check.Checked != v {
			check.SetChecked(v)
		}
	})

	prev := check.OnChanged
	check.OnChanged = func(b bool) {
		value.Set(b)
		if prev != nil {
			prev(b)
		}
	}

	return func() {
		cancel()
		check.OnChanged = prev
	}
}

// BindSlider creates a bidirectional binding between a Slider and a flow.Value[float64].
// Returns an unsubscribe function.
func BindSlider(slider *Slider, value *flow.Value[float64]) func() {
	slider.SetValue(value.Get())
	cancel := value.OnChange(func() {
		v := value.Get()
		if slider.Value != v {
			slider.SetValue(v)
		}
	})

	prev := slider.OnChanged
	slider.OnChanged = func(f float64) {
		value.Set(f)
		if prev != nil {
			prev(f)
		}
	}

	return func() {
		cancel()
		slider.OnChanged = prev
	}
}

// BindProgressBar creates a one-way binding from a flow.Value[float64] to a ProgressBar.
// Returns an unsubscribe function.
func BindProgressBar(bar *ProgressBar, value *flow.Value[float64]) func() {
	bar.SetValue(value.Get())
	return value.OnChange(func() {
		bar.SetValue(value.Get())
	})
}

// StringSource is the interface for anything that provides a string value
// and can be listened to. Satisfied by StringValue, StringDerived, FormatValue.
type StringSource interface {
	GetString() string
	OnChange(func()) func()
}

// Ensure flow types satisfy StringSource via adapter.
// We can't add GetString() to flow.Value directly (it's generic),
// so we use a wrapper.

// StringValue wraps a flow.Value[string] to implement StringSource.
type StringValue struct{ *flow.Value[string] }

func (s StringValue) GetString() string { return s.Get() }

// StringDerived wraps a flow.Derived[string] to implement StringSource.
type StringDerived struct{ *flow.Derived[string] }

func (s StringDerived) GetString() string { return s.Get() }

// FormatValue creates a StringSource from any flow.Value by formatting with fmt.Sprint.
// Useful for binding non-string values to Labels:
//
//	count := flow.New(0)
//	label := widget.NewLabel("")
//	widget.BindLabel(label, widget.FormatValue(count, "Count: %d"))
func FormatValue[T comparable](value *flow.Value[T], format string) StringSource {
	return &formattedSource[T]{value: value, format: format}
}

type formattedSource[T comparable] struct {
	value  *flow.Value[T]
	format string
}

func (f *formattedSource[T]) GetString() string {
	return fmt.Sprintf(f.format, f.value.Get())
}

func (f *formattedSource[T]) OnChange(fn func()) func() {
	return f.value.OnChange(fn)
}
