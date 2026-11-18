package audio

import (
	"encoding/binary"
	"io"
	"math"
	"slices"
	"sync"
	"testing"
	"time"
)

// mixContext is a device whose players the test mixes itself, the way oto's
// internal/mux does: a player pulls its source until it holds a buffer's
// worth, unlocked while it reads, and every block mixed takes its samples
// times a volume that moves from the last block's to the current one across
// the block (readBufferAndAdd); a block that mixed nothing does not move it.
// A paused player adds nothing to the mix.
//
// Mixed blocks wait in a queue, latency blocks long, before they are played,
// as in oto's fifo and the platform's buffer on Android. A suspended device
// plays nothing and keeps its queue, which it plays first on Resume, and Flush
// empties the queue. Each player counts what of its mix is still queued, which
// is what oto's UnplayedSize adds to what it holds.
//
// What the device plays is recorded, one value per frame, from the first
// channel; a suspended device plays silence.
type mixContext struct {
	frames   int  // per block
	latency  int  // blocks queued before one is played
	stallNew bool // new players start stalled (mixPlayer.stall)

	mu        sync.Mutex
	players   []*mixPlayer
	queue     []mixedBlock
	suspended bool
	out       []float32
	// suspends counts the suspends, and loudSuspends those that found sound
	// still queued, which would be cut off and played at the next Resume.
	suspends, loudSuspends, flushes int
	stop, done                      chan struct{}
}

// mixedBlock is a block mixed and not yet played, and how many of its frames
// came from each player.
type mixedBlock struct {
	samples []float32
	from    map[*mixPlayer]int
}

func newMixContext(blockFrames, latency int, period time.Duration) *mixContext {
	c := &mixContext{frames: blockFrames, latency: latency, stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(c.done)
		for {
			select {
			case <-c.stop:
				return
			case <-time.After(period):
			}
			c.mix()
		}
	}()
	return c
}

func (c *mixContext) close() { close(c.stop); <-c.done }

func (c *mixContext) NewPlayer(r io.Reader) devicePlayer {
	c.mu.Lock()
	p := &mixPlayer{src: r, stalled: c.stallNew}
	c.players = append(c.players, p)
	c.mu.Unlock()
	return p
}
func (c *mixContext) Err() error { return nil }

func (c *mixContext) Suspend() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.suspended = true
	c.suspends++
	for _, b := range c.queue {
		for _, v := range b.samples {
			if abs32(v) > 0.01 {
				c.loudSuspends++
				return nil
			}
		}
	}
	return nil
}

func (c *mixContext) Resume() error {
	c.mu.Lock()
	c.suspended = false
	c.mu.Unlock()
	return nil
}

func (c *mixContext) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushes++
	for _, b := range c.queue {
		b.played()
	}
	c.queue = nil
	return nil
}

// played tells b's players that its frames are played or discarded.
func (b mixedBlock) played() {
	for p, n := range b.from {
		p.mu.Lock()
		p.queued -= n
		p.mu.Unlock()
	}
}

// mix plays one block's time: it mixes a block into the queue and plays the
// oldest once the queue is full, or plays silence while suspended.
func (c *mixContext) mix() {
	c.mu.Lock()
	if c.suspended {
		c.out = append(c.out, make([]float32, c.frames)...)
		c.mu.Unlock()
		return
	}
	players := slices.Clone(c.players)
	c.mu.Unlock()

	b := mixedBlock{samples: make([]float32, c.frames), from: map[*mixPlayer]int{}}
	for _, p := range players {
		p.fill()
		if n := p.mixInto(b.samples); n > 0 {
			b.from[p] = n
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue = append(c.queue, b)
	if len(c.queue) <= c.latency {
		c.out = append(c.out, make([]float32, c.frames)...)
		return
	}
	first := c.queue[0]
	c.queue = c.queue[1:]
	first.played()
	c.out = append(c.out, first.samples...)
}

func (c *mixContext) played() []float32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]float32(nil), c.out...)
}

// last is the last frame played, or 0 before any.
func (c *mixContext) last() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.out) == 0 {
		return 0
	}
	return c.out[len(c.out)-1]
}

// blocks counts the blocks the device has played, which is one per mix.
func (c *mixContext) blocks() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.out) / c.frames
}

// waitBlocks waits until the device has mixed n more blocks, rather than for a
// time a loaded machine can overrun (#1320).
func (c *mixContext) waitBlocks(t *testing.T, what string, n int) {
	t.Helper()
	want := c.blocks() + n
	waitFor(t, what, 5*time.Second, func() bool { return c.blocks() >= want })
}

// waitHeard waits until the device plays sound of sign's sign above half of
// level.
func (c *mixContext) waitHeard(t *testing.T, what string, level, sign float32) {
	t.Helper()
	waitFor(t, what, 5*time.Second, func() bool { return c.last()*sign > level/2 })
}

type mixPlayer struct {
	src io.Reader

	mu               sync.Mutex
	buf              []byte
	size             int
	queued           int // frames mixed and still queued in the device
	volume, prev     float64
	playing, closed  bool
	reading, discard bool
	stalled          bool // the source is not read: oto's reader held off
}

