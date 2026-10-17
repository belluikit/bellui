package pipeline

import (
	"image"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// The decode-ahead queue is sized by memory (#110): 120 frames of 1080p were
// about 1 GB. Small videos keep the old depth.
//
// The depths are checked on queueDepth rather than by building each pool, which
// would allocate the ~660 MB this sizing exists to avoid.
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
		depth := queueDepth(tc.w, tc.h)
		if depth != tc.wantDepth {
			t.Errorf("%dx%d: queue depth %d, want %d", tc.w, tc.h, depth, tc.wantDepth)
		}
		if queued := depth * tc.w * tc.h * 4; depth > minQueuedFrames && queued > framePoolBudget {
			t.Errorf("%dx%d: queue is %d MB, over the %d MB budget", tc.w, tc.h, queued>>20, framePoolBudget>>20)
		}
		p := &Pipeline{frameQueue: make(chan timedFrame, depth)}
		if got := p.prerollTarget(); got > depth {
			t.Errorf("%dx%d: Play waits for %d frames but the queue holds %d", tc.w, tc.h, got, depth)
		}
	}
}

// allocFramePool builds the queue at queueDepth and two pool frames more, for
// the frame being decoded into and the one on screen.
func TestAllocFramePoolHoldsTwoMoreThanTheQueue(t *testing.T) {
	p := &Pipeline{}
	p.allocFramePool(16, 16)
	if got, want := cap(p.frameQueue), queueDepth(16, 16); got != want {
		t.Errorf("queue depth %d, want %d", got, want)
	}
	if got, want := len(p.frameRecycle), cap(p.frameQueue)+2; got != want {
		t.Errorf("%d pooled frames, want %d", got, want)
	}
	if b := (<-p.frameRecycle).Bounds(); b.Dx() != 16 || b.Dy() != 16 {
		t.Errorf("pool frame is %v, want 16x16", b)
	}
}

// bigSource is an endless 1080p source with no audio.
type bigSource struct {
	vfrSource
}

func (s *bigSource) VideoSize() (int, int) { return 1920, 1080 }

// With a queue shorter than the pre-roll, Play must start the display once the
// queue is full, not wait out its 2 s timeout for frames that cannot fit. Play
// returns when the pre-roll ends, so its own duration is the measurement.
func TestPlayStartsPromptlyAtHighResolution(t *testing.T) {
	src := &bigSource{vfrSource{
		frame:        image.NewRGBA(image.Rect(0, 0, 1920, 1080)),
		realRate:     30,
		declaredRate: 30,
	}}
	var _ source.Source = src
	p := newPipelineForSource(src, Config{})
	p.SetOnFrame(func(*image.RGBA) {}) // displayLoop recycles the frame itself
	defer p.Close()

	start := time.Now()
	p.Play()
	// The timeout is 200 polls of 10 ms, so a pre-roll that cannot be met takes
	// at least 2 s; filling sixteen 1080p frames takes well under a tenth of it.
	if got := time.Since(start); got > 1500*time.Millisecond {
		t.Errorf("Play returned after %v: it waited for a pre-roll the queue cannot hold", got)
	}
}
