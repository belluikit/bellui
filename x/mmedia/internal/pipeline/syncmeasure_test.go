package pipeline

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// vfrSource emits frames whose timestamps advance at realRate while declaring
// declaredRate as the stream's frame rate.
//
// That mismatch is the whole point. Pacing video by counting frames — start at
// the clock, add 1/declaredRate per frame — drifts steadily when the real
// cadence differs, and nothing ever pulls it back. Pacing each frame against
// its own timestamp is immune. A source that is perfectly constant rate and
// correctly self-described cannot tell the two apart, so this one lies.
type vfrSource struct {
	frame        *image.RGBA
	vcb          source.VideoCallback
	pts          time.Duration
	realRate     float64
	declaredRate float64
}

func (s *vfrSource) HasVideo() bool          { return true }
func (s *vfrSource) HasAudio() bool          { return false }
func (s *vfrSource) VideoSize() (int, int)   { return 16, 16 }
func (s *vfrSource) Duration() time.Duration { return time.Minute }
func (s *vfrSource) Framerate() float64      { return s.declaredRate }
func (s *vfrSource) SampleRate() int         { return 0 }
func (s *vfrSource) Channels() int           { return 0 }

func (s *vfrSource) SetVideoCallback(f source.VideoCallback) { s.vcb = f }
func (s *vfrSource) SetAudioCallback(source.AudioCallback)   {}
func (s *vfrSource) SetAudioChannels(int) error              { return nil }
func (s *vfrSource) SetAudioSampleRate(int) error            { return nil }

func (s *vfrSource) Decode(time.Duration) {
	if s.vcb != nil {
		s.vcb(s.frame, s.pts)
		s.pts += time.Duration(float64(time.Second) / s.realRate)
	}
}

func (s *vfrSource) Seek(time.Duration) {}
func (s *vfrSource) Rewind()            {}
func (s *vfrSource) HasEnded() bool     { return false }
func (s *vfrSource) Close()             {}

// TestSyncOffsetStaysBounded is the regression guard for A/V drift. With no
// audio the clock is wall-driven, so a frame's presentation error is directly
// measurable: clock position minus the frame's timestamp.
//
// Under the previous frame-counting model this test fails — declaring 30fps
// while delivering 25fps means video time advances 33.3ms per frame against a
// real 40ms cadence, so the error grows by ~6.7ms every frame and never
// recovers.
func TestSyncOffsetStaysBounded(t *testing.T) {
	src := &vfrSource{
		frame:        image.NewRGBA(image.Rect(0, 0, 16, 16)),
		realRate:     25,
		declaredRate: 30, // deliberately wrong
	}
	p := newTestPipeline(t, src)
	p.SetOnFrame(func(*image.RGBA) {})

	p.Play()
	time.Sleep(1500 * time.Millisecond)
	st := p.Stats()
	p.Close()

	if st.FramesRendered < 10 {
		t.Fatalf("rendered %d frames, too few to judge sync", st.FramesRendered)
	}

	// Measured separation, not a guess: with timestamp pacing this run sits
	// around 1ms average; replacing it with the old frame-counting model puts
	// the average past -130ms within the same 1.5s and still climbing. 50ms
	// sits clear of scheduler jitter while leaving no room for real drift.
	const tolerance = 50 * time.Millisecond
	if st.AvgSyncOffset > tolerance || st.AvgSyncOffset < -tolerance {
		t.Errorf("average sync offset %v exceeds %v — video is drifting from the clock",
			st.AvgSyncOffset, tolerance)
	}
	t.Logf("rendered=%d dropped=%d last=%v avg=%v max=%v",
		st.FramesRendered, st.FramesDropped, st.SyncOffset, st.AvgSyncOffset, st.MaxSyncOffset)
}

// TestStatsAreRaceFree exercises the counters from a reader while playback
// writes them, which is how a consumer would poll metrics.
func TestStatsAreRaceFree(t *testing.T) {
	src := &vfrSource{
		frame:        image.NewRGBA(image.Rect(0, 0, 16, 16)),
		realRate:     30,
		declaredRate: 30,
	}
	p := newTestPipeline(t, src)
	p.SetOnFrame(func(*image.RGBA) {})

	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			_ = p.Stats()
		}
	}()

	p.Play()
	<-done
	p.Close()
}

// TestRecordSyncTracksMaxAbsolute checks the accumulator itself, including that
// a negative excursion (video ahead) still registers as a magnitude.
func TestRecordSyncTracksMaxAbsolute(t *testing.T) {
	p := &Pipeline{}
	p.recordSync(10 * time.Millisecond)
	p.recordSync(-80 * time.Millisecond)
	p.recordSync(20 * time.Millisecond)

	st := p.Stats()
	if st.MaxSyncOffset != 80*time.Millisecond {
		t.Errorf("max = %v, want 80ms (largest magnitude, sign ignored)", st.MaxSyncOffset)
	}
	if want := (10 - 80 + 20) * time.Millisecond / 3; st.AvgSyncOffset != want {
		t.Errorf("avg = %v, want %v (signed mean)", st.AvgSyncOffset, want)
	}
	if st.SyncOffset != 20*time.Millisecond {
		t.Errorf("last = %v, want 20ms", st.SyncOffset)
	}
}
