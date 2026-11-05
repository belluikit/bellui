//go:build gl && !wasm

package mobile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/internal/driver/mobile/event/lifecycle"
	"github.com/belluikit/bellui/internal/driver/mobile/event/paint"
	"github.com/belluikit/bellui/internal/driver/mobile/gl"
	"github.com/belluikit/bellui/widget"
)

// quietApp is the platform app as far as a lifecycle crossing needs it: the
// paint request a crossing sends is dropped. Anything else hits the nil
// app.App and panics.
type quietApp struct{ app.App }

func (quietApp) Send(any) {}

// currentGL is a draw context that is current: non-nil is all renderReady asks.
type currentGL struct{ gl.Context }

// A hidden app ticks no animation (#965). belluikit/player runs PXF animate
// entries on the driver's animation runner and relies on this: a music app
// plays for hours with the screen off. The mobile driver ticks animations in
// handlePaint after its renderReady gate, which the StageVisible crossing off
// closes; ticking ahead of that gate would pass every other test.
//
// This runs on the GL backend (the mobile CI legs build -tags gl), whose gate
// is the draw context the crossing sets and clears. The WebGPU backends gate
// the same call in the same place on their own signals (native-window
// liveness on Android, surfaceVisible on iOS); measured on devices for player
// ADR-0028, a backgrounded app ticked 0 times in 60 s on a Pixel 9a and an iPad.
func Test_mobileDriver_HiddenAppTicksNoAnimation(t *testing.T) {
	d := &driver{app: quietApp{}}
	w := d.CreateWindow("main").(*window)
	w.SetContent(widget.NewLabel("animating"))
	w.Show()
	c := w.canvas
	c.initialized = true // no painter needed: the canvas is kept clean below

	ticks := 0
	a := &bellui.Animation{
		Duration:    time.Hour,
		RepeatCount: bellui.AnimationRepeatForever,
		Tick:        func(float32) { ticks++ },
	}
	d.StartAnimation(a)
	t.Cleanup(func() { d.StopAnimation(a) })

	cross := func(from, to lifecycle.Stage) {
		d.handleLifecycle(lifecycle.Event{From: from, To: to, DrawContext: currentGL{}}, w)
	}
	paintFrames := func(n int) {
		for range n {
			c.CheckDirtyAndClear() // tick, then nothing to draw
			d.handlePaint(paint.Event{}, w)
		}
	}

	cross(lifecycle.StageAlive, lifecycle.StageVisible)
	paintFrames(5)
	require.Positive(t, ticks, "a visible app ticks its animation (the test's own control)")

	cross(lifecycle.StageVisible, lifecycle.StageAlive)
	hidden := ticks
	paintFrames(20)
	assert.Equal(t, hidden, ticks, "no tick while the app is hidden")

	cross(lifecycle.StageAlive, lifecycle.StageVisible)
	paintFrames(5)
	assert.Greater(t, ticks, hidden, "ticks resume once the app is visible again")
}
