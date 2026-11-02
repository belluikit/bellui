package source

import (
	"errors"
	"testing"
	"time"

	"github.com/belluikit/goffmpeg"
)

// stuckDecoder is a decoder on a slow network: Seek blocks until Interrupt,
// then fails, as goffmpeg's does with AVERROR_EXIT.
type stuckDecoder struct {
	fakeDecoder
	interrupted chan struct{}
	seeking     chan struct{}
}

func newStuckDecoder() *stuckDecoder {
	return &stuckDecoder{
		fakeDecoder: fakeDecoder{duration: 300},
		interrupted: make(chan struct{}),
		seeking:     make(chan struct{}, 1),
	}
}

func (d *stuckDecoder) Seek(float64) error {
	d.seeking <- struct{}{}
	<-d.interrupted
	return errors.New("seek failed: -1414092869")
}

func (d *stuckDecoder) Interrupt() {
	select {
	case <-d.interrupted:
	default:
		close(d.interrupted)
	}
}

// An interrupted seek returns at once and records nothing, since it was asked
// to stop rather than failed; Decode then does nothing, and the next Seek
// reopens the source and makes the seek on the fresh decoder (#884).
func TestInterruptedSeekReopens(t *testing.T) {
	stuck := newStuckDecoder()
	s := newFFmpegSource(stuck)
	fresh := &fakeDecoder{duration: 300}
	opens := 0
	s.open = func() (goffmpeg.IDecoder, error) { opens++; return fresh, nil }

	done := make(chan struct{})
	go func() {
		s.Seek(200 * time.Second)
		close(done)
	}()
	<-stuck.seeking
	start := time.Now()
	s.Interrupt()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Seek did not return after Interrupt")
	}
	t.Logf("Seek returned %v after Interrupt", time.Since(start))

	if s.HasEnded() || s.Err() != nil {
		t.Errorf("after an interrupted seek: ended %v, err %v; want neither", s.HasEnded(), s.Err())
	}
	s.Decode(0)
	if stuck.nexts != 0 || fresh.nexts != 0 {
		t.Errorf("Decode read %d + %d packets while interrupted; want none", stuck.nexts, fresh.nexts)
	}

	s.Seek(0) // the fake's position is always 0, so a later target never lands
	if opens != 1 || stuck.closed != 1 {
		t.Fatalf("after the next Seek: %d opens, old decoder closed %d times; want 1 and 1", opens, stuck.closed)
	}
	if fresh.seeks != 1 {
		t.Errorf("the reopened decoder got %d seeks; want 1", fresh.seeks)
	}
	if s.HasEnded() || s.Err() != nil {
		t.Errorf("after the reopening seek: ended %v, err %v; want neither", s.HasEnded(), s.Err())
	}
	s.Close()
	if fresh.closed != 1 {
		t.Errorf("Close closed the reopened decoder %d times; want once", fresh.closed)
	}
}

// A source that cannot be reopened, one from a reader or a device, is not
// interrupted: that would leave it unusable after a Stop.
func TestInterruptNeedsReopen(t *testing.T) {
	dec := &fakeDecoder{duration: 300}
	s := newFFmpegSource(dec)
	s.Interrupt()
	s.Seek(0)
	if dec.seeks != 1 || s.HasEnded() {
		t.Errorf("after Interrupt on a source with no reopen: %d seeks, ended %v; want the seek made", dec.seeks, s.HasEnded())
	}
}

// A reopen that fails ends the source with the reason, as a failed seek does.
func TestInterruptReopenFails(t *testing.T) {
	stuck := newStuckDecoder()
	stuck.Interrupt() // nothing blocks
	s := newFFmpegSource(stuck)
	s.open = func() (goffmpeg.IDecoder, error) { return nil, errors.New("server gone") }
	s.Interrupt()
	s.Seek(0)
	if !s.HasEnded() || s.Err() == nil {
		t.Fatalf("after a failed reopen: ended %v, err %v; want ended with the reason", s.HasEnded(), s.Err())
	}
	t.Logf("err: %v", s.Err())
}
