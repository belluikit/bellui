package audio

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// recordSuspender records what idleSuspend asks of the device, and can refuse
// a suspend.
type recordSuspender struct {
	calls      []string
	suspendErr error
}

func (r *recordSuspender) Suspend() error {
	r.calls = append(r.calls, "suspend")
	return r.suspendErr
}

func (r *recordSuspender) Resume() error {
	r.calls = append(r.calls, "resume")
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
	if want := []string{"suspend", "resume", "suspend"}; !slices.Equal(dev.calls, want) {
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
// device: Start and Resume count an Output, Pause and Stop take it out, each
// once however often it is called.
func TestOutputCountsWhilePlaying(t *testing.T) {
	o, err := NewOutput(2)
	if err != nil {
		t.Skipf("no audio device: %v", err)
	}
	suspendingWhenIdle(t, true)
	dev := &recordSuspender{}
	device.idle.mu.Lock()
	saved := device.idle.dev
	device.idle.dev = dev
	device.idle.mu.Unlock()
	t.Cleanup(func() {
		device.idle.mu.Lock()
		device.idle.dev = saved
		device.idle.mu.Unlock()
	})
	defer o.Stop()

	o.WriteSamples(tone(500*time.Millisecond, 8000))
	o.Start()
	if !o.Playing() {
		t.Error("not counted as playing after Start")
	}
	o.Pause()
	o.Pause()
	if o.Playing() {
		t.Error("counted as playing after Pause")
	}
	o.Resume()
	o.Resume()
	o.Stop()
	o.Stop()
	if want := []string{"suspend", "resume", "suspend"}; !slices.Equal(dev.calls, want) {
		t.Errorf("device calls %v, want %v", dev.calls, want)
	}
	if NewDetachedOutput(2).Playing() {
		t.Error("a detached Output counts as playing")
	}
}
