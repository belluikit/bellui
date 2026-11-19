package audio

import (
	"fmt"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/oto/v3"
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
	player     devicePlayer
	buffer     *RingBuffer
	pull       *puller // what the player reads the buffer through
	sampleRate int
	channels   int
	volume     float64
	started    bool
	playing    bool // started, and not paused or stopped since

	// settling counts the pauses and stops whose sound the device is still
	// playing (settle): a Start or Resume moves it on, so a settle that
	// finishes after one does not count the Output as idle.
	settling uint64

	// idle is the device's count of playing Outputs (nil for a detached
	// Output), and active whether this one is counted in it.
	idle   *idleSuspend
	active bool

	// Seek rebase: PlaybackPosition = posBase + time(played - playedBase),
	// where played counts the audio the player has played (puller).
	posBase    time.Duration
	playedBase int64

	// flushPending is set by Rebase: oto's player still holds what it pulled
	// before the seek, and is emptied once the new position's samples fill a
	// chunk (#855); see flushPlayer.
	flushPending atomic.Bool
}

// device is the process's one audio device. It is opened by the first
// NewOutput and never again: oto allows a single context per process.
var device struct {
	once     sync.Once
	ctx      otoContext
	channels int   // channel count the device was opened with
	err      error // why it could not be opened, for every later NewOutput
	idle     idleSuspend
}

// otoContext is what an Output uses of *oto.Context. It is an interface so a
// test can stand in a context that opened and then failed, the way oto reports
// a missing device on Linux, which a real *oto.Context cannot be made to do,
// and one whose players a test mixes itself.
type otoContext interface {
	NewPlayer(r io.Reader) devicePlayer
	Err() error
	Suspend() error
	Resume() error
	Flush() error
}

// devicePlayer is what an Output uses of *oto.Player.
type devicePlayer interface {
	Play()
	Pause()
	Close() error
	Seek(offset int64, whence int) (int64, error)
	SetBufferSize(bytes int)
	BufferedSize() int
	UnplayedSize() int
	SetVolume(v float64)
	Volume() float64
	IsPlaying() bool
}

// otoDevice is the real context, its players as devicePlayers.
type otoDevice struct{ *oto.Context }

func (d otoDevice) NewPlayer(r io.Reader) devicePlayer { return d.Context.NewPlayer(r) }

// newOtoContext is oto.NewContext, a variable so a test can make the device
// refuse to open.
var newOtoContext = func(op *oto.NewContextOptions) (otoContext, chan struct{}, error) {
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, nil, err // not a nil *oto.Context in a non-nil interface
	}
	return otoDevice{ctx}, ready, nil
}

