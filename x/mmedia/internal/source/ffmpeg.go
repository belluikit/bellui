package source

import (
	"image"
	"io"
	"time"

	"github.com/belluikit/goffmpeg"
)

// FFmpegSource decodes any container/codec supported by the bundled FFmpeg
// libraries through goffmpeg's unified decoder bridge. Video and audio are
// demuxed and PTS-synchronized by the bridge; Decode drains frames up to the
// elapsed playback position.
type FFmpegSource struct {
	dec      goffmpeg.IDecoder
	vInfo    goffmpeg.VideoInfo
	aInfo    goffmpeg.AudioInfo
	hasVideo bool
	hasAudio bool

	// live marks an open-ended capture device, which has no duration, no
	// timeline to seek within, and no end it is supposed to reach.
	live bool
	// reopen reacquires a released capture device. Set only for a live source;
	// see Rewind for why a device is released rather than held.
	reopen func() (goffmpeg.IDecoder, error)
	// err records why the source stopped, for ErrorSource.
	err error
	// stalledSince is when a live capture last started producing nothing; zero
	// while it is healthy. See tolerateStall.
	stalledSince time.Time
	// ptsOrigin is the first presentation timestamp seen from a live capture,
	// subtracted from the rest. See presentationTime.
	ptsOrigin time.Duration
	ptsBased  bool

	// wantChannels and wantRate are the audio format the pipeline asked for
	// through SetAudioChannels and SetAudioSampleRate; 0 is the stream's own.
	// They are kept because the decoder forgets them: a reacquired device opens
	// at its own format (see acquire), and a seek leaves swresample holding
	// audio from before it (see Seek).
	wantChannels int
	wantRate     int

	videoCallback VideoCallback
	audioCallback AudioCallback
	ended         bool
}

// NewFFmpegSource decodes from r. Container and codec are probed by
// libavformat, so no format detection is needed up front. A reader that also
// implements io.Seeker gets working Seek and an accurate Duration; a plain
// stream is forward-only.
func NewFFmpegSource(r io.Reader) (*FFmpegSource, error) {
	dec, err := goffmpeg.OpenReader(r)
	if err != nil {
		return nil, err
	}
	return newFFmpegSource(dec), nil
}

// NewFFmpegSourceURL decodes from a file path or network URL (http/https are
// compiled in), letting libavformat own the I/O rather than reading through Go.
func NewFFmpegSourceURL(url string) (*FFmpegSource, error) {
	dec, err := goffmpeg.Open(url)
	if err != nil {
		return nil, err
	}
	return newFFmpegSource(dec), nil
}

func newFFmpegSource(dec goffmpeg.IDecoder) *FFmpegSource {
	s := &FFmpegSource{
		dec:   dec,
		vInfo: dec.VideoInfo(),
		aInfo: dec.AudioInfo(),
	}
	s.hasVideo = s.vInfo.Width > 0 && s.vInfo.Height > 0
	s.hasAudio = s.aInfo.SampleRate > 0
	return s
}

func (s *FFmpegSource) HasVideo() bool { return s.hasVideo }
func (s *FFmpegSource) HasAudio() bool { return s.hasAudio }
func (s *FFmpegSource) VideoSize() (int, int) {
	if !s.hasVideo {
		return 0, 0
	}
	return s.vInfo.Width, s.vInfo.Height
}

// IsLive implements LiveSource. A device source answers true; a file or stream
// opened by url or reader answers false.
func (s *FFmpegSource) IsLive() bool { return s.live }

// Err implements ErrorSource, reporting why the source stopped producing media.
func (s *FFmpegSource) Err() error { return s.err }

func (s *FFmpegSource) Duration() time.Duration {
	// A capture device has no duration, and after Rewind released it there is no
	// decoder to ask. Both answer zero, which is also the signal the transport
	// controls key their live presentation off.
	if s.live || s.dec == nil {
		return 0
	}
	return time.Duration(s.dec.Duration() * float64(time.Second))
}

func (s *FFmpegSource) Framerate() float64 {
	if s.vInfo.Framerate > 0 {
		return s.vInfo.Framerate
	}
	return 30.0
}

func (s *FFmpegSource) SampleRate() int {
	if s.aInfo.SampleRate > 0 {
		return s.aInfo.SampleRate
	}
	return 44100
}

func (s *FFmpegSource) Channels() int {
	if s.aInfo.Channels > 0 {
		return s.aInfo.Channels
	}
	return 2
}

func (s *FFmpegSource) SetVideoCallback(f VideoCallback) { s.videoCallback = f }
func (s *FFmpegSource) SetAudioCallback(f AudioCallback) { s.audioCallback = f }

// SetAudioChannels reconfigures the decoder's resampler. swresample performs
// the mix, so a 5.1 source folded to stereo gets proper downmix coefficients
// rather than dropped channels.
func (s *FFmpegSource) SetAudioChannels(n int) error {
	if !s.hasAudio {
		return nil
	}
	s.wantChannels = n
	if s.dec == nil {
		return nil // a released device; acquire applies it on reopening
	}
	if err := s.dec.SetAudioChannels(n); err != nil {
		return err
	}
	s.aInfo = s.dec.AudioInfo()
	return nil
}

