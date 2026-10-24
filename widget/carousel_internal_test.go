package widget

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// carouselAnimApp wraps the test app so a test can hold animations instead of
// having the test driver run them to completion at once, and can turn
// animations off in the settings.
type carouselAnimApp struct {
	bellui.App
	drv    *carouselAnimDriver
	noAnim bool
}

func (a *carouselAnimApp) Driver() bellui.Driver { return a.drv }

func (a *carouselAnimApp) Settings() bellui.Settings {
	return carouselAnimSettings{Settings: a.App.Settings(), noAnim: a.noAnim}
}

type carouselAnimSettings struct {
	bellui.Settings
	noAnim bool
}

func (s carouselAnimSettings) ShowAnimations() bool { return !s.noAnim }

type carouselAnimDriver struct {
	bellui.Driver
	started []*bellui.Animation
	stopped []*bellui.Animation
}

func (d *carouselAnimDriver) StartAnimation(a *bellui.Animation) { d.started = append(d.started, a) }

func (d *carouselAnimDriver) StopAnimation(a *bellui.Animation) { d.stopped = append(d.stopped, a) }

// newCarouselBench builds a 300x200 three-item carousel in a window, with the
// held-animation app and a fake clock that only moves when the test says so.
func newCarouselBench(t *testing.T, noAnim bool) (*Carousel, bellui.Window, *carouselAnimDriver, *time.Duration) {
	t.Helper()
	a := test.NewTempApp(t)
	drv := &carouselAnimDriver{Driver: a.Driver()}
	bellui.SetCurrentApp(&carouselAnimApp{App: a, drv: drv, noAnim: noAnim})
	t.Cleanup(func() { bellui.SetCurrentApp(a) })

	elapsed := new(time.Duration)
	base := time.Unix(1000, 0)
	carouselNow = func() time.Time { return base.Add(*elapsed) }
	t.Cleanup(func() { carouselNow = time.Now })

	c := NewCarousel(NewLabel("1"), NewLabel("2"), NewLabel("3"))
	w := test.NewTempWindow(t, c)
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 200))
	return c, w, drv, elapsed
}

var carouselMid = geo.NewPos(150, 80)

// During a horizontal drag the current item follows the finger and the item
// it reveals slides in beside it, one pitch away; items the track does not
// overlap stay hidden.
func TestCarousel_DragTracksFinger(t *testing.T) {
	// Animations off: each release snaps back at once.
	c, w, _, _ := newCarouselBench(t, true)
	c.Next() // start in the middle so both neighbours exist
	pitch := c.pitch()
	require.Greater(t, pitch, float32(60))

	test.DragSteps(w.Canvas(), carouselMid, -60, 0, 3, func(step int) {
		off := float32(-20 * step)
		assert.Equal(t, off, c.Items[1].Position().X, "current follows the finger")
		assert.True(t, c.Items[2].Visible(), "next peeks in")
		assert.Equal(t, pitch+off, c.Items[2].Position().X)
		assert.False(t, c.Items[0].Visible(), "previous is off the track")
	})

	// Dragging the other way reveals the previous item on the left.
	test.DragSteps(w.Canvas(), carouselMid, 30, 0, 1, func(int) {
		assert.Equal(t, float32(30), c.Items[1].Position().X)
		assert.True(t, c.Items[0].Visible())
		assert.Equal(t, 30-pitch, c.Items[0].Position().X)
		assert.False(t, c.Items[2].Visible())
	})
}

// Under RTL the next item sits to the left, so a drag to the right reveals it.
func TestCarousel_DragTracksFingerRTL(t *testing.T) {
	c, w, _, _ := newCarouselBench(t, false)
	lang.SetLocaleForTests(t, "ar")
	c.Refresh()
	pitch := c.pitch()

	test.DragSteps(w.Canvas(), carouselMid, 40, 0, 1, func(int) {
		assert.Equal(t, float32(40), c.Items[0].Position().X)
		assert.True(t, c.Items[1].Visible())
		assert.Equal(t, 40-pitch, c.Items[1].Position().X, "next is on the left under RTL")
	})
	assert.Equal(t, 0, c.CurrentItem, "40pt, slow: snaps back")
}

// A drag at either end has nothing to reveal: it meets resistance and snaps back.
func TestCarousel_DragResistsPastEnd(t *testing.T) {
	c, w, _, _ := newCarouselBench(t, true)
	test.DragSteps(w.Canvas(), carouselMid, 150, 0, 1, func(int) {
		assert.Equal(t, float32(50), c.Items[0].Position().X)
	})
	assert.Equal(t, 0, c.CurrentItem)
	assert.Equal(t, float32(0), c.Items[0].Position().X)
}