// openDevice opens the audio device once per process and returns it, or the
// error it failed with.
//
// A failure is remembered, not retried. oto marks its one context as created
// before the platform backend runs and never clears the mark (context.go in
// v3.5.1: contextCreated = true precedes newContext), so a second
// oto.NewContext can only fail with "context is already created", which would
// hide the real cause. Returning the first error to every later caller is the
// only answer that is both true and does not block. It used to discard the
// error and receive on the nil ready channel oto returns with it, which blocked
// forever inside the Once and so blocked every later player too (#690).
func openDevice(channels int) (otoContext, int, error) {
	device.once.Do(func() {
		setupSession()
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
		device.idle.dev = ctx
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
	o := newOutput(ctx, got)
	o.idle = &device.idle
	return o, nil
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
	buffer := NewRingBuffer(audioBytes(rate, channels, ringDuration))
	return &Output{
		ctx:        ctx,
		buffer:     buffer,
		pull:       newPuller(buffer, rate*channels*2),
		sampleRate: rate,
		channels:   channels,
		volume:     1.0,
	}
}

// WriteSamples writes PCM audio data, at SampleRate and Channels, to the
// buffer for playback. It is written as decoded: the volume is applied as it
// plays (SetVolume).
func (o *Output) WriteSamples(data []byte) {
	o.buffer.Write(data)
	if o.flushPending.Load() {
		o.flushPlayer()
	}
}

// flushPlayer empties oto's player after a seek once the ring holds a chunk
// from the new position, and fades the new position in (Rebase faded the old
// one out). oto's player holds up to playerChunk it pulled from
// the ring before the seek, and a seek played that much of the old position
// first: 162 ms on macOS (#855). Player.Seek drops it and refills from the
// ring. Doing it at once, at Rebase, refilled the player from the cleared
// ring, which pads with silence (oto spins on a source that returns nothing),
// so the old audio was traded for a gap; with a chunk waiting, it refills
// with the new position's audio. oto pulls even while paused, so a paused
// player is emptied the same way and stays paused.
//
// The player may already hold some of the new position, pulled as it arrived
// in pieces smaller than a chunk. Emptying it dropped that too, unheard but
// counted as played (#1336), so puller.Seek puts it back in the ring.
func (o *Output) flushPlayer() {
	if o.buffer.Available() < playerBufferBytes(o.sampleRate, o.channels) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.flushPending.Swap(false) && o.player != nil {
		_, _ = o.player.Seek(0, io.SeekCurrent)
		// What it pulled from the new position went back into the ring, to be
		// pulled and counted again (puller.Seek).
		o.playedBase += o.pull.takeRestored()
		if o.playing {
			o.fade(o.volume)
		}
	}
}

// fadeTime is how long the sound takes to fade in or out at a start, stop,
// pause, resume or seek. Cut at full level, the wave jumps to or from zero in
// one sample, which is heard as a click; changing songs clicked (#1228). A
// few milliseconds is too short to hear as a fade.
const fadeTime = 8 * time.Millisecond

// fadeWait bounds how long fadeOut waits for the device to play the faded
// audio, for a device that has stopped asking for any.
const fadeWait = 100 * time.Millisecond

// fade moves the player's volume to v over fadeTime. oto applies a volume
// change gradually, across the next block it mixes (readBufferAndAdd in
// oto v3.5.1's internal/mux), so the steps here join into a ramp whose
// length is fadeTime or one block, whichever is longer: a block is 32 ms on
// macOS, and a few on Android. A block that mixes nothing, as while the
// player is empty after a Seek or a Play, leaves the ramp for the samples
// that come next (belluikit/oto#2): it used to be spent on silence, and the
// new position started at full level (#1273). Called under o.mu.
func (o *Output) fade(v float64) {
	from := o.player.Volume()
	for start := time.Now(); ; time.Sleep(time.Millisecond) {
		f := float64(time.Since(start)) / float64(fadeTime)
		if f >= 1 {
			break
		}
		o.player.SetVolume(from + (v-from)*f)
	}
	o.player.SetVolume(v)
}

// fadeOut fades the player to silence and waits until the device has mixed a
// block at that volume, so a pause or a close that follows cuts silence.
// Called under o.mu, with the player playing.
func (o *Output) fadeOut() {
	o.fade(0)
	mark := o.pull.consumed()
	for end := time.Now().Add(fadeWait); o.pull.consumed() == mark && time.Now().Before(end); {
		time.Sleep(time.Millisecond)
	}
}

// Start begins audio playback.
func (o *Output) Start() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.started || o.ctx == nil {
		return
	}
	o.player = o.ctx.NewPlayer(o.pull)
	o.player.SetBufferSize(playerBufferBytes(o.sampleRate, o.channels))
	o.player.SetVolume(0) // faded in once playing
	o.pull.setPlayer(o.player)
	o.settling++ // a stop still being played out no longer makes this idle
	o.setActive(true)
	o.player.Play()
	o.started, o.playing = true, true
	o.fade(o.volume)
}

// setActive counts the Output in the device's playing Outputs, or takes it
// out, so the device is suspended while none plays (idleSuspend). It is
// called under o.mu, before a player plays, and once the device has played
// what a paused or stopped one mixed (settle).
func (o *Output) setActive(on bool) {
	if o.idle == nil || o.active == on {
		return
	}
	o.active = on
	if on {
		o.idle.play()
	} else {
		o.idle.idle()
	}
}

// Playing reports whether the Output is playing: started, and neither paused
// nor stopped since. A detached Output never plays. The device counts it as
// playing a little longer, until it has played what was mixed (settle).
func (o *Output) Playing() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.playing && o.ctx != nil
}

// settleWait bounds how long settle waits for the device to play what a
// paused or stopped player mixed. Its queue is tens of milliseconds on a
// phone's speaker and can be hundreds over Bluetooth.
const settleWait = time.Second