// SetAudioSampleRate has the decoder's swresample convert the rate too, so
// the audio device, fixed at one rate for the process, can play any stream.
func (s *FFmpegSource) SetAudioSampleRate(rate int) error {
	if !s.hasAudio {
		return nil
	}
	s.wantRate = rate
	if s.dec == nil {
		return nil // a released device; acquire applies it on reopening
	}
	if err := s.dec.SetAudioSampleRate(rate); err != nil {
		return err
	}
	s.aInfo = s.dec.AudioInfo()
	return nil
}

// Decode decodes one unit of media per call: for video streams it decodes
// up to the next video frame (dispatching any interleaved audio along the
// way); for audio-only streams it decodes one audio packet. The pipeline's
// decode-ahead loop provides pacing through frame-queue and audio-buffer
// back-pressure, so the elapsed argument is ignored.
func (s *FFmpegSource) Decode(_ time.Duration) {
	if s.ended {
		return
	}
	if !s.acquire() {
		return
	}

	for {
		mt, err := s.dec.Next()
		if err != nil {
			if s.live && s.tolerateStall(err) {
				return
			}
			s.err = err
			s.ended = true
			return
		}
		// A successful read clears any stall in progress, so a device that
		// hiccups and recovers does not carry an error forward.
		s.stalledSince = time.Time{}
		s.err = nil

		switch mt {
		case goffmpeg.MediaTypeVideo:
			if s.videoCallback != nil {
				rgba := image.NewRGBA(image.Rect(0, 0, s.vInfo.Width, s.vInfo.Height))
				if err := s.dec.GetRGBA(rgba); err == nil {
					// Position is the decoded frame's own PTS, set by the
					// bridge from best_effort_timestamp.
					s.videoCallback(rgba, s.presentationTime(s.dec.Position()))
				}
			}
			return // one video frame per call
		case goffmpeg.MediaTypeAudio:
			if s.audioCallback != nil {
				if data, err := s.dec.GetAudio(); err == nil && len(data) > 0 {
					s.audioCallback(data)
				}
			}
			if !s.hasVideo {
				return // audio-only: one packet per call
			}
		case goffmpeg.MediaTypeNone:
			// End of stream.
			s.ended = true
			return
		}
	}
}

// presentationTime converts a decoder position into the presentation timestamp
// the pipeline paces against, rebasing a live capture onto zero.
//
// A capture device stamps frames with absolute host time rather than from the
// start of a stream: a macOS camera reports around 4365842s — the machine's
// uptime, some 50 days — for its very first frame, with correct 16ms deltas after
// it. The pipeline holds each frame until its timestamp comes due on a clock that
// starts at zero, so passing those through stalls display for as long as the
// machine has been up: frames decode until the queue fills and then nothing is
// ever shown. Measured with hack/captureprobe -raw.
//
// Rebasing on the first frame keeps the inter-frame deltas, which is what the A/V
// sync actually needs, while starting where the clock does. The origin is retaken
// after a release, since a reacquired device resumes from absolute time while the
// pipeline clock has been reset.
func (s *FFmpegSource) presentationTime(pos float64) time.Duration {
	pts := time.Duration(pos * float64(time.Second))
	if !s.live {
		return pts
	}
	if !s.ptsBased {
		s.ptsOrigin = pts
		s.ptsBased = true
	}
	return pts - s.ptsOrigin
}

// liveStallTimeout is how long a live capture may produce nothing before the
// source is declared ended. Long enough to cover a slow camera starting up —
// avfoundation in particular takes a moment — and short enough that a device
// which really has gone away is reported rather than waited on forever.
const liveStallTimeout = 5 * time.Second

// liveRetryPause keeps a stalled live source from spinning the decode-ahead loop
// at full speed on a device that simply has no frame ready yet.
const liveRetryPause = 5 * time.Millisecond

// tolerateStall decides whether a failed read from a live capture is a hiccup or
// the end of it, reporting true to retry.
//
// A non-blocking capture device reports "no packet available yet" as an ordinary
// read error: av_read_frame returns EAGAIN. For a file that would mean the stream
// is over, but for a camera it means only that the next frame has not arrived.
// Treating the first one as terminal ends capture within milliseconds of starting
// it — measured on macOS/avfoundation, which stalls after about two frames.
//
// The bridge already retries EAGAIN from avcodec_receive_frame but not from
// av_read_frame, and goffmpeg surfaces the result as an opaque "decoder error:
// <n>" with no sentinel to match on, so there is nothing reliable to test for
// here. Rather than parse that text, every live read failure is treated as
// retryable and time decides: a camera that has produced nothing for
// liveStallTimeout is genuinely gone, which is what the pipeline reports through
// ErrCaptureStopped.
func (s *FFmpegSource) tolerateStall(err error) bool {
	now := time.Now()
	if s.stalledSince.IsZero() {
		s.stalledSince = now
	}
	if now.Sub(s.stalledSince) >= liveStallTimeout {
		return false
	}
	s.err = err
	time.Sleep(liveRetryPause)
	return true
}

