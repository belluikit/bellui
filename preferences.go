package bellui

// Preferences describes the ways that an app can save and load user preferences.
type Preferences interface {
	// Value looks up the value for the given key. Returns (value, true) if found,
	// or (nil, false) if the key does not exist.
	Value(key string) (any, bool)

	// SetValue saves a value for the given key. The value must be one of the
	// supported [PreferenceType] types.
	SetValue(key string, value any)

	// RemoveValue removes a value for the given key.
	RemoveValue(key string)

	// AddChangeListener allows code to be notified when some preferences change.
	// This will fire on any update.
	AddChangeListener(func())
}

// PreferenceType constrains the types that can be stored in [Preferences].
type PreferenceType interface {
	bool | int | float64 | string | []bool | []int | []float64 | []string
}

// Pref retrieves a typed value from preferences. Returns (value, true) if found
// and type-compatible, or (zero, false) otherwise. Handles JSON deserialization
// coercion between int and float64.
func Pref[T PreferenceType](p Preferences, key string) (T, bool) {
	v, ok := p.Value(key)
	if !ok {
		var zero T
		return zero, false
	}
	if t, ok := v.(T); ok {
		return t, true
	}
	return coerce[T](v)
}

// PrefOr retrieves a typed value from preferences, returning fallback if the key
// is missing or the value is not type-compatible.
func PrefOr[T PreferenceType](p Preferences, key string, fallback T) T {
	v, ok := Pref[T](p, key)
	if !ok {
		return fallback
	}
	return v
}

// SetPref saves a typed value to preferences.
func SetPref[T PreferenceType](p Preferences, key string, value T) {
	p.SetValue(key, value)
}

// PrefList retrieves a typed list value from preferences. Handles JSON deserialization
// coercion ([]any -> typed slice, []float64 <-> []int).
func PrefList[T bool | int | float64 | string](p Preferences, key string) ([]T, bool) {
	v, ok := p.Value(key)
	if !ok {
		return nil, false
	}
	if t, ok := v.([]T); ok {
		return t, true
	}
	// Use the coerce machinery for []T via the PreferenceType constraint
	return coerceList[T](v)
}

// PrefListOr retrieves a typed list from preferences, returning fallback if not found.
func PrefListOr[T bool | int | float64 | string](p Preferences, key string, fallback []T) []T {
	v, ok := PrefList[T](p, key)
	if !ok {
		return fallback
	}
	return v
}

// SetPrefList saves a typed list to preferences.
func SetPrefList[T bool | int | float64 | string](p Preferences, key string, value []T) {
	p.SetValue(key, value)
}

// coerceList handles JSON deserialization coercion for list types.
func coerceList[T bool | int | float64 | string](v any) ([]T, bool) {
	var result []T
	ok := false
	// Wrap into the coerce[PreferenceType] machinery by switching on T
	switch any(*new(T)).(type) {
	case bool:
		if r, k := coerce[[]bool](v); k {
			result = any(r).([]T)
			ok = true
		}
	case int:
		if r, k := coerce[[]int](v); k {
			result = any(r).([]T)
			ok = true
		}
	case float64:
		if r, k := coerce[[]float64](v); k {
			result = any(r).([]T)
			ok = true
		}
	case string:
		if r, k := coerce[[]string](v); k {
			result = any(r).([]T)
			ok = true
		}
	}
	return result, ok
}

// coerce handles JSON deserialization mismatches: JSON numbers are always float64,
// so int preferences round-trip as float64. Also handles []any from JSON arrays.
func coerce[T PreferenceType](v any) (T, bool) {
	var zero T
	switch any(zero).(type) {
	case int:
		if f, ok := v.(float64); ok {
			return any(int(f)).(T), true
		}
	case float64:
		if i, ok := v.(int); ok {
			return any(float64(i)).(T), true
		}
	case []int:
		if fs, ok := v.([]float64); ok {
			ints := make([]int, len(fs))
			for i, f := range fs {
				ints[i] = int(f)
			}
			return any(ints).(T), true
		}
		if items, ok := v.([]any); ok {
			ints := make([]int, len(items))
			for i, item := range items {
				switch n := item.(type) {
				case float64:
					ints[i] = int(n)
				case int:
					ints[i] = n
				default:
					return zero, false
				}
			}
			return any(ints).(T), true
		}
	case []float64:
		if is, ok := v.([]int); ok {
			fs := make([]float64, len(is))
			for i, iv := range is {
				fs[i] = float64(iv)
			}
			return any(fs).(T), true
		}
		if items, ok := v.([]any); ok {
			fs := make([]float64, len(items))
			for i, item := range items {
				switch n := item.(type) {
				case float64:
					fs[i] = n
				case int:
					fs[i] = float64(n)
				default:
					return zero, false
				}
			}
			return any(fs).(T), true
		}
	case []bool:
		if items, ok := v.([]any); ok {
			bs := make([]bool, len(items))
			for i, item := range items {
				b, ok := item.(bool)
				if !ok {
					return zero, false
				}
				bs[i] = b
			}
			return any(bs).(T), true
		}
	case []string:
		if items, ok := v.([]any); ok {
			ss := make([]string, len(items))
			for i, item := range items {
				s, ok := item.(string)
				if !ok {
					return zero, false
				}
				ss[i] = s
			}
			return any(ss).(T), true
		}
	}
	return zero, false
}
