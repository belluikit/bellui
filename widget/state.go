package widget

import "github.com/belluikit/bellui/core"

// State is a reactive value holder that automatically refreshes a widget
// when the value changes. It eliminates the need for manual Refresh() calls
// after state mutations.
//
// Usage:
//
//	type MyWidget struct {
//	    widget.BaseWidget
//	    Title *widget.State[string]
//	    Count *widget.State[int]
//	}
//
//	func NewMyWidget() *MyWidget {
//	    w := &MyWidget{}
//	    w.Title = widget.NewState(w, "Hello")
//	    w.Count = widget.NewState(w, 0)
//	    w.ExtendBaseWidget(w)
//	    return w
//	}
//
//	// From any goroutine:
//	w.Title.Set("World")   // automatically refreshes the widget
//	w.Count.Set(42)        // automatically refreshes the widget
//	fmt.Println(w.Title.Get()) // "World"
type State[T comparable] struct {
	value    T
	widget   core.Widget
	onChange func(T)
}

// NewState creates a reactive state bound to a widget.
// When the value changes via Set(), the widget is automatically refreshed.
func NewState[T comparable](w core.Widget, initial T) *State[T] {
	return &State[T]{
		value:  initial,
		widget: w,
	}
}

// NewStateWithCallback creates a reactive state that calls onChange
// in addition to refreshing the widget.
func NewStateWithCallback[T comparable](w core.Widget, initial T, onChange func(T)) *State[T] {
	return &State[T]{
		value:    initial,
		widget:   w,
		onChange: onChange,
	}
}

// Get returns the current value.
func (s *State[T]) Get() T {
	return s.value
}

// Set updates the value and refreshes the bound widget if the value changed.
// It is safe to call from any goroutine.
func (s *State[T]) Set(value T) {
	if s.value == value {
		return
	}
	onMain(func() {
		s.value = value
		if s.onChange != nil {
			s.onChange(value)
		}
		if s.widget != nil {
			s.widget.Refresh()
		}
	})
}