// acquire makes sure there is a decoder to read from, reacquiring a capture
// device that Rewind released. It reports whether decoding can proceed; a
// failure to reacquire is terminal and is recorded for ErrorSource, since the
// pipeline surfaces it as a lost device rather than as an end of stream.
func (s *FFmpegSource) acquire() bool {
	if s.dec != nil {
		return true
	}
	if s.reopen == nil {
		s.ended = true
		return false
	}
	dec, err := s.reopen()
	if err != nil {
		s.err = err
		s.ended = true
		return false
	}
	// A reopened decoder emits the device's own rate and layout. The pipeline
	// asked for the audio device's format once, when it adopted this source, and
	// the audio device cannot follow the decoder back: its format is fixed for
	// the process. Without this, a 44.1 kHz mono microphone stopped and played
	// again wrote mono 44.1 kHz into a 48 kHz stereo device, garbled and fast.
	if err := applyAudioFormat(dec, s.wantChannels, s.wantRate); err != nil {
		dec.Close()
		s.err = err
		s.ended = true
		return false
	}
	s.dec = dec
	// A reacquired device is not required to come back with the geometry it had.
	// Refresh what the decoder reports so frames are allocated at the size it is
	// actually producing; the streaming raster handles a size change by dropping
	// its texture, and hasVideo/hasAudio stay as first negotiated because the
	// pipeline was assembled around them.
	s.vInfo, s.aInfo = dec.VideoInfo(), dec.AudioInfo()
	return true
}

// applyAudioFormat asks dec for the channels and rate a pipeline asked of the
// source (0: the stream's own). A decoder with no audio stream is left alone.
func applyAudioFormat(dec goffmpeg.IDecoder, channels, rate int) error {
	if dec.AudioInfo().SampleRate <= 0 {
		return nil
	}
	if channels != 0 {
		if err := dec.SetAudioChannels(channels); err != nil {
			return err
		}
	}
	if rate != 0 {
		return dec.SetAudioSampleRate(rate)
	}
	return nil
}

// release closes the current decoder but leaves the source usable: the next
// Decode reacquires it through acquire.
func (s *FFmpegSource) release() {
	if s.dec != nil {
		s.dec.Close()
		s.dec = nil
	}
	s.ended = false
	s.err = nil
	s.stalledSince = time.Time{}
	// A reacquired device resumes stamping from absolute time while the pipeline
	// clock restarts at zero, so the rebase origin has to be taken again.
	s.ptsBased = false
}

// Seek moves to the given position. A live capture has no timeline to move
// within — av_seek_frame on a device is undefined rather than merely wasteful —
// so this is a no-op for one. The transport controls disable the seek bar for a
// live source as well; this is the floor under that.
func (s *FFmpegSource) Seek(pos time.Duration) {
	if s.live {
		return
	}
	if s.dec == nil {
		return
	}
	if err := s.dec.Seek(pos.Seconds()); err != nil {
		return
	}
	s.ended = false
	s.err = nil

	// A rate conversion holds a few frames of filter delay, which seeking the
	// demuxer does not flush: the first audio after the seek would start with
	// audio from before it, a click at every seek and every loop. Rebuilding the
	// resampler discards what it buffered (goffmpeg's decoder_set_audio_channels).
	// Should the rebuild fail, goffmpeg restores a working resampler, so the cost
	// is only the stale frames this avoids.
	if s.hasAudio && s.wantRate != 0 {
		_ = s.dec.SetAudioSampleRate(s.wantRate)
	}

	// av_seek_frame lands on the keyframe at or before the target (which
	// can be several seconds earlier depending on the GOP size). Decode and
	// discard until the stream reaches the requested position so the next
	// Decode call delivers frames from the seek target onward. Position()
	// is only meaningful after a decode, so always decode at least once.
	target := pos.Seconds()
	for {
		mt, err := s.dec.Next()
		if err != nil || mt == goffmpeg.MediaTypeNone {
			// Recorded as Decode does, so a seek into a dropped stream is
			// reported as the error it is, not as an end (#839).
			s.err = err
			s.ended = true
			return
		}
		if s.dec.Position() >= target {
			return
		}
	}
}

// Rewind returns a finite source to its start.
//
// A capture device has no start to return to, so it is released instead. The
// pipeline calls this from Stop, and holding a camera open after the user pressed
// stop — with its indicator light on, and with the device unavailable to every
// other application — would be the wrong reading of the word. The next Decode
// reacquires it, so Stop followed by Play works as it does for a file.
func (s *FFmpegSource) Rewind() {
	if s.live {
		s.release()
		return
	}
	s.Seek(0)
}

func (s *FFmpegSource) HasEnded() bool { return s.ended }

func (s *FFmpegSource) Close() {
	if s.dec != nil {
		s.dec.Close()
		s.dec = nil
	}
	// Close is final, unlike release: nothing should reacquire the device after
	// the caller has finished with the source.
	s.reopen = nil
}
