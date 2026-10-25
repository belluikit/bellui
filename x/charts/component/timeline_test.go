package component

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
)

// Playback advances on the UI thread, so the app can read and change the
// timeline from there while it plays. The steps used to run on the ticker's
// goroutine, which -race reports here as a data race on Current (#343).
func TestTimeline_PlaybackStepsOnTheUIThread(t *testing.T) {
	test.NewTempApp(t)
	steps := make(chan int, 16)
	var ts *TimelineState
	ts = NewTimelineState(4, TimelineConfig{Interval: time.Millisecond}, func(i int) { steps <- i })

	bellui.DoAndWait(ts.Play)
	for want := 1; want <= 3; want++ {
		select {
		case got := <-steps:
			if got != want {
				t.Fatalf("step %d, want %d", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("playback stalled before step %d", want)
		}
		bellui.DoAndWait(func() { _ = ts.Current }) // the app reading the timeline as it plays
	}

	var playing bool
	deadline := time.Now().Add(5 * time.Second)
	for {
		bellui.DoAndWait(func() { playing = ts.Playing })
		if !playing || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if playing {
		t.Error("playback did not stop at the last step")
	}
	select {
	case extra := <-steps:
		t.Errorf("a step (%d) ran after the last one", extra)
	case <-time.After(20 * time.Millisecond):
	}
}

// Stop ends the ticker goroutine even when it is not waiting at that moment;
// a dropped send used to leave it blocked forever. Stopping twice is harmless.
func TestTimeline_StopTwiceAndRestart(t *testing.T) {
	test.NewTempApp(t)
	ts := NewTimelineState(3, TimelineConfig{Interval: time.Hour}, nil)
	bellui.DoAndWait(func() {
		ts.Play()
		ts.Stop()
		ts.Stop()
		ts.Play()
		if !ts.Playing {
			t.Error("Play after Stop did not restart playback")
		}
		ts.Stop()
	})
}
