package mmedia

import "time"

// Option configures a player.
type Option func(*options)

type options struct {
	autoPlay bool
	loop     bool
	volume   float64
	downmix  bool // if true, multichannel audio is downmixed to stereo
	onEnd    func()
	onError  func(error)
	onTime   func(current, total time.Duration)
	// captureOptions is forwarded to libavdevice when opening a capture device;
	// it has no meaning for file or url playback.
	captureOptions string
	// scrubPreview asks for a preview frame while the seek bar is hovered. It
	// needs a second decoder, so it is only honourable for a source FFmpeg can
	// reopen; see WithScrubPreview.
	scrubPreview bool
}

// buildOptions applies the functional options over the defaults.
func buildOptions(opts []Option) *options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func defaultOptions() *options {
	return &options{
		volume: 1.0,
	}
}

// WithAutoPlay starts playback immediately after opening media.
func WithAutoPlay() Option {
	return func(o *options) {
		o.autoPlay = true
	}
}

// WithLoop enables looping playback.
//
// It has no effect on a capture player: a camera has no end to loop back from,
// and the end of a live stream is reported as an error rather than as completed
// playback.
func WithLoop() Option {
	return func(o *options) {
		o.loop = true
	}
}

// WithCaptureOptions forwards a comma-separated "key=value" list to libavdevice
// when opening a capture device, for example "framerate=30,pixel_format=nv12".
// The accepted keys are the device format's own, and differ per platform.
//
// Naming a framerate here also opts out of the fallback a zero-config open
// performs: the rate given is tried once and its failure is reported, rather
// than being quietly replaced by one the device happens to accept. Leave it out
// unless the device needs a specific rate.
//
// It is ignored by the file, reader and url constructors, which have no device to
// configure.
func WithCaptureOptions(deviceOptions string) Option {
	return func(o *options) {
		o.captureOptions = deviceOptions
	}
}

// WithScrubPreview shows the frame under the pointer while the seek bar is
// hovered, in a thumbnail floated above it.
//
// It needs a second decoder over the same media — the preview cannot share the
// playback one without moving the playback position — so the media has to be
// something FFmpeg can open twice. That means a path or a URL:
// NewMediaPlayerURL, NewVideoPlayerURL, or NewMediaPlayer handed an *os.File
// whose name is still reachable.
//
// Passing it to a constructor that cannot honour it is an error rather than a
// quiet no-op, so a caller finds out at the point of asking rather than by
// noticing nothing happens. A reader-backed player is the case: there is no
// second reader to give the preview decoder.
//
// A live source gets no preview either, but that is not an error — a camera has
// no timeline to preview along, the seek bar is disabled for the same reason,
// and a player can reasonably be written to handle both kinds of media.
//
// The second decoder is opened lazily, on the first hover: a preview that is
// never used costs no file handle and, for a network URL, no connection.
func WithScrubPreview() Option {
	return func(o *options) {
		o.scrubPreview = true
	}
}

// WithVolume sets the initial playback volume (0.0 to 1.0).
func WithVolume(v float64) Option {
	return func(o *options) {
		o.volume = v
	}
}

// WithOnEnd sets a callback for when playback ends.
func WithOnEnd(f func()) Option {
	return func(o *options) {
		o.onEnd = f
	}
}

// WithOnError sets a callback for playback errors.
func WithOnError(f func(error)) Option {
	return func(o *options) {
		o.onError = f
	}
}

// WithOnTimeUpdate sets a callback for periodic time updates.
func WithOnTimeUpdate(f func(current, total time.Duration)) Option {
	return func(o *options) {
		o.onTime = f
	}
}

// WithDownmix forces multichannel audio (5.1/7.1) to be downmixed to stereo.
// Without this option the stream's native channel layout is passed through,
// provided the audio device accepts it — the device is opened once per process,
// so a later stream is downmixed to whatever the first one established.
func WithDownmix() Option {
	return func(o *options) {
		o.downmix = true
	}
}
