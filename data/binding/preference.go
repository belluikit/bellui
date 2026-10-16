package binding

import (
	"sync/atomic"

	"github.com/belluikit/bellui"
)

const keyTypeMismatchError = "A previous preference binding exists with different type for key: "

// BindPreferenceBool returns a bindable bool value that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceBool(key string, p bellui.Preferences) Bool {
	return bindPreferenceItem[bool](key, p)
}

// BindPreferenceBoolList returns a bound list of bool values that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceBoolList(key string, p bellui.Preferences) BoolList {
	return bindPreferenceListComparable[bool](key, p)
}

// BindPreferenceFloat returns a bindable float64 value that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceFloat(key string, p bellui.Preferences) Float {
	return bindPreferenceItem[float64](key, p)
}

// BindPreferenceFloatList returns a bound list of float64 values that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceFloatList(key string, p bellui.Preferences) FloatList {
	return bindPreferenceListComparable[float64](key, p)
}

// BindPreferenceInt returns a bindable int value that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceInt(key string, p bellui.Preferences) Int {
	return bindPreferenceItem[int](key, p)
}

// BindPreferenceIntList returns a bound list of int values that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceIntList(key string, p bellui.Preferences) IntList {
	return bindPreferenceListComparable[int](key, p)
}

// BindPreferenceString returns a bindable string value that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceString(key string, p bellui.Preferences) String {
	return bindPreferenceItem[string](key, p)
}

// BindPreferenceStringList returns a bound list of string values that is managed by the application preferences.
// Changes to this value will be saved to application storage and when the app starts the previous values will be read.
//
// Since: 1.0
func BindPreferenceStringList(key string, p bellui.Preferences) StringList {
	return bindPreferenceListComparable[string](key, p)
}

// UnbindPreference removes the binding for the given key from the given preferences,
// allowing it to be garbage collected. Future calls to BindPreference* with the same
// key will create a new binding.
//
// Since: 1.0
func UnbindPreference(key string, p bellui.Preferences) {
	prefBinds.unbindPreference(key, p)
}

// CleanupPreferences removes all bindings associated with the given preferences
// instance. This should be called when a preferences instance is no longer needed
// (e.g. when a window is closed) to allow garbage collection.
//
// Since: 1.0
func CleanupPreferences(p bellui.Preferences) {
	prefBinds.cleanupPreferences(p)
}

func bindPreferenceItem[T bool | float64 | int | string](key string, p bellui.Preferences) Item[T] {
	if found, ok := lookupExistingBinding[T](key, p); ok {
		return found
	}

	listen := &prefBoundBase[T]{key: key, prefs: p}
	binds := prefBinds.ensurePreferencesAttached(p)
	binds.Store(key, listen)
	return listen
}

func lookupExistingBinding[T any](key string, p bellui.Preferences) (Item[T], bool) {
	binds := prefBinds.getBindings(p)
	if binds == nil {
		return nil, false
	}

	if listen, ok := binds.Load(key); listen != nil && ok {
		if l, ok := listen.(Item[T]); ok {
			return l, ok
		}
		bellui.LogError(keyTypeMismatchError+key, nil)
	}

	return nil, false
}

func lookupExistingListBinding[T bool | float64 | int | string](key string, p bellui.Preferences) (*prefBoundList[T], bool) {
	binds := prefBinds.getBindings(p)
	if binds == nil {
		return nil, false
	}

	if listen, ok := binds.Load(key); listen != nil && ok {
		if l, ok := listen.(*prefBoundList[T]); ok {
			return l, ok
		}
		bellui.LogError(keyTypeMismatchError+key, nil)
	}

	return nil, false
}

type prefBoundBase[T bool | float64 | int | string] struct {
	base
	key   string
	prefs bellui.Preferences
	cache atomic.Pointer[T]
}

func (b *prefBoundBase[T]) Get() (T, error) {
	cache := bellui.PrefOr[T](b.prefs, b.key, *new(T))
	b.cache.Store(&cache)
	return cache, nil
}

func (b *prefBoundBase[T]) Set(v T) error {
	bellui.SetPref(b.prefs, b.key, v)

	b.lock.RLock()
	defer b.lock.RUnlock()
	b.trigger()
	return nil
}

func (b *prefBoundBase[T]) checkForChange() {
	val := b.cache.Load()
	current := bellui.PrefOr[T](b.prefs, b.key, *new(T))
	if val != nil && current == *val {
		return
	}
	b.trigger()
}

func (b *prefBoundBase[T]) replaceProvider(p bellui.Preferences) {
	b.prefs = p
}

type prefBoundList[T bool | float64 | int | string] struct {
	boundList[T]
	key   string
	prefs bellui.Preferences
}

func (b *prefBoundList[T]) checkForChange() {
	val := *b.val
	updated := bellui.PrefListOr[T](b.prefs, b.key, nil)
	if val == nil || len(updated) != len(val) {
		b.Set(updated)
		return
	}

	// incoming changes to a preference list are not at the child level
	for i, v := range val {
		if i >= len(updated) {
			break
		}

		if !b.comparator(v, updated[i]) {
			_ = b.items[i].(Item[T]).Set(updated[i])
		}
	}
}

func (b *prefBoundList[T]) replaceProvider(p bellui.Preferences) {
	b.prefs = p
}

type internalPrefs = interface{ WriteValues(func(map[string]any)) }

func bindPreferenceListComparable[T bool | float64 | int | string](key string, p bellui.Preferences,
) *prefBoundList[T] {
	if found, ok := lookupExistingListBinding[T](key, p); ok {
		return found
	}

	listen := &prefBoundList[T]{key: key, prefs: p}

	items := bellui.PrefListOr[T](listen.prefs, listen.key, nil)
	listen.boundList = *bindList(nil, func(t1, t2 T) bool { return t1 == t2 })

	listen.boundList.AddListener(NewDataListener(func() {
		cached := *listen.val
		replaced := bellui.PrefListOr[T](listen.prefs, listen.key, nil)
		if len(cached) == len(replaced) {
			return
		}

		bellui.SetPrefList(listen.prefs, listen.key, *listen.val)
		listen.trigger()
	}))

	listen.boundList.parentListener = func(index int) {
		bellui.SetPrefList(listen.prefs, listen.key, *listen.val)

		// the child changes are not seen on the write end so force it
		if prefs, ok := p.(internalPrefs); ok {
			prefs.WriteValues(func(map[string]any) {})
		}
	}
	listen.boundList.Set(items)

	binds := prefBinds.ensurePreferencesAttached(p)
	binds.Store(key, listen)
	return listen
}
