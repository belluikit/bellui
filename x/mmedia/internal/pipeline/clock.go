package pipeline

import (
	"sync"
	"time"
)

// Clock provides the presentation clock for A/V synchronization.
// When audio is present, the audio output drives the clock; between audio
// updates the position is interpolated with wall time so consumers see a
// smooth clock instead of one that advances in audio-chunk-sized steps.
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
}

func NewClock() *Clock {
	return &Clock{}
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
		c.audioTime += time.Since(c.audioSetAt)
		c.audioSetAt = time.Now()
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
	c.useAudio = false
}

// SetAudioTime sets the clock position from the audio subsystem.
func (c *Clock) SetAudioTime(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Audio positions can briefly step backwards (e.g. right after a seek
	// rebase while oto still drains pre-seek samples); never rewind the
	// clock for small regressions or video pacing would stutter.
	if c.useAudio && d < c.audioTime && c.audioTime-d < 500*time.Millisecond {
		return
	}
	c.audioTime = d
	c.audioSetAt = time.Now()
	c.useAudio = true
}

// Time returns the current playback position.
func (c *Clock) Time() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.useAudio {
		if c.running {
			// Interpolate between audio updates for a smooth clock.
			return c.audioTime + time.Since(c.audioSetAt)
		}
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
}
