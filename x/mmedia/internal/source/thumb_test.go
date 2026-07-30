package source

import (
	"errors"
	"testing"
	"time"

	"github.com/belluikit/goffmpeg"
)

const thumbTestMedia = "testdata/sample.mp4"

// TestThumbnailerDecodesAFrame is the basic claim: a preview frame comes back,
// at the video's own geometry.
func TestThumbnailerDecodesAFrame(t *testing.T) {
	th := NewThumbnailer(thumbTestMedia)
	defer th.Close()

	// Nothing is opened until a frame is asked for — a preview that is never
	// hovered costs no file handle.
	if w, h := th.Size(); w != 0 || h != 0 {
		t.Errorf("Size = %dx%d before the first frame, want 0x0 (nothing opened yet)", w, h)
	}

	frame, err := th.FrameAt(time.Second)
	if err != nil {
		t.Fatalf("FrameAt: %v", err)
	}
	if got := frame.Bounds(); got.Dx() != 320 || got.Dy() != 240 {
		t.Errorf("frame bounds = %v, want 320x240", got)
	}
	if w, h := th.Size(); w != 320 || h != 240 {
		t.Errorf("Size = %dx%d after a frame, want 320x240", w, h)
	}
}

// TestThumbnailerFramesDiffer is what makes the preview worth having: asking for
// two positions must give two different pictures, not the same frame twice. A
// seek that failed silently, or a decoder that returned its buffered frame
// regardless, would pass every other check here.
func TestThumbnailerFramesDiffer(t *testing.T) {
	th := NewThumbnailer(thumbTestMedia)
	defer th.Close()

	early, err := th.FrameAt(0)
	if err != nil {
		t.Fatalf("FrameAt(0): %v", err)
	}
	first := append([]byte(nil), early.Pix...)

	late, err := th.FrameAt(3 * time.Second)
	if err != nil {
		t.Fatalf("FrameAt(3s): %v", err)
	}

	if len(first) != len(late.Pix) {
		t.Fatalf("frame sizes differ: %d vs %d", len(first), len(late.Pix))
	}
	same := true
	for i := range first {
		if first[i] != late.Pix[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("the frames at 0s and 3s are pixel-identical — the seek did nothing")
	}
}

// TestThumbnailerStopsAtTheKeyframe is the reason this type exists at all, and
// it is asserted structurally rather than by timing.
//
// FFmpegSource.Seek lands on a keyframe and then decodes forward, discarding
// frames, until Position reaches the exact target — right for playback and far
// too slow to run once per pointer movement. The preview must not do that: it
// reads until the first video frame and stops.
//
// The fake decoder here is the sharpest way to say so. Its Position never
// advances, so the decode-forward loop would run until the stream ended;
// device_test.go says as much, and deliberately avoids calling Seek on a fake
// for that reason. The thumbnailer returns after a single packet.
//
// A timing comparison was tried first and is not worth having: the playback seek
// never calls GetRGBA, so it is not the same work, and the 4-second 320x240
// fixture has no GOP depth for a decode-forward loop to cost anything on. The
// packet count is the property; the speed is a consequence.
func TestThumbnailerStopsAtTheKeyframe(t *testing.T) {
	fake := &fakeDecoder{duration: 3600}
	th := newThumbnailer(func() (goffmpeg.IDecoder, error) { return fake, nil })
	defer th.Close()

	if _, err := th.FrameAt(30 * time.Minute); err != nil {
		t.Fatalf("FrameAt: %v", err)
	}

	if fake.seeks != 1 {
		t.Errorf("decoder seeks = %d, want exactly 1", fake.seeks)
	}
	if fake.nexts != 1 {
		t.Errorf("decoder reads = %d, want 1 — the frame at the keyframe, not a "+
			"decode forward to the exact position", fake.nexts)
	}
}

// TestThumbnailerSkipsInterleavedAudio: the first packet after a seek is
// frequently audio, and returning nothing in that case would give a preview that
// works on some files and not others.
func TestThumbnailerSkipsInterleavedAudio(t *testing.T) {
	fake := &fakeDecoder{duration: 60, leadingAudio: 3}
	th := newThumbnailer(func() (goffmpeg.IDecoder, error) { return fake, nil })
	defer th.Close()

	if _, err := th.FrameAt(time.Second); err != nil {
		t.Fatalf("FrameAt: %v", err)
	}
	if fake.nexts != 4 {
		t.Errorf("decoder reads = %d, want 4 — three audio packets then the video frame",
			fake.nexts)
	}
}

// TestThumbnailerGivesUpOnAStreamWithNoVideo guards the bound on that skip: a
// stream that never yields a video frame must fail rather than spin.
func TestThumbnailerGivesUpOnAStreamWithNoVideo(t *testing.T) {
	fake := &fakeDecoder{duration: 60, leadingAudio: 1 << 30}
	th := newThumbnailer(func() (goffmpeg.IDecoder, error) { return fake, nil })
	defer th.Close()

	if _, err := th.FrameAt(time.Second); err == nil {
		t.Error("FrameAt on a stream with no video frame succeeded, want a bounded failure")
	}
}

func TestThumbnailerClosedIsFinal(t *testing.T) {
	th := NewThumbnailer(thumbTestMedia)
	if _, err := th.FrameAt(0); err != nil {
		t.Fatalf("FrameAt: %v", err)
	}
	th.Close()

	// Reopening after Close would leave a file handle open that nothing owns.
	if _, err := th.FrameAt(0); !errors.Is(err, ErrThumbnailerClosed) {
		t.Errorf("FrameAt after Close = %v, want ErrThumbnailerClosed", err)
	}
	th.Close() // idempotent
}

func TestThumbnailerReportsAnUnopenableSource(t *testing.T) {
	th := NewThumbnailer("/nonexistent/nope.mp4")
	defer th.Close()

	if _, err := th.FrameAt(0); err == nil {
		t.Error("FrameAt on an unopenable source succeeded")
	}
}

// TestThumbnailerClampsNegativePositions: the hover fraction is clamped
// upstream, but a preview asking a decoder to seek before zero should not
// depend on that.
func TestThumbnailerClampsNegativePositions(t *testing.T) {
	th := NewThumbnailer(thumbTestMedia)
	defer th.Close()

	if _, err := th.FrameAt(-5 * time.Second); err != nil {
		t.Errorf("FrameAt(-5s) = %v, want it clamped to the start", err)
	}
}
