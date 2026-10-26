package pipeline

import (
	"testing"
	"time"
)

// TestClockDrivenByAudio checks the presentation clock follows the audio
// position, which is what video frames are paced against.
func TestClockDrivenByAudio(t *testing.T) {
	c := NewClock()
	c.Start()
	c.SetAudioTime(5 * time.Second)

	if got := c.Time(); got < 5*time.Second || got > 5*time.Second+250*time.Millisecond {
		t.Errorf("clock = %v, want ~5s once audio drives it", got)
	}
}

// TestClockIgnoresSmallAudioRegressions guards the anti-stutter rule: audio
// positions can step back slightly after a seek rebase while the device drains
// pre-seek samples, and rewinding the clock would judder the video.
func TestClockIgnoresSmallAudioRegressions(t *testing.T) {
	c := NewClock()
	c.Start()
	c.SetAudioTime(10 * time.Second)
	c.SetAudioTime(9*time.Second + 800*time.Millisecond) // small step back

	if got := c.Time(); got < 10*time.Second {
		t.Errorf("clock = %v, want it to hold at ~10s rather than rewind", got)
	}
}

// TestClockAcceptsLargeAudioJump is the other half: a real seek moves the
// audio position far enough that the clock must follow.
func TestClockAcceptsLargeAudioJump(t *testing.T) {
	c := NewClock()
	c.Start()
	c.SetAudioTime(10 * time.Second)
	c.SetAudioTime(2 * time.Second) // beyond the 500ms tolerance

	if got := c.Time(); got > 3*time.Second {
		t.Errorf("clock = %v, want it to follow the seek back to ~2s", got)
	}
}

// TestSeekRebasesClock covers the path Seek relies on.
func TestSeekRebasesClock(t *testing.T) {
	c := NewClock()
	c.Start()
	c.SetAudioTime(30 * time.Second)
	c.Seek(90 * time.Second)

	if got := c.Time(); got < 90*time.Second || got > 91*time.Second {
		t.Errorf("clock = %v, want ~90s after seeking there", got)
	}
}

// TestTimedFrameCarriesPTS is the regression guard for the sync bug: video
// timing used to be reconstructed by counting frames, so any deviation from
// the nominal frame rate accumulated with no way back. Frames must carry the
// stream's own timestamp instead.
func TestTimedFrameCarriesPTS(t *testing.T) {
	p := &Pipeline{frameQueue: make(chan timedFrame, 2)}
	p.frameQueue <- timedFrame{img: nil, pts: 1234 * time.Millisecond}

	got := <-p.frameQueue
	if got.pts != 1234*time.Millisecond {
		t.Errorf("pts = %v, want it preserved through the queue", got.pts)
	}
}

// TestEndOfStreamSentinel documents that a nil image, not a nil channel value,
// signals end of stream now that the queue carries a struct.
func TestEndOfStreamSentinel(t *testing.T) {
	q := make(chan timedFrame, 1)
	q <- timedFrame{}
	if (<-q).img != nil {
		t.Error("zero timedFrame must represent end of stream")
	}
}

// TestClockSmoothsSteppedAudioPosition pins #504: the audio position moves in
// steps of what the mixer pulls per callback (about 90 ms for 44.1 kHz mono)
// and is sampled at irregular times. The clock used to snap to each sample and
// lurched by up to 50 ms, so video paced on it dropped frames as late. It must
// run smoothly and still track the audio.
//
// Smooth means it drifts from wall time by no more than its slew allows over
// the time between two samples. A fixed bound failed whenever the test woke
// late: a 2 ms sleep that takes 30 ms lets a correct clock drift 7.5 ms (#575).
// Each clock read is bracketed by two wall-time reads, and the bracket's width
// is allowed too: a goroutine preempted between reading the clock and reading
// the time sees a gap the clock did not.
func TestClockSmoothsSteppedAudioPosition(t *testing.T) {
	const step = 90 * time.Millisecond
	c := NewClock()
	c.Start()
	start := time.Now()
	c.SetAudioTime(0)

	// Measure from the first step on: until the audio moves, the clock holds
	// at the start and then jumps to it once, as intended (see
	// TestClockWaitsForTheAudioToStart).
	const margin = time.Millisecond
	read := func() (clock, wall, bracket time.Duration) {
		before := time.Now()
		clock = c.Time()
		after := time.Now()
		return clock, after.Sub(start), after.Sub(before)
	}
	worst, worstGap, worstBracket := time.Duration(0), time.Duration(0), time.Duration(0)
	prevClock, prevWall, prevBracket := read()
	nextUpdate := start
	for i := 0; time.Since(start) < 1500*time.Millisecond; i++ {
		time.Sleep(2 * time.Millisecond)
		now := time.Now()
		if now.After(nextUpdate) {
			// The device's position: real elapsed time, in whole steps.
			pos := now.Sub(start) / step * step
			c.SetAudioTime(pos)
			nextUpdate = now.Add(time.Duration(15+i%3*10) * time.Millisecond) // irregular, 15-35 ms
		}
		clock, wall, bracket := read()
		if wall < 2*step { // not under way yet
			prevClock, prevWall, prevBracket = clock, wall, bracket
			continue
		}
		gap := wall - prevWall
		dev := (clock - prevClock) - gap
		if dev < 0 {
			dev = -dev
		}
		// The two clock reads are each somewhere in their bracket, so the time
		// between them is the gap give or take the brackets, and the clock
		// slews over all of it.
		reads := prevBracket + bracket
		allowed := time.Duration(audioMaxSlew*float64(gap+reads)) + reads
		if excess := dev - allowed; excess > worst {
			worst, worstGap, worstBracket = excess, gap, reads
		}
		prevClock, prevWall, prevBracket = clock, wall, bracket
	}
	if worst > margin {
		t.Errorf("clock lurched by %v more than its slew allows over %v between samples (reads took %v); want it smooth",
			worst, worstGap, worstBracket)
	}
	if lag := time.Since(start) - c.Time(); lag > 2*step || lag < -step {
		t.Errorf("clock is %v behind real time; want it to track the audio within a step or two", lag)
	}
}

// TestClockWaitsForTheAudioToStart: until the device starts playing it reports
// the start position, and a clock that ran on from there put the picture ahead
// of the sound for the first second or two (#504).
func TestClockWaitsForTheAudioToStart(t *testing.T) {
	c := NewClock()
	c.Start()
	for i := 0; i < 20; i++ { // 200 ms of a device that has not started
		c.SetAudioTime(0)
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.Time(); got > 30*time.Millisecond {
		t.Errorf("clock at %v while the audio has not started; want it held near 0", got)
	}
}

// While the device has not started, the clock holds exactly where the audio
// is, however long it waits, instead of running ahead by up to one update
// interval; once the device plays, it interpolates again (#787).
func TestClockHoldsUntilTheAudioPlays(t *testing.T) {
	c := NewClock()
	c.Start()
	c.SetAudioTime(0)
	time.Sleep(30 * time.Millisecond)
	if got := c.Time(); got != 0 {
		t.Errorf("clock at %v 30ms after a device that has not started reported 0; want exactly 0", got)
	}

	c.SetAudioTime(50 * time.Millisecond) // the device has started
	time.Sleep(20 * time.Millisecond)
	if got := c.Time(); got <= 50*time.Millisecond {
		t.Errorf("clock at %v 20ms after the audio reached 50ms; want it running on past it", got)
	}
}
