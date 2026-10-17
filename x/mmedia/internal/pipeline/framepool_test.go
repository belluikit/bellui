package pipeline

import (
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// The decode-ahead pool is sized by memory (#110): 120 frames of 1080p were
// about 1 GB. Small videos keep the old depth.
func TestFramePoolIsSizedByMemory(t *testing.T) {
	for _, tc := range []struct {
		w, h      int
		wantDepth int
	}{
		{320, 240, 120},  // 37 MB: unchanged
		{1280, 720, 36},  // was 450 MB
		{1920, 1080, 16}, // was 1012 MB
		{3840, 2160, 8},  // was 4 GB; the floor keeps some jitter room
	} {
		p := &Pipeline{}
		p.allocFramePool(tc.w, tc.h)
		if got := cap(p.frameQueue); got != tc.wantDepth {
			t.Errorf("%dx%d: queue depth %d, want %d", tc.w, tc.h, got, tc.wantDepth)
		}
		if got := len(p.frameRecycle); got != tc.wantDepth+2 {
			t.Errorf("%dx%d: %d pooled frames, want %d", tc.w, tc.h, got, tc.wantDepth+2)
		}
		poolBytes := (tc.wantDepth + 2) * tc.w * tc.h * 4
		if tc.wantDepth > minQueuedFrames && poolBytes > framePoolBudget+2*tc.w*tc.h*4 {
			t.Errorf("%dx%d: pool is %d MB, over the %d MB budget", tc.w, tc.h, poolBytes>>20, framePoolBudget>>20)
		}
		if p.prerollTarget() > cap(p.frameQueue) {
			t.Errorf("%dx%d: Play waits for %d frames but the queue holds %d", tc.w, tc.h, p.prerollTarget(), cap(p.frameQueue))
		}
	}
}

// bigSource is an endless 1080p source with no audio.
type bigSource struct {
	vfrSource
}

func (s *bigSource) VideoSize() (int, int) { return 1920, 1080 }

// With a queue shorter than the pre-roll, Play must start the display once the
// queue is full, not wait out its 2 s timeout for frames that cannot fit.
func TestPlayStartsPromptlyAtHighResolution(t *testing.T) {
	src := &bigSource{vfrSource{
		frame:        image.NewRGBA(image.Rect(0, 0, 1920, 1080)),
		realRate:     30,
		declaredRate: 30,
	}}
	var _ source.Source = src
	p := newPipelineForSource(src, Config{})
	var first atomic.Int64
	start := time.Now()
	p.SetOnFrame(func(f *image.RGBA) {
		first.CompareAndSwap(0, int64(time.Since(start)))
		p.RecycleFrame(f)
	})

	p.Play()
	time.Sleep(2500 * time.Millisecond)
	p.Close()

	got := time.Duration(first.Load())
	if got == 0 {
		t.Fatal("no frame was displayed")
	}
	if got > time.Second {
		t.Errorf("first frame after %v: Play waited for a pre-roll the queue cannot hold", got)
	}
}
