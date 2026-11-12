package pipeline

import (
	"image"
	"sync"
	"testing"
	"time"
)

// A paused seek whose decoder wakes between the seek storing pausedSeek and
// storing seekPosted still shows the target. The decoder, freed by the drain
// from a send it was blocked in at the pause, could take pausedSeek before
// the seek landed: it decoded a frame at the old position, which the
// generation check rightly refused, and with pausedSeek spent nothing showed
// the target once the seek landed. TestSeekWhilePausedShowsTheTarget hit
// this on a CI runner: nothing shown in 5 s.
func TestPausedSeekRacingTheDecoderShowsTheTarget(t *testing.T) {
	p := newTestPipeline(t, &clipSource{frames: 300})
	defer p.Close()

	var mu sync.Mutex
	var shown []int
	p.SetOnFrame(func(img *image.RGBA) {
		mu.Lock()
		shown = append(shown, frameIndex(img))
		mu.Unlock()
	})

	p.Play()
	time.Sleep(500 * time.Millisecond) // the queue fills and the decoder blocks on it
	p.Pause()
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	before := len(shown)
	mu.Unlock()

	// Seek to 5 s as Seek does, but stop where the decoder could see
	// pausedSeek while seekPosted is still unset, and let it run there.
	target := 5 * time.Second
	p.mu.Lock()
	p.seekMu.Lock()
	p.presentMu.Lock()
	p.seekGen.Add(1)
	p.presentMu.Unlock()
drain:
	for {
		select {
		case tf := <-p.pool.Load().queue:
			p.RecycleFrame(tf.img)
		default:
			break drain
		}
	}
	p.clock.Pause()
	p.clock.Seek(target)
	p.pausedSeek.Store(true)
	time.Sleep(100 * time.Millisecond) // the window, held open
	p.seeking = true
	p.seekTarget, p.seekRewind = target, false
	p.seekPosted.Store(true)
	select {
	case p.seekWake <- struct{}{}:
	default:
	}
	p.seekMu.Unlock()
	p.mu.Unlock()

	var after []int
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		mu.Lock()
		after = append([]int(nil), shown[before:]...)
		mu.Unlock()
		if len(after) > 0 && after[len(after)-1] == 150 {
			break
		}
	}
	if len(after) == 0 || after[len(after)-1] != 150 {
		t.Errorf("frames shown after a paused seek that raced the decoder: %v; want it to end on 150", after)
	}
}
