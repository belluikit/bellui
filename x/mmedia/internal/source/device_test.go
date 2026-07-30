package source

import (
	"errors"
	"image"
	"testing"
	"time"

	"github.com/belluikit/goffmpeg"
)

// fakeDecoder stands in for a capture device so the live semantics — no seeking,
// release on rewind, reacquire on demand — can be tested with no camera present.
type fakeDecoder struct {
	closed   int
	seeks    int
	nexts    int
	duration float64
	nextErr  error
}

func (f *fakeDecoder) VideoInfo() goffmpeg.VideoInfo {
	return goffmpeg.VideoInfo{Width: 16, Height: 16, Framerate: 30}
}
func (f *fakeDecoder) AudioInfo() goffmpeg.AudioInfo { return goffmpeg.AudioInfo{} }
func (f *fakeDecoder) Position() float64             { return 0 }
func (f *fakeDecoder) Duration() float64             { return f.duration }

func (f *fakeDecoder) Seek(float64) error { f.seeks++; return nil }

func (f *fakeDecoder) SetAudioChannels(int) error { return nil }

func (f *fakeDecoder) Next() (goffmpeg.MediaType, error) {
	f.nexts++
	if f.nextErr != nil {
		return goffmpeg.MediaTypeNone, f.nextErr
	}
	return goffmpeg.MediaTypeVideo, nil
}

func (f *fakeDecoder) GetRGBA(*image.RGBA) error { return nil }
func (f *fakeDecoder) GetAudio() ([]byte, error) { return nil, nil }
func (f *fakeDecoder) Close()                    { f.closed++ }

// openCounter hands out a fresh fake per call, recording how many times the
// device was opened.
type openCounter struct {
	opens int
	err   error
	last  *fakeDecoder
}

func (o *openCounter) open() (goffmpeg.IDecoder, error) {
	o.opens++
	if o.err != nil {
		return nil, o.err
	}
	o.last = &fakeDecoder{duration: 120}
	return o.last, nil
}

func TestDeviceSourceIsLiveAndHasNoDuration(t *testing.T) {
	oc := &openCounter{}
	s, err := newDeviceSource(oc.open)
	if err != nil {
		t.Fatalf("newDeviceSource: %v", err)
	}
	defer s.Close()

	if !s.IsLive() {
		t.Error("a device source must report IsLive")
	}
	// The fake reports a 120s duration; a live source must still answer zero,
	// since that is the signal the transport keys its live presentation off.
	if got := s.Duration(); got != 0 {
		t.Errorf("Duration = %v, want 0 even though the decoder claims one", got)
	}
}

func TestDeviceSourceSeekIsNoOp(t *testing.T) {
	oc := &openCounter{}
	s, _ := newDeviceSource(oc.open)
	defer s.Close()

	s.Seek(30 * time.Second)
	if oc.last.seeks != 0 {
		t.Errorf("Seek reached the decoder %d time(s); a device has no timeline to seek",
			oc.last.seeks)
	}
	if oc.last.nexts != 0 {
		t.Error("Seek must not decode on a live source")
	}
}

// TestDeviceSourceRewindReleasesAndReacquires is the Stop/Play cycle: the
// pipeline calls Rewind from Stop, which must let go of the camera, and the next
// Decode must pick it back up.
func TestDeviceSourceRewindReleasesAndReacquires(t *testing.T) {
	oc := &openCounter{}
	s, _ := newDeviceSource(oc.open)
	defer s.Close()
	first := oc.last

	s.Rewind()
	if first.closed != 1 {
		t.Errorf("device closed %d time(s) on Rewind, want 1 — a stopped capture must "+
			"not hold the camera", first.closed)
	}
	if oc.opens != 1 {
		t.Errorf("device opened %d time(s) by Rewind, want it left released until needed", oc.opens)
	}

	s.Decode(0)
	if oc.opens != 2 {
		t.Fatalf("device opened %d time(s) after Decode, want it reacquired", oc.opens)
	}
	if oc.last == first {
		t.Error("expected a fresh decoder after reacquire")
	}
	if s.HasEnded() {
		t.Error("a reacquired device must not report itself ended")
	}
}

