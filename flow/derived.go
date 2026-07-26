package flow

import "sync"

// Derived is a computed value that automatically recomputes when any of
// its source dependencies change. It implements [Source] so other Derived
// values or listeners can depend on it.
//
//	firstName := flow.New("John")
//	lastName := flow.New("Doe")
//
//	fullName := flow.Derive(func() string {
//	    return firstName.Get() + " " + lastName.Get()
//	}, firstName, lastName)
//
//	fmt.Println(fullName.Get()) // "John Doe"
//	firstName.Set("Jane")
//	fmt.Println(fullName.Get()) // "Jane Doe"
type Derived[T comparable] struct {
	mu        sync.RWMutex
	value     T
	compute   func() T
	listeners []listener
	nextID    int
	cancels   []func() // unsubscribe from sources
}

// Derive creates a computed value from a function and its dependencies.
// The function is called immediately to compute the initial value, and
// again whenever any dependency changes.
func Derive[T comparable](compute func() T, deps ...Source) *Derived[T] {
	d := &Derived[T]{
		compute: compute,
		value:   compute(),
	}

	for _, dep := range deps {
		cancel := dep.OnChange(func() {
			d.recompute()
		})
		d.cancels = append(d.cancels, cancel)
	}

	return d
}

// Get returns the current computed value.
func (d *Derived[T]) Get() T {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.value
}

// Listen registers a callback called when the computed value changes.
// Returns an unsubscribe function.
func (d *Derived[T]) Listen(fn func(old, new T)) func() {
	return d.OnChange(func() {
		fn(d.Get(), d.Get())
	})
}

// OnChange implements Source.
func (d *Derived[T]) OnChange(fn func()) func() {
	d.mu.Lock()
	id := d.nextID
	d.nextID++
	d.listeners = append(d.listeners, listener{id: id, fn: fn})
	d.mu.Unlock()

	return func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		for i, l := range d.listeners {
			if l.id == id {
				d.listeners = append(d.listeners[:i], d.listeners[i+1:]...)
				return
			}
		}
	}
}

func (d *Derived[T]) recompute() {
	new := d.compute()

	d.mu.Lock()
	if d.value == new {
		d.mu.Unlock()
		return
	}
	d.value = new
	ls := make([]listener, len(d.listeners))
	copy(ls, d.listeners)
	d.mu.Unlock()

	for _, l := range ls {
		l.fn()
	}
}

// Close unsubscribes from all source dependencies.
// After Close, the derived value will no longer update.
func (d *Derived[T]) Close() {
	for _, cancel := range d.cancels {
		cancel()
	}
	d.cancels = nil
}
