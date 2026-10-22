//go:build !wasm

package mobile

import (
	"image"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/gpu"
)

// captureProbe wraps a canvas painter and counts the calls Capture makes to
// it, and those made off the main goroutine. Capture reads the frame count
// before anything else, so the calls say where its work ran.
type captureProbe struct {
	gpu.Painter
	reads, offMain atomic.Int32
}

func (p *captureProbe) record() {
	p.reads.Add(1)
	if !async.IsMainGoroutine() {
		p.offMain.Add(1)
	}
}

func (p *captureProbe) FramesBegun() uint64 {
	p.record()
	return p.Painter.(interface{ FramesBegun() uint64 }).FramesBegun()
}

func (p *captureProbe) Capture(c core.Canvas) image.Image {
	p.record()
	return p.Painter.Capture(c)
}

// TestCanvas_CaptureFromGoroutineRunsOnMain: a Capture called from a
// goroutine runs on the main goroutine, which paints the window every frame
// and owns the GL context under -tags gl (#573).
func TestCanvas_CaptureFromGoroutineRunsOnMain(t *testing.T) {
	w := d.CreateWindow("Capture thread")
	defer w.Close()
	c := w.Canvas().(*canvas)
	probe := &captureProbe{Painter: c.Painter()}
	c.SetPainter(probe)

	require.False(t, async.IsMainGoroutine(), "the test runs off the main goroutine")
	img := c.Capture()

	require.NotZero(t, probe.reads.Load(), "Capture ran")
	assert.Zero(t, probe.offMain.Load(), "no Capture work ran off the main goroutine")
	// This host has no native surface to paint into, so no frame begins.
	assert.Nil(t, img)
}

// TestCanvas_CaptureOnMainRunsDirectly: a Capture already on the main
// goroutine runs there rather than queueing behind itself, which would
// deadlock.
func TestCanvas_CaptureOnMainRunsDirectly(t *testing.T) {
	w := d.CreateWindow("Capture on main")
	defer w.Close()
	c := w.Canvas().(*canvas)
	probe := &captureProbe{Painter: c.Painter()}
	c.SetPainter(probe)

	done := make(chan struct{})
	go d.DoFromGoroutine(func() {
		c.Capture()
		close(done)
	}, true)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Capture on the main goroutine did not return")
	}
	require.NotZero(t, probe.reads.Load(), "Capture ran")
	assert.Zero(t, probe.offMain.Load())
}