func TestDeviceSourceReacquireFailureIsReported(t *testing.T) {
	oc := &openCounter{}
	s, _ := newDeviceSource(oc.open)
	defer s.Close()

	s.Rewind()
	oc.err = errors.New("device in use by another application")
	s.Decode(0)

	if !s.HasEnded() {
		t.Error("a source that cannot reacquire its device has ended")
	}
	if s.Err() == nil {
		t.Fatal("Err must report why the device could not be reacquired")
	}
	if !errors.Is(s.Err(), oc.err) {
		t.Errorf("Err = %v, want it to wrap the opener's cause", s.Err())
	}
}

// TestDeviceSourceCloseIsFinal separates Close from Rewind: releasing for a stop
// is reversible, finishing with the source is not.
func TestDeviceSourceCloseIsFinal(t *testing.T) {
	oc := &openCounter{}
	s, _ := newDeviceSource(oc.open)

	s.Close()
	if oc.last.closed != 1 {
		t.Errorf("device closed %d time(s) on Close, want 1", oc.last.closed)
	}

	opensBefore := oc.opens
	s.Decode(0)
	if oc.opens != opensBefore {
		t.Error("Decode must not reacquire a device after Close")
	}
	if !s.HasEnded() {
		t.Error("a closed source has ended")
	}
}

// TestDeviceSourceToleratesReadStall is the defect the captureprobe run on real
// macOS hardware exposed: a non-blocking capture device reports "no packet ready
// yet" as an ordinary read error (EAGAIN), and treating the first one as terminal
// ended capture about two frames in. A stall must be a hiccup, not an end.
func TestDeviceSourceToleratesReadStall(t *testing.T) {
	oc := &openCounter{}
	s, _ := newDeviceSource(oc.open)
	defer s.Close()

	oc.last.nextErr = errors.New("decoder error: -35")

	s.Decode(0)
	if s.HasEnded() {
		t.Fatal("a single failed read must not end a live capture — this is the EAGAIN case")
	}
	s.Decode(0)
	if s.HasEnded() {
		t.Error("a repeated stall inside the timeout must still not end the capture")
	}

	// Recovery clears the stall, so a device that hiccups carries no error
	// forward.
	oc.last.nextErr = nil
	s.Decode(0)
	if s.Err() != nil {
		t.Errorf("Err = %v after recovery, want it cleared", s.Err())
	}
	if !s.stalledSince.IsZero() {
		t.Error("a successful read must clear the stall clock")
	}
}

// TestDeviceSourceEndsOnSustainedStall is the other half: a device that really has
// gone away must be reported rather than waited on forever. The stall clock is
// wound back rather than waiting out the real timeout.
func TestDeviceSourceEndsOnSustainedStall(t *testing.T) {
	oc := &openCounter{}
	s, _ := newDeviceSource(oc.open)
	defer s.Close()

	lost := errors.New("device disconnected")
	oc.last.nextErr = lost
	s.Decode(0) // starts the stall clock
	if s.HasEnded() {
		t.Fatal("precondition: the first stall is tolerated")
	}

	s.stalledSince = time.Now().Add(-liveStallTimeout - time.Second)
	s.Decode(0)

	if !s.HasEnded() {
		t.Error("a stall lasting past liveStallTimeout ends the source")
	}
	if !errors.Is(s.Err(), lost) {
		t.Errorf("Err = %v, want the read failure %v", s.Err(), lost)
	}
}

// TestFiniteSourceEndsOnFirstReadError guards that the tolerance is live-only: for
// a file, a read error is the end of the stream and must stay so.
func TestFiniteSourceEndsOnFirstReadError(t *testing.T) {
	dec := &fakeDecoder{duration: 120, nextErr: errors.New("truncated file")}
	s := newFFmpegSource(dec)

	s.Decode(0)
	if !s.HasEnded() {
		t.Error("a finite source ends on a read failure, with no retry")
	}
}

// TestFileSourceIsNotLive guards the default: the optional capability must not
// leak onto ordinary playback sources, or a file would stop being seekable.
func TestFileSourceIsNotLive(t *testing.T) {
	s := newFFmpegSource(&fakeDecoder{duration: 120})
	if s.IsLive() {
		t.Error("a source built without a device opener must not be live")
	}
	if got, want := s.Duration(), 120*time.Second; got != want {
		t.Errorf("Duration = %v, want %v", got, want)
	}
	// Deliberately not calling Seek here: on a finite source it runs the
	// decode-forward loop until Position reaches the target, which a fake whose
	// position never advances would spin on forever. The seek path is covered
	// against real media in ffmpeg_test.go.
	if s.reopen != nil {
		t.Error("a non-device source must have no reacquire path")
	}
}
