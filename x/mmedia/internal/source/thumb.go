package source

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/belluikit/goffmpeg"
)

// ErrThumbnailerClosed reports a frame requested from a Thumbnailer that has
// been closed.
var ErrThumbnailerClosed = errors.New("mmedia: thumbnailer is closed")

// Thumbnailer decodes one frame at an arbitrary position, for scrub preview.
//
// It is a second decoder over the same media, and it has to be: the preview
// cannot share the playback decoder without moving the playback position, which
// is the one thing a preview must not do. That means the media must be
// something FFmpeg can open twice — a path or a URL. A source read through an
// io.Reader cannot have one, because there is no second reader to give it.
//
// The decoder is opened lazily, on the first frame requested. A preview that is
// never used therefore costs nothing: no second file handle, and for a network
// URL no second connection.
type Thumbnailer struct {
	// open produces the decoder, on first use.
	open func() (goffmpeg.IDecoder, error)

	mu     sync.Mutex
	dec    goffmpeg.IDecoder
	closed bool
	// width and height are the decoded geometry, known only once the decoder is
	// open.
	width, height int
}

// NewThumbnailer prepares a preview decoder over url. Nothing is opened here;
// see FrameAt.
func NewThumbnailer(url string) *Thumbnailer {
	return newThumbnailer(func() (goffmpeg.IDecoder, error) {
		dec, err := openURL(url)
		if err != nil {
			return nil, fmt.Errorf("mmedia: opening a second decoder for scrub preview: %w", err)
		}
		return dec, nil
	})
}

// newThumbnailer builds one over an arbitrary opener. Split out for the same
// reason newDeviceSource is: it lets the decode behaviour be asserted against a
// fake decoder, and the property that matters here — how many packets a preview
// reads — is one no real media file can pin down.
func newThumbnailer(open func() (goffmpeg.IDecoder, error)) *Thumbnailer {
	return &Thumbnailer{open: open}
}

// FrameAt decodes the frame at or before pos and returns it.
//
// "At or before" is the whole point. FFmpegSource.Seek lands on the keyframe at
// or before the target and then decodes forward, discarding frames, until it
// reaches the exact position — right for playback, and far too slow to run once
// per pointer movement across a seek bar. This stops at the keyframe: visually
// approximate, which is what every other player shows while scrubbing, and one
// decode rather than a GOP's worth.
//
// The keyframe part costs nothing extra to ask for. goffmpeg's decoder_seek
// already passes AVSEEK_FLAG_BACKWARD, so a seek lands on a keyframe; the
// expense was only ever the loop after it.
//
// Safe to call from any goroutine — calls are serialised, since there is one
// decoder behind them.
func (t *Thumbnailer) FrameAt(pos time.Duration) (*image.RGBA, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, ErrThumbnailerClosed
	}
	if err := t.ensureDecoder(); err != nil {
		return nil, err
	}
	if pos < 0 {
		pos = 0
	}
	if err := t.dec.Seek(pos.Seconds()); err != nil {
		return nil, fmt.Errorf("mmedia: seeking the preview decoder to %v: %w", pos, err)
	}

	// Interleaved audio arrives between the seek and the first video frame, so
	// read until video rather than assuming the first packet is one. The bound
	// is generous but finite: a stream whose next video frame never arrives must
	// fail rather than spin.
	const maxPackets = 256
	for i := 0; i < maxPackets; i++ {
		mt, err := t.dec.Next()
		if err != nil {
			return nil, fmt.Errorf("mmedia: decoding a preview frame at %v: %w", pos, err)
		}
		if mt == goffmpeg.MediaTypeNone {
			return nil, fmt.Errorf("mmedia: no preview frame available at %v", pos)
		}
		if mt != goffmpeg.MediaTypeVideo {
			continue
		}
		rgba := image.NewRGBA(image.Rect(0, 0, t.width, t.height))
		if err := t.dec.GetRGBA(rgba); err != nil {
			return nil, fmt.Errorf("mmedia: reading a preview frame at %v: %w", pos, err)
		}
		return rgba, nil
	}
	return nil, fmt.Errorf("mmedia: no video frame within %d packets of %v", maxPackets, pos)
}

// Size reports the preview frame geometry, or 0x0 before the first frame has
// been decoded — the decoder is not open until then, so there is nothing to ask.
func (t *Thumbnailer) Size() (int, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.width, t.height
}

// ensureDecoder makes sure there is a decoder. Called with t.mu held.
func (t *Thumbnailer) ensureDecoder() error {
	if t.dec != nil {
		return nil
	}
	dec, err := t.open()
	if err != nil {
		return err
	}
	vi := dec.VideoInfo()
	if vi.Width <= 0 || vi.Height <= 0 {
		dec.Close()
		return errors.New("mmedia: the preview decoder found no video stream")
	}
	t.dec, t.width, t.height = dec, vi.Width, vi.Height
	return nil
}

// Close releases the preview decoder. Further frame requests report
// ErrThumbnailerClosed rather than reopening: closing is final, and a preview
// that quietly reopened its media after the player was done with it would hold
// a file handle nobody expects.
func (t *Thumbnailer) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.dec != nil {
		t.dec.Close()
		t.dec = nil
	}
}
