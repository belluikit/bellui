package pipeline

import (
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPauseKeepsTheQueuedFrames pins #527: while paused, the display took
// frames from the queue and threw them away. Once a frame is showing, the
// display waits on the next one's time, which a paused clock never reaches,
// so it only bit a pause before the first frame: straight after Play, while it
// buffers about a second of frames. That second was lost; and a clip short
// enough to sit wholly in the queue reached its end marker, so the pipeline
// reported the end and moved its time to the clip's length.
func TestPauseKeepsTheQueuedFrames(t *testing.T) {
	p := newPipelineForSource(&clipSource{frames: 15}, Config{}) // half a second at 30 fps
	defer p.Close()

	var mu sync.Mutex
	var shown []int
	p.SetOnFrame(func(img *image.RGBA) {
		mu.Lock()
		shown = append(shown, frameIndex(img))
		mu.Unlock()
	})
	var ended atomic.Bool
	p.SetOnEnd(func() { ended.Store(true) })

	p.Play()
	p.Pause()
	held := p.CurrentTime()

	time.Sleep(700 * time.Millisecond) // longer than the whole clip
	if ended.Load() {
		t.Fatal("a paused pipeline reported the end of the clip")
	}
	if p.State() != StatePaused {
		t.Errorf("state %v after a pause, want paused", p.State())
	}
	if now := p.CurrentTime(); now != held {
		t.Errorf("time moved while paused: %v, then %v", held, now)
	}

	p.Play()
	deadline := time.Now().Add(2 * time.Second)
	for !ended.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !ended.Load() {
		t.Fatal("the clip did not end after resuming")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(shown) == 0 || shown[0] != 0 {
		t.Errorf("frames shown after resuming: %v; want the clip from frame 0, nothing thrown away during the pause", shown)
	}
}
