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
