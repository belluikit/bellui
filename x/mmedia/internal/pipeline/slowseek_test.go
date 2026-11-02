package pipeline

import (
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// slowSeekSource is a clip on a slow network: each seek or rewind takes delay,
// as FFmpeg reading an MP3 with no seek table through to the target does, and
// is counted, so a test can tell which goroutine made it and whether it was
// made at all.
type slowSeekSource struct {
	clipSource
	delay    time.Duration
	seeks    atomic.Int32
	rewinds  atomic.Int32
	inSeek   atomic.Bool
	lastSeek atomic.Int64
}

func (s *slowSeekSource) Seek(pos time.Duration) {
	s.inSeek.Store(true)
	time.Sleep(s.delay)
	s.clipSource.Seek(pos)
	s.lastSeek.Store(int64(pos))
	s.seeks.Add(1)
	s.inSeek.Store(false)
}

func (s *slowSeekSource) Rewind() {
	s.inSeek.Store(true)
	time.Sleep(s.delay)
	s.clipSource.Rewind()
	s.rewinds.Add(1)
	s.inSeek.Store(false)
}

// frameLog records the index of every frame shown.
type frameLog struct {
	mu  sync.Mutex
	got []int
}

func (l *frameLog) record(img *image.RGBA) {
	l.mu.Lock()
	l.got = append(l.got, frameIndex(img))
	l.mu.Unlock()
}

func (l *frameLog) since(n int) []int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]int(nil), l.got[n:]...)
}

func (l *frameLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.got)
}

// TestSeekDoesNotWaitForTheSource pins #872: Seek moved the source itself, on
// the caller's goroutine, and on a network stream that is a request and a
// decode up to the target, seconds of a frozen UI. The decoder moves it now.
// Seek returns at once, the clock waits at the target, nothing from before the
// seek is shown, and once the source is there the first frame is the target's.
func TestSeekDoesNotWaitForTheSource(t *testing.T) {
	for _, state := range []string{"playing", "paused", "stopped"} {
		t.Run(state, func(t *testing.T) {
			src := &slowSeekSource{clipSource: clipSource{frames: 300}, delay: 600 * time.Millisecond}
			p := newTestPipeline(t, src)
			defer p.Close()
			var shown frameLog
			p.SetOnFrame(shown.record)

			if state != "stopped" {
				p.Play()
				time.Sleep(300 * time.Millisecond) // about frame 9
			}
			if state == "paused" {
				p.Pause()
			}

			start := time.Now()
			p.Seek(5 * time.Second) // frame 150
			if took := time.Since(start); took > 100*time.Millisecond {
				t.Fatalf("Seek took %v; want it back without waiting for the source's %v", took, src.delay)
			}
			if now := p.CurrentTime(); now != 5*time.Second {
				t.Errorf("CurrentTime %v straight after Seek; want the target, 5s", now)
			}

			mark := shown.len()
			if state == "stopped" {
				// Play waits for frames before it shows any, and those are
				// on the far side of the seek: that wait is the display's.
				start := time.Now()
				p.Play()
				if took := time.Since(start); took > 100*time.Millisecond {
					t.Errorf("Play after Seek took %v; want it back without waiting for the source", took)
				}
			}
			time.Sleep(src.delay / 2)
			if !src.inSeek.Load() {
				t.Fatal("the source is not seeking half way through its seek: the test measures nothing")
			}
			if now := p.CurrentTime(); now != 5*time.Second {
				t.Errorf("CurrentTime %v while the source is still seeking; want it held at 5s", now)
			}

			time.Sleep(src.delay)
			if src.seeks.Load() != 1 {
				t.Fatalf("source seeked %d times; want once", src.seeks.Load())
			}
			if state == "paused" {
				p.Play()
			}
			time.Sleep(300 * time.Millisecond)

			after := shown.since(mark)
			if len(after) == 0 {
				t.Fatal("no frame shown after the seek landed")
			}
			if after[0] != 150 {
				t.Errorf("first frame after the seek is %d; want the target, 150 (shown: %v)", after[0], after)
			}
			for _, i := range after {
				if i < 150 {
					t.Errorf("frame %d, from before the seek, was shown after it (shown: %v)", i, after)
					break
				}
			}
			if now := p.CurrentTime(); now < 5*time.Second || now > 6*time.Second {
				t.Errorf("CurrentTime %v after the seek landed and played on; want just past 5s", now)
			}
		})
	}
}

