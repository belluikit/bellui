package pipeline

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui/x/mmedia/internal/audio"
	"github.com/belluikit/bellui/x/mmedia/internal/source"
)

// OnFrameFunc is called when a new video frame is ready for display.
type OnFrameFunc func(frame *image.RGBA)

// OnTimeFunc is called periodically with the current playback position and total duration.
type OnTimeFunc func(current, total time.Duration)

// OnEndFunc is called when playback reaches the end of the media.
type OnEndFunc func()

// OnErrorFunc is called when an error occurs during playback.
type OnErrorFunc func(err error)

// Pipeline orchestrates media demuxing, decoding, and presentation.
// PlaybackStats holds real-time playback metrics.
type PlaybackStats struct {
	FramesDecoded  int64
	FramesDropped  int64
	FramesRendered int64

	// SyncOffset is the presentation error of the most recently shown frame:
	// how far the presentation clock had passed that frame's timestamp when it
	// was displayed. Positive means video is running behind audio.
	//
	// A healthy stream sits within a frame interval of zero and stays there.
	// A value that grows without bound is the signature of video time being
	// derived from something other than the stream's own timestamps.
	SyncOffset time.Duration
	// AvgSyncOffset is the mean signed offset over the session — the number to
	// watch for drift, since transient spikes average out but bias does not.
	AvgSyncOffset time.Duration
	// MaxSyncOffset is the largest absolute offset seen.
	MaxSyncOffset time.Duration
}

type Pipeline struct {
	mu sync.Mutex

	src         source.Source
	state       *StateMachine
	clock       *Clock
	audioOutput *audio.Output

	hasVideo bool
	hasAudio bool
	// live marks an open-ended capture source: no duration, no seeking, and no
	// end of stream that means playback finished.
	live bool

	onFrame OnFrameFunc
	onTime  OnTimeFunc
	onEnd   OnEndFunc
	onError OnErrorFunc

	stopCh chan struct{}
	wg     sync.WaitGroup // tracks decode goroutine lifecycle

	// decodeMu serializes access to the source between the decode-ahead
	// goroutine and Seek, which both touch the decoder.
	decodeMu sync.Mutex

	videoWidth  int
	videoHeight int
	duration    time.Duration
	framerate   float64

	// Playback counters. Written from both the decode and display goroutines
	// and read by Stats, so they are atomic rather than mutex-guarded — the
	// display loop must not block on a caller polling metrics.
	framesDecoded  atomic.Int64
	framesDropped  atomic.Int64
	framesRendered atomic.Int64

	// A/V sync measurement, in nanoseconds.
	syncLastNs   atomic.Int64
	syncSumNs    atomic.Int64
	syncCount    atomic.Int64
	syncMaxAbsNs atomic.Int64

	frameInterval time.Duration // target time per frame
	pendingFrame  *image.RGBA   // most recent decoded frame (may be dropped)
	pendingPTS    time.Duration // that frame's presentation timestamp
	framePending  bool          // whether pendingFrame is new

	// Decode-ahead buffer: pre-allocated frames cycle through
	// frameQueue (decode→display) and frameRecycle (display→decode).
	// All RGBA frames are allocated once at startup — zero GC pressure.
	frameQueue   chan timedFrame
	frameRecycle chan *image.RGBA
}

// timedFrame pairs a decoded frame with its presentation timestamp. The
// display loop paces against that timestamp rather than counting frames, so
// video stays locked to the stream's timeline instead of drifting away from
// the audio clock. A nil img is the end-of-stream sentinel.
type timedFrame struct {
	img *image.RGBA
	pts time.Duration
}

// NewPipeline creates a new playback pipeline from a reader.
//
// The container and codecs are probed by libavformat, so any format the
// bundled FFmpeg supports plays without an up-front format guess. If r also
// implements io.Seeker, seeking and duration work; a plain stream is
// forward-only. When r is an *os.File the path is handed to FFmpeg directly,
// which avoids bouncing every byte through a Go callback.
func NewPipeline(r io.Reader, cfg ...Config) (*Pipeline, error) {
	if r == nil {
		return nil, fmt.Errorf("nil reader")
	}

	if f, ok := r.(*os.File); ok {
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			if src, err := source.NewFFmpegSourceURL(f.Name()); err == nil {
				return newPipelineForSource(src, firstConfig(cfg)), nil
			}
			// Fall through to the reader path — the file may not be
			// reachable by name (unlinked, or an inherited descriptor).
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to rewind reader: %w", err)
			}
		}
	}

	src, err := source.NewFFmpegSource(r)
	if err != nil {
		return nil, err
	}
	return newPipelineForSource(src, firstConfig(cfg)), nil
}