// settle counts the Output as idle once the device has played what p mixed,
// the faded end of it, and closes p then when close is set. Until then the
// device keeps running and the fade is heard. Counted idle at once, the last
// Output let Android suspend the device with that sound still queued: the
// fade was cut off, which clicked, and Resume played the rest at the start of
// the next song (#1228). oto counts the sound queued for the device in
// UnplayedSize; where the platform does not report it, UnplayedSize is what
// the player holds, and the Output is idle at once, as before. Called under
// o.mu, after p.Pause.
func (o *Output) settle(p devicePlayer, close bool) {
	o.settling++
	gen := o.settling
	go func() {
		for end := time.Now().Add(settleWait); p.UnplayedSize() > p.BufferedSize() && time.Now().Before(end); {
			time.Sleep(5 * time.Millisecond)
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		if close {
			_ = p.Close()
		}
		if o.settling == gen {
			o.setActive(false)
		}
	}()
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

// Pause pauses audio playback, fading it out first (fadeOut): it returns up to
// fadeTime and a device block later. The device plays the fade out after it
// returns (settle).
func (o *Output) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.player != nil {
		if o.playing {
			o.fadeOut()
		}
		o.player.Pause()
		o.playing = false
		o.settle(o.player, false)
	}
}

// Resume resumes audio playback, fading it in.
func (o *Output) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.player != nil && !o.playing {
		o.player.SetVolume(0) // while paused, so oto starts from silence
		o.settling++          // the pause's sound no longer makes this idle
		o.setActive(true)
		o.player.Play()
		o.playing = true
		if !o.flushPending.Load() { // a seek's flush fades in
			o.fade(o.volume)
		}
	}
}

// Stop stops audio playback, fading it out first (fadeOut): it returns up to
// fadeTime and a device block later. The device plays the fade out after it
// returns, and the player is closed then (settle).
func (o *Output) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.flushPending.Store(false)
	if o.player != nil {
		if o.playing {
			o.fadeOut()
		}
		o.player.Pause()
		o.settle(o.player, true)
		o.player = nil
	} else {
		o.setActive(false)
	}
	o.playing = false
	o.buffer.Reset()
	o.pull.reset()
	// The resets zero the byte counts, so the last seek's base goes with
	// them: kept, the next playback read as that seek's target until as many
	// bytes had been played again as before it (#833).
	o.posBase, o.playedBase = 0, 0
	o.started = false
}

// SetVolume sets the playback volume (0.0 to 1.0). It is the player's volume,
// which oto applies as it mixes, so a change is heard within the device's own
// buffer. It used to scale samples as they were written to the ring, so the
// decoder's lead, a second or more, played at the old volume first: a mute
// took 1.2 s to be heard (#1032).
func (o *Output) SetVolume(v float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.volume = math.Max(0, math.Min(1, v))
	if o.player != nil && !o.flushPending.Load() { // a seek's flush fades in to it
		o.player.SetVolume(o.volume)
	}
}

// Volume returns the current volume.
func (o *Output) Volume() float64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.volume
}

// PlaybackPosition returns the current audio playback position: the audio
// the player has played, counted by the puller, so neither what it holds
// unplayed nor the silence it pads an empty buffer with reads as played.
func (o *Output) PlaybackPosition() time.Duration {
	o.mu.Lock()
	posBase, playedBase := o.posBase, o.playedBase
	o.mu.Unlock()
	played, _ := o.pull.played()
	return posBase + o.bytesDuration(max(0, played-playedBase))
}

// Unplayed is how much audio, in bytes, is still to be played: in the buffer,
// or pulled by the player and not yet played. It is zero once the last of
// what was written has played, which is the end of a track; the buffer alone
// empties up to two of the player's chunks before that (#882).
func (o *Output) Unplayed() int {
	played, pulled := o.pull.played()
	return o.buffer.Available() + int(pulled-played)
}

// bytesDuration is n bytes of audio at the Output's format, in whole sample
// frames.
func (o *Output) bytesDuration(n int64) time.Duration {
	frames := n / int64(2*o.channels) // 16-bit samples
	return time.Duration(frames) * time.Second / time.Duration(o.sampleRate)
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
//
// A playing player is faded out first and stays silent until flushPlayer
// fades the new position in. Cleared, the ring pads the player with silence
// once it has played what it holds, and a network seek can take seconds, so
// the old audio would otherwise stop dead there (#1228).
func (o *Output) Rebase(pos time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.player != nil && o.playing && !o.flushPending.Load() {
		o.fadeOut()
	}
	o.buffer.Clear()
	o.flushPending.Store(o.player != nil) // see flushPlayer
	o.posBase = pos
	_, o.playedBase = o.pull.played() // all of it, once played or flushed, is before pos
}

// Level is the RMS, 0 to 1, of the audio playing now: the 20 ms window at the
// playback position, with the volume applied. It is what is heard, not what
// was last handed to the device, which runs a pull ahead. The volume is the
// current one, as oto applies it to what the player holds as well (SetVolume).
func (o *Output) Level() float32 {
	volume := float32(o.Volume())
	played, _ := o.pull.played()
	o.pull.mu.Lock()
	defer o.pull.mu.Unlock()
	return o.pull.level.at(played) * volume
}
