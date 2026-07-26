package internal

import (
	"reflect"
	"sync"

	"github.com/belluikit/bellui"
)

// InMemoryPreferences provides an implementation of the bellui.Preferences API that is stored in memory.
type InMemoryPreferences struct {
	values          map[string]any
	lock            sync.RWMutex
	changeListeners []func()
}

// Declare conformity with Preferences interface
var _ bellui.Preferences = (*InMemoryPreferences)(nil)

// AddChangeListener allows code to be notified when some preferences change. This will fire on any update.
// The passed 'listener' should not try to write values.
func (p *InMemoryPreferences) AddChangeListener(listener func()) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.changeListeners = append(p.changeListeners, listener)
}

// ChangeListenersCopy returns a thread-safe copy of the registered change listeners.
// Used internally during cloud provider transitions.
func (p *InMemoryPreferences) ChangeListenersCopy() []func() {
	p.lock.RLock()
	defer p.lock.RUnlock()

	listeners := make([]func(), len(p.changeListeners))
	copy(listeners, p.changeListeners)
	return listeners
}

// Value looks up a value for the given key.
func (p *InMemoryPreferences) Value(key string) (any, bool) {
	return p.get(key)
}

// SetValue saves a value for the given key.
func (p *InMemoryPreferences) SetValue(key string, value any) {
	p.set(key, value)
}

// RemoveValue deletes a value on the given key
func (p *InMemoryPreferences) RemoveValue(key string) {
	p.remove(key)
}

// ReadValues provides read access to the underlying value map - for internal use only...
// You should not retain a reference to the map nor write to the values in the callback function
func (p *InMemoryPreferences) ReadValues(fn func(map[string]any)) {
	p.lock.RLock()
	fn(p.values)
	p.lock.RUnlock()
}

// WriteValues provides write access to the underlying value map - for internal use only...
// You should not retain a reference to the map passed to the callback function
func (p *InMemoryPreferences) WriteValues(fn func(map[string]any)) {
	p.lock.Lock()
	fn(p.values)
	p.lock.Unlock()

	p.fireChange()
}

// NewInMemoryPreferences creates a new preferences implementation stored in memory
func NewInMemoryPreferences() *InMemoryPreferences {
	return &InMemoryPreferences{values: make(map[string]any)}
}

func (p *InMemoryPreferences) fireChange() {
	p.lock.RLock()
	listeners := make([]func(), len(p.changeListeners))
	copy(listeners, p.changeListeners)
	p.lock.RUnlock()

	for _, l := range listeners {
		l()
	}
}

func (p *InMemoryPreferences) get(key string) (any, bool) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	v, ok := p.values[key]
	return v, ok
}

func (p *InMemoryPreferences) remove(key string) {
	p.lock.Lock()
	delete(p.values, key)
	p.lock.Unlock()

	p.fireChange()
}

func (p *InMemoryPreferences) set(key string, value any) {
	p.lock.Lock()

	if reflect.TypeOf(value).Kind() == reflect.Slice {
		s := reflect.ValueOf(value)
		old := reflect.ValueOf(p.values[key])
		if p.values[key] != nil && s.Len() == old.Len() {
			changed := false
			for i := 0; i < s.Len(); i++ {
				if s.Index(i).Interface() != old.Index(i).Interface() {
					changed = true
					break
				}
			}
			if !changed {
				p.lock.Unlock()
				return
			}
		}
	} else {
		if stored, ok := p.values[key]; ok && stored == value {
			p.lock.Unlock()
			return
		}
	}

	p.values[key] = value
	p.lock.Unlock()

	p.fireChange()
}
