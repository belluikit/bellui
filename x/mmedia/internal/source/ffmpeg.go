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
func (s *FFmpegSource) Duration() time.Duration {
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
	if err := s.dec.SetAudioChannels(n); err != nil {
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

	for {
		mt, err := s.dec.Next()
		if err != nil {
			s.ended = true
			return
		}

		switch mt {
		case goffmpeg.MediaTypeVideo:
			if s.videoCallback != nil {
				rgba := image.NewRGBA(image.Rect(0, 0, s.vInfo.Width, s.vInfo.Height))
				if err := s.dec.GetRGBA(rgba); err == nil {
					// Position is the decoded frame's own PTS, set by the
					// bridge from best_effort_timestamp.
					pts := time.Duration(s.dec.Position() * float64(time.Second))
					s.videoCallback(rgba, pts)
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

func (s *FFmpegSource) Seek(pos time.Duration) {
	if err := s.dec.Seek(pos.Seconds()); err != nil {
		return
	}
	s.ended = false

	// av_seek_frame lands on the keyframe at or before the target (which
	// can be several seconds earlier depending on the GOP size). Decode and
	// discard until the stream reaches the requested position so the next
	// Decode call delivers frames from the seek target onward. Position()
	// is only meaningful after a decode, so always decode at least once.
	target := pos.Seconds()
	for {
		mt, err := s.dec.Next()
		if err != nil || mt == goffmpeg.MediaTypeNone {
			s.ended = true
			return
		}
		if s.dec.Position() >= target {
			return
		}
	}
}

func (s *FFmpegSource) Rewind() {
	s.Seek(0)
}
func (s *FFmpegSource) HasEnded() bool { return s.ended }

func (s *FFmpegSource) Close() {
	if s.dec != nil {
		s.dec.Close()
	}
}
