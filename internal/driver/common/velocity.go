package common

import (
	"time"

	"github.com/belluikit/bellui/geo"
)

// Release velocity (#751). A released touch drag or scroll coasts from the
// finger's speed when it let go, measured over the last moves, not from the
// distance of the last move event alone: a drag whose last move was large but
// that was then held still before lifting must not coast.
const (
	// velocityWindow is how far back the moves that set the release speed
	// reach. Android's VelocityTracker uses the same horizon.
	velocityWindow = 100 * time.Millisecond
	// velocityStopped is how long without a move before the release makes
	// the finger count as stopped: lifting it then gives no momentum. As
	// Android's VelocityTracker assumes a pointer stopped after 40ms.
	velocityStopped = 40 * time.Millisecond
	// coastStep is the interval between two momentum steps (DragDecay,
	// DragCoast and the drivers' scroll momentum run at about 60 a second).
	// The seed is the distance the finger would cover in one step.
	coastStep = 16 * time.Millisecond

	velocitySamples = 16
)

type velocitySample struct {
	pos geo.Position
	at  time.Time
}

// velocityTracker keeps the latest positions of one contact (or of a
// two-finger gesture's midpoint) to tell how fast it was moving when released.
type velocityTracker struct {
	samples [velocitySamples]velocitySample
	n, next int
}

// reset starts a new movement at pos.
func (v *velocityTracker) reset(pos geo.Position, at time.Time) {
	v.n, v.next = 0, 0
	v.add(pos, at)
}

// add records the contact at pos.
func (v *velocityTracker) add(pos geo.Position, at time.Time) {
	v.samples[v.next] = velocitySample{pos, at}
	v.next = (v.next + 1) % velocitySamples
	if v.n < velocitySamples {
		v.n++
	}
}

// seed is the momentum seed for a release at pos: the distance the contact
// moved per coastStep over the last velocityWindow, or nothing when it had
// stopped before lifting. Movement inside a single step is not scaled up, so a
// burst of moves delivered together cannot seed more than the distance it
// covered.
func (v *velocityTracker) seed(pos geo.Position, at time.Time) geo.Delta {
	if v.n == 0 {
		return geo.Delta{}
	}
	newest := v.samples[(v.next+velocitySamples-1)%velocitySamples]
	if at.Sub(newest.at) > velocityStopped {
		return geo.Delta{} // held still before lifting
	}
	oldest := velocitySample{pos, at}
	for i := 0; i < v.n; i++ {
		s := v.samples[i]
		if at.Sub(s.at) <= velocityWindow && s.at.Before(oldest.at) {
			oldest = s
		}
	}
	span := at.Sub(oldest.at)
	if span < coastStep {
		span = coastStep
	}
	f := float32(coastStep) / float32(span)
	return geo.Delta{DX: (pos.X - oldest.pos.X) * f, DY: (pos.Y - oldest.pos.Y) * f}
}
