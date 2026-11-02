package pipeline

import (
	"image"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// timeLog records the time updates a pipeline sends.
type timeLog struct {
	mu  sync.Mutex
	got []time.Duration
}

func (l *timeLog) record(current, _ time.Duration) {
	l.mu.Lock()
	l.got = append(l.got, current)
	l.mu.Unlock()
}

// since returns the updates after the first n.
func (l *timeLog) since(n int) []time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]time.Duration(nil), l.got[n:]...)
}

func (l *timeLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.got)
}

// shortSong is a finite audio-only stream that carries no samples, so it plays
// to its end on a detached device without a sound card.
type shortSong struct {
	decodes int
}

func (s *shortSong) HasVideo() bool                        { return false }
func (s *shortSong) HasAudio() bool                        { return true }
func (s *shortSong) VideoSize() (int, int)                 { return 0, 0 }
func (s *shortSong) Duration() time.Duration               { return 300 * time.Millisecond }
func (s *shortSong) Framerate() float64                    { return 0 }
func (s *shortSong) SampleRate() int                       { return 48000 }
func (s *shortSong) Channels() int                         { return 2 }
func (s *shortSong) SetVideoCallback(source.VideoCallback) {}
func (s *shortSong) SetAudioCallback(source.AudioCallback) {}
func (s *shortSong) SetAudioChannels(int) error            { return nil }
func (s *shortSong) SetAudioSampleRate(int) error          { return nil }
func (s *shortSong) Decode(time.Duration)                  { s.decodes++; time.Sleep(10 * time.Millisecond) }
func (s *shortSong) Seek(time.Duration)                    {}
func (s *shortSong) Rewind()                               { s.decodes = 0 }
func (s *shortSong) HasEnded() bool                        { return s.decodes >= 30 }
func (s *shortSong) Close()                                {}

// TestNoTimeUpdatesAfterTheEnd pins #879: an audio-only stream's display loop
// outlives the end, and its ticker went on reporting the end position ten
// times a second for as long as the player stayed open.
func TestNoTimeUpdatesAfterTheEnd(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	p := newTestPipeline(t, &shortSong{})
	defer p.Close()

	var log timeLog
	p.SetOnTime(log.record)
	ended := make(chan struct{})
	p.SetOnEnd(func() { close(ended) })

	p.Play()
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("the song did not end")
	}
	atEnd := log.len()

	time.Sleep(500 * time.Millisecond) // five ticks' worth
	if after := log.since(atEnd); len(after) > 0 {
		t.Errorf("%d time updates after the end, want none: %v", len(after), after)
	}
}

// TestPauseAndSeekReportOnceEach pins #879's other half: while paused the
// ticker reported the same position every 100 ms. Now a pause reports once,
// where playback rests, and so does each seek made while paused, since nothing
// else would tell a progress bar or a script that the position moved.
func TestPauseAndSeekReportOnceEach(t *testing.T) {
	p := newTestPipeline(t, &clipSource{frames: 300})
	defer p.Close()
	p.SetOnFrame(func(*image.RGBA) {})

	var log timeLog
	p.SetOnTime(log.record)

	p.Play()
	time.Sleep(300 * time.Millisecond)
	if log.len() == 0 {
		t.Fatal("no time updates while playing")
	}

	before := log.len()
	p.Pause()
	held := p.CurrentTime()
	paused := log.since(before)
	// A tick that saw the state playing just before Pause may land as well;
	// Pause's own report is the one that must be there.
	if len(paused) == 0 || paused[len(paused)-1] != held {
		t.Fatalf("updates from Pause: %v; want the last at the resting position %v", paused, held)
	}

	time.Sleep(50 * time.Millisecond) // let a racing tick land
	quiet := log.len()
	time.Sleep(500 * time.Millisecond)
	if idle := log.since(quiet); len(idle) > 0 {
		t.Errorf("%d time updates while paused, want none: %v", len(idle), idle)
	}

	before = log.len()
	p.Seek(5 * time.Second)
	if got := log.since(before); len(got) != 1 || got[0] != 5*time.Second {
		t.Errorf("updates from a seek while paused: %v; want one, at 5s", got)
	}
}