// NewPipelineURL creates a playback pipeline from a file path or network URL
// (http and https are compiled in), letting FFmpeg own the I/O.
func NewPipelineURL(url string, cfg ...Config) (*Pipeline, error) {
	src, err := source.NewFFmpegSourceURL(url)
	if err != nil {
		return nil, err
	}
	return newPipelineForSource(src, firstConfig(cfg)), nil
}

// NewPipelineDevice creates a playback pipeline over a live capture device.
// format, url and options are the per-platform libavdevice contract; see
// source.NewFFmpegSourceDevice, and the public capture entry points for the
// resolution of platform naming.
//
// The resulting pipeline reports IsLive: Seek is refused, and the end of the
// stream is reported through OnError rather than OnEnd.
func NewPipelineDevice(format, url, options string, cfg ...Config) (*Pipeline, error) {
	src, err := source.NewFFmpegSourceDevice(format, url, options)
	if err != nil {
		return nil, err
	}
	return newPipelineForSource(src, firstConfig(cfg)), nil
}

// Config tunes playback behaviour that cannot be inferred from the media.
type Config struct {
	// Downmix folds multichannel audio (5.1/7.1) to stereo. When false the
	// stream's native layout is preserved, as far as the audio device allows.
	Downmix bool
}

func firstConfig(cfg []Config) Config {
	if len(cfg) > 0 {
		return cfg[0]
	}
	return Config{}
}

// newPipelineForSource assembles a pipeline around an already-created source.
func newPipelineForSource(src source.Source, cfg Config) *Pipeline {
	p := &Pipeline{
		src:   src,
		clock: NewClock(),
	}

	p.state = NewStateMachine(nil)

	p.hasVideo = src.HasVideo()
	p.hasAudio = src.HasAudio()
	p.duration = src.Duration()

	// Live capture is an asserted capability, so a source that does not implement
	// it — including the test stubs — is finite, which is the previous behaviour.
	if ls, ok := src.(source.LiveSource); ok {
		p.live = ls.IsLive()
	}

	if p.hasVideo {
		p.videoWidth, p.videoHeight = src.VideoSize()
		p.framerate = src.Framerate()
	}

	if p.hasAudio {
		// Ask for the stream's own layout unless the caller wants stereo.
		want := src.Channels()
		if cfg.Downmix {
			want = 2
		}
		p.audioOutput = audio.NewOutput(src.SampleRate(), want)

		// The device may not grant it: oto's context is created once per
		// process, so an earlier stream can already have fixed the channel
		// count. Whatever it settled on is what we must feed it, so push that
		// back down to the decoder and let swresample do the mixing.
		if got := p.audioOutput.Channels(); got != src.Channels() {
			if err := src.SetAudioChannels(got); err != nil && p.onError != nil {
				p.onError(fmt.Errorf("audio channel negotiation to %d: %w", got, err))
			}
		}

		// Configure resampler in case the oto context was initialized at a
		// different rate (oto only initializes once per process).
		p.audioOutput.SetSourceSampleRate(src.SampleRate())
	}

	// Pre-allocate frame pool: all RGBA buffers created once at startup.
	// Frames cycle: frameRecycle → decode → frameQueue → display → frameRecycle.
	if p.hasVideo {
		const bufSize = 120
		w, h := p.videoWidth, p.videoHeight
		if w <= 0 {
			w = 640
		}
		if h <= 0 {
			h = 360
		}
		p.frameQueue = make(chan timedFrame, bufSize)
		p.frameRecycle = make(chan *image.RGBA, bufSize+2)
		for i := 0; i < bufSize+2; i++ {
			p.frameRecycle <- image.NewRGBA(image.Rect(0, 0, w, h))
		}
	}

	// Wire up source callbacks.
	src.SetVideoCallback(func(rgba *image.RGBA, pts time.Duration) {
		if p.onFrame == nil {
			return
		}
		// Get a pre-allocated frame from the recycle pool.
		var pooled *image.RGBA
		select {
		case pooled = <-p.frameRecycle:
		default:
			// Pool exhausted — drop frame to avoid allocation.
			p.framesDropped.Add(1)
			return
		}
		copy(pooled.Pix, rgba.Pix)
		p.pendingFrame = pooled
		p.pendingPTS = pts
		p.framePending = true
	})

	src.SetAudioCallback(func(data []byte) {
		if p.audioOutput == nil {
			return
		}
		p.audioOutput.WriteSamples(data)
		p.clock.SetAudioTime(p.audioOutput.PlaybackPosition())
	})

	return p
}

