package mmedia

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// shortClip is a finite 30 fps video-only stream of a few frames.
type shortClip struct {
	vcb    source.VideoCallback
	frames int
	next   int
}

func (s *shortClip) HasVideo() bool          { return true }
func (s *shortClip) HasAudio() bool          { return false }
func (s *shortClip) VideoSize() (int, int)   { return 16, 16 }
func (s *shortClip) Duration() time.Duration { return time.Duration(s.frames) * time.Second / 30 }
func (s *shortClip) Framerate() float64      { return 30 }
func (s *shortClip) SampleRate() int         { return 0 }
func (s *shortClip) Channels() int           { return 0 }
func (s *shortClip) SetVideoCallback(f source.VideoCallback) {
	s.vcb = f
}
func (s *shortClip) SetAudioCallback(source.AudioCallback) {}
func (s *shortClip) SetAudioChannels(int) error            { return nil }
func (s *shortClip) Decode(time.Duration) {
	if s.next < s.frames && s.vcb != nil {
		s.vcb(image.NewRGBA(image.Rect(0, 0, 16, 16)), time.Duration(s.next)*time.Second/30)
		s.next++
	}
}
func (s *shortClip) Seek(pos time.Duration) { s.next = int(pos * 30 / time.Second) }
func (s *shortClip) Rewind()                { s.next = 0 }
func (s *shortClip) HasEnded() bool         { return s.next >= s.frames }
func (s *shortClip) Close()                 {}

// TestLoopKeepsPlaying pins #502: the loop restart ran Stop on the pipeline's
// own display goroutine, which Stop waits for, so the first wrap deadlocked.
// After it the picture froze, the time ran past the end, and Pause had no
// effect on the clock.
func TestLoopKeepsPlaying(t *testing.T) {
	test.NewTempApp(t)
	clip := &shortClip{frames: 15} // half a second
	mp, err := newMediaPlayerFromPipeline(pipeline.NewPipelineForSource(clip), buildOptions([]Option{WithLoop()}), "")
	if err != nil {
		t.Fatal(err)
	}
	defer mp.Close()

	mp.Play()
	time.Sleep(1600 * time.Millisecond) // three wraps

	before := mp.Stats().FramesRendered
	time.Sleep(300 * time.Millisecond)
	if after := mp.Stats().FramesRendered; after <= before {
		t.Fatalf("no frames rendered after the loop wrapped (%d, then %d)", before, after)
	}
	if now := mp.CurrentTime(); now > mp.Duration() {
		t.Errorf("time %v ran past the %v clip", now, mp.Duration())
	}

	mp.Pause()
	if mp.IsPlaying() {
		t.Error("still playing after Pause")
	}
	held := mp.CurrentTime()
	time.Sleep(200 * time.Millisecond)
	if now := mp.CurrentTime(); now != held {
		t.Errorf("time moved while paused: %v, then %v", held, now)
	}
}

// TestVideoPlayerLoopKeepsPlaying is TestLoopKeepsPlaying for VideoPlayer,
// whose loop restart had the same fault (#502).
func TestVideoPlayerLoopKeepsPlaying(t *testing.T) {
	test.NewTempApp(t)
	clip := &shortClip{frames: 15}
	vp, err := newVideoPlayerFromPipeline(pipeline.NewPipelineForSource(clip), buildOptions([]Option{WithLoop()}))
	if err != nil {
		t.Fatal(err)
	}
	defer vp.Close()

	vp.Play()
	time.Sleep(1600 * time.Millisecond) // three wraps
	if !playingWithin(vp.IsPlaying, 500*time.Millisecond) {
		t.Error("not playing after the loop wrapped")
	}
	if now := vp.CurrentTime(); now > vp.Duration() {
		t.Errorf("time %v ran past the %v clip", now, vp.Duration())
	}
	vp.Pause()
	held := vp.CurrentTime()
	time.Sleep(200 * time.Millisecond)
	if now := vp.CurrentTime(); vp.IsPlaying() || now != held {
		t.Errorf("after Pause: playing=%v, time %v then %v", vp.IsPlaying(), held, now)
	}
}

// playingWithin reports whether isPlaying turns true within d. A looping
// player is briefly not playing while it restarts at a wrap, so a single look
// can land in that gap.
func playingWithin(isPlaying func() bool, d time.Duration) bool {
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		if isPlaying() {
			return true
		}
	}
	return false
}

// closeWithin closes a player, failing the test rather than hanging it when a
// pipeline goroutine is stuck (the #527 AudioPlayer deadlock).
func closeWithin(t *testing.T, closeFn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { closeFn(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("Close did not return: a pipeline goroutine is stuck")
	}
}

// TestAudioPlayerLoopKeepsPlaying is TestLoopKeepsPlaying for AudioPlayer.
// Its loop restart ran Stop on the pipeline goroutine that reported the end,
// which Stop waits for, so the first wrap deadlocked (#527, as #502 was for
// the other players).
func TestAudioPlayerLoopKeepsPlaying(t *testing.T) {
	test.NewTempApp(t)
	clip := &shortClip{frames: 15}
	ap, err := newAudioPlayerFromPipeline(pipeline.NewPipelineForSource(clip), buildOptions([]Option{WithLoop()}))
	if err != nil {
		t.Fatal(err)
	}
	defer closeWithin(t, ap.Close)

	ap.Play()
	time.Sleep(1600 * time.Millisecond) // three wraps
	if !playingWithin(ap.IsPlaying, 500*time.Millisecond) {
		t.Error("not playing after the loop wrapped")
	}
	if now := ap.CurrentTime(); now > ap.Duration() {
		t.Errorf("time %v ran past the %v clip", now, ap.Duration())
	}
}

// TestLoopRestartRespectsPause pins #527's rule, which every player's loop
// restart goes through: a Pause or Stop before the restart prevents it, so a
// pause that lands as the clip wraps is not undone. Once Play has asked for
// playback again, the restart happens.
func TestLoopRestartRespectsPause(t *testing.T) {
	test.NewTempApp(t)
	mp, err := newMediaPlayerFromPipeline(pipeline.NewPipelineForSource(&shortClip{frames: 15}), buildOptions([]Option{WithLoop()}), "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeWithin(t, mp.Close)

	mp.Play()
	mp.Pause()
	held := mp.CurrentTime()
	if mp.transport.restart(mp.pipe) {
		t.Error("the loop restarted a paused player")
	}
	if mp.IsPlaying() {
		t.Error("playing after a restart of a paused player")
	}
	time.Sleep(100 * time.Millisecond)
	if now := mp.CurrentTime(); now != held {
		t.Errorf("time moved while paused: %v, then %v", held, now)
	}

	mp.Stop()
	if mp.transport.restart(mp.pipe) {
		t.Error("the loop restarted a stopped player")
	}

	mp.Play()
	if !mp.transport.restart(mp.pipe) {
		t.Error("the loop did not restart a playing player")
	}
	if !mp.IsPlaying() {
		t.Error("not playing after the restart")
	}
}
