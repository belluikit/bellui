package pipeline

import (
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// interruptibleSource is a slowSeekSource that honours an interrupt the way
// FFmpegSource does (#884): a seek or rewind blocked in its delay returns at
// once without moving, Decode does nothing until the next seek or rewind, and
// that one reopens the source first.
type interruptibleSource struct {
	slowSeekSource
	mu          sync.Mutex
	interrupt   chan struct{} // closed by Interrupt
	interrupted atomic.Bool
	interrupts  atomic.Int32
	reopens     atomic.Int32
	cut         atomic.Int32 // seeks and rewinds cut short
}

var _ source.Interruptible = (*interruptibleSource)(nil)

func newInterruptibleSource(frames int, delay time.Duration) *interruptibleSource {
	return &interruptibleSource{
		slowSeekSource: slowSeekSource{clipSource: clipSource{frames: frames}, delay: delay},
		interrupt:      make(chan struct{}),
	}
}

func (s *interruptibleSource) Interrupt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interrupts.Add(1)
	if !s.interrupted.Load() {
		s.interrupted.Store(true)
		close(s.interrupt)
	}
}

// move is a seek or rewind: reopen if interrupted, then wait out the delay
// unless interrupted again.
func (s *interruptibleSource) move(land func()) {
	if s.interrupted.Load() {
		s.mu.Lock()
		s.interrupt = make(chan struct{})
		s.interrupted.Store(false)
		s.mu.Unlock()
		s.reopens.Add(1)
		s.clipSource.Rewind() // a fresh open is at the start
	}
	s.mu.Lock()
	ch := s.interrupt
	s.mu.Unlock()
	s.inSeek.Store(true)
	defer s.inSeek.Store(false)
	select {
	case <-time.After(s.delay):
		land()
	case <-ch:
		s.cut.Add(1)
	}
}

func (s *interruptibleSource) Seek(pos time.Duration) {
	s.move(func() {
		s.clipSource.Seek(pos)
		s.lastSeek.Store(int64(pos))
		s.seeks.Add(1)
	})
}

func (s *interruptibleSource) Rewind() {
	s.move(func() {
		s.clipSource.Rewind()
		s.rewinds.Add(1)
	})
}

func (s *interruptibleSource) Decode(elapsed time.Duration) {
	if s.interrupted.Load() {
		return
	}
	s.clipSource.Decode(elapsed)
}

// TestHaltDuringASlowSeekReturnsPromptly pins #884: Close, Stop and SwapSource
// joined the decoder, which waited out the seek it was in: on a network MP3
// with no seek table, seconds. They interrupt the source now, and return about
// as soon as it lets go.
func TestHaltDuringASlowSeekReturnsPromptly(t *testing.T) {
	const delay = 2 * time.Second
	for _, op := range []string{"close", "stop", "swap"} {
		t.Run(op, func(t *testing.T) {
			src := newInterruptibleSource(300, delay)
			p := newTestPipeline(t, src)
			p.SetOnFrame(func(*image.RGBA) {})
			p.Play()
			time.Sleep(200 * time.Millisecond)
			p.Seek(8 * time.Second)
			time.Sleep(100 * time.Millisecond)
			if !src.inSeek.Load() {
				t.Fatal("the source is not seeking: the test measures nothing")
			}

			start := time.Now()
			switch op {
			case "close":
				p.Close()
			case "stop":
				p.Stop()
			case "swap":
				if err := p.SwapSource(&clipSource{frames: 300}); err != nil {
					t.Fatal(err)
				}
			}
			took := time.Since(start)
			if op != "close" {
				defer p.Close()
			}
			t.Logf("%s during the seek took %v", op, took)
			if took > 300*time.Millisecond {
				t.Errorf("%s took %v during a seek of %v; want it without waiting for the seek", op, took, delay)
			}
			if src.interrupts.Load() == 0 || src.cut.Load() != 1 {
				t.Errorf("source interrupted %d times, %d seeks cut short; want the seek cut short",
					src.interrupts.Load(), src.cut.Load())
			}
		})
	}
}

// A Stop that cut a seek short still plays from the start on the next Play:
// the rewind it posts reopens the source.
func TestStopDuringASlowSeekThenPlay(t *testing.T) {
	src := newInterruptibleSource(300, 300*time.Millisecond)
	p := newTestPipeline(t, src)
	defer p.Close()
	var shown frameLog
	p.SetOnFrame(shown.record)

	p.Play()
	time.Sleep(200 * time.Millisecond)
	p.Seek(8 * time.Second)
	time.Sleep(100 * time.Millisecond)
	p.Stop()
	mark := shown.len()
	p.Play()
	time.Sleep(src.delay + 300*time.Millisecond)

	if src.reopens.Load() != 1 || src.rewinds.Load() != 1 {
		t.Errorf("after Stop and Play: %d reopens, %d rewinds; want one of each", src.reopens.Load(), src.rewinds.Load())
	}
	after := shown.since(mark)
	if len(after) == 0 || after[0] != 0 {
		t.Errorf("frames after Stop and Play: %v; want them to start at frame 0", after)
	}
}

// A Stop with no seek in flight does not interrupt: there is nothing to cut
// short, and an interrupted source costs a reopen on the next Play.
func TestStopWithoutASeekDoesNotInterrupt(t *testing.T) {
	src := newInterruptibleSource(300, 100*time.Millisecond)
	p := newTestPipeline(t, src)
	defer p.Close()
	p.SetOnFrame(func(*image.RGBA) {})

	p.Play()
	time.Sleep(300 * time.Millisecond)
	p.Stop()
	if n := src.interrupts.Load(); n != 0 {
		t.Errorf("Stop with no seek in flight interrupted the source %d times; want none", n)
	}
}
