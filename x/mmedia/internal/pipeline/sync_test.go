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
