package source

import "github.com/belluikit/goffmpeg"

// NewFFmpegSourceDevice opens a live capture device — a camera or a microphone —
// through libavdevice.
//
// This is deliberately not NewFFmpegSourceURL with a device path: Open(url) and
// OpenDevice(format, url, options) are different calls. libavformat needs an
// explicit AVInputFormat to treat "/dev/video0" or "video=Logitech BRIO" as a
// device; given only a url it probes them as file paths and fails.
//
// format, url and options are the raw goffmpeg.OpenDevice contract, which differs
// per platform. Resolving that is the job of the public capture entry points, not
// of this layer.
//
// The result implements LiveSource, so Duration is 0, Seek is a no-op, Rewind
// releases the device rather than rewinding it, and the pipeline reports the end
// of the stream as a lost device rather than as completed playback.
func NewFFmpegSourceDevice(format, url, options string) (*FFmpegSource, error) {
	return newDeviceSource(func() (goffmpeg.IDecoder, error) {
		return goffmpeg.OpenDevice(format, url, options)
	})
}

// newDeviceSource builds a live source over an opener that can be called again to
// reacquire the device. Split from NewFFmpegSourceDevice so the release and
// reacquire behaviour is testable against a fake decoder, with no camera present.
func newDeviceSource(open func() (goffmpeg.IDecoder, error)) (*FFmpegSource, error) {
	dec, err := open()
	if err != nil {
		return nil, err
	}
	s := newFFmpegSource(dec)
	s.live = true
	s.reopen = open
	return s, nil
}

// Compile-time proof that a device source satisfies the optional capabilities the
// pipeline asserts for. Without these, dropping either method would silently
// downgrade a capture to a finite source rather than fail the build.
var (
	_ LiveSource  = (*FFmpegSource)(nil)
	_ ErrorSource = (*FFmpegSource)(nil)
)
