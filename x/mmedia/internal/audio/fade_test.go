package audio

import (
	"encoding/binary"
	"io"
	"math"
	"sync"
	"testing"
	"time"
)

// mixContext is a device whose one player the test mixes itself, the way
// oto v3.5.1's internal/mux does: the player pulls its source until it holds
// a buffer's worth, unlocked while it reads, and every block mixed takes its
// samples times a volume that moves from the last block's to the current one
// across the block (readBufferAndAdd). A paused player adds nothing to the
// mix. What the device would play is recorded, one value per frame, from the
// first channel.
type mixContext struct {
	frames int // per block

	mu     sync.Mutex
	player *mixPlayer
	out    []float32
	stop   chan struct{}
	done   chan struct{}
}

func newMixContext(blockFrames int, period time.Duration) *mixContext {
	c := &mixContext{frames: blockFrames, stop: make(chan struct{}), done: make(chan struct{})}
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
	p := &mixPlayer{src: r}
	c.mu.Lock()
	c.player = p
	c.mu.Unlock()
	return p
}
func (c *mixContext) Err() error     { return nil }
func (c *mixContext) Suspend() error { return nil }
func (c *mixContext) Resume() error  { return nil }

// mix plays one block.
func (c *mixContext) mix() {
	c.mu.Lock()
	p := c.player
	c.mu.Unlock()
	block := make([]float32, c.frames)
	if p != nil {
		p.fill()
		p.mixInto(block)
	}
	c.mu.Lock()
	c.out = append(c.out, block...)
	c.mu.Unlock()
}

func (c *mixContext) played() []float32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]float32(nil), c.out...)
}

type mixPlayer struct {
	src io.Reader

	mu               sync.Mutex
	buf              []byte
	size             int
	volume, prev     float64
	playing, closed  bool
	reading, discard bool
}

func (p *mixPlayer) fill() {
	p.mu.Lock()
	if p.closed || p.reading || len(p.buf) >= p.size {
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

func (p *mixPlayer) mixInto(block []float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing || p.closed {
		return
	}
	n := min(len(p.buf)/4, len(block)) // 16-bit stereo frames
	for i := range n {
		v := float64(int16(binary.LittleEndian.Uint16(p.buf[4*i:]))) / (1 << 15)
		g := p.volume
		if p.volume != p.prev {
			r := float64(i) / float64(n)
			g = p.volume*r + p.prev*(1-r)
		}
		block[i] = float32(v * g)
	}
	p.prev = p.volume
	p.buf = p.buf[4*n:]
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
func (p *mixPlayer) SetBufferSize(n int) { p.mu.Lock(); p.size = n; p.mu.Unlock() }
func (p *mixPlayer) BufferedSize() int   { p.mu.Lock(); defer p.mu.Unlock(); return len(p.buf) }
func (p *mixPlayer) Volume() float64     { p.mu.Lock(); defer p.mu.Unlock(); return p.volume }
func (p *mixPlayer) IsPlaying() bool     { p.mu.Lock(); defer p.mu.Unlock(); return p.playing }
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
// now fades over fadeTime or a device block, whichever is longer. It plays a
// constant level, so the edges are the only steps, through a device with
// Android's small blocks and with macOS's 32 ms ones.
func TestEdgesFade(t *testing.T) {
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
			c := newMixContext(dev.frames, dev.period)
			o := newOutput(c, 2)
			pause := func() { time.Sleep(10 * dev.period) }

			o.WriteSamples(tone(time.Second, level))
			o.Start() // up
			pause()
			o.Pause() // down
			pause()
			o.Resume() // up
			pause()
			o.Rebase(time.Second)                     // down
			o.WriteSamples(tone(time.Second, -level)) // up, at the new position
			pause()
			o.Stop() // down
			o.WriteSamples(tone(time.Second, level))
			o.Start() // up: the next song
			pause()
			o.Stop() // down
			pause()
			c.close()

			steps := edges(c.played(), full)
			for i, s := range steps {
				// A fade of the full level over 8 ms of 48 kHz moves it
				// by 1/384 of it a frame; a cut moves it all at once.
				if s > full/50 {
					t.Errorf("edge %d steps by %.4f in one frame (%.0f%% of the level); want a fade", i, s, 100*s/full)
				}
			}
			// The positive marker: every edge played was found and checked.
			if len(steps) != 8 {
				t.Errorf("found %d edges in what was played; want 8 (start, pause, resume, seek out, seek in, stop, start, stop)", len(steps))
			}
			t.Logf("%d edges, largest steps %v", len(steps), steps)
		})
	}
}
