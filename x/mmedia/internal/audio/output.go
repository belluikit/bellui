package audio

import (
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// DeviceSampleRate is the rate the audio device is opened at, whatever the
// media. oto allows one device per process, so its format is fixed by the
// first Output. Fixing the rate at 48 kHz, the usual rate for video, makes it
// the same in every process; every stream at another rate is converted to it
// by its decoder's swresample (FFmpeg quality), not here. It used to follow
// the first stream, and later streams at other rates went through a linear
// resampler that lost 3.4 dB at 15 kHz and dropped frames at chunk
// boundaries (#506, #582).
const DeviceSampleRate = 48000

// defaultChannels is the device's channel count when the caller asks for none.
// The pipeline's deviceChannels is the policy: stereo unless the app set
// another count (#707).
const defaultChannels = 2

// Output manages audio playback via ebitengine/oto.
type Output struct {
	mu         sync.Mutex
	ctx        otoContext // nil for a detached Output
	player     *oto.Player
	buffer     *RingBuffer
	sampleRate int
	channels   int
	volume     float64
	started    bool

	// Seek rebase: PlaybackPosition = posBase + time(totalRead - readBase).
	posBase  time.Duration
	readBase int64
}

// device is the process's one audio device. It is opened by the first
// NewOutput and never again: oto allows a single context per process.
var device struct {
	once     sync.Once
	ctx      otoContext
	channels int   // channel count the device was opened with
	err      error // why it could not be opened, for every later NewOutput
}

// otoContext is what an Output uses of *oto.Context. It is an interface so a
// test can stand in a context that opened and then failed, the way oto reports
// a missing device on Linux, which a real *oto.Context cannot be made to do.
type otoContext interface {
	NewPlayer(r io.Reader) *oto.Player
	Err() error
}

// newOtoContext is oto.NewContext, a variable so a test can make the device
// refuse to open.
var newOtoContext = func(op *oto.NewContextOptions) (otoContext, chan struct{}, error) {
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, nil, err // not a nil *oto.Context in a non-nil interface
	}
	return ctx, ready, nil
}

// openDevice opens the audio device once per process and returns it, or the
// error it failed with.
//
// A failure is remembered, not retried. oto marks its one context as created
// before the platform backend runs and never clears the mark (context.go in
// v3.4.0: contextCreated = true precedes newContext), so a second
// oto.NewContext can only fail with "context is already created", which would
// hide the real cause. Returning the first error to every later caller is the
// only answer that is both true and does not block. It used to discard the
// error and receive on the nil ready channel oto returns with it, which blocked
// forever inside the Once and so blocked every later player too (#690).
func openDevice(channels int) (otoContext, int, error) {
	device.once.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   DeviceSampleRate,
			ChannelCount: channels,
			Format:       oto.FormatSignedInt16LE,
		}
		ctx, ready, err := newOtoContext(op)
		if err != nil {
			device.err = fmt.Errorf("open audio device (%d Hz, %d channels): %w", DeviceSampleRate, channels, err)
			return
		}
		<-ready
		// Most backends finish opening on their own goroutine and report a
		// failure there, with a nil error from NewContext: ALSA when no device
		// opens (a headless Linux box), CoreAudio when its queue will not
		// start. ready is closed either way, and the error is only in Err.
		// An error here is final: oto's write loop has stopped on it.
		if err := ctx.Err(); err != nil {
			device.err = fmt.Errorf("open audio device (%d Hz, %d channels): %w", DeviceSampleRate, channels, err)
			return
		}
		device.ctx, device.channels = ctx, channels
	})
	return device.ctx, device.channels, device.err
}

// NewOutput creates an audio output. channels is what the first Output in
// the process opens the device with (0 or less means stereo; which count to
// ask for is the caller's policy); every later Output plays at the channel
// count the device already has. The rate is
// always DeviceSampleRate. Callers have their decoder convert to SampleRate
// and Channels before writing.
//
// It returns an error when the device cannot be opened: no audio device, or
// one that refuses the format. That error is the first open's, returned to
// every later call too, because the device is opened only once.
func NewOutput(channels int) (*Output, error) {
	if channels <= 0 {
		channels = defaultChannels
	}
	ctx, got, err := openDevice(channels)
	if err != nil {
		return nil, err
	}
	return newOutput(ctx, got), nil
}

// NewDetachedOutput is an Output on no device, for tests above this package
// that need a pipeline with audio and no sound card. It plays nothing: Start
// does nothing, so its position never moves, and writes past its capacity
// overwrite the oldest samples, as on a device that has stalled.
func NewDetachedOutput(channels int) *Output {
	return newOutput(nil, channels)
}

// newOutput builds an Output playing PCM at DeviceSampleRate and channels on
// ctx.
//
// Everything that turns bytes into time (the playback position, the ring and
// pull sizes) uses that rate, which is the device's: a stream's own rate never
// reaches here, because its decoder converts it. Counting the position at the
// stream's rate is how a 48 kHz stream once played at 0.918x (#506).
func newOutput(ctx otoContext, channels int) *Output {
	const rate = DeviceSampleRate
	return &Output{
		ctx:        ctx,
		buffer:     NewRingBuffer(audioBytes(rate, channels, ringDuration)),
		sampleRate: rate,
		channels:   channels,
		volume:     1.0,
	}
}

