package audio

import (
	"errors"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

// recordSuspender records what idleSuspend asks of the device, and can refuse
// a suspend.
type recordSuspender struct {
	mu         sync.Mutex
	calls      []string
	suspendErr error
}

func (r *recordSuspender) record(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

// recorded is the calls so far.
func (r *recordSuspender) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

func (r *recordSuspender) Suspend() error {
	r.record("suspend")
	return r.suspendErr
}

func (r *recordSuspender) Resume() error {
	r.record("resume")
	return nil
}

func (r *recordSuspender) Flush() error {
	r.record("flush")
	return nil
}

// suspendingWhenIdle turns on the Android behaviour for one test.
func suspendingWhenIdle(t *testing.T, on bool) {
	t.Helper()
	saved := suspendWhenIdle
	suspendWhenIdle = on
	t.Cleanup(func() { suspendWhenIdle = saved })
}

// TestIdleSuspendWaitsForTheLastOutput pins #1052: the device is suspended only
// when no Output plays, and resumed once, before the next one plays.
func TestIdleSuspendWaitsForTheLastOutput(t *testing.T) {
	suspendingWhenIdle(t, true)
	dev := &recordSuspender{}
	s := &idleSuspend{dev: dev}

	s.play()
	s.play()
	s.idle()
	if len(dev.calls) != 0 {
		t.Fatalf("one of two Outputs paused, and the device got %v; want nothing", dev.calls)
	}
	s.idle()
	s.play()
	s.play()
	s.idle()
	s.idle()
	// Each suspend discards what the device still had queued (#1228).
	if want := []string{"suspend", "flush", "resume", "suspend", "flush"}; !slices.Equal(dev.calls, want) {
		t.Errorf("device calls %v, want %v", dev.calls, want)
	}
}

// TestIdleSuspendIsAndroidOnly: elsewhere the device is never suspended.
func TestIdleSuspendIsAndroidOnly(t *testing.T) {
	suspendingWhenIdle(t, false)
	dev := &recordSuspender{}
	s := &idleSuspend{dev: dev}
	s.play()
	s.idle()
	s.play()
	if len(dev.calls) != 0 {
		t.Errorf("device calls %v with suspendWhenIdle off; want none", dev.calls)
	}
}

// TestIdleSuspendResumesOnlyWhatItSuspended: a suspend that failed left the
// device running, so the next play has nothing to resume.
func TestIdleSuspendResumesOnlyWhatItSuspended(t *testing.T) {
	suspendingWhenIdle(t, true)
	dev := &recordSuspender{suspendErr: errors.New("refused")}
	s := &idleSuspend{dev: dev}
	s.play()
	s.idle()
	s.play()
	if want := []string{"suspend"}; !slices.Equal(dev.calls, want) {
		t.Errorf("device calls %v, want %v", dev.calls, want)
	}
}

// TestOutputCountsWhilePlaying wires idleSuspend to Outputs on the real
// device: Start and Resume count an Output, Pause and Stop take it out once
// the device has played what it mixed (settle), each once however often it is
// called.
func TestOutputCountsWhilePlaying(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	suspendingWhenIdle(t, true)
	dev := &recordSuspender{}
	// An earlier test can leave the real device suspended, on Android, and
	// the recorder would then be asked to resume it first. It is resumed here
	// for real, so that the recorder sees only this test's calls.
	device.idle.mu.Lock()
	saved := device.idle.dev
	if device.idle.suspended {
		if err := saved.Resume(); err != nil {
			t.Fatalf("resuming the device an earlier test suspended: %v", err)
		}
		device.idle.suspended = false
	}
	device.idle.dev = dev
	device.idle.mu.Unlock()
	t.Cleanup(func() {
		device.idle.mu.Lock()
		device.idle.dev = saved
		device.idle.mu.Unlock()
	})
	defer o.Stop()
	calls := func(n int) func() bool { return func() bool { return len(dev.recorded()) >= n } }

	o.WriteSamples(tone(500*time.Millisecond, 8000))
	o.Start()
	if !o.Playing() {
		t.Error("not playing after Start")
	}
	o.Pause()
	o.Pause()
	if o.Playing() {
		t.Error("playing after Pause")
	}
	waitFor(t, "the device to be suspended after the pause", 2*settleWait, calls(2))
	o.Resume()
	o.Resume()
	o.Stop()
	o.Stop()
	waitFor(t, "the device to be suspended after the stop", 2*settleWait, calls(5))
	if want := []string{"suspend", "flush", "resume", "suspend", "flush"}; !slices.Equal(dev.recorded(), want) {
		t.Errorf("device calls %v, want %v", dev.recorded(), want)
	}
	if NewDetachedOutput(2).Playing() {
		t.Error("a detached Output counts as playing")
	}
}

// The device is suspended while idle on Android and iOS, and nowhere else
// (#1052, #1242), and this build's setting is its platform's.
func TestSuspendsWhenIdlePlatforms(t *testing.T) {
	for goos, want := range map[string]bool{
		"android": true, "ios": true,
		"darwin": false, "linux": false, "windows": false, "js": false,
	} {
		if got := suspendsWhenIdle(goos); got != want {
			t.Errorf("suspendsWhenIdle(%q) = %v, want %v", goos, got, want)
		}
	}
	if suspendWhenIdle != suspendsWhenIdle(runtime.GOOS) {
		t.Errorf("suspendWhenIdle = %v on %s", suspendWhenIdle, runtime.GOOS)
	}
}
