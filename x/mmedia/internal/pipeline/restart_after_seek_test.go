package pipeline

import (
	"testing"
	"time"
)

// A restart plays from the start, whatever was seeked to before it. The audio
// output kept the last seek's base across a Stop while its byte count went back
// to zero, so after a restart the clock read the old seek target until as much
// audio had played again as had before that seek: the picture held there for
// seconds (#833).
func TestRestartAfterSeekStartsFromZero(t *testing.T) {
	p, err := NewPipelineURL("../source/testdata/sample.mp4") // 4 s, video and audio
	if err != nil {
		t.Fatalf("NewPipelineURL: %v", err)
	}
	defer p.Close()
	if !p.HasAudio() || p.audioOutput == nil {
		t.Skip("no audio device: the audio clock is what this pins")
	}

	p.Play()
	time.Sleep(1500 * time.Millisecond)
	p.Seek(3 * time.Second)
	time.Sleep(300 * time.Millisecond)
	p.Stop()
	p.Play()

	var samples []time.Duration
	for range 8 {
		time.Sleep(100 * time.Millisecond)
		samples = append(samples, p.CurrentTime())
	}
	if last := samples[len(samples)-1]; last > 1500*time.Millisecond {
		t.Errorf("clock after a restart: %v; want it running from zero, not from the earlier seek's 3s", samples)
	}
	if samples[len(samples)-1] <= samples[2] {
		t.Errorf("clock after a restart: %v; want it moving", samples)
	}
}
