package widget

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
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

// NewCarouselBenchForTest exposes newCarouselBench to the external tests that
// need internal/driver/common, which this package cannot import in its own
// tests (common imports container, which imports widget). started reports the
// animations started so far.
func NewCarouselBenchForTest(t *testing.T, noAnim bool) (c *Carousel, w bellui.Window, started func() []*bellui.Animation, clock *time.Duration) {
	c, w, drv, clock := newCarouselBench(t, noAnim)
	return c, w, func() []*bellui.Animation { return drv.started }, clock
}

// A flick against content still settling goes the way it was flicked. After a
// forward page the content settles from the right; a forward flick that
// catches it while it is still more than a fifth of the width out stays on the
// new item rather than paging back.
func TestCarousel_FlickAgainstCaughtSettle(t *testing.T) {
	c, w, drv, clock := newCarouselBench(t, false)
	test.Drag(w.Canvas(), carouselMid, -100, 0) // pages to 1, settling from pitch-100
	require.Equal(t, 1, c.CurrentItem)
	require.Len(t, drv.started, 1)
	drv.started[0].Tick(0.2)
	require.Greater(t, c.offset-30, c.Size().Width/5, "the release below sits past the distance threshold")

	test.DragSteps(w.Canvas(), carouselMid, -30, 0, 3, func(int) { *clock += 10 * time.Millisecond })
	assert.Equal(t, 1, c.CurrentItem, "a forward flick does not page back")
}

// A Next or Previous during a gesture that is not horizontal moves the content
// to the new item; it is not held for a finger the carousel does not follow.
func TestCarousel_ButtonDuringVerticalDrag(t *testing.T) {
	c, _, _, _ := newCarouselBench(t, false)
	c.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: geo.NewPos(150, 60)}, Dragged: geo.NewDelta(0, -20)})
	c.Next()
	c.DragEnd()
	assert.Equal(t, 1, c.CurrentItem)
	assert.True(t, c.Items[1].Visible())
	assert.Equal(t, float32(0), c.Items[1].Position().X)
	assert.Equal(t, float32(0), c.offset)
}

// A drag that catches a snap-back past the first item starts from where the
// content is: the resistance applies to the finger's travel, so the content
// does not jump.
func TestCarousel_DragCatchesEdgeSnapBack(t *testing.T) {
	c, w, drv, _ := newCarouselBench(t, false)
	test.Drag(w.Canvas(), carouselMid, 150, 0) // resisted to 50, snapping back
	require.Len(t, drv.started, 1)
	drv.started[0].Tick(0.5)
	require.Equal(t, float32(25), c.Items[0].Position().X)

	test.DragSteps(w.Canvas(), carouselMid, 6, 0, 1, func(int) {
		assert.InDelta(t, 27, c.Items[0].Position().X, 0.001, "25 plus a third of 6")
	})
}

// A settle that outlives the carousel's renderer does not build a new one.
func TestCarousel_SettleWithoutRendererBuildsNone(t *testing.T) {
	test.NewTempApp(t)
	c := NewCarousel(NewLabel("1"), NewLabel("2"))
	c.offset = 30
	c.settle()
	assert.False(t, cache.IsRendered(c))
}

// An item appended after the renderer was built is on the track, gets a dot,
// and shows when paged to (#761). Before the fix the track kept the slice
// header captured at creation, so the new item was laid out but never drawn.
func TestCarousel_AppendedItemIsDrawn(t *testing.T) {
	c, _, _, _ := newCarouselBench(t, true)
	r := test.TempWidgetRenderer(t, c).(*carouselRenderer)
	added := NewLabel("4")
	c.Items = append(c.Items, added)
	c.Refresh()

	assert.Contains(t, r.track.Objects, core.CanvasObject(added), "the track holds the new item")
	assert.Len(t, r.dots, 4, "one dot per item")
	for _, dot := range r.dots {
		assert.Contains(t, r.Objects(), core.CanvasObject(dot), "every dot is drawn")
	}
	c.Next()
	c.Next()
	c.Next()
	assert.Equal(t, 3, c.CurrentItem)
	assert.True(t, added.Visible())
	assert.Equal(t, float32(0), added.Position().X)
}

// Truncating Items below CurrentItem clamps it on Refresh instead of paging
// from out of range (#761).
func TestCarousel_TruncatedItemsClampCurrent(t *testing.T) {
	c, _, _, _ := newCarouselBench(t, true)
	r := test.TempWidgetRenderer(t, c).(*carouselRenderer)
	c.Next()
	c.Next()
	require.Equal(t, 2, c.CurrentItem)

	c.Items = c.Items[:1]
	assert.NotPanics(t, c.Refresh)
	assert.Equal(t, 0, c.CurrentItem)
	assert.Len(t, r.dots, 1)
	assert.True(t, c.Items[0].Visible())

	c.Items = nil
	assert.NotPanics(t, c.Refresh)
	assert.Equal(t, 0, c.CurrentItem)
	assert.Empty(t, r.dots)
}
