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

	// seekGen counts seeks. Every frame carries the generation it was decoded
	// in, and the display drops a frame from an earlier one: after a seek, the
	// frame the display was holding and the one the decoder was handing over
	// both belong to the old position (#501). seekWake tells a display waiting
	// on such a frame to look again now rather than when its time comes, which
	// after a backward seek can be seconds away.
	seekGen  atomic.Uint64
	seekWake chan struct{}
	// presentMu makes the display's last generation check and its onFrame
	// one step a seek cannot land inside: the display holds it across both,
	// and Seek holds it to bump seekGen. Without it a frame checked just
	// before a seek was shown just after it (#819). onFrame must not call
	// back into the pipeline while it is held.
	presentMu sync.Mutex
	// pausedSeek asks the idle decoder for the frame at a seek made while
	// paused, which nothing else decodes or shows until Play (#831).
	pausedSeek atomic.Bool

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
	// The pool's RGBA frames are allocated once, when the geometry is set; the
	// source still hands over a freshly decoded frame that is copied into one.
	frameQueue   chan timedFrame
	frameRecycle chan *image.RGBA

	prerollEnd prerollExit // why the last Play stopped waiting (held under mu)
}

// presentHook, when set, runs in the display between its last generation
// check and onFrame. Tests use it to hold a frame there while a seek runs.
var presentHook func(img *image.RGBA)

// timedFrame pairs a decoded frame with its presentation timestamp. The
// display loop paces against that timestamp rather than counting frames, so
// video stays locked to the stream's timeline instead of drifting away from
// the audio clock. A nil img is the end-of-stream sentinel.
type timedFrame struct {
	img *image.RGBA
	pts time.Duration
	gen uint64 // the seek generation it was decoded in (see Pipeline.seekGen)
}

