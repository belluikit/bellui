package mmedia

import (
	"errors"
	"io"
	"os"
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
	aspect   *aspectLayout
	controls *TransportControls
	opts     *options
	loop     bool

	// preview is the scrub thumbnail, and its second decoder. nil unless
	// WithScrubPreview was given and this source can have one.
	preview *scrubPreview

	// capture is set only for a player opened on a capture device, and is what
	// SetCaptureDevice needs to reopen on another one. Guarded by mu.
	capture *captureBinding

	hasVideo bool
	minSize  geo.Size
}

// NewMediaPlayer creates a new media player widget from an io.Reader.
// The container and codecs are probed automatically. A reader that also
// implements io.Seeker supports seeking; a plain stream is forward-only.
// A scrub preview (WithScrubPreview) needs a source FFmpeg can open twice, so it
// works here only when r is an *os.File whose name is still reachable. Any other
// reader is an error rather than a silent absence of preview.
func NewMediaPlayer(r io.Reader, opts ...Option) (*MediaPlayer, error) {
	o := buildOptions(opts)
	pipe, err := pipeline.NewPipeline(r, pipeline.Config{Downmix: o.downmix})
	if err != nil {
		return nil, err
	}
	return newMediaPlayerFromPipeline(pipe, o, reopenableURL(r))
}

// reopenableURL is the path a preview decoder could open a second time, or "" if
// this reader has none.
//
// Only an *os.File has one, and only while its name still resolves — an
// unlinked file or an inherited descriptor does not, which is the same case
// pipeline.NewPipeline falls back to the reader path for. Checking with Stat
// here means WithScrubPreview reports the problem at construction rather than
// leaving a preview that silently never draws.
func reopenableURL(r io.Reader) string {
	f, ok := r.(*os.File)
	if !ok {
		return ""
	}
	name := f.Name()
	if name == "" {
		return ""
	}
	if _, err := os.Stat(name); err != nil {
		return ""
	}
	return name
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
	return newMediaPlayerFromPipeline(pipe, o, url)
}

