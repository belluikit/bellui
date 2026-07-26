package filter

import "time"

// DateFilter filters date/time values.
type DateFilter struct {
	Mode   DateFilterMode
	Date   time.Time // reference date for comparison
	ToDate time.Time // for range mode
}

// DateFilterMode specifies how date values are compared.
type DateFilterMode int

const (
	DateEquals DateFilterMode = iota
	DateBefore
	DateAfter
	DateInRange
	DateNotEquals
)

// Matches implements Filter.
func (f *DateFilter) Matches(value any) bool {
	t := toTime(value)
	if t.IsZero() {
		return false
	}
	switch f.Mode {
	case DateEquals:
		return sameDay(t, f.Date)
	case DateBefore:
		return t.Before(f.Date)
	case DateAfter:
		return t.After(f.Date)
	case DateInRange:
		return !t.Before(f.Date) && !t.After(f.ToDate)
	case DateNotEquals:
		return !sameDay(t, f.Date)
	}
	return true
}

func toTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case *time.Time:
		if t != nil {
			return *t
		}
	case string:
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			return parsed
		}
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
