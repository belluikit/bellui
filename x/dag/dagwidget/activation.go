package dagwidget

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// doubleTapRadius is how far (in screen pixels) two taps may land apart and
// still count as a double-tap.
const doubleTapRadius float32 = 12

// tapTracker detects double-taps from successive Tapped calls. Widgets that
// activate deliberately do NOT implement event.DoubleTappable: the driver
// would then delay every single tap by the double-tap interval to
// disambiguate, lagging selection on the most-clicked widgets. Tracking the
// previous tap keeps selection instant; a second tap within the delay (and
// within a small radius) additionally activates.
type tapTracker struct {
	lastTime time.Time
	lastPos  geo.Position
}

// doubleTap records a tap and reports whether it completes a double-tap.
func (t *tapTracker) doubleTap(ev *event.PointEvent) bool {
	now := time.Now()
	pos := ev.AbsolutePosition
	isDouble := !t.lastTime.IsZero() &&
		now.Sub(t.lastTime) <= bellui.DoubleTapDelay() &&
		dist32(pos, t.lastPos) <= doubleTapRadius
	if isDouble {
		// Reset so a triple tap does not activate twice.
		t.lastTime = time.Time{}
	} else {
		t.lastTime = now
	}
	t.lastPos = pos
	return isDouble
}
