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
	p := newPipelineForSource(src, Config{})

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
