package mmedia

import (
	"image"
	"slices"
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

// TestAudioPlayerSeekBarReportsTheSeek is #1051: a drag on the widget's seek
// bar reaches WithOnSeek once, with where playback was and where it went, and
// before the time update that reports the new position.
func TestAudioPlayerSeekBarReportsTheSeek(t *testing.T) {
	test.NewTempApp(t)
	var mu sync.Mutex
	var events []string
	type seek struct{ from, to time.Duration }
	var seeks []seek
	pb := newAudioPlaybackFromPipeline(clipPipeline(t, 300), buildOptions([]Option{
		WithOnSeek(func(from, to time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, "seek")
			seeks = append(seeks, seek{from, to})
		}),
		WithOnTimeUpdate(func(time.Duration, time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, "time")
		}),
	}))
	defer closeWithin(t, pb.Close)
	recorded := func() ([]string, []seek) {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(events), slices.Clone(seeks)
	}

	// Paused, so the only time update is the one the seek reports.
	pb.Play()
	if !playingWithin(pb.IsPlaying, 500*time.Millisecond) {
		t.Fatal("not playing after Play")
	}
	time.Sleep(200 * time.Millisecond)
	pb.Pause()
	from := pb.CurrentTime()
	if from <= 0 {
		t.Fatalf("precondition: paused at %v, want past the start", from)
	}
	ap := NewAudioPlayerFor(pb)
	mu.Lock()
	events, seeks = nil, nil
	mu.Unlock()

	dragSeekBarToEnd(ap.controls)
	if ev, _ := recorded(); slices.Contains(ev, "seek") {
		t.Fatalf("a drag still in flight reported a seek: %v", ev)
	}
	ap.controls.seekBar.DragEnd()

	ev, got := recorded()
	if want := []seek{{from, pb.Duration()}}; !slices.Equal(got, want) {
		t.Errorf("seeks reported %v, want %v", got, want)
	}
	if want := []string{"seek", "time"}; !slices.Equal(ev, want) {
		t.Errorf("callbacks ran %v, want %v", ev, want)
	}
}

// TestAudioPlaybackReportsEverySeek: WithOnSeek hears the app's own seeks as
// well as the widget's, with the target clamped to the media, and not Stop's
// rewind to the start.
func TestAudioPlaybackReportsEverySeek(t *testing.T) {
	type seek struct{ from, to time.Duration }
	var mu sync.Mutex
	var seeks []seek
	pb := newAudioPlaybackFromPipeline(clipPipeline(t, 300), buildOptions([]Option{
		WithOnSeek(func(from, to time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			seeks = append(seeks, seek{from, to})
		}),
	}))
	defer closeWithin(t, pb.Close)

	total := pb.Duration()
	pb.Seek(2 * time.Second)
	pb.Seek(time.Hour)
	pb.Seek(-time.Second)
	pb.Play()
	if !playingWithin(pb.IsPlaying, 500*time.Millisecond) {
		t.Fatal("not playing after Play")
	}
	pb.Stop()

	mu.Lock()
	defer mu.Unlock()
	want := []seek{{0, 2 * time.Second}, {2 * time.Second, total}, {total, 0}}
	if !slices.Equal(seeks, want) {
		t.Errorf("seeks reported %v, want %v", seeks, want)
	}
}
