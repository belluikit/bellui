// Package filter provides filter implementations for the data grid.
package filter

// Filter is the interface that all column filters implement.
type Filter interface {
	// Matches returns true if the value passes the filter.
	Matches(value any) bool
}

// TextFilter filters string values using various match modes.
type TextFilter struct {
	Mode  TextFilterMode
	Value string
}

// TextFilterMode specifies how text values are compared.
type TextFilterMode int

const (
	TextContains TextFilterMode = iota
	TextEquals
	TextStartsWith
	TextEndsWith
	TextNotContains
	TextNotEquals
)

// Matches implements Filter.
func (f *TextFilter) Matches(value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	switch f.Mode {
	case TextContains:
		return contains(s, f.Value)
	case TextEquals:
		return s == f.Value
	case TextStartsWith:
		return len(s) >= len(f.Value) && s[:len(f.Value)] == f.Value
	case TextEndsWith:
		return len(s) >= len(f.Value) && s[len(s)-len(f.Value):] == f.Value
	case TextNotContains:
		return !contains(s, f.Value)
	case TextNotEquals:
		return s != f.Value
	}
	return true
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// NumberFilter filters numeric values.
type NumberFilter struct {
	Mode  NumberFilterMode
	Value float64
	To    float64 // for range mode
}

// NumberFilterMode specifies how numeric values are compared.
type NumberFilterMode int

const (
	NumberEquals NumberFilterMode = iota
	NumberGreaterThan
	NumberLessThan
	NumberGreaterThanOrEqual
	NumberLessThanOrEqual
	NumberInRange
	NumberNotEqual
)

// Matches implements Filter.
func (f *NumberFilter) Matches(value any) bool {
	v := toFloat64(value)
	switch f.Mode {
	case NumberEquals:
		return v == f.Value
	case NumberGreaterThan:
		return v > f.Value
	case NumberLessThan:
		return v < f.Value
	case NumberGreaterThanOrEqual:
		return v >= f.Value
	case NumberLessThanOrEqual:
		return v <= f.Value
	case NumberInRange:
		return v >= f.Value && v <= f.To
	case NumberNotEqual:
		return v != f.Value
	}
	return true
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	}
	return 0
}

// SetFilter filters values against a set of allowed values.
type SetFilter struct {
	Values map[string]bool
}

// Matches implements Filter.
func (f *SetFilter) Matches(value any) bool {
	if len(f.Values) == 0 {
		return true
	}
	s, ok := value.(string)
	if !ok {
		return false
	}
	return f.Values[s]
}
