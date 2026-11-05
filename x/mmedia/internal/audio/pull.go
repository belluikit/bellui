package audio

import (
	"sync"
	"sync/atomic"
	"time"
)

// heldCounter is what the puller asks of oto's player: how many bytes it
// holds, pulled and not yet played. An interface so a test can stand in a
// player and hold a pull where oto's goroutine can be preempted.
type heldCounter interface {
	BufferedSize() int
}

// puller is the reader oto's player pulls from. It reads the ring and records
// each pull, which is what lets the Output say how much has been played.
//
// The position used to be the ring's read count less what the player holds,
// and both halves were wrong at a pull (#878). oto releases its lock while it
// reads from us and adds the bytes to its buffer only after we return, so a
// position read in between counted a chunk the player did not yet hold:
// 150 ms ahead, measured 30 times in 8 s on a loaded Mac, and a test failure
// on macos-15. And when the ring runs dry it pads the pull with silence, which
// the player then holds as if it were audio still to play: at the end of a
// track the position fell back by up to a chunk, and the end was reported
// with 0.2 to 0.3 s of the track still in the player (#882).
//
// So each pull is recorded as how much audio it carried, at the front, and
// how long it was, padding included. Where the player has got to is the total
// pulled less what it holds; the record says how much of that was audio.
type puller struct {
	ring        *RingBuffer
	bytesPerSec int
	now         func() time.Time       // time.Now; a test's clock
	held        atomic.Pointer[player] // nil while there is no player

	mu     sync.Mutex
	seq    uint64 // pulls returned
	pulled int64  // bytes handed to the player, padding included
	audio  int64  // of those, audio from the ring
	recent [pullHistory]pull
	n      int // pulls in recent, newest at recent[(n-1)%pullHistory]

	// The newest pull, until the next: heldAtPull is what the player held
	// while we were filling it, and pulledAt when. Holding more than that,
	// the player has added it, since playing only takes from what it holds.
	// Holding no more, it has not, unless it has since played a whole pull's
	// worth: oto pulls again once it holds less than a chunk, but on a loaded
	// machine it can be late, and it was, playing 160 ms past a pull. So the
	// test is made only for half a pull's playing time after it; within that,
	// oto cannot have played a whole pull even with a device that takes 32 ms
	// at a time, as CoreAudio did. After it the pull counts as added: oto adds
	// it microseconds after we return unless its goroutine is held off for
	// longer than that half.
	pending    bool
	level      levels // the RMS of what is pulled, by window
	pendingLen int
	heldAtPull int
	pulledAt   time.Time
}

// pull is one read by the player: where in the pulled bytes it ended, and
// how many bytes of audio it began with; silence filled the rest.
type pull struct {
	end   int64
	audio int
}

// pullHistory is how many pulls are remembered. The player holds at most two
// chunks (it pulls one when it holds less than one), so the bytes it holds lie
// within the last three pulls; the rest is margin.
const pullHistory = 8

// newPuller reads ring, whose audio plays at bytesPerSec.
func newPuller(ring *RingBuffer, bytesPerSec int) *puller {
	return &puller{ring: ring, bytesPerSec: bytesPerSec, now: time.Now, level: newLevels(bytesPerSec)}
}

// Read is the player's pull. It must not take the Output's lock: oto calls it
// from inside Player.Seek, which flushPlayer makes holding that lock.
func (p *puller) Read(b []byte) (int, error) {
	held, holding := p.heldNow()

	p.mu.Lock()
	defer p.mu.Unlock()
	n := p.ring.read(b)
	p.level.add(p.audio, b[:n])
	p.pulled += int64(len(b))
	p.audio += int64(n)
	p.recent[p.n%pullHistory] = pull{end: p.pulled, audio: n}
	p.n++
	p.seq++
	// This pull is the one pending: the player added the last before it
	// asked for this. With no player, as when a test reads as the device
	// would, a pull is played as soon as it is taken.
	p.pending, p.pendingLen, p.heldAtPull, p.pulledAt = holding, len(b), held, p.now()
	return len(b), nil
}

// Seek is the ring's: Player.Seek, which empties the player, needs its source
// to be an io.Seeker.
func (p *puller) Seek(offset int64, whence int) (int64, error) {
	return p.ring.Seek(offset, whence)
}

// player is the player the puller asks.
type player struct{ heldCounter }

// heldNow is what the player holds, and whether there is one.
func (p *puller) heldNow() (int, bool) {
	if pl := p.held.Load(); pl != nil {
		return pl.BufferedSize(), true
	}
	return 0, false
}

// setPlayer starts asking h what it holds. Called before the player plays,
// so its first pull already sees it.
func (p *puller) setPlayer(h heldCounter) {
	p.held.Store(&player{h})
}

// played is how many bytes of audio the player has played, and how many it
// has pulled.
func (p *puller) played() (played, pulled int64) {
	for range 4 {
		p.mu.Lock()
		seq := p.seq
		p.mu.Unlock()
		held, _ := p.heldNow()
		p.mu.Lock()
		if p.seq != seq {
			p.mu.Unlock()
			continue // a pull returned meanwhile: held may or may not count it
		}
		played = p.playedLocked(held)
		pulled = p.audio
		p.mu.Unlock()
		return played, pulled
	}
	// The player is pulling as fast as it is asked: take it as holding what
	// it held at the last pull, and that pull. That is the position at the
	// pull, which can lag but not lead.
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playedLocked(p.heldAtPull + p.pendingLen), p.audio
}

func (p *puller) playedLocked(held int) int64 {
	added := p.pulled
	if p.pending && held <= p.heldAtPull && p.now().Sub(p.pulledAt) < p.duration(p.pendingLen)/2 {
		added -= int64(p.pendingLen) // returned to the player, not yet added
	}
	done := added - int64(held) // where in the pulled bytes playing has got to

	// Walk back from the newest pull, taking off its audio not yet played.
	played := p.audio
	for i := 0; i < min(p.n, pullHistory); i++ {
		pl := p.recent[(p.n-1-i)%pullHistory]
		if done >= pl.end {
			break
		}
		start := pl.end - int64(p.lenOf(i))
		into := max(0, done-start)
		played -= int64(pl.audio) - min(int64(pl.audio), into)
	}
	return max(0, played)
}

// duration is how long n bytes play for.
func (p *puller) duration(n int) time.Duration {
	return time.Duration(int64(n) * int64(time.Second) / int64(p.bytesPerSec))
}

// lenOf is the length of the i-th newest pull.
func (p *puller) lenOf(i int) int {
	pl := p.recent[(p.n-1-i)%pullHistory]
	if i+1 >= min(p.n, pullHistory) {
		if p.n <= pullHistory {
			return int(pl.end) // the first pull started at zero
		}
		return 0 // the one before it is forgotten: count the whole pull as unplayed
	}
	return int(pl.end - p.recent[(p.n-2-i)%pullHistory].end)
}

// reset forgets every pull, for a player that is gone.
func (p *puller) reset() {
	p.held.Store(nil)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	p.pulled, p.audio, p.n = 0, 0, 0
	p.pending = false
	p.level.reset()
}