func (p *mixPlayer) fill() {
	p.mu.Lock()
	if p.closed || p.reading || p.stalled || len(p.buf) >= p.size {
		p.mu.Unlock()
		return
	}
	p.reading, p.discard = true, false
	n := p.size
	p.mu.Unlock()

	b := make([]byte, n)
	_, _ = p.src.Read(b) // unlocked, as oto reads

	p.mu.Lock()
	if !p.discard {
		p.buf = append(p.buf, b...)
	}
	p.reading = false
	p.mu.Unlock()
}

// mixInto adds the player's next block to block, and reports how many frames
// it added.
func (p *mixPlayer) mixInto(block []float32) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing || p.closed {
		return 0
	}
	n := min(len(p.buf)/4, len(block)) // 16-bit stereo frames
	for i := range n {
		v := float64(int16(binary.LittleEndian.Uint16(p.buf[4*i:]))) / (1 << 15)
		g := p.volume
		if p.volume != p.prev {
			r := float64(i) / float64(n)
			g = p.volume*r + p.prev*(1-r)
		}
		block[i] += float32(v * g)
	}
	// A block that mixed nothing leaves the ramp for the next, as oto has
	// since belluikit/oto#2 (#1273).
	if n > 0 {
		p.prev = p.volume
	}
	p.buf = p.buf[4*n:]
	p.queued += n
	return n
}

func (p *mixPlayer) Play()  { p.mu.Lock(); p.playing = true; p.mu.Unlock() }
func (p *mixPlayer) Pause() { p.mu.Lock(); p.playing = false; p.mu.Unlock() }
func (p *mixPlayer) Close() error {
	p.mu.Lock()
	p.closed, p.playing = true, false
	p.mu.Unlock()
	return nil
}

func (p *mixPlayer) Seek(offset int64, whence int) (int64, error) {
	p.mu.Lock()
	p.buf, p.discard = p.buf[:0], true
	p.mu.Unlock()
	return p.src.(io.Seeker).Seek(offset, whence)
}

// stall stops the player reading its source, or lets it read again: oto's
// reader goroutine held off while the device goes on mixing what the player
// holds.
func (p *mixPlayer) stall(on bool) { p.mu.Lock(); p.stalled = on; p.mu.Unlock() }

func (p *mixPlayer) SetBufferSize(n int) { p.mu.Lock(); p.size = n; p.mu.Unlock() }
func (p *mixPlayer) BufferedSize() int   { p.mu.Lock(); defer p.mu.Unlock(); return len(p.buf) }
func (p *mixPlayer) UnplayedSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.buf) + 4*p.queued
}
func (p *mixPlayer) Volume() float64 { p.mu.Lock(); defer p.mu.Unlock(); return p.volume }
func (p *mixPlayer) IsPlaying() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.playing }
func (p *mixPlayer) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = v
	if !p.playing {
		p.prev = v
	}
}

// edges finds where out moves between loud (above half of level, of either
// sign) and silent, or from one sign of loud to the other, and returns, for
// each, the largest step from one frame to the next within it. A cut is one
// step of the whole level, or twice it; a fade is many small ones.
func edges(out []float32, level float32) (steps []float32) {
	const quiet = 0.01
	last, lastAt := 0, -1 // +1 or -1 loud, 0 silent; where it last was
	for i, v := range out {
		var now int
		switch {
		case v > level/2:
			now = 1
		case v < -level/2:
			now = -1
		case abs32(v) < quiet:
			now = 0
		default:
			continue
		}
		if lastAt >= 0 && now != last {
			var step float32
			for j := lastAt + 1; j <= i; j++ {
				step = max(step, abs32(out[j]-out[j-1]))
			}
			steps = append(steps, step)
		}
		last, lastAt = now, i
	}
	return steps
}

func abs32(v float32) float32 { return float32(math.Abs(float64(v))) }

