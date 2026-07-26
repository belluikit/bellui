package dataset

import "time"

// TimeInterval represents a regular time step for gap filling.
type TimeInterval int

const (
	IntervalDay TimeInterval = iota
	IntervalWeek
	IntervalMonth
	IntervalYear
	IntervalHour
	IntervalMinute
)

// ZeroFillTimeSeries fills gaps in a time series with zero values.
// The input timestamps and values must be sorted by time.
// Returns new timestamps and values with gaps filled at the given interval.
func ZeroFillTimeSeries(timestamps []time.Time, values []float64, interval TimeInterval) ([]time.Time, []float64) {
	if len(timestamps) == 0 || len(timestamps) != len(values) {
		return timestamps, values
	}

	var filledTimes []time.Time
	var filledValues []float64

	// Build a map of existing data
	existing := make(map[string]float64)
	for i, t := range timestamps {
		key := timeKey(t, interval)
		existing[key] = values[i]
	}

	// Walk from first to last at the given interval
	current := truncateTime(timestamps[0], interval)
	end := truncateTime(timestamps[len(timestamps)-1], interval)

	for !current.After(end) {
		key := timeKey(current, interval)
		if val, ok := existing[key]; ok {
			filledTimes = append(filledTimes, current)
			filledValues = append(filledValues, val)
		} else {
			filledTimes = append(filledTimes, current)
			filledValues = append(filledValues, 0) // zero-fill the gap
		}
		current = advanceTime(current, interval)
	}

	return filledTimes, filledValues
}

func truncateTime(t time.Time, interval TimeInterval) time.Time {
	switch interval {
	case IntervalMinute:
		return t.Truncate(time.Minute)
	case IntervalHour:
		return t.Truncate(time.Hour)
	case IntervalDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case IntervalWeek:
		// Truncate to Monday
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return d.AddDate(0, 0, -(weekday - 1))
	case IntervalMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case IntervalYear:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	}
	return t
}

func advanceTime(t time.Time, interval TimeInterval) time.Time {
	switch interval {
	case IntervalMinute:
		return t.Add(time.Minute)
	case IntervalHour:
		return t.Add(time.Hour)
	case IntervalDay:
		return t.AddDate(0, 0, 1)
	case IntervalWeek:
		return t.AddDate(0, 0, 7)
	case IntervalMonth:
		return t.AddDate(0, 1, 0)
	case IntervalYear:
		return t.AddDate(1, 0, 0)
	}
	return t.AddDate(0, 0, 1)
}

func timeKey(t time.Time, interval TimeInterval) string {
	switch interval {
	case IntervalMinute:
		return t.Format("2006-01-02T15:04")
	case IntervalHour:
		return t.Format("2006-01-02T15")
	case IntervalDay:
		return t.Format("2006-01-02")
	case IntervalWeek:
		return t.Format("2006-01-02")
	case IntervalMonth:
		return t.Format("2006-01")
	case IntervalYear:
		return t.Format("2006")
	}
	return t.Format("2006-01-02")
}
