package widget

import "time"

// dragSpeed measures how fast a drag was moving when the finger lifted, as
// Android's VelocityTracker does: over the samples of the last
// carouselVelocityWindow, and zero when the finger had stopped for
// carouselStoppedAfter. Carousel, WheelPicker and SwipeArea share it.
type dragSpeed struct {
	samples []dragSample
}

type dragSample struct {
	at time.Time
	d  float32 // the drag's accumulated travel at that time
}

// reset forgets the samples of the last gesture.
func (s *dragSpeed) reset() { s.samples = s.samples[:0] }

// add records that the drag had travelled d by now.
func (s *dragSpeed) add(now time.Time, d float32) {
	s.samples = append(s.samples, dragSample{at: now, d: d})
	for len(s.samples) > 2 && now.Sub(s.samples[0].at) > carouselVelocityWindow {
		s.samples = s.samples[1:]
	}
}

// at is the drag's speed, in points per second, over the last
// carouselVelocityWindow before now; zero when the finger had stopped.
func (s *dragSpeed) at(now time.Time) float32 {
	n := len(s.samples)
	if n < 2 {
		return 0
	}
	last := s.samples[n-1]
	if now.Sub(last.at) > carouselStoppedAfter {
		return 0
	}
	first := s.samples[0]
	for _, p := range s.samples[:n-1] {
		if now.Sub(p.at) <= carouselVelocityWindow {
			first = p
			break
		}
	}
	dt := last.at.Sub(first.at).Seconds()
	if dt <= 0 {
		return 0
	}
	return (last.d - first.d) / float32(dt)
}
