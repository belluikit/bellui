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
	if !vp.IsPlaying() {
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
