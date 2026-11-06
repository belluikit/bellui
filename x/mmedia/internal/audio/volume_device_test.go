package audio

import (
	"testing"
	"time"
)

// TestMuteIsHeardAtOnce pins #1032: the volume was applied to samples as they
// were written to the ring, and the decoder runs up to a second ahead of
// playback, so after SetVolume(0) the level stayed above zero for 1.2 s on a
// Mac. The volume is the player's now, applied as oto mixes. It needs a real
// audio device: on the detached one nothing plays.
func TestMuteIsHeardAtOnce(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	defer o.Stop()
	o.WriteSamples(tone(1500*time.Millisecond, 8000)) // decoded well ahead, as the pipeline does
	o.Start()
	waitFor(t, "the tone to be heard", time.Second, func() bool { return o.Level() > 0 })

	o.SetVolume(0)
	muted := time.Now()
	waitFor(t, "the level to reach 0 after SetVolume(0)", time.Second, func() bool { return o.Level() == 0 })
	if took := time.Since(muted); took > 200*time.Millisecond {
		t.Errorf("the level reached 0 %v after SetVolume(0); want within 200 ms", took)
	}
	if v := o.player.Volume(); v != 0 {
		t.Errorf("the player's volume is %v after SetVolume(0); want 0", v)
	}

	o.SetVolume(1)
	waitFor(t, "the level to come back after SetVolume(1)", 200*time.Millisecond, func() bool { return o.Level() > 0 })
}

// TestANewPlayerTakesTheVolume pins that a volume set while stopped reaches
// the player the next Start makes, since the volume lives on the player.
func TestANewPlayerTakesTheVolume(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	defer o.Stop()
	o.SetVolume(0.25)
	o.Start()
	if v := o.player.Volume(); v != 0.25 {
		t.Errorf("the player's volume is %v after SetVolume(0.25) then Start; want 0.25", v)
	}
	o.Stop()
	o.SetVolume(0.5)
	o.Start()
	if v := o.player.Volume(); v != 0.5 {
		t.Errorf("the next player's volume is %v; want 0.5", v)
	}
}

// waitFor polls cond every 5 ms and fails the test if it is not met within d.
func waitFor(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(d); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("timed out after %v waiting for %s", d, what)
		}
	}
}
