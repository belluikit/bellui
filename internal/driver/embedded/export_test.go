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
func StepTouchClock(w bellui.Window) {
	base := time.Unix(1000, 0)
	var readings atomic.Int64
	w.(*noosWindow).c.SetClock(func() time.Time {
		return base.Add(time.Duration(readings.Add(1)) * time.Millisecond)
	})
}
