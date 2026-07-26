// Package mathutil provides math helpers for chart layout and axis computation.
package mathutil

import "math"

// Clamp restricts v to the range [lo, hi].
func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ClampF32 restricts v to the range [lo, hi] for float32.
func ClampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Lerp linearly interpolates between a and b by t in [0, 1].
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// LerpF32 linearly interpolates between a and b by t in [0, 1] for float32.
func LerpF32(a, b, t float32) float32 {
	return a + (b-a)*t
}

// InverseLerp returns t such that Lerp(a, b, t) == v.
// Returns 0 if a == b.
func InverseLerp(a, b, v float64) float64 {
	if a == b {
		return 0
	}
	return (v - a) / (b - a)
}

// NiceNumber rounds a number to a "nice" value for axis tick intervals.
// If round is true, it rounds to a nice number; otherwise it takes the ceiling.
func NiceNumber(x float64, round bool) float64 {
	if x == 0 {
		return 0
	}
	neg := x < 0
	if neg {
		x = -x
	}
	exp := math.Floor(math.Log10(x))
	frac := x / math.Pow(10, exp)

	var nice float64
	if round {
		switch {
		case frac < 1.5:
			nice = 1
		case frac < 3:
			nice = 2
		case frac < 7:
			nice = 5
		default:
			nice = 10
		}
	} else {
		switch {
		case frac <= 1:
			nice = 1
		case frac <= 2:
			nice = 2
		case frac <= 5:
			nice = 5
		default:
			nice = 10
		}
	}

	result := nice * math.Pow(10, exp)
	if neg {
		return -result
	}
	return result
}

// NiceTicks computes nice axis tick values for a data range.
// It returns the adjusted min, max, and tick interval.
func NiceTicks(dataMin, dataMax float64, targetCount int) (niceMin, niceMax, interval float64) {
	if targetCount <= 0 {
		targetCount = 5
	}
	if dataMin == dataMax {
		if dataMin == 0 {
			return -1, 1, 0.5
		}
		delta := math.Abs(dataMin) * 0.1
		dataMin -= delta
		dataMax += delta
	}

	rang := NiceNumber(dataMax-dataMin, false)
	interval = NiceNumber(rang/float64(targetCount), true)

	niceMin = math.Floor(dataMin/interval) * interval
	niceMax = math.Ceil(dataMax/interval) * interval

	return niceMin, niceMax, interval
}

// TickValues generates the slice of tick values between min and max (inclusive)
// with the given interval.
func TickValues(min, max, interval float64) []float64 {
	if interval <= 0 {
		return []float64{min}
	}
	count := int(math.Round((max-min)/interval)) + 1
	if count > 1000 {
		count = 1000
	}
	ticks := make([]float64, 0, count)
	for v := min; v <= max+interval*0.001; v += interval {
		ticks = append(ticks, v)
	}
	return ticks
}