// newMediaPlayerFromPipeline wires a widget around an already-built pipeline,
// shared by the reader, URL and capture constructors.
//
// previewURL is what a scrub preview would open a second time, or "" when this
// source has nothing to reopen.
func newMediaPlayerFromPipeline(pipe *pipeline.Pipeline, o *options, previewURL string) (*MediaPlayer, error) {
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
	// Held as a field rather than built inside CreateRenderer: a capture device
	// switch can change the video geometry, and the letterboxing has to follow
	// it without rebuilding the widget tree.
	mp.aspect = &aspectLayout{width: vw, height: vh}

	// Create transport controls.
	mp.controls = NewTransportControls()
	mp.controls.OnPlay = func() { mp.Play() }
	mp.controls.OnPause = func() { mp.Pause() }
	mp.controls.OnStop = func() { mp.Stop() }
	mp.controls.OnSeek = func(pos time.Duration) { mp.Seek(pos) }
	mp.controls.OnVolume = func(v float64) { mp.SetVolume(v) }
	// Tell the transport the duration up front. It otherwise learns it from the
	// first playback time tick, which leaves the seek bar enabled but inert
	// before anything has played: both OnChangeEnded and the scrub preview
	// convert a fraction through tc.duration and do nothing while it is zero.
	// Scrubbing a file that has not been started yet is exactly what a preview
	// is for, so it cannot wait for the first tick.
	//
	// Before SetSeekable, so a live source ends on that call's "live" marker
	// rather than on the elapsed-time form SetTime writes for a zero duration.
	mp.controls.SetTime(0, pipe.Duration())
	// A source with no duration is live: nothing to seek within.
	mp.controls.SetSeekable(pipe.Duration() > 0)

	if err := mp.enableScrubPreview(previewURL); err != nil {
		pipe.Close()
		return nil, err
	}

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

// ErrNoScrubPreviewSource reports WithScrubPreview asked of a player whose media
// cannot be opened a second time.
//
// An error rather than a quiet absence of preview: a caller who asked for one
// should find out at the point of asking, not by noticing that hovering does
// nothing. See WithScrubPreview for which sources can have one.
var ErrNoScrubPreviewSource = errors.New(
	"mmedia: scrub preview needs a path or url the media can be reopened from; " +
		"this player was built on a reader that cannot be",
)

// enableScrubPreview installs the preview if one was asked for and this source
// can have one.
func (mp *MediaPlayer) enableScrubPreview(url string) error {
	if !mp.opts.scrubPreview || !mp.hasVideo {
		return nil
	}
	// A live capture has no timeline to preview along and its seek bar is
	// disabled, so there is nothing to do and nothing to complain about — a
	// player written to show either a file or a camera can pass the option
	// unconditionally.
	if mp.pipe.IsLive() || mp.pipe.Duration() <= 0 {
		return nil
	}
	if url == "" {
		return ErrNoScrubPreviewSource
	}

	mp.preview = newScrubPreview(url)
	mp.preview.onResize = func(size geo.Size) {
		mp.controls.SetScrubPreview(mp.preview.object(), size)
	}
	mp.controls.SetScrubPreview(mp.preview.object(), mp.preview.size())
	mp.controls.OnScrub = func(at time.Duration) {
		mp.preview.setTime(at)
		mp.preview.request(at)
	}
	return nil
}

// ErrNotACapturePlayer reports SetCaptureDevice called on a player that is
// showing a file, a reader or a url. There is no device to switch away from,
// and silently doing nothing would leave a picker looking as though it worked.
var ErrNotACapturePlayer = errors.New("mmedia: this player was not opened on a capture device")

// SetCaptureDevice switches a capture player to another camera without
// rebuilding it, so the widget keeps its place, its size and its focus.
//
// device follows NewCapturePlayer's rules: a device name, a positional index as
// a string on macOS, or "" for the platform default. Switching to the device
// already showing is a no-op and reports no error.
//
// A failure leaves the player on the device it has, still running. The new
// device is opened before the old one is released, which is what makes that
// possible — and is also why switching to a camera some other application holds
// fails cleanly rather than losing the one being shown. The cost is that the two
// devices are briefly open together; every platform here allows that for two
// *different* devices, and the same-device case is the no-op above.
//
// This blocks for as long as the device takes to open and the pipeline takes to
// come to rest — comfortably long enough to be visible if it runs on the main
// goroutine, so call it from a goroutine. It is safe to do so: the widget
// updates it makes are marshalled back. CaptureSelect already does this.
func (mp *MediaPlayer) SetCaptureDevice(device string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.capture == nil {
		return ErrNotACapturePlayer
	}
	url, err := resolveCaptureDevice(mp.capture.format, device)
	if err != nil {
		return err
	}
	if url == mp.capture.url {
		return nil
	}

	src, err := openCaptureSource(mp.capture.format, url, mp.capture.options)
	if err != nil {
		return err
	}
	if err := mp.pipe.SwapSource(src); err != nil {
		// The pipeline refused it, so nothing else will ever close it. Releasing
		// here is the difference between a rejected switch and a leaked camera.
		src.Close()
		return err
	}

	mp.capture.url = url
	mp.applyVideoGeometry()
	return nil
}

// CaptureDevice reports the selector of the device this player is showing, or
// "" if it was not opened on one.
func (mp *MediaPlayer) CaptureDevice() string {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if mp.capture == nil {
		return ""
	}
	return mp.capture.url
}

// applyVideoGeometry resizes the video surface and the letterboxing to whatever
// the pipeline is now producing.
//
// A new camera is under no obligation to match the old one's resolution, and
// two things downstream are sized from it: the surface's pixel buffers, which
// would otherwise truncate or leave a stale tail of the previous frame, and the
// aspect layout, which would otherwise letterbox to the wrong ratio. Both are
// mutated in place — rebuilding them would mean rebuilding the widget tree,
// which is the flicker this whole path exists to avoid.
//
// Called with mp.mu held, from whichever goroutine drove the switch; the widget
// work inside hops to the main goroutine on its own.
func (mp *MediaPlayer) applyVideoGeometry() {
	if !mp.hasVideo {
		return
	}
	w, h := mp.pipe.VideoSize()
	if w <= 0 || h <= 0 {
		return
	}
	mp.minSize = geo.NewSize(float32(w), float32(h)+48)
	mp.surface.resize(w, h)

	bellui.Do(func() {
		mp.aspect.width, mp.aspect.height = w, h
		mp.Refresh()
	})
}

// PlaybackStats holds real-time performance metrics.
type PlaybackStats = pipeline.PlaybackStats

// Stats returns current playback performance metrics.
// Useful for monitoring frame drops, decode latency, and A/V sync.
func (mp *MediaPlayer) Stats() PlaybackStats {
	return mp.pipe.Stats()
}

// Close releases all resources, including the scrub preview's second decoder.
func (mp *MediaPlayer) Close() {
	if mp.preview != nil {
		mp.preview.close()
	}
	mp.pipe.Close()
}

// CreateRenderer returns the Bellui widget renderer.
func (mp *MediaPlayer) CreateRenderer() core.WidgetRenderer {
	var content *bellui.Container
	if mp.hasVideo {
		// Raster always stretches, so the aspect ratio is preserved by the
		// layout instead of by a fill mode.
		video := container.New(mp.aspect, mp.surface.Object())
		// Controls last, so they paint over the video. BorderLayout does not care
		// about the order, but the painter does — and the scrub preview is drawn
		// above the controls' own bounds, over the video, so it would be hidden
		// underneath if the video came second.
		content = container.New(
			layout.NewBorderLayout(nil, mp.controls, nil, nil),
			video,
			mp.controls,
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
