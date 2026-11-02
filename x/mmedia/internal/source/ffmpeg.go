package source

import (
	"errors"
	"fmt"
	"image"
	"io"
	"strings"
	"sync"
	"sync/atomic"
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

	// open opens the source again, for a source opened by URL; nil for a
	// reader or a device. Interrupt leaves the decoder good only for closing,
	// so only a source that can be reopened can be interrupted (#884).
	open func() (goffmpeg.IDecoder, error)
	// decMu orders Interrupt, which runs on whichever goroutine is stopping
	// the pipeline, against the decode goroutine replacing dec. It guards
	// writes to dec and every access to interrupts; the decode goroutine reads
	// dec without it, as the only writer besides Close.
	decMu sync.Mutex
	// interrupts counts Interrupt calls, so a reopen can tell whether another
	// came in while it was opening.
	interrupts uint64
	// interrupted is set by Interrupt and cleared by the reopen it calls for.
	interrupted atomic.Bool
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

// NewFFmpegSourceURL decodes from a file path or network URL, letting
// libavformat own the I/O rather than reading through Go. An http(s) URL
// reconnects on its own; see openURL. On Android an https URL is read through
// Go instead, since its FFmpeg has no TLS; see httpsInGo.
func NewFFmpegSourceURL(url string) (*FFmpegSource, error) {
	dec, err := openURL(url)
	if err != nil {
		return nil, err
	}
	s := newFFmpegSource(dec)
	s.open = func() (goffmpeg.IDecoder, error) { return openURL(url) }
	return s, nil
}

// httpOptions make FFmpeg ride out a network that drops or stalls (#839).
// reconnect picks a dropped connection up again from the byte it stopped at,
// and rw_timeout (in microseconds) turns a server that goes silent without
// closing into a drop, which would otherwise block the read forever. A server
// that stays gone still fails, within a few seconds (reconnect_delay_max), and
// the pipeline reports it through OnError.
//
// Measured on an MP3 over HTTP: a drop and a 5 s stall both played on to the
// end; without these, a drop ended playback and a stall hung it. HLS did no
// better and no worse with them.
const httpOptions = "reconnect=1,reconnect_delay_max=2,rw_timeout=5000000"

// liveHTTPOptions add reconnect_streamed, which reconnects a stream the server
// cannot seek in. That is right for a live stream: a dropped one with it ends
// in OnError, and without it in a clean end, as if the station had finished.
// It is wrong for a file from a server that ignores Range. The reconnect asks
// for the rest, the server answers 200 from byte 0, and FFmpeg plays the
// file's start as the continuation, with no error (#868). Measured on a 180 s
// MP3 cut at byte 200000 and seeked to 177 s: decoding ran on to 192.5 s.
const liveHTTPOptions = httpOptions + ",reconnect_streamed=1"

// openURL opens url, with httpOptions for an http(s) URL. A purego
// libgoffmpeg too old for per-open options gets a plain open rather than none.
//
// FFmpeg reports nothing about the server, so the duration tells a live stream
// from a file: a stream with no length has none. Such a stream is opened again
// with liveHTTPOptions. A file pays nothing extra; a live stream pays a second
// connection. A server that sends no length for a file whose duration FFmpeg
// can still read (a VBR MP3's Xing header) is taken for a file, and a drop
// there ends cleanly, as every live stream's did without reconnect_streamed.
func openURL(url string) (goffmpeg.IDecoder, error) {
	lower := strings.ToLower(url)
	if httpsInGo && strings.HasPrefix(lower, "https://") {
		return openHTTPSInGo(url)
	}
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return goffmpeg.Open(url)
	}
	dec, err := goffmpeg.OpenWithOptions(url, httpOptions)
	if errors.Is(err, goffmpeg.ErrOpenOptionsUnavailable) {
		return goffmpeg.Open(url)
	}
	if err != nil || dec.Duration() > 0 {
		return dec, err
	}
	dec.Close()
	return goffmpeg.OpenWithOptions(url, liveHTTPOptions)
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
// IgnoreVideo makes the source audio-only: it reports no video, and Decode
// skips video packets. For a player with no picture, so that a video stream
// never paces its audio (#850). A song's embedded cover art is no longer one:
// goffmpeg skips it since v0.6.1. Call it before the source is handed to a
// pipeline.
func (s *FFmpegSource) IgnoreVideo() {
	s.hasVideo = false
	s.vInfo = goffmpeg.VideoInfo{}
	s.videoCallback = nil
}

func (s *FFmpegSource) Decode(_ time.Duration) {
	// An interrupted source is nowhere in particular until a Seek or Rewind
	// reopens it; see Interrupt.
	if s.ended || s.interrupted.Load() {
		return
	}
	if !s.acquire() {
		return
	}

	for {
		mt, err := s.dec.Next()
		if err != nil {
			if s.interrupted.Load() {
				return // asked to stop, not a failure
			}
			if s.live && s.tolerateStall(err) {
				return
			}
			s.err = s.readErr(err)
			s.ended = true
			return
		}
		// A successful read clears any stall in progress, so a device that
		// hiccups and recovers does not carry an error forward.
		s.stalledSince = time.Time{}
		s.err = nil

		switch mt {
		case goffmpeg.MediaTypeVideo:
			if !s.hasVideo {
				continue // IgnoreVideo
			}
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
	if !s.reopenIfInterrupted() {
		return
	}
	if s.dec == nil {
		return
	}
	if err := s.dec.Seek(pos.Seconds()); err != nil {
		if s.interrupted.Load() {
			return // asked to stop, not a failure; the next Seek reopens
		}
		// Playing on from where it was let the caller believe the seek had
		// landed (#869). The source stops instead, and the pipeline reports
		// why through OnError.
		s.err = fmt.Errorf("mmedia: seek to %v failed: %w", pos, err)
		s.ended = true
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
		if s.interrupted.Load() {
			return
		}
		if err != nil || mt == goffmpeg.MediaTypeNone {
			// Recorded as Decode does, so a seek into a dropped stream is
			// reported as the error it is, not as an end (#839).
			s.err = s.readErr(err)
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

// Interrupt implements Interruptible: it makes the decoder abandon the I/O it
// is blocked in, a far seek on a network stream above all, so the pipeline
// can stop without waiting for it (#884). Measured on an https MP3 with no
// seek table, FFmpeg returns about 2 ms after the interrupt from a seek that
// otherwise takes seconds.
//
// A source opened from a reader or a device cannot be reopened, so for one
// this does nothing.
func (s *FFmpegSource) Interrupt() {
	if s.open == nil || s.live {
		return
	}
	s.decMu.Lock()
	defer s.decMu.Unlock()
	s.interrupts++
	s.interrupted.Store(true)
	if s.dec != nil {
		s.dec.Interrupt()
	}
}

// reopenIfInterrupted replaces an interrupted decoder with a fresh one, at the
// start of the media; the caller seeks from there. It reports whether there
// is a decoder to use. A failure to reopen ends the source with the reason.
func (s *FFmpegSource) reopenIfInterrupted() bool {
	if !s.interrupted.Load() {
		return true
	}
	s.decMu.Lock()
	gen := s.interrupts
	s.decMu.Unlock()

	dec, err := s.open()
	if err == nil {
		if err = applyAudioFormat(dec, s.wantChannels, s.wantRate); err != nil {
			dec.Close()
		}
	}
	if err != nil {
		s.err = fmt.Errorf("mmedia: reopening after an interrupt: %w", err)
		s.ended = true
		return false
	}

	s.decMu.Lock()
	old := s.dec
	s.dec = dec
	again := s.interrupts != gen
	if again {
		dec.Interrupt() // interrupted again while this one was opening
	} else {
		s.interrupted.Store(false)
	}
	s.decMu.Unlock()
	if old != nil {
		old.Close()
	}
	if s.hasVideo {
		s.vInfo = dec.VideoInfo()
	}
	s.aInfo = dec.AudioInfo()
	s.ended, s.err = false, nil
	return !again
}

// readErr is err, or for a source read through Go, the reason the read
// failed, which the decoder can only report as an opaque code.
func (s *FFmpegSource) readErr(err error) error {
	if d, ok := s.dec.(*readerDecoder); ok && err != nil {
		return d.readErr(err)
	}
	return err
}

func (s *FFmpegSource) Close() {
	s.decMu.Lock()
	dec := s.dec
	s.dec = nil
	s.decMu.Unlock()
	if dec != nil {
		dec.Close()
	}
	// Close is final, unlike release: nothing should reacquire the device after
	// the caller has finished with the source.
	s.reopen = nil
}
