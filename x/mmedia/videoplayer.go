package mmedia

import (
	"io"
	"sync"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// VideoPlayer is a Bellui widget that displays video frames.
// Decoding is handled by FFmpeg, so any container and codec the bundled
// libraries support will play.
type VideoPlayer struct {
	widget.BaseWidget

	mu      sync.Mutex
	pipe    *pipeline.Pipeline
	surface *videoSurface
	minSize geo.Size
	opts    *options
	loop    bool
}

// NewVideoPlayer creates a new video player widget from an io.Reader.
// The container and codecs are probed automatically. A reader that also
// implements io.Seeker supports seeking; a plain stream is forward-only.
func NewVideoPlayer(r io.Reader, opts ...Option) (*VideoPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipeline(r, pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, err
	}
	return newVideoPlayerFromPipeline(pipe, o)
}

// NewVideoPlayerURL plays from a file path or network URL, letting FFmpeg own
// the I/O rather than reading through Go. Unlike the io.Reader form this
// supports seeking within a network stream (via range requests) and formats
// that fetch their own data, such as HLS — a playlist read as a byte stream
// is only text.
//
// http and https are compiled in, as are the hls and rtsp demuxers and the
// rtmp protocol.
func NewVideoPlayerURL(url string, opts ...Option) (*VideoPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipelineURL(url, pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, err
	}
	return newVideoPlayerFromPipeline(pipe, o)
}

// newVideoPlayerFromPipeline wires a widget around an already-built pipeline,
// shared by the reader and URL constructors.
func newVideoPlayerFromPipeline(pipe *pipeline.Pipeline, o *options) (*VideoPlayer, error) {
	vp := &VideoPlayer{
		pipe:    pipe,
		opts:    o,
		loop:    o.loop,
		minSize: geo.NewSize(320, 240),
	}

	// Set initial video size as min size.
	if pipe.HasVideo() {
		w, h := pipe.VideoSize()
		if w > 0 && h > 0 {
			vp.minSize = geo.NewSize(float32(w), float32(h))
		}
	}

	// Video renders through a streaming raster; WriteFrame copies the pixels
	// synchronously, because the pipeline recycles each frame as soon as the
	// callback returns.
	vw, vh := pipe.VideoSize()
	vp.surface = newVideoSurface(vw, vh)

	// Wire up pipeline callbacks.
	pipe.SetOnFrame(vp.surface.WriteFrame)

	pipe.SetOnTime(func(current, total time.Duration) {
		if o.onTime != nil {
			o.onTime(current, total)
		}
	})

	pipe.SetOnEnd(func() {
		if vp.loop {
			vp.pipe.Stop()
			vp.pipe.Play()
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

	vp.ExtendBaseWidget(vp)

	if o.autoPlay {
		vp.Play()
	}

	return vp, nil
}

// Play starts video playback.
func (vp *VideoPlayer) Play() {
	vp.pipe.Play()
}

// Pause pauses video playback.
func (vp *VideoPlayer) Pause() {
	vp.pipe.Pause()
}

// Stop stops playback and resets to the beginning.
func (vp *VideoPlayer) Stop() {
	vp.pipe.Stop()
}

// Seek seeks to the given position.
func (vp *VideoPlayer) Seek(pos time.Duration) {
	vp.pipe.Seek(pos)
}

// SetVolume sets audio volume (0.0 to 1.0).
func (vp *VideoPlayer) SetVolume(v float64) {
	vp.pipe.SetVolume(v)
}

// Volume returns the current audio volume.
func (vp *VideoPlayer) Volume() float64 {
	return vp.pipe.Volume()
}

// Duration returns the total media duration.
func (vp *VideoPlayer) Duration() time.Duration {
	return vp.pipe.Duration()
}

// CurrentTime returns the current playback position.
func (vp *VideoPlayer) CurrentTime() time.Duration {
	return vp.pipe.CurrentTime()
}

// IsPlaying returns true if playback is active.
func (vp *VideoPlayer) IsPlaying() bool {
	return vp.pipe.State() == pipeline.StatePlaying
}

// SetLoop enables or disables looping.
func (vp *VideoPlayer) SetLoop(loop bool) {
	vp.loop = loop
}

// Close releases all resources.
func (vp *VideoPlayer) Close() {
	vp.pipe.Close()
}

// CreateRenderer returns the Bellui widget renderer.
func (vp *VideoPlayer) CreateRenderer() core.WidgetRenderer {
	return &videoRenderer{player: vp}
}

type videoRenderer struct {
	player *VideoPlayer
}

func (r *videoRenderer) Layout(size geo.Size) {
	w, h := r.player.pipe.VideoSize()
	(&aspectLayout{width: w, height: h}).Layout(r.Objects(), size)
}

func (r *videoRenderer) MinSize() geo.Size {
	return r.player.minSize
}

func (r *videoRenderer) Refresh() {
	r.player.surface.Refresh()
}

func (r *videoRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.player.surface.Object()}
}

func (r *videoRenderer) Destroy() {
	r.player.Close()
}
