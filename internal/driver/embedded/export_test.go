package embedded

import (
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
)

// StepTouchClock stamps the touch samples of w a millisecond apart, one
// millisecond per reading, so a test's flick reads as moves delivered within
// one coast step and released while still moving, however slowly a loaded
// machine delivers them (#786). The samples must not share one instant: the
// release speed is measured from the oldest sample before the release.
//
// The returned hold moves the clock on by d, for a test whose finger is held
// still: on this clock, wall time spent waiting does not count, so without it
// a drag "held" for a second is released a few milliseconds after its last
// move and coasts (#830).
func StepTouchClock(w bellui.Window) (hold func(d time.Duration)) {
	base := time.Unix(1000, 0)
	var readings, held atomic.Int64
	w.(*noosWindow).c.SetClock(func() time.Time {
		return base.Add(time.Duration(readings.Add(1))*time.Millisecond + time.Duration(held.Load()))
	})
	return func(d time.Duration) { held.Add(int64(d)) }
}
