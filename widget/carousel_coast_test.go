package widget_test

import (
	"testing"
	"time"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mobile and embedded drivers coast a released drag through
// common.DragCoast: copies of the release event, at the release position, with
// decaying deltas, then DragEnd. The carousel takes the first coasted event as
// the release, so a slow drag is not carried over the distance threshold by
// the coast, the content does not glide with it, and the late DragEnd is a
// no-op.
func TestCarousel_DriverCoastIsTheRelease(t *testing.T) {
	c, _, started, clock := widget.NewCarouselBenchForTest(t, false)
	require.Greater(t, c.Size().Width/5, float32(55))
	var coaster common.DragCoast

	finger := func(x float32, dx float32, step time.Duration) {
		*clock += step
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(dx, 0)})
	}
	// A slow 51pt drag (about 100pt/s), released while still moving 3pt a step.
	x := float32(150)
	for range 17 {
		x -= 3
		finger(x, -3, 30*time.Millisecond)
	}
	at := c.Items[0].Position().X
	require.InDelta(t, -50, at, 3)

	// The coast's sum (about 35pt) would carry the drag past a fifth of the width.
	steps := runCoast(t, &coaster, c, x, -3, clock, func(i int) {
		if i == 0 {
			require.Len(t, started(), 1, "the first coasted event is the release: the settle starts")
		}
		assert.Equal(t, at, c.Items[0].Position().X, "the coast does not move the content")
	})
	require.Greater(t, steps, 5, "the coast ran")
	assert.Equal(t, 0, c.CurrentItem, "slow drag snapped back despite the coast")
	assert.Len(t, started(), 1, "the late DragEnd starts nothing")
	started()[0].Tick(1)
	assert.Equal(t, float32(0), c.Items[0].Position().X)

	// A fast flick under the threshold, then its coast: pages at the release.
	x = 150
	for range 3 {
		x -= 10
		finger(x, -10, 10*time.Millisecond)
	}
	runCoast(t, &coaster, c, x, -10, clock, func(i int) {
		if i == 0 {
			assert.Equal(t, 1, c.CurrentItem, "the flick paged at the release")
		}
	})
	assert.Equal(t, 1, c.CurrentItem)
}

// A touch during a coast stops it (common.DragCoast.Stop delivers DragEnd
// first, #751); the new drag then tracks from where the content is.
func TestCarousel_TouchDuringCoastTracks(t *testing.T) {
	c, _, started, clock := widget.NewCarouselBenchForTest(t, true)
	var coaster common.DragCoast
	q := make(chan func(), 64)
	x := float32(150)
	for range 3 {
		x -= 10
		*clock += 10 * time.Millisecond
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(-10, 0)})
	}
	coaster.Release(c, &event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(-10, 0)}, func(f func()) { q <- f })
	(<-q)() // one coast step: the release, paged
	assert.Equal(t, 1, c.CurrentItem)
	assert.Empty(t, started(), "animations off: snapped")

	coaster.Stop() // the new touch goes down
	for range 20 {
		*clock += 10 * time.Millisecond
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x+float32(1), 80)}, Dragged: geo.NewDelta(1, 0)})
		x++
	}
	assert.Equal(t, float32(20), c.Items[1].Position().X, "the new drag tracks the finger")
	// Steps the stopped coast had queued run late and must do nothing.
	drain(q, 100*time.Millisecond)
	assert.Equal(t, float32(20), c.Items[1].Position().X)
	c.DragEnd()
	assert.Equal(t, 1, c.CurrentItem)
	assert.Equal(t, float32(0), c.Items[1].Position().X)
}

// runCoast releases the drag through the real driver coast at release
// position x with the last delta dx, running each queued step on the test
// goroutine (the main goroutine's stand-in), and returns how many ran.
func runCoast(t *testing.T, coaster *common.DragCoast, c *widget.Carousel, x, dx float32, clock *time.Duration, each func(int)) int {
	t.Helper()
	q := make(chan func(), 64)
	coaster.Release(c, &event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(dx, 0)}, func(f func()) { q <- f })
	n := 0
	for {
		select {
		case f := <-q:
			*clock += 16 * time.Millisecond
			f()
			each(n)
			n++
		case <-time.After(time.Second):
			return n
		}
	}
}

func drain(q chan func(), wait time.Duration) {
	for {
		select {
		case f := <-q:
			f()
		case <-time.After(wait):
			return
		}
	}
}
