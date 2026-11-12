package source

import (
	"fmt"
	"image"
	"time"
)

// VideoFrame is a decoded video frame that has not been converted to RGBA yet.
// The source does not choose where the pixels go: the callback reads the frame
// into a buffer of its own, so playback converts each frame once, straight into
// a frame of the pipeline's pool, and allocates nothing per frame (#431).
//
// A VideoFrame is valid only during the VideoCallback it is passed to. Read
// must not be called after the callback returns.
type VideoFrame struct {
	// Width and Height are the frame's own size. They can differ from the
	// size the source declared at open, which a device that declares 0x0
	// does.
	Width, Height int
	// PTS is the frame's presentation timestamp from the stream, which the
	// pipeline paces against the audio clock — deriving video time from a
	// frame counter instead lets the two drift apart with no way back.
	PTS time.Duration
	// Read converts the frame into dst, which must be Width x Height. It
	// writes every pixel of dst, or returns an error.
	Read func(dst *image.RGBA) error
}

// VideoCallback is called when a decoded video frame is ready. A callback
// that does not want the frame — the pool is exhausted, say — returns without
// calling Read, and the conversion is never paid for.
type VideoCallback func(frame VideoFrame)

// RGBAFrame is a VideoFrame over pixels that are already RGBA: Read copies img.
// For sources that produce RGBA themselves, which the test stubs do.
func RGBAFrame(img *image.RGBA, pts time.Duration) VideoFrame {
	return VideoFrame{
		Width:  img.Rect.Dx(),
		Height: img.Rect.Dy(),
		PTS:    pts,
		Read: func(dst *image.RGBA) error {
			if dst.Rect.Size() != img.Rect.Size() {
				return fmt.Errorf("frame is %v, destination %v", img.Rect.Size(), dst.Rect.Size())
			}
			copy(dst.Pix, img.Pix)
			return nil
		},
	}
}

// AudioCallback is called when decoded PCM audio samples (S16LE) are ready.
type AudioCallback func(data []byte)

// Source abstracts a media source that can produce video frames and/or audio samples.
type Source interface {
	HasVideo() bool
	HasAudio() bool
	VideoSize() (width, height int)
	Duration() time.Duration
	Framerate() float64
	SampleRate() int
	Channels() int

	SetVideoCallback(VideoCallback)
	SetAudioCallback(AudioCallback)

	// SetAudioChannels asks the source to emit n interleaved S16 channels;
	// n <= 0 restores the stream's native layout. Channels reports the result,
	// which may differ from n. Call before decoding starts.
	SetAudioChannels(n int) error

	// SetAudioSampleRate asks the source to emit audio at rate Hz; rate <= 0
	// restores the stream's own rate. SampleRate reports the result. Call
	// before decoding starts.
	SetAudioSampleRate(rate int) error

	// Decode decodes media for the given elapsed duration, calling callbacks as needed.
	Decode(elapsed time.Duration)

	// Seek moves to the given position.
	Seek(pos time.Duration)

	// Rewind resets to the beginning.
	Rewind()

	// HasEnded returns true if the source has been fully consumed.
	HasEnded() bool

	// Close releases resources.
	Close()
}

// LiveSource is an optional capability, asserted rather than required: a source
// that is an open-ended capture — a camera, a microphone, a live stream — rather
// than a finite recording. A source that does not implement it is finite.
//
// It exists because a live source breaks two assumptions the rest of the
// pipeline makes about a file. Duration is 0 and Seek has no timeline to move
// within; and the stream is not supposed to run out, so reaching the end of one
// means the device was lost rather than that playback completed.
type LiveSource interface {
	Source

	// IsLive reports whether this source is an open-ended capture.
	IsLive() bool
}

// ErrorSource is an optional capability: a source that can say why it stopped
// producing media. The pipeline uses it to report a lost capture device with its
// actual cause — "device in use by another application" is the common one, and
// is worth a great deal more to a caller than a generic failure.
type ErrorSource interface {
	// Err returns the error that stopped the source, or nil if it stopped
	// cleanly or is still running.
	Err() error
}

// Interruptible is an optional capability: a source whose blocking I/O another
// goroutine can cut short. A far seek in a network MP3 with no seek table reads
// the file through to the target, seconds of I/O, and the pipeline's Close,
// Stop and SwapSource would otherwise wait for it (#884).
type Interruptible interface {
	// Interrupt makes a Decode, Seek or Rewind that is blocked on I/O return
	// promptly, without recording an error: it was asked to stop, it did not
	// fail. The interrupted call leaves the source nowhere in particular, so
	// Decode does nothing until the next Seek or Rewind, which reopens the
	// source first. Safe to call from any goroutine.
	Interrupt()
}
