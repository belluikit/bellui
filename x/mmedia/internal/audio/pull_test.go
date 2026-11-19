package audio

import (
	"encoding/binary"
	"testing"
	"time"
)

// fakePlayer is oto's player as the puller sees it: what it holds, set by the
// test, so a pull can be held between our returning it and oto adding it,
// where oto's goroutine can be preempted.
type fakePlayer struct{ held int }

func (f *fakePlayer) BufferedSize() int { return f.held }

// pullerOutput is a detached Output whose player is f, on a clock the test
// moves.
func pullerOutput(f *fakePlayer) (*Output, *time.Time) {
	o := NewDetachedOutput(2)
	now := time.Unix(0, 0)
	o.pull.now = func() time.Time { return now }
	o.pull.setPlayer(f)
	return o, &now
}

// chunk is one pull of the player, the size oto asks for.
var chunk = audioBytes(DeviceSampleRate, 2, playerChunk)

// TestAPullCountsOnceThePlayerHoldsIt pins #878: the ring counted a pull as
// read when it returned it, and oto adds it to what it holds only afterwards,
// so a position read in between counted the whole pull as played. That was
// 150 ms ahead, a flake of TestSeekDropsWhatThePlayerPulled on macos-15, and
// measured 30 times in 8 s of playback on a loaded Mac.
func TestAPullCountsOnceThePlayerHoldsIt(t *testing.T) {
	f := &fakePlayer{}
	o, now := pullerOutput(f)
	o.WriteSamples(tone(time.Second, 1000))

	buf := make([]byte, chunk)
	o.pull.Read(buf)
	f.held = chunk // oto added it
	if got := o.PlaybackPosition(); got != 0 {
		t.Fatalf("position %v with the first pull held and nothing played", got)
	}

	*now = now.Add(100 * time.Millisecond)
	f.held = chunk / 3 // played two thirds of it
	o.pull.Read(buf)   // and pulled the next, which oto has not added yet
	want := playerChunk * 2 / 3
	if got := o.PlaybackPosition(); got != want {
		t.Errorf("position %v between the pull and oto adding it; want %v, not a chunk more", got, want)
	}

	f.held += chunk // added
	if got := o.PlaybackPosition(); got != want {
		t.Errorf("position %v once the pull is added; want %v", got, want)
	}
	f.held -= chunk / 3
	if got := o.PlaybackPosition(); got != playerChunk {
		t.Errorf("position %v after playing the rest of the first pull; want %v", got, playerChunk)
	}
}

// TestALatePlayerStillCounts: oto pulls again once it holds less than a
// chunk, but loaded it can be late, playing a whole chunk past the pull
// before the next. Holding no more than at the pull then does not mean the
// pull is waiting to be added, and taking it that way read a chunk behind.
func TestALatePlayerStillCounts(t *testing.T) {
	f := &fakePlayer{}
	o, now := pullerOutput(f)
	o.WriteSamples(tone(time.Second, 1000))
	o.pull.Read(make([]byte, chunk))
	f.held = chunk - 1 // added, and a byte played
	o.pull.Read(make([]byte, chunk))
	f.held += chunk

	*now = now.Add(playerChunk + 10*time.Millisecond)
	f.held -= chunk + 1000 // a chunk and a little played, no new pull yet
	if got, want := o.PlaybackPosition(), o.bytesDuration(int64(chunk+1001)); got != want {
		t.Errorf("position %v; want %v, what the player played of the pull", got, want)
	}
}

// TestPaddingIsNotUnplayedAudio pins #882's other half: with the ring empty
// the player pads its pull with silence, and that silence was counted as
// audio still to play. At the end of a track the position fell back by up to
// a chunk, and the end was reported with the last 0.2 to 0.3 s still in the
// player, cut off by whatever came next.
func TestPaddingIsNotUnplayedAudio(t *testing.T) {
	f := &fakePlayer{}
	o, _ := pullerOutput(f)
	last := tone(10*time.Millisecond, 1000) // the end of the track
	o.WriteSamples(last)

	o.pull.Read(make([]byte, chunk)) // the last of it, then silence
	f.held = chunk
	if got := o.Unplayed(); got != len(last) {
		t.Errorf("unplayed %d bytes with the end of the track in the player; want %d", got, len(last))
	}
	if got := o.PlaybackPosition(); got != 0 {
		t.Errorf("position %v; want 0", got)
	}

	f.held -= len(last) // played the audio; the silence is left
	if got := o.Unplayed(); got != 0 {
		t.Errorf("unplayed %d bytes with only silence left in the player; want 0", got)
	}
	if got := o.PlaybackPosition(); got != 10*time.Millisecond {
		t.Errorf("position %v with the track played out; want its 10ms", got)
	}

	o.pull.Read(make([]byte, chunk)) // more silence
	f.held += chunk
	if got := o.PlaybackPosition(); got != 10*time.Millisecond {
		t.Errorf("position %v after a pull of silence; want it to stay at 10ms", got)
	}
}

