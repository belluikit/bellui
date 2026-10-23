package audio

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/oto/v3"
)

// failDevice makes the device refuse to open the way oto v3.4.0 does when it
// fails synchronously (darwin's initializeAPI): a nil context, a nil ready
// channel and the error. It restores the real open, and a fresh device, when
// the test ends.
func failDevice(t *testing.T, err error) *int {
	t.Helper()
	return fakeDevice(t, func() (otoContext, chan struct{}, error) { return nil, nil, err })
}

// fakeDevice replaces oto.NewContext with open for one test, counting calls.
func fakeDevice(t *testing.T, open func() (otoContext, chan struct{}, error)) *int {
	t.Helper()
	calls := new(int)
	saved := newOtoContext
	newOtoContext = func(*oto.NewContextOptions) (otoContext, chan struct{}, error) {
		*calls++
		return open()
	}
	resetDevice()
	t.Cleanup(func() {
		newOtoContext = saved
		resetDevice()
	})
	return calls
}

// lateFailContext is a context oto handed back with no error, whose backend
// then failed on its own goroutine: how ALSA reports no sound card, and
// CoreAudio a queue that will not start.
type lateFailContext struct{ err error }

func (c lateFailContext) NewPlayer(io.Reader) *oto.Player { panic("NewPlayer on a failed device") }
func (c lateFailContext) Err() error                      { return c.err }

// closedReady is a ready channel the backend has already closed.
func closedReady() chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

func resetDevice() {
	device.once = sync.Once{}
	device.ctx, device.channels, device.err = nil, 0, nil
}

// newOutputWithin runs NewOutput and fails the test, rather than hanging it,
// if NewOutput does not return.
func newOutputWithin(t *testing.T, channels int) (*Output, error) {
	t.Helper()
	type result struct {
		o   *Output
		err error
	}
	done := make(chan result, 1)
	go func() {
		o, err := NewOutput(channels)
		done <- result{o, err}
	}()
	select {
	case r := <-done:
		return r.o, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("NewOutput did not return: a device that fails to open hangs it (#690)")
		return nil, nil
	}
}

// TestNewOutputReturnsTheDeviceError pins #690: a device that will not open
// used to block NewOutput forever, on oto's nil ready channel, inside the Once
// every later caller waits on. It is an error now.
func TestNewOutputReturnsTheDeviceError(t *testing.T) {
	refused := errors.New("no audio device")
	failDevice(t, refused)

	o, err := newOutputWithin(t, 2)
	if !errors.Is(err, refused) {
		t.Fatalf("NewOutput error = %v, want it to wrap %v", err, refused)
	}
	if o != nil {
		t.Error("NewOutput returned an Output along with the error")
	}
}

// TestNewOutputRemembersTheDeviceError pins the choice not to retry: oto
// latches its one context as created even when creating it failed, so a retry
// could only report "context is already created". Every later NewOutput gets
// the first, true error, without asking oto again and without blocking.
func TestNewOutputRemembersTheDeviceError(t *testing.T) {
	refused := errors.New("format refused")
	calls := failDevice(t, refused)

	_, first := newOutputWithin(t, 2)
	_, second := newOutputWithin(t, 6)
	if !errors.Is(first, refused) || !errors.Is(second, refused) {
		t.Fatalf("errors = %v, %v; want both to wrap %v", first, second, refused)
	}
	if *calls != 1 {
		t.Errorf("oto.NewContext called %d times, want once", *calls)
	}
}

// TestNewOutputReturnsALateDeviceError is the no-sound-card case as oto
// really reports it on Linux: NewContext succeeds, ready closes, and the
// failure is only in Err. The fakes above return the error from NewContext
// itself, which on Linux never happens.
func TestNewOutputReturnsALateDeviceError(t *testing.T) {
	refused := errors.New(`oto: ALSA error at snd_pcm_open: "default": No such file or directory`)
	fakeDevice(t, func() (otoContext, chan struct{}, error) {
		return lateFailContext{refused}, closedReady(), nil
	})

	o, err := newOutputWithin(t, 2)
	if !errors.Is(err, refused) {
		t.Fatalf("NewOutput error = %v, want it to wrap %v", err, refused)
	}
	if o != nil {
		t.Error("NewOutput returned an Output on a device that failed to open")
	}
}

// TestNewOutputOpensAWorkingDeviceOnce is the positive control: a context
// that opens with no error is shared by every Output, at the first caller's
// channel count.
func TestNewOutputOpensAWorkingDeviceOnce(t *testing.T) {
	calls := fakeDevice(t, func() (otoContext, chan struct{}, error) {
		return lateFailContext{}, closedReady(), nil
	})

	first, err := newOutputWithin(t, 6)
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}
	second, err := newOutputWithin(t, 2)
	if err != nil {
		t.Fatalf("second NewOutput: %v", err)
	}
	if first.Channels() != 6 || second.Channels() != 6 {
		t.Errorf("channels = %d, %d; want both 6, the device's", first.Channels(), second.Channels())
	}
	if *calls != 1 {
		t.Errorf("oto.NewContext called %d times, want once", *calls)
	}
}

// TestADetachedOutputIgnoresStart: a pipeline built on a detached Output is
// played like any other, and Start on no device used to dereference a nil
// context.
func TestADetachedOutputIgnoresStart(t *testing.T) {
	o := NewDetachedOutput(2)
	o.Start()
	o.Pause()
	o.Resume()
	o.Stop()
	if got := o.PlaybackPosition(); got != 0 {
		t.Errorf("a detached Output's position moved to %v", got)
	}
}
