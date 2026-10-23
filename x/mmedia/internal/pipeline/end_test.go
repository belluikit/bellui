package pipeline

import (
	"image"
	"sync"
	"testing"
	"time"
)

// TestEndReportsTheFullDuration: time updates come every 100 ms, so the last
// one before the end of a stream fell short of it, and nothing reported the
// end itself: a progress bar stopped short of the end, and with nothing to stop
// it the clock ran on past the duration.
func TestEndReportsTheFullDuration(t *testing.T) {
	clip := &clipSource{frames: 20} // two thirds of a second
	p := newTestPipeline(t, clip)
	defer p.Close()
	p.SetOnFrame(func(*image.RGBA) {})

	var mu sync.Mutex
	var last time.Duration
	p.SetOnTime(func(current, _ time.Duration) {
		mu.Lock()
		last = current
		mu.Unlock()
	})
	ended := make(chan struct{})
	p.SetOnEnd(func() { close(ended) })

	p.Play()
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream did not end")
	}

	mu.Lock()
	reported := last
	mu.Unlock()
	if reported != clip.Duration() {
		t.Errorf("last time reported %v; want the %v duration", reported, clip.Duration())
	}
	time.Sleep(200 * time.Millisecond)
	if now := p.CurrentTime(); now != clip.Duration() {
		t.Errorf("time %v after the end; want it to rest at %v", now, clip.Duration())
	}
}