// A seek made while the source is still on its way to an earlier one replaces
// it: the source goes on to the latest target, and nothing from the earlier one
// is shown.
func TestSeekDuringASeekGoesToTheLatest(t *testing.T) {
	src := &slowSeekSource{clipSource: clipSource{frames: 300}, delay: 400 * time.Millisecond}
	p := newTestPipeline(t, src)
	defer p.Close()
	var shown frameLog
	p.SetOnFrame(shown.record)

	p.Play()
	time.Sleep(200 * time.Millisecond)
	mark := shown.len()
	p.Seek(3 * time.Second) // frame 90
	time.Sleep(src.delay / 2)
	p.Seek(6 * time.Second) // frame 180
	time.Sleep(2*src.delay + 300*time.Millisecond)

	if got := time.Duration(src.lastSeek.Load()); got != 6*time.Second {
		t.Errorf("the source's last seek went to %v; want the latest target, 6s", got)
	}
	after := shown.since(mark)
	if len(after) == 0 || after[0] != 180 {
		t.Fatalf("frames after the two seeks: %v; want them to start at the latest target, 180", after)
	}
}

// TestCloseMakesNoRequest pins #872's other half: Close stopped the pipeline,
// and Stop rewinds the source, a seek to 0 and so one more request on a
// network stream, made only to close it. Stop's rewind waits for the next
// Play now, and Close makes none.
func TestCloseMakesNoRequest(t *testing.T) {
	for _, stopFirst := range []bool{false, true} {
		name := "close"
		if stopFirst {
			name = "stop then close"
		}
		t.Run(name, func(t *testing.T) {
			src := &slowSeekSource{clipSource: clipSource{frames: 300}, delay: 300 * time.Millisecond}
			p := newTestPipeline(t, src)
			p.SetOnFrame(func(*image.RGBA) {})
			p.Play()
			p.Seek(time.Second)
			time.Sleep(src.delay + 200*time.Millisecond)
			if src.seeks.Load() != 1 {
				t.Fatalf("source seeked %d times before Close; want the one seek", src.seeks.Load())
			}

			start := time.Now()
			if stopFirst {
				p.Stop()
			}
			p.Close()
			if took := time.Since(start); took > src.delay/2 {
				t.Errorf("Close took %v; want it without a request to the source", took)
			}
			if n, r := src.seeks.Load(), src.rewinds.Load(); n != 1 || r != 0 {
				t.Errorf("source got %d seeks and %d rewinds by the end of Close; want only the one seek", n, r)
			}
		})
	}
}

// Stop still rewinds: the rewind it posts is made by the next Play's decoder,
// before anything is decoded, so playback starts again from the first frame.
func TestStopThenPlayStartsFromTheStart(t *testing.T) {
	src := &slowSeekSource{clipSource: clipSource{frames: 300}, delay: 200 * time.Millisecond}
	p := newTestPipeline(t, src)
	defer p.Close()
	var shown frameLog
	p.SetOnFrame(shown.record)

	p.Play()
	time.Sleep(500 * time.Millisecond)
	start := time.Now()
	p.Stop()
	if took := time.Since(start); took > src.delay/2 {
		t.Errorf("Stop took %v; want it without waiting for the rewind", took)
	}
	if src.rewinds.Load() != 0 {
		t.Fatal("Stop rewound the source itself; want the rewind left to the next Play")
	}
	mark := shown.len()
	p.Play()
	time.Sleep(src.delay + 300*time.Millisecond)

	if src.rewinds.Load() != 1 {
		t.Errorf("source rewound %d times by the next Play; want once", src.rewinds.Load())
	}
	after := shown.since(mark)
	if len(after) == 0 || after[0] != 0 {
		t.Errorf("frames after Stop and Play: %v; want them to start at frame 0", after)
	}
}

// The decoder of a video reaches the end well before the display does, and it
// used to return there. A seek in that time discarded the end-of-stream marker
// with the frames, and with no decoder left the display waited for ever: no
// picture, and no OnEnd. It decodes on from the target now.
func TestSeekAfterTheDecoderReachedTheEnd(t *testing.T) {
	src := &clipSource{frames: 60} // 2 s, all of it queued within milliseconds
	p := newTestPipeline(t, src)
	defer p.Close()
	var shown frameLog
	p.SetOnFrame(shown.record)
	ended := make(chan struct{})
	p.SetOnEnd(func() { close(ended) })

	p.Play()
	deadline := time.Now().Add(2 * time.Second)
	for !p.sourceEnded() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !p.sourceEnded() {
		t.Fatal("the decoder never reached the end: the test measures nothing")
	}
	if shown.len() > 40 {
		t.Fatalf("the display has shown %d of 60 frames already: the test needs it well behind the decoder", shown.len())
	}

	mark := shown.len()
	p.Seek(time.Second) // frame 30
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatalf("no OnEnd after a seek made once the decoder had reached the end (frames shown since: %v)", shown.since(mark))
	}
	if after := shown.since(mark); len(after) == 0 || after[0] != 30 {
		t.Errorf("frames after the seek: %v; want them to start at the target, 30", after)
	}
}
