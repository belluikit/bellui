package audio

import (
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	defaultSampleRate = 44100
	defaultChannels   = 2
	defaultBufferSize = 8192 * 16 // 128 KB: ~740 ms at 44.1 kHz stereo, ~1.5 s mono
)

// Output manages audio playback via ebitengine/oto.
type Output struct {
	mu         sync.Mutex
	ctx        *oto.Context
	player     *oto.Player
	buffer     *RingBuffer
	resampler  *Resampler
	sampleRate int
	channels   int
	volume     float64
	started    bool

	// Seek rebase: PlaybackPosition = posBase + time(totalRead - readBase).
	posBase  time.Duration
	readBase int64
}

var (
	otoOnce       sync.Once
	otoCtx        *oto.Context
	otoSampleRate int // rate the oto context was initialized with
	otoChannels   int
)

func initOto(sampleRate int, channels int) *oto.Context {
	otoOnce.Do(func() {
		otoSampleRate = sampleRate
		otoChannels = channels
		op := &oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: channels,
			Format:       oto.FormatSignedInt16LE,
		}
		var readyChan chan struct{}
		otoCtx, readyChan, _ = oto.NewContext(op)
		<-readyChan
	})
	return otoCtx
}

// NewOutput creates a new audio output.
func NewOutput(sampleRate, channels int) *Output {
	if sampleRate <= 0 {
		sampleRate = defaultSampleRate
	}
	if channels <= 0 {
		channels = defaultChannels
	}

	ctx := initOto(sampleRate, channels)
	buf := NewRingBuffer(defaultBufferSize)

	// oto's context is created once per process, so the first stream fixes the
	// device's channel count for every stream after it. Unlike a sample-rate
	// mismatch, which the resampler absorbs, feeding the device a different
	// number of interleaved channels would garble both the audio and the
	// byte-derived playback clock. Report what the device actually plays so
	// the caller can ask its decoder for exactly that many channels.
	if otoChannels > 0 {
		channels = otoChannels
	}

	o := &Output{
		ctx:        ctx,
		buffer:     buf,
		sampleRate: sampleRate,
		channels:   channels,
		volume:     1.0,
	}

	// Auto-configure resampling if oto was initialized at a different rate.
	if otoSampleRate != sampleRate && otoSampleRate > 0 {
		o.resampler = NewResampler(sampleRate, otoSampleRate, channels)
	}

	return o
}

// SetSourceSampleRate configures resampling if the source rate differs from output.
func (o *Output) SetSourceSampleRate(srcRate int) {
	if srcRate != o.sampleRate && srcRate > 0 {
		o.resampler = NewResampler(srcRate, o.sampleRate, o.channels)
	}
}

// WriteSamples writes PCM audio data to the buffer for playback.
// Automatically resamples if source and output rates differ.
func (o *Output) WriteSamples(data []byte) {
	if o.resampler != nil && o.resampler.NeedsResampling() {
		data = o.resampler.Resample(data)
	}
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
	if o.started {
		return
	}
	o.player = o.ctx.NewPlayer(o.buffer)
	o.player.SetBufferSize(playerBufferBytes(o.sampleRate, o.channels))
	o.player.Play()
	o.started = true
}

// Pause pauses audio playback.
// playerChunk is how much audio the device player pulls from the ring buffer
// at a time. It is a duration, not a byte count, because what has to cover it
// is the decoder's lead, and that is bounded in time: at 1080p60 the video queue
// stops the decoder about 270 ms ahead (#504). A fixed 32 KB was 170 ms of
// 48 kHz stereo but 370 ms of 44.1 kHz mono, longer than that lead, so every
// pull came up short and was padded with silence that the playback position
// does not count: the clock, and the video paced on it, ran at half speed.
// 60 ms was too small for the device and slowed the clock too; 100 to 200 ms
// all played at full speed.
const playerChunk = 150 * time.Millisecond

// playerBufferBytes is playerChunk of 16-bit audio at rate and channels.
func playerBufferBytes(rate, channels int) int {
	frames := int(int64(rate) * int64(playerChunk) / int64(time.Second))
	return frames * channels * 2
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

// SampleRate returns the configured sample rate.
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
