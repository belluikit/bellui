package mmedia

import (
	"io"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// MediaPlayer is a Bellui widget that combines video display with audio playback
// and transport controls. It is the primary widget for media playback.
type MediaPlayer struct {
	widget.BaseWidget

	mu       sync.Mutex
	pipe     *pipeline.Pipeline
	surface  *videoSurface
	controls *TransportControls
	opts     *options
	loop     bool

	hasVideo bool
	minSize  geo.Size
}

// NewMediaPlayer creates a new media player widget from an io.Reader.
// The container and codecs are probed automatically. A reader that also
// implements io.Seeker supports seeking; a plain stream is forward-only.
func NewMediaPlayer(r io.Reader, opts ...Option) (*MediaPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipeline(r, pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, err
	}
	return newMediaPlayerFromPipeline(pipe, o)
}

// NewMediaPlayerURL plays from a file path or network URL, letting FFmpeg own
// the I/O rather than reading through Go. Unlike the io.Reader form this
// supports seeking within a network stream (via range requests) and formats
// that fetch their own data, such as HLS — a playlist read as a byte stream
// is only text.
//
// http and https are compiled in, as are the hls and rtsp demuxers and the
// rtmp protocol.
func NewMediaPlayerURL(url string, opts ...Option) (*MediaPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipelineURL(url, pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, err
	}
	return newMediaPlayerFromPipeline(pipe, o)
}

// newMediaPlayerFromPipeline wires a widget around an already-built pipeline,
// shared by the reader and URL constructors.
func newMediaPlayerFromPipeline(pipe *pipeline.Pipeline, o *options) (*MediaPlayer, error) {
	mp := &MediaPlayer{
		pipe:     pipe,
		opts:     o,
		loop:     o.loop,
		hasVideo: pipe.HasVideo(),
		minSize:  geo.NewSize(400, 300),
	}

	if mp.hasVideo {
		w, h := pipe.VideoSize()
		if w > 0 && h > 0 {
			// Add space for controls below video.
			mp.minSize = geo.NewSize(float32(w), float32(h)+48)
		}
	}

	// Video renders through a streaming raster, which updates its GL texture
	// in place rather than rebuilding it every frame.
	vw, vh := pipe.VideoSize()
	mp.surface = newVideoSurface(vw, vh)

	// Create transport controls.
	mp.controls = NewTransportControls()
	mp.controls.OnPlay = func() { mp.Play() }
	mp.controls.OnPause = func() { mp.Pause() }
	mp.controls.OnStop = func() { mp.Stop() }
	mp.controls.OnSeek = func(pos time.Duration) { mp.Seek(pos) }
	mp.controls.OnVolume = func(v float64) { mp.SetVolume(v) }
	// A source with no duration is live: nothing to seek within.
	mp.controls.SetSeekable(pipe.Duration() > 0)

	// Wire up pipeline callbacks.
	if mp.hasVideo {
		// WriteFrame copies synchronously — the pipeline recycles the frame as
		// soon as this returns — and schedules the repaint itself.
		pipe.SetOnFrame(mp.surface.WriteFrame)
	}

	pipe.SetOnTime(func(current, total time.Duration) {
		bellui.Do(func() {
			mp.controls.SetTime(current, total)
		})
		if o.onTime != nil {
			o.onTime(current, total)
		}
	})

	pipe.SetOnEnd(func() {
		bellui.Do(func() {
			mp.controls.SetPlaying(false)
		})
		if mp.loop {
			mp.pipe.Stop()
			mp.pipe.Play()
			bellui.Do(func() {
				mp.controls.SetPlaying(true)
			})
			return
		}
		if o.onEnd != nil {
			o.onEnd()
		}
	})

	pipe.SetOnError(func(err error) {
		// A live capture reports a lost device here rather than through OnEnd, so
		// this is the only place the transport learns that playback stopped.
		if pipe.IsLive() {
			bellui.Do(func() {
				mp.controls.SetPlaying(false)
			})
		}
		if o.onError != nil {
			o.onError(err)
		}
	})

	pipe.SetVolume(o.volume)

	mp.ExtendBaseWidget(mp)

	if o.autoPlay {
		mp.Play()
	}

	return mp, nil
}

// Play starts media playback.
func (mp *MediaPlayer) Play() {
	mp.pipe.Play()
	mp.controls.SetPlaying(true)
}

// Pause pauses media playback.
func (mp *MediaPlayer) Pause() {
	mp.pipe.Pause()
	mp.controls.SetPlaying(false)
}

// Stop stops playback and resets to the beginning.
func (mp *MediaPlayer) Stop() {
	mp.pipe.Stop()
	mp.controls.SetPlaying(false)
	// Stopping also rewinds the source, so the transport must follow it back
	// to zero — the pipeline's time updates have stopped by now.
	mp.controls.Reset()
}

// Seek seeks to the given position.
func (mp *MediaPlayer) Seek(pos time.Duration) {
	mp.pipe.Seek(pos)
}

// SetVolume sets audio volume (0.0 to 1.0).
func (mp *MediaPlayer) SetVolume(v float64) {
	mp.pipe.SetVolume(v)
	mp.controls.SetVolume(v)
}

// Volume returns the current audio volume.
func (mp *MediaPlayer) Volume() float64 {
	return mp.pipe.Volume()
}

// VideoSize returns the video dimensions (0,0 if no video).
func (mp *MediaPlayer) VideoSize() (int, int) {
	return mp.pipe.VideoSize()
}

// Duration returns the total media duration.
func (mp *MediaPlayer) Duration() time.Duration {
	return mp.pipe.Duration()
}

// CurrentTime returns the current playback position.
func (mp *MediaPlayer) CurrentTime() time.Duration {
	return mp.pipe.CurrentTime()
}

// IsPlaying returns true if playback is active.
func (mp *MediaPlayer) IsPlaying() bool {
	return mp.pipe.State() == pipeline.StatePlaying
}

// SetLoop enables or disables looping.
func (mp *MediaPlayer) SetLoop(loop bool) {
	mp.loop = loop
}

// PlaybackStats holds real-time performance metrics.
type PlaybackStats = pipeline.PlaybackStats

// Stats returns current playback performance metrics.
// Useful for monitoring frame drops, decode latency, and A/V sync.
func (mp *MediaPlayer) Stats() PlaybackStats {
	return mp.pipe.Stats()
}

// Close releases all resources.
func (mp *MediaPlayer) Close() {
	mp.pipe.Close()
}

// CreateRenderer returns the Bellui widget renderer.
func (mp *MediaPlayer) CreateRenderer() core.WidgetRenderer {
	var content *bellui.Container
	if mp.hasVideo {
		vw, vh := mp.pipe.VideoSize()
		// Raster always stretches, so the aspect ratio is preserved by the
		// layout instead of by a fill mode.
		video := container.New(&aspectLayout{width: vw, height: vh}, mp.surface.Object())
		content = container.New(
			layout.NewBorderLayout(nil, mp.controls, nil, nil),
			mp.controls,
			video,
		)
	} else {
		label := widget.NewLabel("Audio Only")
		label.Alignment = bellui.TextAlignCenter
		content = container.New(
			layout.NewBorderLayout(nil, mp.controls, nil, nil),
			mp.controls,
			label,
		)
	}

	return widget.NewSimpleRenderer(content)
}