// On release the content animates to the snapped item: CurrentItem changes at
// once, and the content slides from where the finger left it to rest, with the
// MD3 standard easing over the medium-2 duration.
func TestCarousel_ReleaseAnimatesToSnappedItem(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, false)
	pitch := c.pitch()

	test.Drag(w.Canvas(), carouselMid, -100, 0)
	assert.Equal(t, 1, c.CurrentItem)
	require.Len(t, drv.started, 1)
	a := drv.started[0]
	assert.Equal(t, 300*time.Millisecond, a.Duration)
	assert.NotNil(t, a.Curve)
	assert.Equal(t, bellui.AnimationEaseStandard(0.3), a.Curve(0.3))

	// The new current item starts where the finger left it: one pitch right of
	// the old item, which was at -100.
	assert.Equal(t, pitch-100, c.Items[1].Position().X)
	assert.True(t, c.Items[0].Visible(), "the old item is still sliding out")

	a.Tick(0.5)
	assert.InDelta(t, (pitch-100)/2, c.Items[1].Position().X, 0.01)
	a.Tick(1)
	assert.Equal(t, float32(0), c.Items[1].Position().X)
	assert.False(t, c.Items[0].Visible())
	assert.Nil(t, c.settling)
}

// A short slow drag animates back to the current item.
func TestCarousel_ReleaseAnimatesBack(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, false)
	test.Drag(w.Canvas(), carouselMid, -30, 0)
	assert.Equal(t, 0, c.CurrentItem)
	require.Len(t, drv.started, 1)
	assert.Equal(t, float32(-30), c.Items[0].Position().X)
	drv.started[0].Tick(1)
	assert.Equal(t, float32(0), c.Items[0].Position().X)
}

// With animations off the release snaps instantly and starts no animation.
func TestCarousel_ReleaseSnapsWithoutAnimations(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, true)
	test.Drag(w.Canvas(), carouselMid, -100, 0)
	assert.Equal(t, 1, c.CurrentItem)
	assert.Empty(t, drv.started)
	assert.Equal(t, float32(0), c.Items[1].Position().X)
	assert.False(t, c.Items[0].Visible())

	test.Drag(w.Canvas(), carouselMid, -30, 0) // snap back, also instant
	assert.Equal(t, 1, c.CurrentItem)
	assert.Empty(t, drv.started)
	assert.Equal(t, float32(0), c.Items[1].Position().X)
}

// A fast flick pages even when it is shorter than a fifth of the width; a slow
// drag of the same length snaps back, and so does a flick whose finger stopped
// before lifting.
func TestCarousel_FlickPages(t *testing.T) {
	c, w, _, clock := newCarouselBench(t, true)
	require.Greater(t, c.Size().Width/5, float32(30), "the drags below must be under the distance threshold")

	// 30pt in three 10ms steps: 1000pt/s.
	test.DragSteps(w.Canvas(), carouselMid, -30, 0, 3, func(int) { *clock += 10 * time.Millisecond })
	assert.Equal(t, 1, c.CurrentItem, "fast flick pages")

	// 30pt in ten 30ms steps: 100pt/s.
	test.DragSteps(w.Canvas(), carouselMid, -30, 0, 10, func(int) { *clock += 30 * time.Millisecond })
	assert.Equal(t, 1, c.CurrentItem, "slow short drag snaps back")

	// Fast, then held still for 50ms before release.
	test.DragSteps(w.Canvas(), carouselMid, -30, 0, 3, func(step int) {
		*clock += 10 * time.Millisecond
		if step == 3 {
			*clock += 50 * time.Millisecond
		}
	})
	assert.Equal(t, 1, c.CurrentItem, "a flick that stopped before release snaps back")

	// A flick shorter than minSwipe does not page however fast.
	test.DragSteps(w.Canvas(), carouselMid, -15, 0, 3, func(int) { *clock += 5 * time.Millisecond })
	assert.Equal(t, 1, c.CurrentItem, "under minSwipe")

	// Flicking back the other way pages back.
	test.DragSteps(w.Canvas(), carouselMid, 30, 0, 3, func(int) { *clock += 10 * time.Millisecond })
	assert.Equal(t, 0, c.CurrentItem)
}

// The flick threshold sits at carouselFlickVelocity: just under it snaps back,
// at it pages.
func TestCarousel_FlickThreshold(t *testing.T) {
	c, w, _, clock := newCarouselBench(t, true)
	// Three 10pt steps; velocity is measured between the first and last sample
	// (20pt), so a step time of 20/(2*v) seconds gives speed v.
	step := func(v float64) time.Duration { return time.Duration(float64(time.Second) * 10 / v) }

	test.DragSteps(w.Canvas(), carouselMid, -30, 0, 3, func(int) { *clock += step(carouselFlickVelocity * 0.9) })
	assert.Equal(t, 0, c.CurrentItem, "below the threshold")
	test.DragSteps(w.Canvas(), carouselMid, -30, 0, 3, func(int) { *clock += step(carouselFlickVelocity * 1.01) })
	assert.Equal(t, 1, c.CurrentItem, "at the threshold")
}

