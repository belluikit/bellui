package pipeline

import (
	"image"
	"sync/atomic"
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
		p := &Pipeline{}
		p.pool.Store(&framePool{queue: make(chan timedFrame, depth)})
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
	if got, want := cap(p.pool.Load().queue), queueDepth(16, 16); got != want {
		t.Errorf("queue depth %d, want %d", got, want)
	}
	if got, want := len(p.pool.Load().recycle), cap(p.pool.Load().queue)+2; got != want {
		t.Errorf("%d pooled frames, want %d", got, want)
	}
	if b := (<-p.pool.Load().recycle).Bounds(); b.Dx() != 16 || b.Dy() != 16 {
		t.Errorf("pool frame is %v, want 16x16", b)
	}
}

// bigSource is an endless source of 1080p frames whose Read writes nothing, so
// the pipeline sizes its queue for 1080p but the fill costs next to nothing
// even under -race on a slow runner (#812).
type bigSource struct {
	vfrSource
}

func (s *bigSource) VideoSize() (int, int) { return 1920, 1080 }

func (s *bigSource) Decode(time.Duration) {
	if s.vcb != nil {
		s.vcb(source.VideoFrame{Width: 1920, Height: 1080, PTS: s.pts, Read: func(*image.RGBA) error { return nil }})
		s.pts += time.Duration(float64(time.Second) / s.realRate)
	}
}

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
	if got := cap(p.pool.Load().queue); got >= prerollFrames {
		t.Fatalf("queue holds %d frames, not fewer than the %d-frame pre-roll: the test exercises nothing", got, prerollFrames)
	}

	p.Play()
	got := prerollWaiting
	for deadline := time.Now().Add(5 * time.Second); got == prerollWaiting && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		got = prerollExit(p.prerollEnd.Load())
	}
	if got != prerollQueued {
		t.Errorf("pre-roll ended with %v, want %v (the queue full)", got, prerollQueued)
	}
}

// A live capture produces frames only in real time and skips the pre-roll, so
// it gets a short fixed queue rather than the memory-sized one: 3 frames, not
// 16, at 1080p (#711).
func TestLiveSourceQueueIsShortAt1080p(t *testing.T) {
	p := newTestPipeline(t, liveShapedSource{newShapedSource(1920, 1080)})
	defer p.Close()
	pool := p.pool.Load()
	if got := cap(pool.queue); got != liveQueuedFrames || liveQueuedFrames != 3 {
		t.Errorf("live 1080p queue holds %d frames, want 3", got)
	}
	if got := len(pool.recycle); got != liveQueuedFrames+2 {
		t.Errorf("%d pooled frames, want %d", got, liveQueuedFrames+2)
	}
	if got := queueDepth(1920, 1080); got == liveQueuedFrames {
		t.Fatalf("a finite 1080p source has the same depth (%d): the test exercises nothing", got)
	}
}

// A device that declares 0x0 at open gets the fallback pool. Its first frame,
// at its real size, rebuilds the pool before anything is decoded into it, so
// every frame arrives whole: 1280x720, not truncated into 640x360 (#712). The
// display is already waiting on the old pool's queue, as a live source starts
// it with no pre-roll, and has to move to the new one.
func TestUndeclaredGeometryRebuildsThePoolOnTheFirstFrame(t *testing.T) {
	src := newShapedSource(0, 0)
	src.frame = image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for i := range src.frame.Pix {
		src.frame.Pix[i] = 0x5C
	}
	p := newTestPipeline(t, liveShapedSource{src})
	defer p.Close()
	if pool := p.pool.Load(); pool.w != 640 || pool.h != 360 {
		t.Fatalf("precondition: the fallback pool is %dx%d, want 640x360", pool.w, pool.h)
	}

	frames := make(chan image.Rectangle, 64)
	whole := atomic.Bool{}
	whole.Store(true)
	p.SetOnFrame(func(img *image.RGBA) {
		if img.Pix[len(img.Pix)-1] != 0x5C {
			whole.Store(false)
		}
		select {
		case frames <- img.Rect:
		default:
		}
	})
	p.Play()

	for i := 0; i < 5; i++ {
		select {
		case r := <-frames:
			if r.Dx() != 1280 || r.Dy() != 720 {
				t.Fatalf("frame %d is %v, want 1280x720", i, r)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d frames shown: the display never moved to the rebuilt pool", i)
		}
	}
	if !whole.Load() {
		t.Error("a frame was cut short: its last pixel is not the source's")
	}
	if w, h := p.VideoSize(); w != 1280 || h != 720 {
		t.Errorf("VideoSize = %dx%d, want the frames' 1280x720", w, h)
	}
	if got := cap(p.pool.Load().queue); got != liveQueuedFrames {
		t.Errorf("rebuilt live queue holds %d frames, want %d", got, liveQueuedFrames)
	}
}

// The source converts each frame straight into a pool frame: receiving one
// allocates nothing (#431). It used to allocate a full frame per frame, about
// 250 MB/s at 1080p30, and then copy it.
func TestReceivingAFrameAllocatesNothing(t *testing.T) {
	p := newTestPipeline(t, newShapedSource(64, 64))
	defer p.Close()
	p.SetOnFrame(func(*image.RGBA) {})
	read := func(dst *image.RGBA) error { dst.Pix[0] = 1; return nil }
	f := source.VideoFrame{Width: 64, Height: 64, Read: read}
	allocs := testing.AllocsPerRun(100, func() {
		p.receiveFrame(f)
		p.RecycleFrame(p.pendingFrame)
	})
	if allocs != 0 {
		t.Errorf("receiving a frame allocates %.0f times, want 0", allocs)
	}
}

// A frame the pool has no room for is dropped before it is converted, so a
// drop costs no conversion.
func TestADroppedFrameIsNotConverted(t *testing.T) {
	p := newTestPipeline(t, newShapedSource(16, 16))
	defer p.Close()
	p.SetOnFrame(func(*image.RGBA) {})
	pool := p.pool.Load()
	for len(pool.recycle) > 0 {
		<-pool.recycle
	}
	read := false
	p.receiveFrame(source.VideoFrame{Width: 16, Height: 16, Read: func(*image.RGBA) error { read = true; return nil }})
	if read {
		t.Error("a frame with no pool frame free was converted anyway")
	}
	if got := p.Stats().FramesDropped; got != 1 {
		t.Errorf("FramesDropped = %d, want 1", got)
	}
}
