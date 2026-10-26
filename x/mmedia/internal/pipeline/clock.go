package pipeline

import (
	"sync"
	"time"
)

// Clock provides the presentation clock for A/V synchronization.
// When audio is present, the audio output drives the clock; between audio
// updates the position is interpolated with wall time so consumers see a
// smooth clock instead of one that advances in audio-chunk-sized steps. Until
// the device starts playing it holds where the audio is (see acquiring).
// When only video is present, wall-clock time is used.
type Clock struct {
	mu        sync.RWMutex
	startTime time.Time
	offset    time.Duration // accumulated time before last resume
	running   bool

	// Audio-driven clock: set externally by the audio output layer.
	audioTime  time.Duration
	audioSetAt time.Time // when audioTime was last updated
	useAudio   bool
	// audioRate is how fast the audio-driven clock runs against wall time
	// since audioSetAt: 1 when it agrees with the audio, a little off 1 while
	// it catches up with it (see SetAudioTime).
	audioRate float64
	// acquiring holds the clock to the reported audio position, snapping
	// rather than following smoothly, until that position first moves past
	// acquireFrom: the device reports where playback starts until it really
	// starts, and running on from there put the picture ahead of the sound for
	// the first second or two.
	acquiring   bool
	acquireFrom time.Duration
}

func NewClock() *Clock {
	return &Clock{audioRate: 1}
}

// audioNow is the audio-driven position at now. Caller holds mu.
func (c *Clock) audioNow(now time.Time) time.Duration {
	rate := c.audioRate
	if rate == 0 { // a zero Clock, not made by NewClock
		rate = 1
	}
	return c.audioTime + time.Duration(float64(now.Sub(c.audioSetAt))*rate)
}

// Start begins or resumes the clock.
func (c *Clock) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		c.startTime = time.Now()
		c.audioSetAt = time.Now()
		c.running = true
	}
}

// Pause freezes the clock, accumulating elapsed time.
func (c *Clock) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		c.offset += time.Since(c.startTime)
		now := time.Now()
		c.audioTime = c.audioNow(now)
		c.audioSetAt = now
		c.running = false
	}
}

// Reset stops the clock and zeros it.
func (c *Clock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset = 0
	c.running = false
	c.audioTime = 0
	c.audioRate = 1
	c.useAudio = false
}

// Following the audio. The device's position moves in steps of whatever the
// mixer pulls per callback (about 90 ms for 44.1 kHz mono), and it is sampled
// whenever the decoder happens to write audio. Snapping the clock to each
// sample made it lurch by up to 50 ms either way, and video paced on it dropped
// about eight frames a second as late (#504). So the clock keeps running and
// changes its rate instead: a little slower while it is ahead of the audio, a
// little faster while behind, by at most audioMaxSlew. It never steps back for
// a small error: audio positions dip briefly after a seek rebase while the
// device drains pre-seek samples, and rewinding would judder the video. Only a
// large error is taken as a jump in the audio itself (a seek, a stall) and
// followed at once.
const (
	audioJumpAhead = 200 * time.Millisecond // audio further ahead than this: jump to it
	audioJumpBack  = 500 * time.Millisecond // audio further behind than this: jump back to it
	audioSettle    = 500 * time.Millisecond // an error is worked off over about this long
	audioMaxSlew   = 0.25                   // at most 25% off wall-clock speed
)

// SetAudioTime sets the clock position from the audio subsystem.
func (c *Clock) SetAudioTime(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if !c.useAudio || !c.running {
		if !c.useAudio {
			c.acquiring, c.acquireFrom = true, d
		}
		c.audioTime, c.audioSetAt, c.audioRate, c.useAudio = d, now, 1, true
		return
	}
	if c.acquiring {
		if d > c.acquireFrom {
			c.acquiring = false // the device is playing: follow it smoothly from here
		}
		if d < c.audioTime && c.audioTime-d <= audioJumpBack {
			d = c.audioTime // hold rather than step back a little (see audioJumpBack)
		}
		c.audioTime, c.audioSetAt, c.audioRate = d, now, 1
		return
	}
	cur := c.audioNow(now)
	diff := d - cur
	if diff > audioJumpAhead || diff < -audioJumpBack {
		c.audioTime, c.audioSetAt, c.audioRate = d, now, 1
		return
	}
	rate := 1 + float64(diff)/float64(audioSettle)
	c.audioRate = min(max(rate, 1-audioMaxSlew), 1+audioMaxSlew)
	c.audioTime, c.audioSetAt = cur, now
}

// Time returns the current playback position.
func (c *Clock) Time() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.useAudio {
		if c.running && !c.acquiring {
			// Interpolate between audio updates for a smooth clock.
			return c.audioNow(time.Now())
		}
		// Paused, or the device has not started playing: hold where the
		// audio is. Interpolating while acquiring ran the clock ahead of a
		// device that had not started, by up to one update interval (#787).
		return c.audioTime
	}
	if c.running {
		return c.offset + time.Since(c.startTime)
	}
	return c.offset
}

// Seek sets the clock to a specific position.
func (c *Clock) Seek(pos time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset = pos
	c.startTime = time.Now()
	c.audioTime = pos
	c.audioSetAt = time.Now()
	c.audioRate = 1
	c.acquiring, c.acquireFrom = c.useAudio, pos
}
