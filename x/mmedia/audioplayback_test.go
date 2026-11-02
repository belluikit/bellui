package mmedia

import (
	"image"
	"sync"
	"testing"
	"time"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// clipPipeline stands in for an audio pipeline with shortClip, which needs no
// audio device. A video pipeline with nothing taking its frames runs through
// the clip at once, so this one is given somewhere to put them; a real audio
// pipeline has no video and plays at the device's pace.
func clipPipeline(t *testing.T, frames int) *pipeline.Pipeline {
	t.Helper()
	p := newTestPipeline(t, &shortClip{frames: frames})
	p.SetOnFrame(func(*image.RGBA) {})
	return p
}

// TestAudioPlaybackPlaysWithNoApp is #881's first half: an AudioPlayback plays,
// seeks and ends with no app, window or widget.
func TestAudioPlaybackPlaysWithNoApp(t *testing.T) {
	var mu sync.Mutex
	var last time.Duration
	pb := newAudioPlaybackFromPipeline(clipPipeline(t, 300), buildOptions([]Option{
		WithOnTimeUpdate(func(current, _ time.Duration) {
			mu.Lock()
			last = current
			mu.Unlock()
		}),
	}))
	defer closeWithin(t, pb.Close)
	reported := func() time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return last
	}

	pb.Play()
	if !playingWithin(pb.IsPlaying, 500*time.Millisecond) {
		t.Fatal("not playing after Play")
	}
	pb.Seek(5 * time.Second)
	time.Sleep(300 * time.Millisecond)
	if now := pb.CurrentTime(); now <= 5*time.Second || reported() < 5*time.Second {
		t.Errorf("time %v, last reported %v, 300ms after a seek to 5s; want both past it", now, reported())
	}

	pb.Pause()
	if pb.IsPlaying() {
		t.Error("playing after Pause")
	}
	if held := pb.CurrentTime(); reported() != held {
		t.Errorf("last reported %v after Pause; want where it rests, %v", reported(), held)
	}
}

// TestAudioPlaybackEndsWithNoApp: the end reaches the app's OnEnd with no app
// running, at the full duration.
func TestAudioPlaybackEndsWithNoApp(t *testing.T) {
	ended := make(chan struct{})
	pb := newAudioPlaybackFromPipeline(clipPipeline(t, 15), buildOptions([]Option{
		WithAutoPlay(),
		WithOnEnd(func() { close(ended) }),
	}))
	defer closeWithin(t, pb.Close)
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("did not end")
	}
	if pb.IsPlaying() || pb.CurrentTime() != pb.Duration() {
		t.Errorf("after the end: playing %v, time %v of %v", pb.IsPlaying(), pb.CurrentTime(), pb.Duration())
	}
}

// TestAudioPlayerAttachesWithoutInterrupting is #881's second half: widgets
// come and go on a playing AudioPlayback, as screens do, and it plays on.
func TestAudioPlayerAttachesWithoutInterrupting(t *testing.T) {
	test.NewTempApp(t)
	pb := newAudioPlaybackFromPipeline(clipPipeline(t, 300), buildOptions(nil))
	defer closeWithin(t, pb.Close)
	pb.Play()
	time.Sleep(200 * time.Millisecond)

	first := NewAudioPlayerFor(pb)
	if !first.controls.playing {
		t.Error("a widget attached to a playing playback shows it stopped")
	}
	if first.Playback() != pb {
		t.Error("Playback is not the playback the widget was made for")
	}

	// Another holder of the playback pauses and resumes it; the widget follows.
	pb.Pause()
	if first.controls.playing {
		t.Error("the widget did not follow a Pause made on the playback")
	}
	pb.Play()

	first.Detach()
	if first.Playback() != nil {
		t.Error("Playback is not nil after Detach")
	}
	label := first.controls.timeLabel.Text
	before := pb.CurrentTime()
	time.Sleep(300 * time.Millisecond)
	if !pb.IsPlaying() || pb.CurrentTime() <= before {
		t.Fatalf("Detach interrupted playback: playing %v, time %v then %v", pb.IsPlaying(), before, pb.CurrentTime())
	}
	if first.controls.timeLabel.Text != label {
		t.Error("a detached widget still shows the playback's time")
	}
	first.Play() // detached: does nothing, and must not panic

	// A widget the app attached does not close the playback with it.
	second := NewAudioPlayerFor(pb)
	second.Close()
	if !pb.IsPlaying() {
		t.Error("closing an attached widget stopped the playback")
	}
}

// TestAudioPlayerClosesItsOwnPlayback: the widget's own constructors open a
// playback for it, and Close still closes that, as before #881.
func TestAudioPlayerClosesItsOwnPlayback(t *testing.T) {
	test.NewTempApp(t)
	ap, err := newAudioPlayerFromPipeline(clipPipeline(t, 300), buildOptions(nil))
	if err != nil {
		t.Fatal(err)
	}
	pb := ap.Playback()
	ap.Play()
	if !playingWithin(pb.IsPlaying, 500*time.Millisecond) {
		t.Fatal("not playing after Play")
	}
	closeWithin(t, ap.Close)
	if pb.IsPlaying() {
		t.Error("the widget's own playback still plays after Close")
	}
}
