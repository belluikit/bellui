package pipeline

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// stubSource decodes one synthetic frame per call, forever.
type stubSource struct {
	frame  *image.RGBA
	vcb    source.VideoCallback
	pts    time.Duration // advances like a real 30fps stream
	closed bool
}

func (s *stubSource) HasVideo() bool          { return true }
func (s *stubSource) HasAudio() bool          { return false }
func (s *stubSource) VideoSize() (int, int)   { return 16, 16 }
func (s *stubSource) Duration() time.Duration { return 0 }
func (s *stubSource) Framerate() float64      { return 30 }
func (s *stubSource) SampleRate() int         { return 0 }
func (s *stubSource) Channels() int           { return 0 }
func (s *stubSource) SetVideoCallback(f source.VideoCallback) {
	s.vcb = f
}
func (s *stubSource) SetAudioCallback(source.AudioCallback) {}
func (s *stubSource) SetAudioChannels(int) error            { return nil }
func (s *stubSource) SetAudioSampleRate(int) error          { return nil }
func (s *stubSource) Decode(time.Duration) {
	if s.vcb != nil {
		s.vcb(s.frame, s.pts)
		s.pts += time.Second / 30
	}
}
func (s *stubSource) Seek(time.Duration) {}
func (s *stubSource) Rewind()            {}
func (s *stubSource) HasEnded() bool     { return false }
func (s *stubSource) Close()             { s.closed = true }

// TestPipeline_CloseDuringPlayback guards against the wg.Wait deadlock:
// Stop set p.stopCh to nil while decodeAhead/displayLoop still selected on
// the field, and a select on a nil channel blocks forever.
func TestPipeline_CloseDuringPlayback(t *testing.T) {
	// Build a pipeline around the stub via the exported surface.
	src := &stubSource{frame: image.NewRGBA(image.Rect(0, 0, 16, 16))}
	p := newTestPipeline(t, src)

	p.Play()
	time.Sleep(200 * time.Millisecond) // let the loops spin

	done := make(chan struct{})
	go func() {
		p.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close deadlocked during playback")
	}

	if !src.closed {
		t.Error("source was not closed")
	}
}

// TestStopReturnsEveryFrameToThePool covers a Stop that lands while the queue
// is full, which is where the decoder sits for most of playback: blocked
// handing over a frame it has already taken from the pool. That frame used to
// be lost, and with one frame short the next Play found the pool empty on every
// decode and dropped about twenty million frames in 300 ms.
func TestStopReturnsEveryFrameToThePool(t *testing.T) {
	src := &stubSource{frame: image.NewRGBA(image.Rect(0, 0, 16, 16))}
	p := newTestPipeline(t, src)
	p.SetOnFrame(func(*image.RGBA) {})
	defer p.Close()
	pool := cap(p.frameRecycle)

	for cycle := 0; cycle < 2; cycle++ {
		p.Play()
		deadline := time.Now().Add(2 * time.Second)
		for len(p.frameQueue) < cap(p.frameQueue) && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if len(p.frameQueue) < cap(p.frameQueue) {
			t.Fatalf("cycle %d: queue never filled (%d of %d)", cycle, len(p.frameQueue), cap(p.frameQueue))
		}
		p.Stop()
		if got := len(p.frameRecycle) + len(p.frameQueue); got != pool {
			t.Errorf("cycle %d: %d of %d frames back in the pool after Stop", cycle, got, pool)
		}
	}
}
