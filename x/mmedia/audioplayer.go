package mmedia

import (
	"io"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// AudioPlayer is a Bellui widget that provides audio playback with transport controls.
// Decoding is handled by FFmpeg, so any audio format the bundled libraries
// support will play.
type AudioPlayer struct {
	widget.BaseWidget

	pipe     *pipeline.Pipeline
	controls *TransportControls
	opts     *options
	loop     bool
}

// NewAudioPlayer creates a new audio player widget from an io.Reader.
func NewAudioPlayer(r io.Reader, opts ...Option) (*AudioPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipeline(r, pipeline.Config{Downmix: o.downmix})
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
// rtmp protocol.
func NewAudioPlayerURL(url string, opts ...Option) (*AudioPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipelineURL(url, pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, err
	}
	return newAudioPlayerFromPipeline(pipe, o)
}

// newAudioPlayerFromPipeline wires a widget around an already-built pipeline,
// shared by the reader and URL constructors.
func newAudioPlayerFromPipeline(pipe *pipeline.Pipeline, o *options) (*AudioPlayer, error) {
	ap := &AudioPlayer{
		pipe: pipe,
		opts: o,
		loop: o.loop,
	}

	ap.controls = NewTransportControls()
	ap.controls.OnPlay = func() { ap.Play() }
	ap.controls.OnPause = func() { ap.Pause() }
	ap.controls.OnStop = func() { ap.Stop() }
	ap.controls.OnSeek = func(pos time.Duration) { ap.Seek(pos) }
	ap.controls.OnVolume = func(v float64) { ap.SetVolume(v) }
	// A source with no duration is live: nothing to seek within.
	ap.controls.SetSeekable(pipe.Duration() > 0)

	pipe.SetOnTime(func(current, total time.Duration) {
		bellui.Do(func() {
			ap.controls.SetTime(current, total)
		})
		if o.onTime != nil {
			o.onTime(current, total)
		}
	})

	pipe.SetOnEnd(func() {
		bellui.Do(func() {
			ap.controls.SetPlaying(false)
		})
		if ap.loop {
			ap.pipe.Stop()
			ap.pipe.Play()
			bellui.Do(func() {
				ap.controls.SetPlaying(true)
			})
			return
		}
		if o.onEnd != nil {
			o.onEnd()
		}
	})

	pipe.SetOnError(func(err error) {
		if o.onError != nil {
			o.onError(err)
		}
	})

	pipe.SetVolume(o.volume)

	ap.ExtendBaseWidget(ap)

	if o.autoPlay {
		ap.Play()
	}

	return ap, nil
}

// Play starts audio playback.
func (ap *AudioPlayer) Play() {
	ap.pipe.Play()
	ap.controls.SetPlaying(true)
}

// Pause pauses audio playback.
func (ap *AudioPlayer) Pause() {
	ap.pipe.Pause()
	ap.controls.SetPlaying(false)
}

// Stop stops playback and resets to the beginning.
func (ap *AudioPlayer) Stop() {
	ap.pipe.Stop()
	ap.controls.SetPlaying(false)
	// Stopping also rewinds the source, so the transport must follow it back
	// to zero — the pipeline's time updates have stopped by now.
	ap.controls.Reset()
}

// Seek seeks to the given position.
func (ap *AudioPlayer) Seek(pos time.Duration) {
	ap.pipe.Seek(pos)
}

// SetVolume sets audio volume (0.0 to 1.0).
func (ap *AudioPlayer) SetVolume(v float64) {
	ap.pipe.SetVolume(v)
	ap.controls.SetVolume(v)
}

// Volume returns the current audio volume.
func (ap *AudioPlayer) Volume() float64 {
	return ap.pipe.Volume()
}

// Duration returns the total media duration.
func (ap *AudioPlayer) Duration() time.Duration {
	return ap.pipe.Duration()
}

// CurrentTime returns the current playback position.
func (ap *AudioPlayer) CurrentTime() time.Duration {
	return ap.pipe.CurrentTime()
}

// IsPlaying returns true if playback is active.
func (ap *AudioPlayer) IsPlaying() bool {
	return ap.pipe.State() == pipeline.StatePlaying
}

// SetLoop enables or disables looping.
func (ap *AudioPlayer) SetLoop(loop bool) {
	ap.loop = loop
}

// Close releases all resources.
func (ap *AudioPlayer) Close() {
	ap.pipe.Close()
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