// SetOnFrame sets the callback for new video frames.
func (p *Pipeline) SetOnFrame(f OnFrameFunc) { p.onFrame = f }

// SetOnTime sets the callback for time updates.
func (p *Pipeline) SetOnTime(f OnTimeFunc) { p.onTime = f }

// SetOnEnd sets the callback for playback end.
func (p *Pipeline) SetOnEnd(f OnEndFunc) { p.onEnd = f }

// SetOnError sets the callback for errors.
func (p *Pipeline) SetOnError(f OnErrorFunc) { p.onError = f }

// HasVideo returns true if the media contains a video stream.
func (p *Pipeline) HasVideo() bool { return p.hasVideo }

// HasAudio returns true if the media contains an audio stream.
func (p *Pipeline) HasAudio() bool { return p.hasAudio }

// VideoSize returns the video dimensions.
func (p *Pipeline) VideoSize() (int, int) { return p.videoWidth, p.videoHeight }

// Duration returns the total media duration.
func (p *Pipeline) Duration() time.Duration { return p.duration }

// Framerate returns the video framerate.
func (p *Pipeline) Framerate() float64 { return p.framerate }

// State returns the current playback state.
func (p *Pipeline) State() State { return p.state.Get() }

// Play starts playback.
func (p *Pipeline) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state.IsPlaying() {
		return
	}

	if p.state.IsPaused() {
		p.resume()
		return
	}

	// Capture the channel locally: Stop sets p.stopCh to nil while the
	// goroutines are still running, and a select on a nil channel blocks
	// forever (Close used to hang on wg.Wait because of this).
	stopCh := make(chan struct{})
	p.stopCh = stopCh
	p.state.Set(StatePlaying)
	p.clock.Start()

	if p.hasAudio {
		p.audioOutput.Start()
	}

	// Start decode-ahead goroutine: fills frameQueue independently.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.decodeAhead(stopCh)
	}()

	// Wait for a few frames to buffer before starting the display loop.
	// This avoids the initial stutter from an empty queue. Sources that
	// interleave audio with video stall the decoder on the audio-ahead
	// limit instead of reaching the frame target, so stop waiting as soon
	// as that limit is hit.
	//
	// A live capture is deliberately excluded: it produces frames only in real
	// time, so waiting for a second of them buys no smoothness and costs a second
	// of latency on a feed whose whole value is being current.
	if p.hasVideo && p.frameQueue != nil && !p.live {
		target := 30               // buffer ~1 second before starting display
		for i := 0; i < 200; i++ { // up to ~2s max wait
			if len(p.frameQueue) >= target || p.audioAheadFull() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Start display loop: reads from frameQueue at framerate.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.displayLoop(stopCh)
	}()
}

// audioAheadFull reports whether the audio output buffer is at least half
// full. The ring buffer overwrites its oldest samples on overflow, so the
// decoder must never run much further ahead than the playback position.
func (p *Pipeline) audioAheadFull() bool {
	return p.audioOutput != nil && p.audioOutput.Buffered() > p.audioOutput.BufferCap()/2
}

// Pause pauses playback.
func (p *Pipeline) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.state.IsPlaying() {
		return
	}

	p.state.Set(StatePaused)
	p.clock.Pause()

	if p.hasAudio {
		p.audioOutput.Pause()
	}
}

func (p *Pipeline) resume() {
	p.state.Set(StatePlaying)
	p.clock.Start()

	if p.hasAudio {
		p.audioOutput.Resume()
	}
}

// Stop stops playback and resets position.
func (p *Pipeline) Stop() {
	p.mu.Lock()

	if p.state.IsStopped() {
		p.mu.Unlock()
		return
	}

	p.state.Set(StateStopped)
	if p.stopCh != nil {
		close(p.stopCh)
		p.stopCh = nil
	}
	p.mu.Unlock()

	// Wait for decode goroutine to exit (outside the lock to avoid deadlock).
	p.wg.Wait()

	// Drain frameQueue back into recycle pool so frames are available on next Play.
	if p.frameQueue != nil {
		for {
			select {
			case tf := <-p.frameQueue:
				p.RecycleFrame(tf.img)
			default:
				goto drained
			}
		}
	drained:
	}

	p.clock.Reset()

	if p.hasAudio {
		p.audioOutput.Stop()
	}

	p.src.Rewind()
}

