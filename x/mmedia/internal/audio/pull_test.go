package audio

import (
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