// TestASeekCountsFromWhatItPulls: after a seek the position counts only what
// the player plays from the new position, whether it plays out what it held
// from before the seek or drops it.
func TestASeekCountsFromWhatItPulls(t *testing.T) {
	f := &fakePlayer{}
	o, _ := pullerOutput(f)
	o.WriteSamples(tone(time.Second, 1000))
	o.pull.Read(make([]byte, chunk))
	f.held = chunk

	o.Rebase(5 * time.Second)
	o.WriteSamples(tone(time.Second, 2000))
	if got := o.PlaybackPosition(); got != 5*time.Second {
		t.Errorf("position %v straight after the seek; want 5s", got)
	}
	f.held = chunk / 2 // half the old position's audio played
	if got := o.PlaybackPosition(); got != 5*time.Second {
		t.Errorf("position %v while the player plays what it held from before the seek; want 5s", got)
	}

	f.held = 0 // dropped the rest (flushPlayer)
	o.pull.Read(make([]byte, chunk))
	f.held = chunk
	f.held -= chunk / 3
	if got, want := o.PlaybackPosition(), 5*time.Second+playerChunk/3; got != want {
		t.Errorf("position %v; want %v, what it played since the seek", got, want)
	}
}

// TestASeekKeepsWhatThePlayerPulledBeforeTheFlush pins #1336: after a seek,
// oto pulls the new position's first samples from the ring as they arrive,
// and flushPlayer's Player.Seek then drops everything the player holds, those
// samples with it. They were never heard, but the puller counted them as
// played, so the new position started late and the position ran ahead by
// them: 64 ms on macos-15, the player holding nothing.
func TestASeekKeepsWhatThePlayerPulledBeforeTheFlush(t *testing.T) {
	c := newMixContext(1024, 0, time.Hour) // the test mixes, and fills the player
	defer c.close()
	o := newOutput(c, 2)
	defer o.Stop()
	o.WriteSamples(tone(time.Second, 1000)) // the old position
	o.Start()
	p := c.players[0]
	p.fill() // a chunk of the old position

	o.Rebase(5 * time.Second)
	for range 10 {
		c.mix() // the player plays out what it held, faded out, and pads with silence
	}
	o.WriteSamples(tone(50*time.Millisecond, 2000)) // less than a chunk: no flush yet
	for i := 0; o.Buffered() > 0; i++ {
		if i == 100 {
			t.Fatal("the player never pulled the first samples of the new position")
		}
		c.mix() // until oto has pulled them, as it does while the decoder catches up
	}
	o.WriteSamples(tone(time.Second, 3000)) // a chunk is waiting: flushPlayer
	p.fill()                                // the flush emptied the player: it refills

	p.mu.Lock()
	head := int16(binary.LittleEndian.Uint16(p.buf))
	p.mu.Unlock()
	if head != 2000 {
		t.Errorf("the player starts the new position with sample %d; want 2000, the first written after the seek", head)
	}
	if got := o.PlaybackPosition(); got != 5*time.Second {
		t.Errorf("position %v after the flush, nothing of the new position played; want 5s", got)
	}
}

// TestASeekNearTheEndPlaysTheTail pins #1340: after a seek within a chunk of
// the end, the ring never held the chunk flushPlayer waits for, so the player
// was never faded back in, and the rest of the track played at the volume
// Rebase faded it out to: silent. EndOfStream flushes with what there is.
func TestASeekNearTheEndPlaysTheTail(t *testing.T) {
	c := newMixContext(1024, 0, time.Hour) // the test mixes
	defer c.close()
	o := newOutput(c, 2)
	defer o.Stop()
	o.WriteSamples(tone(time.Second, 1000))
	o.Start()
	for range 5 {
		c.mix()
	}
	o.Rebase(5 * time.Second)
	for range 10 {
		c.mix() // the device plays the fade out, as Rebase waits for it to
	}
	const level = 16000
	tail := tone(100*time.Millisecond, level) // the last of the track: less than a chunk
	o.WriteSamples(tail)
	o.EndOfStream()

	before := len(c.played())
	for i := 0; o.Unplayed() > 0; i++ {
		if i == 100 {
			t.Fatal("the tail never played out")
		}
		c.mix()
	}
	out := c.played()[before:]
	var peak float32
	loud := 0
	for _, v := range out {
		peak = max(peak, abs32(v))
		if abs32(v) > level/32768.0/2 {
			loud++
		}
	}
	if want := float32(level) / 32768 * 0.95; peak < want {
		t.Errorf("the tail peaked at %.3f; want about %.3f, the level written", peak, float32(level)/32768)
	}
	// Faded in, so most of it, not all, is above half its level.
	frames := len(tail) / 4
	if loud < frames*3/4 {
		t.Errorf("%d of the tail's %d frames played above half its level; want most of them", loud, frames)
	}
	// The first edge is the tail coming in; the last is the track's own end,
	// which the test tone cuts at full level.
	steps := edges(out, float32(level)/32768)
	if len(steps) == 0 {
		t.Fatal("the tail never came in")
	}
	if steps[0] > 0.05 {
		t.Errorf("the tail comes in with a step of %.3f: a click, not a fade", steps[0])
	}
}
