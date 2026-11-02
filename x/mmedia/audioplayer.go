package mmedia

import (
	"io"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// AudioPlayer is a Bellui widget that provides audio playback with transport controls.
// Decoding is handled by FFmpeg, so any audio format the bundled libraries
// support will play. It plays only a file's audio: a video stream, such as a
// song's embedded cover art, is ignored.
//
// The widget is a view of an [AudioPlayback]. [NewAudioPlayer] and
// [NewAudioPlayerURL] open one the widget owns, which its Close closes. For
// playback that outlives a screen, the app opens the AudioPlayback itself and
// shows it with [NewAudioPlayerFor]; that widget's Close only detaches it.
type AudioPlayer struct {
	widget.BaseWidget

	controls *TransportControls

	mu   sync.Mutex
	pb   *AudioPlayback // nil once detached
	owns bool           // pb was opened for this widget, and closes with it
}

// NewAudioPlayer creates a new audio player widget from an io.Reader.
//
// A stream with audio fails here when the audio device will not open or the
// stream cannot be converted to its format; see the package documentation.
func NewAudioPlayer(r io.Reader, opts ...Option) (*AudioPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewAudioPipeline(r)
	if err != nil {
		return nil, err
	}
	return newAudioPlayerFromPipeline(pipe, o)
}

// NewAudioPlayerURL plays from a file path or network URL, letting FFmpeg own
// the I/O rather than reading through Go. Unlike the io.Reader form this
// supports seeking within a network stream (via range requests) and formats
// that fetch their own data, such as HLS — a playlist read as a byte stream
// is only text.
//
// http and https are compiled in, as are the hls and rtsp demuxers and the
// rtmp protocol. On Android, where FFmpeg has no TLS, an https URL is read through
// Go instead; it streams and seeks as on other platforms, but an https HLS
// playlist fails to open.
//
// A stream with audio fails here when the audio device will not open or the
// stream cannot be converted to its format; see the package documentation.
func NewAudioPlayerURL(url string, opts ...Option) (*AudioPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewAudioPipelineURL(url)
	if err != nil {
		return nil, err
	}
	return newAudioPlayerFromPipeline(pipe, o)
}

// newAudioPlayerFromPipeline opens a playback around an already-built pipeline
// and a widget that owns it, shared by the reader and URL constructors.
func newAudioPlayerFromPipeline(pipe *pipeline.Pipeline, o *options) (*AudioPlayer, error) {
	ap := NewAudioPlayerFor(newAudioPlaybackFromPipeline(pipe, o))
	ap.owns = true
	return ap, nil
}

// NewAudioPlayerFor returns transport controls for pb, which keeps playing as
// it was: attaching neither starts, stops nor rewinds it. The widget follows
// pb until [AudioPlayer.Detach] or Close, and neither closes pb, so a screen
// can come and go while the song plays on. A nil pb gives a detached widget.
func NewAudioPlayerFor(pb *AudioPlayback) *AudioPlayer {
	ap := &AudioPlayer{pb: pb}

	ap.controls = NewTransportControls()
	ap.controls.OnPlay = func() { ap.Play() }
	ap.controls.OnPause = func() { ap.Pause() }
	ap.controls.OnStop = func() { ap.Stop() }
	ap.controls.OnSeek = func(pos time.Duration) { ap.Seek(pos) }
	ap.controls.OnVolume = func(v float64) { ap.SetVolume(v) }

	if pb != nil {
		// A source with no duration is live: nothing to seek within.
		ap.controls.SetSeekable(pb.Duration() > 0)
		ap.controls.SetVolume(pb.Volume())
		ap.controls.SetTime(pb.CurrentTime(), pb.Duration())
		ap.controls.SetPlaying(pb.IsPlaying())
		pb.attach(ap)
	} else {
		ap.controls.SetSeekable(false)
	}

	ap.ExtendBaseWidget(ap)
	return ap
}

// Playback returns the AudioPlayback this widget shows, or nil once detached.
func (ap *AudioPlayer) Playback() *AudioPlayback {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.pb
}

// Detach stops the widget following its playback, which carries on as it was.
// The controls are left showing it stopped, and the widget's playback methods
// do nothing from then on. Detaching a widget that owns its playback hands
// that playback to whoever holds it from [AudioPlayer.Playback]: the widget's
// Close no longer closes it.
func (ap *AudioPlayer) Detach() {
	ap.mu.Lock()
	pb := ap.pb
	ap.pb, ap.owns = nil, false
	ap.mu.Unlock()
	if pb == nil {
		return
	}
	pb.detach(ap)
	bellui.Do(func() {
		ap.controls.SetPlaying(false)
		ap.controls.SetSeekable(false)
	})
}

// Play starts audio playback.
func (ap *AudioPlayer) Play() {
	if pb := ap.Playback(); pb != nil {
		pb.Play()
	}
}

// Pause pauses audio playback.
func (ap *AudioPlayer) Pause() {
	if pb := ap.Playback(); pb != nil {
		pb.Pause()
	}
}

// Stop stops playback and resets to the beginning.
func (ap *AudioPlayer) Stop() {
	if pb := ap.Playback(); pb != nil {
		pb.Stop()
	}
}

// Seek seeks to the given position.
func (ap *AudioPlayer) Seek(pos time.Duration) {
	if pb := ap.Playback(); pb != nil {
		pb.Seek(pos)
	}
}

// SetVolume sets audio volume (0.0 to 1.0).
func (ap *AudioPlayer) SetVolume(v float64) {
	if pb := ap.Playback(); pb != nil {
		pb.SetVolume(v)
	}
}

// Volume returns the current audio volume, or 0 once detached.
func (ap *AudioPlayer) Volume() float64 {
	if pb := ap.Playback(); pb != nil {
		return pb.Volume()
	}
	return 0
}

// Duration returns the total media duration, or 0 once detached.
func (ap *AudioPlayer) Duration() time.Duration {
	if pb := ap.Playback(); pb != nil {
		return pb.Duration()
	}
	return 0
}

// CurrentTime returns the current playback position, or 0 once detached.
func (ap *AudioPlayer) CurrentTime() time.Duration {
	if pb := ap.Playback(); pb != nil {
		return pb.CurrentTime()
	}
	return 0
}

// IsPlaying returns true if playback is active.
func (ap *AudioPlayer) IsPlaying() bool {
	pb := ap.Playback()
	return pb != nil && pb.IsPlaying()
}

// SetLoop enables or disables looping.
func (ap *AudioPlayer) SetLoop(loop bool) {
	if pb := ap.Playback(); pb != nil {
		pb.SetLoop(loop)
	}
}

// Close releases the widget's playback if the widget opened it, and otherwise
// only detaches from it, leaving it playing.
func (ap *AudioPlayer) Close() {
	ap.mu.Lock()
	pb, owns := ap.pb, ap.owns
	ap.mu.Unlock()
	if pb != nil && owns {
		pb.Close()
	}
	ap.Detach()
}

// following reports whether the widget still shows pb. A change pb reports
// can arrive after a Detach, and must not reach the controls then.
func (ap *AudioPlayer) following(pb *AudioPlayback) bool {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.pb == pb
}

func (ap *AudioPlayer) showTime(pb *AudioPlayback, current, total time.Duration) {
	bellui.Do(func() {
		if ap.following(pb) {
			ap.controls.SetTime(current, total)
		}
	})
}

func (ap *AudioPlayer) showPlaying(pb *AudioPlayback, playing bool) {
	bellui.Do(func() {
		if ap.following(pb) {
			ap.controls.SetPlaying(playing)
		}
	})
}

// showStopped follows a Stop, which also rewinds the source: the controls go
// back to zero, since the pipeline's time updates have stopped by now.
func (ap *AudioPlayer) showStopped(pb *AudioPlayback) {
	bellui.Do(func() {
		if ap.following(pb) {
			ap.controls.SetPlaying(false)
			ap.controls.Reset()
		}
	})
}

func (ap *AudioPlayer) showVolume(pb *AudioPlayback, v float64) {
	bellui.Do(func() {
		if ap.following(pb) {
			ap.controls.SetVolume(v)
		}
	})
}

// CreateRenderer returns the Bellui widget renderer.
func (ap *AudioPlayer) CreateRenderer() core.WidgetRenderer {
	titleLabel := widget.NewLabel("Audio Player")
	titleLabel.TextStyle = bellui.TextStyle{Bold: true}

	content := container.NewVBox(
		titleLabel,
		ap.controls,
	)

	return widget.NewSimpleRenderer(content)
}