// Tapping a navigation button while the release is still settling retargets
// the animation from where the content is, rather than jumping.
func TestCarousel_ButtonRetargetsSettle(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, false)
	pitch := c.pitch()

	test.Drag(w.Canvas(), carouselMid, -100, 0) // pages to 1, settling from pitch-100
	require.Len(t, drv.started, 1)
	first := drv.started[0]
	first.Tick(0.5)
	at := c.Items[1].Position().X

	c.Next() // tapped mid-flight
	assert.Equal(t, 2, c.CurrentItem)
	assert.Equal(t, at, c.Items[1].Position().X, "no jump: item 1 is where it was")
	assert.False(t, c.Items[2].Visible(), "item 2 is still a pitch to the right, off the track")
	require.Len(t, drv.started, 2)
	assert.Contains(t, drv.stopped, first)

	first.Tick(0.9) // a stale tick of the superseded animation is ignored
	assert.Equal(t, at, c.Items[1].Position().X)
	drv.started[1].Tick(0.5)
	assert.InDelta(t, (at+pitch)/2, c.Items[2].Position().X, 0.01, "slides in from where it was")
	drv.started[1].Tick(1)
	assert.Equal(t, float32(0), c.Items[2].Position().X)
}

// A new drag catches content that is still settling, from where it is.
func TestCarousel_DragCatchesSettle(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, false)
	test.Drag(w.Canvas(), carouselMid, -30, 0) // snapping back from -30
	require.Len(t, drv.started, 1)
	drv.started[0].Tick(0.5) // now at -15

	test.DragSteps(w.Canvas(), carouselMid, -10, 0, 1, func(int) {
		assert.Equal(t, float32(-25), c.Items[0].Position().X)
	})
	assert.Contains(t, drv.stopped, drv.started[0])
}

// A gesture that starts vertical moves nothing and pages nothing.
func TestCarousel_VerticalDragIgnored(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, false)
	test.DragSteps(w.Canvas(), carouselMid, -5, -100, 4, func(int) {
		assert.Equal(t, float32(0), c.Items[0].Position().X)
	})
	assert.Equal(t, 0, c.CurrentItem)
	assert.Empty(t, drv.started)
}

// The mobile and embedded drivers coast a released drag: they re-send the
// release event with decaying deltas, at the release position, before
// DragEnd (driver.tapUpCanvas). The carousel takes the first coasted event as
// the release, so a slow drag is not carried over the distance threshold by
// the coast, the content does not glide with it, and the late DragEnd is a
// no-op.
func TestCarousel_DriverCoastIsTheRelease(t *testing.T) {
	c, _, drv, clock := newCarouselBench(t, false)
	require.Greater(t, c.Size().Width/5, float32(55))

	finger := func(x float32, dx float32) {
		*clock += 30 * time.Millisecond
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(dx, 0)})
	}
	// A slow 50pt drag (about 100pt/s), released while still moving 3pt a step.
	x := float32(150)
	for range 17 {
		x -= 3
		finger(x, -3)
	}
	at := c.Items[0].Position().X
	require.InDelta(t, -50, at, 3)

	// The driver's coast: one event, re-sent with decaying deltas. Its sum
	// (about 30pt) would carry the drag past a fifth of the width.
	coast := &event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(-3, 0)}
	for i := 0; i < 12; i++ {
		*clock += 16 * time.Millisecond
		coast.Dragged.DX *= 0.92
		c.Dragged(coast)
		if i == 0 {
			require.Len(t, drv.started, 1, "the first coasted event is the release: the settle starts")
		}
		assert.Equal(t, at, c.Items[0].Position().X, "the coast does not move the content")
	}
	c.DragEnd()
	assert.Equal(t, 0, c.CurrentItem, "slow drag snapped back despite the coast")
	assert.Len(t, drv.started, 1, "the late DragEnd starts nothing")
	drv.started[0].Tick(1)
	assert.Equal(t, float32(0), c.Items[0].Position().X)

	// A fast flick under the threshold, then its coast: pages at the release.
	for i, x := 0, float32(140); i < 3; i, x = i+1, x-10 { // ends at x=120
		*clock += 10 * time.Millisecond
		c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(x, 80)}, Dragged: geo.NewDelta(-10, 0)})
	}
	coast = &event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(120, 80)}, Dragged: geo.NewDelta(-9.2, 0)}
	*clock += 16 * time.Millisecond
	c.Dragged(coast)
	assert.Equal(t, 1, c.CurrentItem, "the flick paged at the release")
	c.Dragged(coast)
	c.DragEnd()
	assert.Equal(t, 1, c.CurrentItem)
}
