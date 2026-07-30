package source

import (
	"image"
	"time"
)

// VideoCallback is called when a decoded video frame is ready. pts is the
// frame's presentation timestamp from the stream, which the pipeline paces
// against the audio clock — deriving video time from a frame counter instead
// lets the two drift apart with no way back.
type VideoCallback func(frame *image.RGBA, pts time.Duration)

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