// TestEdgesFade pins #1228: a stop, pause, resume, seek or start cut the
// sound at full level, and changing songs clicked on a Pixel 9a. Each edge
// now fades over fadeTime or a device block, whichever is longer, and the
// device plays the fade before it is suspended. It plays a constant level, so
// the edges are the only steps, through a device with Android's small blocks
// and with macOS's 32 ms ones, each with a queue of three blocks that a
// suspend keeps, as Android's does.
func TestEdgesFade(t *testing.T) {
	suspendingWhenIdle(t, true)
	const level = 16000
	full := float32(level) / (1 << 15)
	for _, dev := range []struct {
		name   string
		frames int
		period time.Duration
	}{
		{"small blocks", 96, 2 * time.Millisecond},
		{"macOS blocks", 1536, 32 * time.Millisecond},
	} {
		t.Run(dev.name, func(t *testing.T) {
			c := newMixContext(dev.frames, 3, dev.period)
			o := newOutput(c, 2)
			o.idle = &idleSuspend{dev: c}
			// Each step waits for what the next one needs, not for a fixed
			// time, which a loaded machine overran (#1273): an up waits for
			// the full level to be played, so the down after it fades from
			// there; a pause or a stop waits for the device to be suspended,
			// which is after it has played the fade out; and the seek waits
			// for silence before the new position, as a network seek has.
			heard := func(what string, sign float32) { c.waitHeard(t, what, full, sign) }
			quiet := func(what string) {
				waitFor(t, what, 5*time.Second, func() bool { return abs32(c.last()) < 0.01 })
			}
			suspended := func(what string, n int) {
				waitFor(t, what, 5*time.Second, func() bool {
					c.mu.Lock()
					defer c.mu.Unlock()
					return c.suspends >= n
				})
			}

			o.WriteSamples(tone(time.Second, level))
			o.Start() // up
			heard("the start", 1)
			o.Pause() // down, then the device is suspended
			suspended("the pause to suspend the device", 1)
			o.Resume() // up
			heard("the resume", 1)
			o.Rebase(time.Second) // down
			quiet("the seek's fade out")
			o.WriteSamples(tone(time.Second, -level)) // up, at the new position
			heard("the new position", -1)
			o.Stop() // down, then the device is suspended
			suspended("the stop to suspend the device", 2)
			o.WriteSamples(tone(time.Second, level))
			o.Start() // up: the next song
			heard("the next song", 1)
			o.Stop() // down, then the device is suspended
			suspended("the last stop to suspend the device", 3)
			c.close()

			steps := edges(c.played(), full)
			for i, s := range steps {
				// A fade of the full level over 8 ms of 48 kHz moves it
				// by 1/384 of it a frame; a cut moves it all at once.
				if s > full/50 {
					t.Errorf("edge %d steps by %.4f in one frame (%.0f%% of the level); want a fade", i, s, 100*s/full)
				}
			}
			// The positive markers: every edge played was found and checked,
			// and the device was suspended after the pause and both stops.
			if len(steps) != 8 {
				t.Errorf("found %d edges in what was played; want 8 (start, pause, resume, seek out, seek in, stop, start, stop)", len(steps))
			}
			c.mu.Lock()
			suspends, loud, flushes := c.suspends, c.loudSuspends, c.flushes
			c.mu.Unlock()
			if suspends != 3 {
				t.Errorf("the device was suspended %d times; want 3 (pause, stop, stop)", suspends)
			}
			if loud != 0 {
				t.Errorf("%d of %d suspends found sound still queued, to be cut off and played at the next Resume", loud, suspends)
			}
			if flushes != suspends {
				t.Errorf("%d flushes for %d suspends; want one after each", flushes, suspends)
			}
			t.Logf("%d edges, largest steps %v; %d suspends", len(steps), steps, suspends)
		})
	}
}

// TestSeekFadesInAfterAStalledRefill pins #1273: after a seek empties the
// player, the device can mix blocks before oto's reader refills it. A block
// mixed from nothing still moved the player's volume on, so the fade-in was
// spent on silence and the new position started at full level.
func TestSeekFadesInAfterAStalledRefill(t *testing.T) {
	const level = 16000
	full := float32(level) / (1 << 15)
	c := newMixContext(96, 3, 2*time.Millisecond)
	defer c.close()
	o := newOutput(c, 2)
	o.WriteSamples(tone(time.Second, level))
	o.Start()
	c.waitHeard(t, "the start", full, 1)
	o.Rebase(time.Second)
	p := o.player.(*mixPlayer)
	p.stall(true)
	o.WriteSamples(tone(time.Second, -level))               // the seek and its fade-in
	c.waitBlocks(t, "blocks mixed from an empty player", 3) // what used to spend the fade
	p.stall(false)
	c.waitHeard(t, "the new position", full, -1)

	steps := edges(c.played(), full)
	if len(steps) != 3 {
		t.Fatalf("found %d edges, want 3 (start, seek out, seek in): %v", len(steps), steps)
	}
	if s := steps[2]; s > full/50 {
		t.Errorf("the seek in steps by %.4f in one frame (%.0f%% of the level); want a fade", s, 100*s/full)
	}
}

// TestStartFadesInAfterAStalledFill is the same for a new player: the device
// can mix it before oto's reader has filled it at all.
func TestStartFadesInAfterAStalledFill(t *testing.T) {
	const level = 16000
	full := float32(level) / (1 << 15)
	c := newMixContext(96, 3, 2*time.Millisecond)
	defer c.close()
	c.stallNew = true
	o := newOutput(c, 2)
	o.WriteSamples(tone(time.Second, level))
	o.Start()
	c.waitBlocks(t, "blocks mixed from an empty player", 3)
	o.player.(*mixPlayer).stall(false)
	c.waitHeard(t, "the start", full, 1)

	steps := edges(c.played(), full)
	if len(steps) != 1 {
		t.Fatalf("found %d edges, want 1 (start): %v", len(steps), steps)
	}
	if s := steps[0]; s > full/50 {
		t.Errorf("the start steps by %.4f in one frame (%.0f%% of the level); want a fade", s, 100*s/full)
	}
}
