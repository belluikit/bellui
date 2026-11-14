package audio

import (
	"testing"
	"time"
)

// TestFadesTakeLittleTime pins what the fades of #1228 cost on a real
// device: Pause and Stop wait for the faded audio to be mixed, which takes
// fadeTime and up to a device block (32 ms on macOS), and no more than
// fadeWait past the fade even on a device that stops asking. Taking less than
// fadeTime would mean there was no fade. It needs a real audio device.
//
// Resume is held to the fade alone from below. It also restarts a device that
// was suspended meanwhile, which on Android is the platform's own time: 8 ms
// mostly and up to about 200 ms on a Pixel 9a, and 128-155 ms on every resume
// before #1228 (measured 2026-11-14).
func TestFadesTakeLittleTime(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	defer o.Stop()
	o.WriteSamples(tone(time.Second, 8000))
	o.Start()
	waitFor(t, "the tone to be heard", time.Second, func() bool { return o.Level() > 0 })

	for _, step := range []struct {
		name string
		do   func()
	}{{"Pause", o.Pause}, {"Resume", o.Resume}, {"Stop", o.Stop}} {
		start := time.Now()
		step.do()
		took := time.Since(start)
		t.Logf("%s took %v", step.name, took)
		if took < fadeTime {
			t.Errorf("%s took %v; want at least the fade, %v", step.name, took, fadeTime)
		}
		if limit := fadeTime + fadeWait; step.name != "Resume" && took > limit {
			t.Errorf("%s took %v; want at most %v", step.name, took, limit)
		}
	}
}
