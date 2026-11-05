package mmedia

import (
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/audio"
)

// TestLevelWhilePlayingAndPaused pins AudioPlayback.Level (player#258): above
// zero while a tone plays through a real device, and 0 once paused. Without a
// device the players play silent (TestMain), so there is nothing to measure.
func TestLevelWhilePlayingAndPaused(t *testing.T) {
	if out, err := audio.NewOutput(2); err != nil {
		t.Skipf("no audio device: %v", err)
	} else {
		out.Stop()
	}
	f, err := os.Open("internal/source/testdata/long.mp3")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pb, err := NewAudioPlayback(f)
	if err != nil {
		t.Fatal(err)
	}
	defer pb.Close()

	if got := pb.Level(); got != 0 {
		t.Errorf("level %v before Play", got)
	}
	pb.Play()
	var peak float32
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && peak < 0.01; time.Sleep(20 * time.Millisecond) {
		peak = max(peak, pb.Level())
	}
	if peak < 0.01 {
		t.Fatalf("level stayed at %v for 3 s of a playing tone", peak)
	}
	t.Logf("level while playing: %.3f", peak)

	pb.Pause()
	if got := pb.Level(); got != 0 {
		t.Errorf("level %v while paused", got)
	}
}