// IsLive reports whether the source is an open-ended capture rather than a
// finite recording. A live pipeline reports a zero Duration, refuses Seek, and
// reports the end of its stream through OnError instead of OnEnd.
func (p *Pipeline) IsLive() bool { return p.live }

// ErrCaptureStopped reports that a live capture stopped producing media without
// the source being able to say why. A cause, where the source has one — the
// common case being a device already in use by another application — is wrapped
// instead.
var ErrCaptureStopped = errors.New("mmedia: capture device stopped producing media")

// reportEnd signals that the source ran out, from whichever loop noticed first.
//
// For a finite source that is completion, which is OnEnd — the callback that
// drives looping and end-of-playback UI. A live source is not supposed to
// finish, so reaching its end means the device was lost or could not be
// reacquired; that is reported through OnError, and OnEnd deliberately does not
// fire, which is also what makes WithLoop inert for a capture rather than
// spinning on a device that is gone.
func (p *Pipeline) reportEnd() {
	p.state.Set(StateStopped)
	if p.live {
		if p.onError != nil {
			p.onError(p.captureError())
		}
		return
	}
	if p.onEnd != nil {
		p.onEnd()
	}
}

// captureError prefers the source's own reason over the generic one.
func (p *Pipeline) captureError() error {
	if es, ok := p.src.(source.ErrorSource); ok {
		if err := es.Err(); err != nil {
			return fmt.Errorf("%w: %w", ErrCaptureStopped, err)
		}
	}
	return ErrCaptureStopped
}

