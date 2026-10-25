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

// The carousel does not ask for drag momentum (#751): it pages at the
// release, from the finger's speed, as direct manipulation does.
func TestCarousel_HasNoDragMomentum(t *testing.T) {
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"))
	if m, ok := any(c).(event.MomentumDraggable); ok {
		assert.False(t, m.DragMomentum())
	}
}

// The mobile and embedded drivers release a touch drag through
// common.DragCoast. For the carousel that is the release itself: DragEnd
// arrives at once and no coast step is queued, so a slow drag snaps back and
// a fast flick pages, both at the release.
func TestCarousel_DriverReleaseEndsAtOnce(t *testing.T) {
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

	queued := release(&coaster, c, x, -3)
	require.Len(t, started(), 1, "DragEnd came with the release: the settle starts")
	assert.Equal(t, at, c.Items[0].Position().X, "the settle starts where the finger left the content")
	assert.Zero(t, queued, "nothing coasts")
	assert.Equal(t, 0, c.CurrentItem, "slow drag snapped back")
	started()[0].Tick(1)
	assert.Equal(t, float32(0), c.Items[0].Position().X)

	// A fast flick under the distance threshold pages at the release.
	x = 150
	for range 3 {
		x -= 10
		finger(x, -10, 10*time.Millisecond)
	}
	assert.Zero(t, release(&coaster, c, x, -10), "nothing coasts")
	assert.Equal(t, 1, c.CurrentItem, "the flick paged at the release")
}

// A touch right after a release starts a new drag that tracks the finger:
// there is no coast for it to stop, and no late DragEnd to cut it short.
func TestCarousel_TouchAfterReleaseTracks(t *testing.T) {
	c, _, started, clock := widget.NewCarouselBenchForTest(t, true)
	var coaster common.DragCoast
	x := float32(150)
	for range 3 {
		x -= 10
		*clock += 10 * time.Millisecond
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(-10, 0)})
	}
	assert.Zero(t, release(&coaster, c, x, -10))
	assert.Equal(t, 1, c.CurrentItem)
	assert.Empty(t, started(), "animations off: snapped")

	coaster.Stop() // the new touch goes down
	for range 20 {
		*clock += 10 * time.Millisecond
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x+float32(1), 80)}, Dragged: geo.NewDelta(1, 0)})
		x++
	}
	assert.Equal(t, float32(20), c.Items[1].Position().X, "the new drag tracks the finger")
	c.DragEnd()
	assert.Equal(t, 1, c.CurrentItem)
	assert.Equal(t, float32(0), c.Items[1].Position().X)
}

// release releases the drag through the real driver coast at release
// position x with the last delta dx, and returns how many coast steps it
// queued within 100ms; it waits no longer, so a regression fails instead of
// hanging.
func release(coaster *common.DragCoast, c *widget.Carousel, x, dx float32) int {
	q := make(chan func(), 64)
	coaster.Release(c, &event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(dx, 0)}, func(f func()) { q <- f })
	n := 0
	for {
		select {
		case f := <-q:
			f()
			n++
		case <-time.After(100 * time.Millisecond):
			return n
		}
	}
}
