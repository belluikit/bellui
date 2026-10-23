package audio

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/oto/v3"
)

// failDevice makes the device refuse to open the way oto v3.4.0 does: a nil
// context, a nil ready channel and the error. It restores the real open, and a
// fresh device, when the test ends.
func failDevice(t *testing.T, err error) *int {
	t.Helper()
	calls := new(int)
	saved := newOtoContext
	newOtoContext = func(*oto.NewContextOptions) (*oto.Context, chan struct{}, error) {
		*calls++
		return nil, nil, err
	}
	resetDevice()
	t.Cleanup(func() {
		newOtoContext = saved
		resetDevice()
	})
	return calls
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