// NewPipeline creates a new playback pipeline from a reader.
//
// The container and codecs are probed by libavformat, so any format the
// bundled FFmpeg supports plays without an up-front format guess. If r also
// implements io.Seeker, seeking and duration work; a plain stream is
// forward-only. When r is an *os.File the path is handed to FFmpeg directly,
// which avoids bouncing every byte through a Go callback.
func NewPipeline(r io.Reader) (*Pipeline, error) {
	if r == nil {
		return nil, fmt.Errorf("nil reader")
	}

	if f, ok := r.(*os.File); ok {
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			if src, err := source.NewFFmpegSourceURL(f.Name()); err == nil {
				return newOwnedPipeline(src)
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
	return newOwnedPipeline(src)
}

// NewPipelineURL creates a playback pipeline from a file path or network URL
// (http and https are compiled in), letting FFmpeg own the I/O.
func NewPipelineURL(url string) (*Pipeline, error) {
	src, err := source.NewFFmpegSourceURL(url)
	if err != nil {
		return nil, err
	}
	return newOwnedPipeline(src)
}

// newOwnedPipeline builds a pipeline around a source this package opened, and
// closes the source if the pipeline cannot be built, since no caller has it.
func newOwnedPipeline(src source.Source) (*Pipeline, error) {
	p, err := newPipelineForSource(src)
	if err != nil {
		src.Close()
		return nil, err
	}
	return p, nil
}

// There is deliberately no NewPipelineDevice alongside these. Opening a capture
// device is not one call: the per-platform selector rules and the framerate
// fallback (#95) both live above this package, and a device switch (#96) has to
// build the source on its own anyway so it can hand it to SwapSource. The
// capture entry points open a source.Source and pass it to NewPipelineForSource.
//
// The resulting pipeline reports IsLive: Seek is refused, and the end of the
// stream is reported through OnError rather than OnEnd.

// newAudioOutput is audio.NewOutput, a variable so a test can build a pipeline
// with audio on a machine with no device, or make the device refuse to open.
var newAudioOutput = audio.NewOutput

// SetAudioOutputForTests makes every pipeline built from now on open its audio
// output with open, until restore is called. It is for tests above this
// package, which build real players on files with sound and so would fail on a
// machine with no sound card (a CI runner), since a device that will not open
// fails construction. Not for use outside tests: it is not safe to call while
// pipelines are being built.
func SetAudioOutputForTests(open func(channels int) (*audio.Output, error)) (restore func()) {
	saved := newAudioOutput
	newAudioOutput = open
	return func() { newAudioOutput = saved }
}

// newPipelineForSource assembles a pipeline around an already-created source.
//
// It fails when the stream has audio and the device will not open (#690), or
// the source cannot be converted to the device's format (#698): a player built
// on either would play silent or at the wrong speed, and nothing would say why.
// On failure it leaves src open, for the caller to close; nothing else it
// built needs releasing, since no goroutine starts before Play and the device
// itself is the process's.
func newPipelineForSource(src source.Source) (*Pipeline, error) {
	p := &Pipeline{
		clock:    NewClock(),
		state:    NewStateMachine(nil),
		hasVideo: src.HasVideo(),
		hasAudio: src.HasAudio(),
		seekWake: make(chan struct{}, 1),
	}

	if p.hasAudio {
		out, err := newAudioOutput(deviceChannels())
		if err != nil {
			return nil, err
		}
		p.audioOutput = out
	}

	if err := p.convertAudio(src); err != nil {
		return nil, err
	}
	p.adoptSource(src)
	return p, nil
}

// audioDevice is the channel count the audio device is opened with, and
// whether that is settled. The device is the process's (oto allows one), so
// this is too.
var audioDevice struct {
	mu       sync.Mutex
	channels int  // 0 until SetDeviceChannels: stereo
	fixed    bool // a pipeline with audio has asked for the device
}

// deviceChannels is the channel count a pipeline asks the audio device for,
// and the channel policy (#707): stereo, unless the app asked for another
// count with SetDeviceChannels before the first pipeline with audio. The
// stream's own layout plays no part. Every stream is converted to the device's
// channels by its decoder's swresample, so 5.1 and 7.1 are downmixed to stereo
// by default, and mono plays on both channels.
//
// It used to open at the layout of the first stream when that was surround
// (#506). A stereo-only device refusing 6 channels then failed the open, and
// because the device opens once per process, every later player with audio
// failed too, stereo ones included, until the app restarted (#707). Surround
// output is now the app's choice, made knowing its device, not a side effect
// of what happened to play first.
//
// Calling it settles the count: SetDeviceChannels refuses a different one from
// then on, because the device about to be opened with it cannot be reopened.
func deviceChannels() int {
	audioDevice.mu.Lock()
	defer audioDevice.mu.Unlock()
	audioDevice.fixed = true
	if audioDevice.channels > 0 {
		return audioDevice.channels
	}
	return stereo
}

// stereo is the device's channel count unless the app asks for another.
const stereo = 2

// SetDeviceChannels sets the channel count the audio device is opened with,
// in place of stereo: 6 for 5.1, 8 for 7.1. Streams with fewer channels are
// upmixed to it and streams with more downmixed, by swresample.
//
// It must be called before the first pipeline with audio is built, since that
// opens the device, once for the process. After that it returns an error unless
// n is the count already in use; it never changes the device silently. n must
// be positive. A device that refuses n fails every pipeline with audio, the
// first one's error repeated (audio.NewOutput).
func SetDeviceChannels(n int) error {
	if n <= 0 {
		return fmt.Errorf("audio device channels must be positive, not %d", n)
	}
	audioDevice.mu.Lock()
	defer audioDevice.mu.Unlock()
	if audioDevice.fixed {
		cur := audioDevice.channels
		if cur <= 0 {
			cur = stereo
		}
		if n != cur {
			return fmt.Errorf("audio device channels: already %d, cannot change to %d: the device was opened by the first player with audio, and a process has one", cur, n)
		}
		return nil
	}
	audioDevice.channels = n
	return nil
}

// NewPipelineForSource assembles a pipeline around an already-created source.
//
// It is what the capture entry points use: they have to build the source
// themselves, because opening a device is where the per-platform selector rules
// and the framerate fallback live, and neither belongs down here.
//
// It returns an error when the source has audio and the audio device cannot
// be opened, or the source cannot be converted to the device's format. The
// source is then left open, and the caller still owns it.
func NewPipelineForSource(src source.Source) (*Pipeline, error) {
	return newPipelineForSource(src)
}

// adoptSource makes src the pipeline's source: it takes the geometry, timing
// and audio configuration from it, sizes the frame pool to match, and wires the
// decode callbacks.
//
// src's audio has already been converted to the device's format
// (convertAudio), so nothing here can fail.
//
// Shared by construction and SwapSource, which is the point — a swapped-in
// source must end up configured exactly as one passed to the constructor would,
// and two copies of this would drift. hasVideo/hasAudio are deliberately *not*
// set here: they are fixed at construction, because the widget above was built
// around them, and SwapSource refuses a source that would change them.
func (p *Pipeline) adoptSource(src source.Source) {
	p.src = src
	p.duration = src.Duration()

	// Live capture is an asserted capability, so a source that does not implement
	// it — including the test stubs — is finite, which is the previous behaviour.
	p.live = false
	if ls, ok := src.(source.LiveSource); ok {
		p.live = ls.IsLive()
	}

	if p.hasVideo {
		w, h := src.VideoSize()
		p.framerate = src.Framerate()
		// Only rebuild the pool when the geometry actually moved. A swap between
		// two cameras of the same resolution — the common case — then costs
		// nothing, where a rebuild would drop and reallocate the whole pool.
		if w != p.videoWidth || h != p.videoHeight || p.frameQueue == nil {
			p.videoWidth, p.videoHeight = w, h
			p.allocFramePool(w, h)
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
}

// convertAudio asks src for exactly what the audio device plays. The device is
// opened once per process and its format is fixed for the life of it, so the
// source's decoder (swresample) converts the channels and the rate.
//
// It runs before adoptSource, on construction and on SwapSource alike, and
// touches nothing but src, so a failure leaves the pipeline as it was and src
// unwired: construction fails on it, and SwapSource refuses the source. Either
// way a stream is never played at its own rate on the device, which is 8.8%
// fast for a 44.1 kHz stream (#698).
func (p *Pipeline) convertAudio(src source.Source) error {
	if !p.hasAudio {
		return nil
	}
	var err error
	if got := p.audioOutput.Channels(); got != src.Channels() {
		if e := src.SetAudioChannels(got); e != nil {
			err = errors.Join(err, fmt.Errorf("audio channel negotiation to %d: %w", got, e))
		}
	}
	if rate := p.audioOutput.SampleRate(); rate != src.SampleRate() {
		if e := src.SetAudioSampleRate(rate); e != nil {
			err = errors.Join(err, fmt.Errorf("audio sample rate conversion to %d: %w", rate, e))
		}
	}
	return err
}

// allocFramePool builds the decode-ahead buffer: all RGBA frames are allocated
// once, then cycle frameRecycle → decode → frameQueue → display → frameRecycle,
// so the queue itself allocates nothing in steady state. (The FFmpeg source
// still allocates each decoded frame before it is copied into the pool.)
//
// The old pool is dropped before the new one is built rather than after, so a
// device switch never holds two of them at once.
func (p *Pipeline) allocFramePool(w, h int) {
	if w <= 0 {
		w = 640
	}
	if h <= 0 {
		h = 360
	}

	p.frameQueue = nil
	p.frameRecycle = nil
	p.pendingFrame = nil
	p.framePending = false

	bufSize := queueDepth(w, h)
	p.frameQueue = make(chan timedFrame, bufSize)
	p.frameRecycle = make(chan *image.RGBA, bufSize+2)
	for i := 0; i < bufSize+2; i++ {
		p.frameRecycle <- image.NewRGBA(image.Rect(0, 0, w, h))
	}
}

// The decode-ahead queue is sized by memory, not by frame count (#110). A fixed
// 120 frames was 37 MB at 320x240 but about 1 GB at 1080p and 4 GB at 4K, for a
// depth whose only job is to absorb decode jitter, which does not grow with the
// resolution.
const (
	framePoolBudget = 128 << 20 // bytes of queued RGBA frames
	maxQueuedFrames = 120       // the old fixed depth: small videos keep it
	minQueuedFrames = 8         // floor for 4K and up, where the budget buys ~4
	prerollFrames   = 30        // Play waits for this many (~1s) before display
)

// queueDepth returns how many decoded frames of w x h are queued ahead of the
// display: as many as fit in framePoolBudget, kept between minQueuedFrames and
// maxQueuedFrames. The pool holds two more, for the frame being decoded into
// and the one on screen.
func queueDepth(w, h int) int {
	n := framePoolBudget / (w * h * 4)
	return max(minQueuedFrames, min(maxQueuedFrames, n))
}

// prerollTarget is how many queued frames Play waits for before it starts the
// display: prerollFrames, or the whole queue when the queue is shorter. Waiting
// for more than the queue holds would only ever end on the 2 s timeout.
//
// A whole queue is reachable because nothing but the decoder holds a pool frame
// before the display starts, and a stop returns every frame to the pool
// (TestStopReturnsEveryFrameToThePool).
func (p *Pipeline) prerollTarget() int {
	return min(prerollFrames, cap(p.frameQueue))
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
	p.restAfterEnd()

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
	p.prerollEnd = prerollSkipped
	if p.hasVideo && p.frameQueue != nil && !p.live {
		p.prerollEnd = p.preroll()
	}

	// Start display loop: reads from frameQueue at framerate.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.displayLoop(stopCh)
	}()
}

// prerollExit is why Play stopped waiting for frames before starting the
// display.
type prerollExit int

const (
	prerollSkipped     prerollExit = iota // no wait: no video, or a live feed
	prerollQueued                         // the target number of frames is queued
	prerollAudioAhead                     // the decoder stalled on the audio-ahead limit
	prerollSourceEnded                    // a clip shorter than the target ended
	prerollTimedOut                       // none of those within ~2 s
)

func (e prerollExit) String() string {
	return [...]string{"skipped", "queued", "audio ahead", "source ended", "timed out"}[e]
}

// preroll waits, up to ~2 s, for prerollTarget frames to be queued.
func (p *Pipeline) preroll() prerollExit {
	target := p.prerollTarget() // ~1 second, or a full queue at high resolution
	for i := 0; i < 200; i++ {  // up to ~2s max wait
		switch {
		case len(p.frameQueue) >= target:
			return prerollQueued
		case p.audioAheadFull():
			return prerollAudioAhead
		case p.sourceEnded():
			// A clip shorter than the target ends first: stop waiting then,
			// or every start and every loop of it stalls the full two seconds.
			return prerollSourceEnded
		}
		time.Sleep(10 * time.Millisecond)
	}
	return prerollTimedOut
}

// sourceEnded reports whether the decoder has reached the end of the source.
func (p *Pipeline) sourceEnded() bool {
	p.decodeMu.Lock()
	defer p.decodeMu.Unlock()
	return p.src.HasEnded()
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

// restAfterEnd brings a stream that ran out, at its end or on an error, to
// rest before Play starts it again, and plays it from the start — or from a
// position sought since, which is how an app resumes a dropped stream.
//
// Without it Play started a second set of goroutines over the first: an
// audio-only stream's display loop outlives the end it reports, its stop
// channel was overwritten unclosed, and the next Stop or Close waited on it
// forever. And the source was still at its end, so nothing played (#842).
func (p *Pipeline) restAfterEnd() {
	p.mu.Lock()
	ranOut := p.state.IsStopped() && p.stopCh != nil
	p.mu.Unlock()
	if !ranOut {
		return
	}
	pos := p.CurrentTime()
	if !p.halt() {
		return
	}
	if p.src.HasEnded() {
		p.src.Rewind()
		return
	}
	p.Seek(pos) // halt reset the clock and the audio position
}

// Stop stops playback and resets position.
func (p *Pipeline) Stop() {
	if !p.halt() {
		return
	}
	p.src.Rewind()
}

// halt brings the pipeline to rest — decode and display goroutines joined,
// frames returned to the pool, clock and audio device reset — without touching
// the source. It reports whether there was anything to stop.
//
// Split out of Stop because the two callers want different things done to the
// source afterwards: Stop rewinds it (and, for a capture, that releases the
// camera), while SwapSource is about to close it and replace it entirely.
func (p *Pipeline) halt() bool {
	p.mu.Lock()

	// Stopped with the goroutines' channel still open is a stream that played
	// to its end (reportEnd sets the state; nothing has joined the goroutines
	// or reset the clock yet), and that still needs bringing to rest: a Stop
	// after the end returned early here, so it rewound nothing and a looping
	// player restarted on a source that had already ended, over and over
	// (#502).
	if p.state.IsStopped() && p.stopCh == nil {
		p.mu.Unlock()
		return false
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
	return true
}

// ErrSwapMedia reports that a source offered to SwapSource does not carry the
// same kinds of media as the one it would replace.
//
// It is a refusal rather than a best effort on purpose. The widget above the
// pipeline was built around the answers to HasVideo and HasAudio — a video
// surface or an "Audio Only" label, an audio output device or none — and none of
// that can be rebuilt from down here. Swapping a camera for a microphone would
// leave a player rendering a surface nothing writes to.
var ErrSwapMedia = errors.New("mmedia: the replacement source does not carry the same media")

// SwapSource replaces the pipeline's source in place, keeping everything built
// around it: the widget, the video surface, the audio device and the transport.
//
// This exists because the alternative does not work. Rebuilding the player to
// change camera flickers, loses layout and drops focus, so the switch has to
// happen inside the live pipeline: goroutines joined, frames returned to the
// pool, old source closed, new one adopted, clock restarted. Playback resumes
// only if it was running, so swapping while paused leaves it paused.
//
// The caller opens the new source, and must do so *before* calling this, which
// is what makes a failed switch survivable: an open that fails never reaches
// here and the pipeline keeps the source it has. If this returns an error the
// source passed in was not adopted and the caller still owns it.
//
// Three things are deliberately retaken rather than carried across:
//
//   - The clock restarts at zero. A capture device stamps frames with absolute
//     host time, and the new device's origin is its own; FFmpegSource rebases on
//     its first frame, which it can only do against a clock that starts where it
//     expects.
//   - The frame pool is rebuilt when the geometry moves, and only then.
//   - The audio format is asked of the new source, not of the device. The
//     device is opened once per process, at DeviceSampleRate and
//     deviceChannels, and cannot change, so the new source's
//     decoder is asked to convert to both. A source that cannot convert is
//     refused, before anything is torn down, like one of the wrong shape.
//
// Playback counters are left running. They measure the session rather than the
// device, the same way they survive a seek.
func (p *Pipeline) SwapSource(src source.Source) error {
	if src == nil {
		return fmt.Errorf("%w: no source given", ErrSwapMedia)
	}
	// Validated before anything is torn down, so a refused swap is not merely
	// survivable but invisible: playback never stops.
	if src.HasVideo() != p.hasVideo || src.HasAudio() != p.hasAudio {
		return fmt.Errorf("%w: this pipeline carries %s and the replacement carries %s",
			ErrSwapMedia, describeMedia(p.hasVideo, p.hasAudio),
			describeMedia(src.HasVideo(), src.HasAudio()))
	}
	if err := p.convertAudio(src); err != nil {
		return err
	}

	// Paused counts as running: the user switching camera mid-pause expects to
	// still be paused afterwards, not stopped.
	resume := p.state.IsPlaying()
	wasPaused := p.state.IsPaused()

	p.halt()

	old := p.src
	p.adoptSource(src)
	if old != nil {
		old.Close()
	}

	switch {
	case resume:
		p.Play()
	case wasPaused:
		p.Play()
		p.Pause()
	}
	return nil
}

// describeMedia names a video/audio combination for ErrSwapMedia.
func describeMedia(video, audio bool) string {
	switch {
	case video && audio:
		return "video and audio"
	case video:
		return "video only"
	case audio:
		return "audio only"
	default:
		return "neither video nor audio"
	}
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
//
// A finite source that stopped on a read error did not complete either: a
// network stream whose connection dropped reads as an I/O error from FFmpeg,
// and reporting that as OnEnd with the clock at the duration told the app the
// track had finished (#839). It goes to OnError with the clock left where
// playback stopped, so the app can show it, retry, or resume from there.
func (p *Pipeline) reportEnd() {
	p.state.Set(StateStopped)
	if p.live {
		if p.onError != nil {
			p.onError(p.captureError())
		}
		return
	}
	if err := p.sourceError(); err != nil {
		p.clock.Pause()
		if p.onTime != nil {
			p.onTime(p.clock.Time(), p.duration)
		}
		if p.onError != nil {
			p.onError(err)
		}
		return
	}
	// Rest the clock at the end and report it. Time updates come every 100 ms,
	// so the last one fell short of the end and a progress bar stopped there;
	// and with nothing to stop it the clock ran on past the duration.
	p.clock.Pause()
	if p.duration > 0 {
		p.clock.Seek(p.duration)
	}
	if p.onTime != nil {
		p.onTime(p.clock.Time(), p.duration)
	}
	if p.onEnd != nil {
		p.onEnd()
	}
}

// captureError prefers the source's own reason over the generic one.
func (p *Pipeline) captureError() error {
	if err := p.sourceError(); err != nil {
		return fmt.Errorf("%w: %w", ErrCaptureStopped, err)
	}
	return ErrCaptureStopped
}

// sourceError is the error that stopped the source, or nil when it ended
// cleanly or cannot say.
func (p *Pipeline) sourceError() error {
	if es, ok := p.src.(source.ErrorSource); ok {
		return es.Err()
	}
	return nil
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

	// Everything up to the clock seek stays under decodeMu, so the decoder
	// cannot decode the new position until the old one is discarded. Draining
	// after releasing it also took the first frames decoded at the target, and
	// the picture froze until the first frame left was due (#513).
	p.decodeMu.Lock()
	defer p.decodeMu.Unlock()
	p.src.Seek(pos)
	// Under decodeMu, no frame decoded before the seek gets the new
	// generation; under presentMu, none checked before it is shown after it.
	p.presentMu.Lock()
	p.seekGen.Add(1)
	p.presentMu.Unlock()

	// Discard everything decoded before the seek: queued video frames and
	// buffered (unplayed) audio. The audio position is rebased so the
	// consumed-bytes clock continues from the seek target. A frame the decoder
	// was blocked sending can still arrive after the drain; it carries the old
	// generation, and the display discards it.
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
	if p.hasVideo && p.state.IsPaused() {
		p.pausedSeek.Store(true)
	}

	select {
	case p.seekWake <- struct{}{}:
	default: // a wake is already pending
	}
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
			if p.state.IsPaused() && p.pausedSeek.CompareAndSwap(true, false) {
				p.showPausedSeek(step)
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		p.pausedSeek.Store(false) // playing: the display shows the target

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
		gen := p.seekGen.Load()
		// Read under decodeMu: Seek clears the source's end flag while holding it.
		ended := p.src.HasEnded()
		p.decodeMu.Unlock()

		if ended {
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
			case p.frameQueue <- timedFrame{img: p.pendingFrame, pts: p.pendingPTS, gen: gen}:
			case <-stop:
				// The frame never reached the queue, so halt's drain cannot
				// return it. Losing it would leave the decoder one frame short
				// for good: with the display holding one and the queue full,
				// every decode then finds the pool empty and drops its frame.
				p.RecycleFrame(p.pendingFrame)
				return
			}
		}
	}
}

// showPausedSeek decodes the frame at the target of a seek made while paused
// and shows it, leaving the pipeline paused. It bypasses the queue: the paused
// display leaves that alone (#527), and the next decode after it is the frame
// that follows the target, so resuming carries on from there.
func (p *Pipeline) showPausedSeek(step time.Duration) {
	p.decodeMu.Lock()
	p.framePending = false
	// One video frame per Decode; a second try covers an empty frame pool.
	for i := 0; i < 2 && !p.framePending && !p.src.HasEnded(); i++ {
		p.src.Decode(step)
	}
	img, ok := p.pendingFrame, p.framePending
	p.framePending = false
	gen := p.seekGen.Load()
	p.decodeMu.Unlock()
	if !ok {
		return
	}
	p.framesDecoded.Add(1)

	p.presentMu.Lock()
	if gen == p.seekGen.Load() { // no later seek moved past it
		if p.onFrame != nil {
			p.onFrame(img)
		}
		p.framesRendered.Add(1)
	}
	p.presentMu.Unlock()
	p.RecycleFrame(img)
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
		// While paused, leave the queue alone. A frame taken now could only be
		// thrown away, so resuming lost up to a queue's worth of picture, and
		// taking the end-of-stream marker ended a paused clip, moving its time
		// to the end (#527).
		if p.state.IsPaused() {
			select {
			case <-stop:
				return
			case <-timeTicker.C:
				if p.onTime != nil {
					p.onTime(p.clock.Time(), p.duration)
				}
			case <-time.After(10 * time.Millisecond):
			}
			continue
		}

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

			if !p.state.IsPlaying() || tf.gen != p.seekGen.Load() {
				p.RecycleFrame(tf.img) // paused, or decoded before the latest seek
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
			stale := false
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
				case <-p.seekWake:
				case <-time.After(ahead):
				}
				if tf.gen != p.seekGen.Load() {
					stale = true
					break
				}
			}
			if stale {
				p.RecycleFrame(tf.img)
				continue
			}

			// Late by more than two frames: skip ahead, but only when a fresher
			// frame is already decoded. Dropping the only frame available would
			// starve the display without closing the gap.
			if p.clock.Time()-tf.pts > 2*minInterval && len(p.frameQueue) > 0 {
				p.RecycleFrame(tf.img)
				p.framesDropped.Add(1)
				continue
			}

			p.presentMu.Lock()
			if tf.gen != p.seekGen.Load() { // a seek landed since the last check
				p.presentMu.Unlock()
				p.RecycleFrame(tf.img)
				continue
			}
			// Record the presentation error before handing the frame over, so
			// the measurement reflects pacing rather than render cost.
			p.recordSync(p.clock.Time() - tf.pts)
			if presentHook != nil {
				presentHook(tf.img)
			}
			if p.onFrame != nil {
				p.onFrame(tf.img)
			}
			p.presentMu.Unlock()
			p.RecycleFrame(tf.img)
			p.framesRendered.Add(1)

		case <-timeTicker.C:
			if p.onTime != nil {
				p.onTime(p.clock.Time(), p.duration)
			}
		}
	}
}
