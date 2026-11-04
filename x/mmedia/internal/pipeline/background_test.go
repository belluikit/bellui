package pipeline

import (
	"testing"
	"time"
)

// twelveSeconds is shortSong with a 12 s duration, the length of #846's song.
type twelveSeconds struct{ shortSong }

func (*twelveSeconds) Duration() time.Duration { return 12 * time.Second }

// staleClock puts p's clock where it is when an app returns from the
// background: running on the audio, last told at 7 s, 15 s ago.
func staleClock(p *Pipeline) {
	p.clock.mu.Lock()
	p.clock.useAudio, p.clock.running, p.clock.acquiring = true, true, false
	p.clock.audioTime, p.clock.audioRate = 7*time.Second, 1
	p.clock.audioSetAt = time.Now().Add(-15 * time.Second)
	p.clock.mu.Unlock()
}

// TestTickReadsTheDeviceAfterTheBackground pins #846: iOS stops a
// backgrounded app's audio, nothing is written meanwhile, and the clock ran
// on wall time, so the first ticks back read 21 s of a 12 s song. The tick
// asks the device first, which stood still at 7 s.
func TestTickReadsTheDeviceAfterTheBackground(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	p := newTestPipeline(t, &twelveSeconds{})
	defer p.Close()
	var log timeLog
	p.SetOnTime(log.record)

	p.audioOutput.Rebase(7 * time.Second)
	staleClock(p)
	p.state.Set(StatePlaying)
	p.tick()

	got := log.since(0)
	if len(got) != 1 {
		t.Fatalf("got %d updates, want 1", len(got))
	}
	if got[0] < 7*time.Second || got[0] > 7*time.Second+50*time.Millisecond {
		t.Errorf("tick after the background reported %v, want the device's 7s", got[0])
	}
}

// TestTimeUpdateNeverPastTheDuration pins #846's other half: whatever the
// clock reads, an update stops at the duration. Here a seek is on its way, so
// the tick cannot ask the device and the clock is still 15 s stale.
func TestTimeUpdateNeverPastTheDuration(t *testing.T) {
	withAudioOutput(t, detachedDevice)
	p := newTestPipeline(t, &twelveSeconds{})
	defer p.Close()
	var log timeLog
	p.SetOnTime(log.record)

	staleClock(p)
	p.seekGen.Add(1)
	p.state.Set(StatePlaying)
	p.tick()

	got := log.since(0)
	if len(got) != 1 || got[0] != 12*time.Second {
		t.Errorf("updates %v, want one at the 12s duration", got)
	}
}
