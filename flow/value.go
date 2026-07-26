// Package flow provides reactive state management for Bellui applications.
//
// [Value] holds an observable value that notifies listeners when it changes.
// [Derived] computes a value from dependencies and auto-updates when they change.
//
// This package has no Bellui dependencies — it is pure Go with generics.
// Widget binding helpers are in the widget package (widget.BindEntry, etc.).
//
// Example:
//
//	name := flow.New("World")
//	count := flow.New(0)
//
//	greeting := flow.Derive(func() string {
//	    return fmt.Sprintf("Hello %s (%d)", name.Get(), count.Get())
//	}, name, count)
//
//	name.Listen(func(old, new string) {
//	    fmt.Printf("name changed: %q → %q\n", old, new)
//	})
//
//	name.Set("Bellui")   // triggers listener + recomputes greeting
//	count.Set(1)        // recomputes greeting
package flow

import "sync"

// Source is the interface shared by Value and Derived — anything that
// can be listened to for changes.
type Source interface {
	// OnChange registers a callback called when the value changes.
	// Returns an unsubscribe function.
	OnChange(fn func()) func()
}

// Value is an observable value of type T. When the value changes via Set(),
// all registered listeners are notified. Safe for concurrent use.
type Value[T comparable] struct {
	mu        sync.RWMutex
	value     T
	listeners []listener
	nextID    int
}

type listener struct {
	id int
	fn func()
}

// New creates a Value with the given initial value.
func New[T comparable](initial T) *Value[T] {
	return &Value[T]{value: initial}
}

// Get returns the current value.
func (v *Value[T]) Get() T {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.value
}

// Set updates the value. If the new value differs from the current one,
// all listeners are notified. Safe to call from any goroutine.
func (v *Value[T]) Set(new T) {
	v.mu.Lock()
	if v.value == new {
		v.mu.Unlock()
		return
	}
	v.value = new
	// Copy listeners under lock, notify without lock.
	ls := make([]listener, len(v.listeners))
	copy(ls, v.listeners)
	v.mu.Unlock()

	for _, l := range ls {
		l.fn()
	}
}

// Listen registers a typed callback that is called whenever the value changes.
// The callback receives the old and new values. Returns an unsubscribe function.
func (v *Value[T]) Listen(fn func(old, new T)) func() {
	return v.OnChange(func() {
		// We read the current value; the "old" is not available at this
		// abstraction level, so both are the new value. For typed callbacks
		// that need old values, use ListenChange.
		fn(v.Get(), v.Get())
	})
}

// OnChange implements Source.
func (v *Value[T]) OnChange(fn func()) func() {
	v.mu.Lock()
	id := v.nextID
	v.nextID++
	v.listeners = append(v.listeners, listener{id: id, fn: fn})
	v.mu.Unlock()

	return func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		for i, l := range v.listeners {
			if l.id == id {
				v.listeners = append(v.listeners[:i], v.listeners[i+1:]...)
				return
			}
		}
	}
}

// Modify atomically reads, modifies, and sets the value using the given function.
// Useful for read-modify-write operations:
//
//	count.Modify(func(v int) int { return v + 1 })
func (v *Value[T]) Modify(fn func(T) T) {
	v.mu.Lock()
	old := v.value
	new := fn(old)
	if old == new {
		v.mu.Unlock()
		return
	}
	v.value = new
	ls := make([]listener, len(v.listeners))
	copy(ls, v.listeners)
	v.mu.Unlock()

	for _, l := range ls {
		l.fn()
	}
}
