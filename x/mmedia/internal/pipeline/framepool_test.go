package pipeline

import (
	"image"
	"testing"

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

// bigSource is an endless source that declares 1080p, so the pipeline sizes
// its queue for 1080p, but hands over a 16x16 frame. The copy into a pool frame
// then moves 1 KB, not 8 MB, and the fill costs next to nothing even under
// -race on a slow runner (#812).
type bigSource struct {
	vfrSource
}

func (s *bigSource) VideoSize() (int, int) { return 1920, 1080 }

// With a queue shorter than the pre-roll, Play must start the display once the
// queue is full, not wait out its 2 s timeout for frames that cannot fit. The
// test asserts which way the pre-roll ended rather than timing it: a duration
// limit reads a slow runner as the bug (#812).
func TestPlayStartsPromptlyAtHighResolution(t *testing.T) {
	src := &bigSource{vfrSource{
		frame:        image.NewRGBA(image.Rect(0, 0, 16, 16)),
		realRate:     30,
		declaredRate: 30,
	}}
	var _ source.Source = src
	p := newTestPipeline(t, src)
	p.SetOnFrame(func(*image.RGBA) {}) // displayLoop recycles the frame itself
	defer p.Close()
	if got := cap(p.frameQueue); got >= prerollFrames {
		t.Fatalf("queue holds %d frames, not fewer than the %d-frame pre-roll: the test exercises nothing", got, prerollFrames)
	}

	p.Play()
	p.mu.Lock()
	got := p.prerollEnd
	p.mu.Unlock()
	if got != prerollQueued {
		t.Errorf("pre-roll ended with %v, want %v (the queue full)", got, prerollQueued)
	}
}
