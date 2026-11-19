package audio

import (
	"math"
	"testing"
	"time"
)

// square is d of a stereo square wave at amp, 16-bit, at the device rate:
// its RMS is amp/32768 exactly.
func square(d time.Duration, amp int16) []byte {
	frames := int(d.Seconds() * DeviceSampleRate)
	b := make([]byte, 0, frames*4)
	for i := 0; i < frames; i++ {
		s := amp
		if i%2 == 1 {
			s = -amp
		}
		for c := 0; c < 2; c++ {
			b = append(b, byte(s), byte(uint16(s)>>8))
		}
	}
	return b
}

func near(t *testing.T, what string, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > 0.002 {
		t.Errorf("%s: level %.4f, want %.4f", what, got, want)
	}
}

// TestLevelIsTheRMSOfWhatIsPlayed pins player#258's level: the RMS of the
// audio, 0 to 1, with the volume applied, and 0 once stopped.
func TestLevelIsTheRMSOfWhatIsPlayed(t *testing.T) {
	o := NewDetachedOutput(2) // no player: a pull is played when taken
	if got := o.Level(); got != 0 {
		t.Fatalf("level %v before any audio", got)
	}
	o.WriteSamples(square(200*time.Millisecond, 16384))
	o.pull.Read(make([]byte, chunk))
	near(t, "half-scale square wave", o.Level(), 0.5)

	o.Stop()
	if got := o.Level(); got != 0 {
		t.Errorf("level %v after Stop", got)
	}

	o.SetVolume(0.5)
	o.WriteSamples(square(200*time.Millisecond, 16384))
	o.pull.Read(make([]byte, chunk))
	near(t, "half-scale square wave at volume 0.5", o.Level(), 0.25)
}

// TestLevelFollowsWhatIsHeardNotWhatIsPulled pins that the level is the
// window at the playback position. The player pulls a chunk (150 ms) or more
// ahead of what it plays, so the level of the last pull leads the sound.
func TestLevelFollowsWhatIsHeardNotWhatIsPulled(t *testing.T) {
	f := &fakePlayer{}
	o, now := pullerOutput(f)
	o.WriteSamples(square(playerChunk, 16384)) // loud
	o.WriteSamples(make([]byte, chunk))        // then silence

	buf := make([]byte, chunk)
	o.pull.Read(buf) // the loud chunk
	f.held = chunk
	*now = now.Add(playerChunk)
	o.pull.Read(buf) // the silent one, pulled while the loud one plays
	f.held = chunk + chunk/2
	*now = now.Add(playerChunk)
	near(t, "half the loud chunk played, the silent one pulled", o.Level(), 0.5)

	f.held = chunk / 2
	near(t, "into the silent chunk", o.Level(), 0)
}

// TestLevelFallsWithTheFades pins #1342: the level took the volume setting,
// not the volume the player is at, so while a pause or a seek had the player
// faded to silence it reported the full level of audio nobody heard (0.060
// during a seek's silence, measured on a Mac, with the player at 0.00). It
// plays a constant level through a device the test mixes, so the level is
// v/32768 times the player's volume and nothing else.
func TestLevelFallsWithTheFades(t *testing.T) {
	const v = 16384
	full := float32(v) / (1 << 15)
	c := newMixContext(96, 3, 2*time.Millisecond)
	defer c.close()
	o := newOutput(c, 2)
	defer o.Stop()
	playing := func(what string, want float32) {
		t.Helper()
		waitFor(t, what, 5*time.Second, func() bool { return math.Abs(float64(o.Level()-want)) < 0.002 })
	}

	o.WriteSamples(tone(time.Second, v))
	o.Start()
	playing("the tone at full volume", full)
	o.SetVolume(0.5)
	playing("the tone at volume 0.5", full/2)
	o.SetVolume(1)
	playing("the tone back at full volume", full)

	o.Pause() // returns once faded out
	if got := o.Level(); got != 0 {
		t.Errorf("level %.4f while paused, with the player at volume %.2f; want 0", got, o.PlayingVolume())
	}
	o.Resume()
	playing("the tone after the resume", full)

	o.Rebase(time.Second) // a seek: silent until the new position arrives
	if got := o.Level(); got != 0 {
		t.Errorf("level %.4f during the seek's silence, with the player at volume %.2f; want 0", got, o.PlayingVolume())
	}
	o.WriteSamples(tone(time.Second, v))
	playing("the new position", full)
}
