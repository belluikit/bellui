package test

import (
	"image/color"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

func Test_driver_AbsolutePositionForObject(t *testing.T) {
	d := &driver{}
	w := d.CreateWindow("Test Window")
	o := belluicanvas.NewRectangle(color.Black)
	w.SetContent(o)
	w.Resize(geo.NewSize(320, 200))

	t.Run("for padded window", func(t *testing.T) {
		w.SetPadded(true)
		assert.Equal(t, geo.NewPos(2, 1), d.AbsolutePositionForObject(o), "safe area offset (2,3) is subtracted")
	})

	t.Run("for non-padded window", func(t *testing.T) {
		w.SetPadded(false)
		assert.Equal(t, geo.NewPos(-2, -3), d.AbsolutePositionForObject(o), "safe area offset (2,3) is subtracted")
	})
}

func TestDriver_CreateWindow(t *testing.T) {
	d := &driver{}
	w := d.CreateWindow("Test Window")

	assert.Equal(t, "Test Window", w.Title())
}

func TestDriver_DoFromGoroutineNested(t *testing.T) {
	NewTempApp(t)

	ran := false
	bellui.DoAndWait(func() {
		// a nested call from within a running callback must not deadlock
		bellui.Do(func() {
			ran = true
		})
	})

	assert.True(t, ran)
}

// Do on the test driver has run its function by the time it returns, under
// every tag set. A bare -tags mobile build used to hand it to a new goroutine
// instead, because the test app registers no main goroutine and the mobile
// build counts an unregistered caller as main (#625).
func TestDriver_DoRunsBeforeReturning(t *testing.T) {
	NewTempApp(t)

	ran := false
	bellui.Do(func() { ran = true })

	assert.True(t, ran)
}

// A goroutine leaked from (or still in flight at the end of) a previous test
// keeps calling bellui.Do against the previous test app's driver while the next
// test's work runs on a fresh driver instance. The Do mutex is package-level,
// shared across driver instances, so those callbacks are still serialized and
// carry happens-before edges (pre-rebrand tracker #1049).
func TestDriver_DoFromGoroutineSerializedAcrossInstances(t *testing.T) {
	d1 := NewDriver()
	d2 := NewDriver()

	var active, overlaps atomic.Int32
	shared := 0 // plain int: -race verifies the cross-instance happens-before edges
	body := func() {
		if active.Add(1) > 1 {
			overlaps.Add(1)
		}
		shared++
		time.Sleep(100 * time.Microsecond)
		active.Add(-1)
	}

	const calls = 20
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		d := bellui.Driver(d1)
		if i%2 == 1 {
			d = d2
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.DoFromGoroutine(body, true)
		}()
	}
	wg.Wait()

	assert.Zero(t, overlaps.Load(), "Do callbacks through different driver instances must not run concurrently")
	bellui.Driver(d1).DoFromGoroutine(func() {
		assert.Equal(t, calls, shared)
	}, true)
}
