package mmedia

import (
	"io"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/pipeline"
)

// AudioPlayback plays audio with no user interface. It needs no widget, window
// or app, so playback can outlive any screen that shows it: an app keeps the
// AudioPlayback and attaches an [AudioPlayer] to it with [NewAudioPlayerFor]
// whenever a screen wants transport controls (#881).
//
// Like AudioPlayer, it plays only a file's audio. The options are the players'
// own; WithOnEnd, WithOnError and WithOnTimeUpdate run on the pipeline's
// goroutines, so a callback that waits for Stop or Close deadlocks.
type AudioPlayback struct {
	pipe *pipeline.Pipeline
	opts *options
	loop atomic.Bool

	transport transport // orders Play/Pause/Stop/Close against a loop restart (#527)

	mu    sync.Mutex
	views []*AudioPlayer // attached, told of every change
}

// NewAudioPlayback opens audio from an io.Reader for playback with no widget.
//
// It fails when the audio device will not open or the stream cannot be
// converted to its format; see the package documentation.
func NewAudioPlayback(r io.Reader, opts ...Option) (*AudioPlayback, error) {
	pipe, err := pipeline.NewAudioPipeline(r)
	if err != nil {
		return nil, err
	}
	return newAudioPlaybackFromPipeline(pipe, buildOptions(opts)), nil
}

// NewAudioPlaybackURL opens a file path or network URL for playback with no
// widget. What it can open is what [NewAudioPlayerURL] can.
func NewAudioPlaybackURL(url string, opts ...Option) (*AudioPlayback, error) {
	pipe, err := pipeline.NewAudioPipelineURL(url)
	if err != nil {
		return nil, err
	}
	return newAudioPlaybackFromPipeline(pipe, buildOptions(opts)), nil
}

func newAudioPlaybackFromPipeline(pipe *pipeline.Pipeline, o *options) *AudioPlayback {
	pb := &AudioPlayback{pipe: pipe, opts: o}
	pb.loop.Store(o.loop)

	pipe.SetOnTime(func(current, total time.Duration) {
		for _, v := range pb.attached() {
			v.showTime(pb, current, total)
		}
		if o.onTime != nil {
			o.onTime(current, total)
		}
	})

	pipe.SetOnEnd(func() {
		pb.showPlaying(false)
		if pb.loop.Load() {
			// Not on this goroutine: OnEnd runs on the pipeline's decode
			// goroutine, and Stop waits for it to finish (#502, #527).
			go func() {
				if pb.transport.restart(pb.pipe) {
					pb.showPlaying(true)
				}
			}()
			return
		}
		if o.onEnd != nil {
			o.onEnd()
		}
	})

	pipe.SetOnError(func(err error) {
		// Playback stopped, on a lost connection or a read error (#839).
		pb.showPlaying(false)
		if o.onError != nil {
			o.onError(err)
		}
	})

	pipe.SetVolume(o.volume)

	if o.autoPlay {
		pb.Play()
	}
	return pb
}

// Play starts playback, or resumes it from a pause.
func (pb *AudioPlayback) Play() {
	pb.transport.play(pb.pipe)
	pb.showPlaying(true)
}

// Pause pauses playback.
func (pb *AudioPlayback) Pause() {
	pb.transport.pause(pb.pipe)
	pb.showPlaying(false)
}

// Stop stops playback and resets to the beginning.
func (pb *AudioPlayback) Stop() {
	pb.transport.stop(pb.pipe)
	pb.showStopped()
}

// Seek seeks to the given position.
func (pb *AudioPlayback) Seek(pos time.Duration) {
	pb.pipe.Seek(pos)
}

// SetVolume sets audio volume (0.0 to 1.0).
func (pb *AudioPlayback) SetVolume(v float64) {
	pb.pipe.SetVolume(v)
	for _, view := range pb.attached() {
		view.showVolume(pb, v)
	}
}

// Volume returns the current audio volume.
func (pb *AudioPlayback) Volume() float64 {
	return pb.pipe.Volume()
}

// Duration returns the total media duration, or 0 for a live stream.
func (pb *AudioPlayback) Duration() time.Duration {
	return pb.pipe.Duration()
}

// CurrentTime returns the current playback position.
func (pb *AudioPlayback) CurrentTime() time.Duration {
	return pb.pipe.CurrentTime()
}

// Level is how loud the playback is right now: the RMS, 0 to 1, of the 20 ms
// of audio being heard, with the volume applied. It is 0 while paused or
// stopped. It is a pull API, cheap enough to call once a frame, for a
// visualizer or an animation that follows the music (belluikit/player#258).
func (pb *AudioPlayback) Level() float32 {
	return pb.pipe.Level()
}

// IsPlaying returns true if playback is active.
func (pb *AudioPlayback) IsPlaying() bool {
	return pb.pipe.State() == pipeline.StatePlaying
}

// SetLoop enables or disables looping.
func (pb *AudioPlayback) SetLoop(loop bool) {
	pb.loop.Store(loop)
}

// Close stops playback and releases the decoder. Attached views stay attached
// and show it stopped.
func (pb *AudioPlayback) Close() {
	pb.transport.close(pb.pipe)
	pb.showStopped()
}

func (pb *AudioPlayback) attach(v *AudioPlayer) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.views = append(pb.views, v)
}

func (pb *AudioPlayback) detach(v *AudioPlayer) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.views = slices.DeleteFunc(pb.views, func(w *AudioPlayer) bool { return w == v })
}

// attached returns the views to tell of a change. It is a copy, so a view can
// attach or detach while one is being told.
func (pb *AudioPlayback) attached() []*AudioPlayer {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	return slices.Clone(pb.views)
}

func (pb *AudioPlayback) showPlaying(playing bool) {
	for _, v := range pb.attached() {
		v.showPlaying(pb, playing)
	}
}

func (pb *AudioPlayback) showStopped() {
	for _, v := range pb.attached() {
		v.showStopped(pb)
	}
}