// WriteSamples writes PCM audio data, at SampleRate and Channels, to the
// buffer for playback.
func (o *Output) WriteSamples(data []byte) {
	if o.volume < 1.0 {
		scaled := make([]byte, len(data))
		copy(scaled, data)
		o.applyVolume(scaled)
		o.buffer.Write(scaled)
	} else {
		o.buffer.Write(data)
	}
}

// Start begins audio playback.
func (o *Output) Start() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.started || o.ctx == nil {
		return
	}
	o.player = o.ctx.NewPlayer(o.buffer)
	o.player.SetBufferSize(playerBufferBytes(o.sampleRate, o.channels))
	o.player.Play()
	o.started = true
}

// Pause pauses audio playback.
// ringDuration is how much decoded audio the ring buffer holds, and the
// decoder stops at half of it (Pipeline.audioAheadFull). It is sized in time:
// a fixed 128 KB was 0.68 s of 48 kHz stereo, so audio could run at most 0.34 s
// ahead of playback, but FFmpeg's threaded video decoder hands frames out
// several frames after their packets, by more than that. The decoder sat on the
// audio limit with the newest video frame already late, every frame was shown
// late, and A/V sync averaged 25 ms with peaks of 100 ms (#504). With a second
// of audio allowed ahead, video is decoded before it is due.
const ringDuration = 2 * time.Second

// playerChunk is how much audio the device player pulls from the ring buffer
// at a time. It is a duration, not a byte count, because what has to cover it
// is the decoder's lead, and at 1080p60 the video queue stops the decoder about
// 270 ms ahead (#504). A fixed 32 KB was 170 ms of 48 kHz stereo but 370 ms of
// 44.1 kHz mono, longer than that lead, so every pull came up short and was
// padded with silence that the playback position does not count: the clock,
// and the video paced on it, ran at half speed. 60 ms was too small for the
// device and slowed the clock too; 100 to 200 ms all played at full speed.
const playerChunk = 150 * time.Millisecond

// audioBytes is d of 16-bit audio at rate and channels, in whole sample frames.
func audioBytes(rate, channels int, d time.Duration) int {
	frames := int(int64(rate) * int64(d) / int64(time.Second))
	return frames * channels * 2
}

// playerBufferBytes is playerChunk of 16-bit audio at rate and channels.
func playerBufferBytes(rate, channels int) int {
	return audioBytes(rate, channels, playerChunk)
}

func (o *Output) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.player != nil {
		o.player.Pause()
	}
}

// Resume resumes audio playback.
func (o *Output) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.player != nil {
		o.player.Play()
	}
}

// Stop stops audio playback.
func (o *Output) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.player != nil {
		o.player.Pause()
		o.player.Close()
		o.player = nil
	}
	o.buffer.Reset()
	o.started = false
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (o *Output) SetVolume(v float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.volume = math.Max(0, math.Min(1, v))
}

// Volume returns the current volume.
func (o *Output) Volume() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.volume
}

// PlaybackPosition returns the current audio playback position based on bytes
// consumed, minus what is still sitting unplayed in oto's internal buffer —
// otherwise the clock runs ahead of the actual sound by oto's buffering.
func (o *Output) PlaybackPosition() time.Duration {
	o.mu.Lock()
	posBase, readBase := o.posBase, o.readBase
	player := o.player
	o.mu.Unlock()
	bytesRead := o.buffer.TotalRead() - readBase
	if player != nil {
		bytesRead -= int64(player.BufferedSize())
	}
	if bytesRead < 0 {
		bytesRead = 0
	}
	bytesPerSample := 2 // 16-bit signed int
	bytesPerFrame := bytesPerSample * o.channels
	frames := bytesRead / int64(bytesPerFrame)
	return posBase + time.Duration(frames)*time.Second/time.Duration(o.sampleRate)
}

// SampleRate is the rate the Output plays at: DeviceSampleRate, which a
// stream's decoder converts to.
func (o *Output) SampleRate() int {
	return o.sampleRate
}

// Channels returns the configured channel count.
func (o *Output) Channels() int {
	return o.channels
}

func (o *Output) applyVolume(data []byte) {
	for i := 0; i < len(data)-1; i += 2 {
		sample := int16(data[i]) | int16(data[i+1])<<8
		sample = int16(float64(sample) * o.volume)
		data[i] = byte(sample)
		data[i+1] = byte(sample >> 8)
	}
}

// Buffered returns the number of PCM bytes currently waiting in the
// output buffer (decoded but not yet consumed by the audio device).
func (o *Output) Buffered() int {
	return o.buffer.Available()
}

// BufferCap returns the total capacity of the output buffer in bytes.
func (o *Output) BufferCap() int {
	return o.buffer.Cap()
}

// Rebase clears any buffered (not yet played) samples and rebases the
// playback position to pos. Used after seeks: PlaybackPosition is derived
// from total bytes consumed, which keeps counting across the seek.
func (o *Output) Rebase(pos time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buffer.Clear()
	o.posBase = pos
	o.readBase = o.buffer.TotalRead()
}
