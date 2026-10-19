package pipeline

import (
	"image"
	"testing"
	"time"
)

// TestShortClipStartsPromptly: Play waits for about a second of frames before
// showing any, and a clip shorter than that never filled the wait, so it sat the
// full two-second limit before its first frame, and again on every loop.
func TestShortClipStartsPromptly(t *testing.T) {
	p := newPipelineForSource(&clipSource{frames: 15}, Config{}) // half a second
	defer p.Close()
	first := make(chan time.Time, 1)
	p.SetOnFrame(func(*image.RGBA) {
		select {
		case first <- time.Now():
		default:
		}
	})

	start := time.Now()
	p.Play()
	select {
	case at := <-first:
		if wait := at.Sub(start); wait > 500*time.Millisecond {
			t.Errorf("first frame of a half-second clip after %v; want within 500 ms", wait)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no frame within 3 s")
	}
}