// Seek seeks to the given position. A live capture has no timeline to move
// within, so this is refused for one rather than passed down to an undefined
// av_seek_frame on a device.
func (p *Pipeline) Seek(pos time.Duration) {
	if p.live {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if pos < 0 {
		pos = 0
	}
	if p.duration > 0 && pos > p.duration {
		pos = p.duration
	}

	p.decodeMu.Lock()
	p.src.Seek(pos)
	p.decodeMu.Unlock()

	// Discard everything decoded before the seek: queued video frames and
	// buffered (unplayed) audio. The audio position is rebased so the
	// consumed-bytes clock continues from the seek target.
	if p.frameQueue != nil {
	drainLoop:
		for {
			select {
			case tf := <-p.frameQueue:
				p.RecycleFrame(tf.img)
			default:
				break drainLoop
			}
		}
	}
	if p.audioOutput != nil {
		p.audioOutput.Rebase(pos)
	}
	p.clock.Seek(pos)
}

// RecycleFrame returns a displayed frame to the pre-allocated pool for reuse.
// Call this after the frame pixels have been copied to the display surface.
func (p *Pipeline) RecycleFrame(frame *image.RGBA) {
	if p.frameRecycle == nil || frame == nil {
		return
	}
	select {
	case p.frameRecycle <- frame:
	default:
		// Pool full — discard (shouldn't happen with correct sizing).
	}
}

// SetVolume sets audio volume (0.0 to 1.0).
func (p *Pipeline) SetVolume(v float64) {
	if p.audioOutput != nil {
		p.audioOutput.SetVolume(v)
	}
}

// Volume returns the current audio volume.
func (p *Pipeline) Volume() float64 {
	if p.audioOutput != nil {
		return p.audioOutput.Volume()
	}
	return 0
}

// CurrentTime returns the current playback position.
func (p *Pipeline) CurrentTime() time.Duration {
	return p.clock.Time()
}

// Stats returns current playback performance metrics.
func (p *Pipeline) Stats() PlaybackStats {
	st := PlaybackStats{
		FramesDecoded:  p.framesDecoded.Load(),
		FramesDropped:  p.framesDropped.Load(),
		FramesRendered: p.framesRendered.Load(),
		SyncOffset:     time.Duration(p.syncLastNs.Load()),
		MaxSyncOffset:  time.Duration(p.syncMaxAbsNs.Load()),
	}
	if n := p.syncCount.Load(); n > 0 {
		st.AvgSyncOffset = time.Duration(p.syncSumNs.Load() / n)
	}
	return st
}

// recordSync accumulates the presentation error of one displayed frame.
func (p *Pipeline) recordSync(offset time.Duration) {
	ns := int64(offset)
	p.syncLastNs.Store(ns)
	p.syncSumNs.Add(ns)
	p.syncCount.Add(1)

	abs := ns
	if abs < 0 {
		abs = -abs
	}
	for {
		cur := p.syncMaxAbsNs.Load()
		if abs <= cur || p.syncMaxAbsNs.CompareAndSwap(cur, abs) {
			break
		}
	}
}

// Close releases all resources.
func (p *Pipeline) Close() {
	p.Stop()
	p.src.Close()
}

// decodeAhead runs in its own goroutine, decoding frames as fast as possible
// into the frameQueue buffer. This decouples decode from display timing.
// Back-pressure from a full frameQueue naturally throttles decode speed.
func (p *Pipeline) decodeAhead(stop <-chan struct{}) {
	// One decode step's worth of media time. The FFmpeg source decodes a
	// fixed unit per call and ignores this, but the loop still uses it to
	// derive the frame interval used for display pacing.
	step := 20 * time.Millisecond
	if p.hasVideo && p.framerate > 0 {
		p.frameInterval = time.Duration(float64(time.Second) / p.framerate)
		step = p.frameInterval
	}

	for {
		select {
		case <-stop:
			return
		default:
		}

		if !p.state.IsPlaying() {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Audio back-pressure: the output ring buffer overwrites its
		// oldest samples when overrun, so pause decoding while playback
		// catches up. (Video has its own back-pressure via frameQueue.)
		if p.audioAheadFull() {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		// Decode one step — back-pressure above throttles the pace.
		p.decodeMu.Lock()
		p.framePending = false
		p.src.Decode(step)
		p.decodeMu.Unlock()

		if p.src.HasEnded() {
			if p.frameQueue != nil {
				select {
				case p.frameQueue <- timedFrame{}: // nil img: end of stream
				case <-stop:
				}
				return
			}
			// Audio-only: let the buffered samples play out, then finish.
			for p.audioOutput != nil && p.audioOutput.Buffered() > 0 {
				select {
				case <-stop:
					return
				default:
				}
				time.Sleep(20 * time.Millisecond)
			}
			p.reportEnd()
			return
		}

		if p.framePending && p.pendingFrame != nil {
			p.framesDecoded.Add(1)
			// Blocks if buffer is full (back-pressure from display rate).
			select {
			case p.frameQueue <- timedFrame{img: p.pendingFrame, pts: p.pendingPTS}:
			case <-stop:
				return
			}
		}
	}
}

// displayLoop pulls frames from the buffer and displays them.
// Uses a minimum frame interval to cap the display rate at the video's
// native framerate, but adapts naturally when the decoder is slower.
func (p *Pipeline) displayLoop(stop <-chan struct{}) {
	minInterval := 10 * time.Millisecond
	if p.hasVideo && p.framerate > 0 {
		minInterval = time.Duration(float64(time.Second) / p.framerate)
	}

	timeTicker := time.NewTicker(100 * time.Millisecond)
	defer timeTicker.Stop()

	for {
		// Wait for the next frame (blocking). This naturally adapts to
		// the decode rate — no pauses when the decoder is slow, it just
		// runs at whatever fps the decoder can sustain.
		select {
		case <-stop:
			return

		case tf := <-p.frameQueue:
			if tf.img == nil {
				p.reportEnd()
				return
			}

			if !p.state.IsPlaying() {
				p.RecycleFrame(tf.img)
				continue
			}

			// A/V sync: hold the frame until its own presentation timestamp
			// comes due on the presentation clock (audio-driven when audio is
			// present, wall clock otherwise). Pacing against the frame's PTS
			// rather than a running frame count means error cannot accumulate:
			// every frame is corrected against the stream's own timeline.
			//
			// Re-read the clock after each sleep — Seek moves it concurrently,
			// and waiting on a stale deadline would stall the display.
			for {
				ahead := tf.pts - p.clock.Time()
				if ahead <= 2*time.Millisecond {
					break
				}
				if ahead > 100*time.Millisecond {
					ahead = 100 * time.Millisecond
				}
				select {
				case <-stop:
					p.RecycleFrame(tf.img)
					return
				case <-time.After(ahead):
				}
			}

			// Late by more than two frames: skip ahead, but only when a fresher
			// frame is already decoded. Dropping the only frame available would
			// starve the display without closing the gap.
			if p.clock.Time()-tf.pts > 2*minInterval && len(p.frameQueue) > 0 {
				p.RecycleFrame(tf.img)
				p.framesDropped.Add(1)
				continue
			}

			// Record the presentation error before handing the frame over, so
			// the measurement reflects pacing rather than render cost.
			p.recordSync(p.clock.Time() - tf.pts)

			if p.onFrame != nil {
				p.onFrame(tf.img)
			}
			p.RecycleFrame(tf.img)
			p.framesRendered.Add(1)

		case <-timeTicker.C:
			if p.onTime != nil {
				p.onTime(p.clock.Time(), p.duration)
			}
		}
	}
}
