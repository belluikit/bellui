//go:build (ci || android || ios || mobile) && (!windows || !ci) && !wasm

// The page Scroll is draggable only where internal/widget/scroller_mobile.go
// is built, which is what this test is about: one finger drags, two pan.

package mobile

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// chartLike keeps one-finger drags and declines touch scrolls, the way an
// x/charts chart does, so two fingers over it must scroll the page instead.
type chartLike struct {
	widget.BaseWidget
	dragged  int
	scrolls  []event.ScrollSource
	declined int
}

func newChartLike() *chartLike {
	c := &chartLike{}
	c.ExtendBaseWidget(c)
	return c
}

func (c *chartLike) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(belluicanvas.NewRectangle(color.Gray{Y: 0x80}))
}

func (c *chartLike) MinSize() geo.Size              { return geo.NewSize(100, 100) }
func (c *chartLike) Dragged(*event.DragEvent)       { c.dragged++ }
func (c *chartLike) DragEnd()                       {}
func (c *chartLike) Scrolled(ev *event.ScrollEvent) { c.TryScrolled(ev) }
func (c *chartLike) TryScrolled(ev *event.ScrollEvent) bool {
	c.scrolls = append(c.scrolls, ev.Source)
	if ev.Source == event.ScrollSourceTouch {
		c.declined++
		return false
	}
	return true
}

// panPage lays out a page taller than the canvas with a chart-like widget at
// its top, and returns the page's Scroll and the widget.
func panPage(t *testing.T) (*canvas, *container.Scroll, *chartLike) {
	t.Helper()
	chart := newChartLike()
	filler := belluicanvas.NewRectangle(color.Transparent)
	filler.SetMinSize(geo.NewSize(100, 400))
	page := container.NewVScroll(container.NewVBox(chart, filler))
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(page)
	c.Resize(geo.NewSize(120, 200))
	require.Zero(t, page.Offset.Y)
	return c, page, chart
}

// Two fingers moving up together over the chart scroll the page: the steps
// reach the chart first, as touch scrolls, and it passes them on (#402).
func Test_canvas_TwoFingerPanScrollsThePage(t *testing.T) {
	c, page, chart := panPage(t)
	drag := func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }

	c.TapDown(geo.NewPos(30, 60), 0)
	c.TapDown(geo.NewPos(60, 60), 1)
	for y := float32(55); y >= 35; y -= 5 {
		c.TapMove(geo.NewPos(30, y), 0, drag)
		c.TapMove(geo.NewPos(60, y), 1, drag)
	}

	assert.Greater(t, page.Offset.Y, float32(0), "the page scrolled down as the fingers moved up")
	assert.NotEmpty(t, chart.scrolls, "the chart under the fingers was offered the scroll first")
	for _, src := range chart.scrolls {
		assert.Equal(t, event.ScrollSourceTouch, src)
	}
	assert.Equal(t, len(chart.scrolls), chart.declined, "the chart passed every touch step on")
	assert.Zero(t, chart.dragged, "two fingers do not drag the chart")

	// Releasing the pan is not a tap.
	tapped := false
	tap := func(event.Tappable, *event.PointEvent) { tapped = true }
	c.TapUp(geo.NewPos(30, 35), 0, tap, nil, nil, nil, drag)
	c.TapUp(geo.NewPos(60, 35), 1, tap, nil, nil, nil, drag)
	assert.False(t, tapped)
}

// One finger over the chart still drags the chart, not the page.
func Test_canvas_OneFingerStillDragsTheChart(t *testing.T) {
	c, page, chart := panPage(t)
	drag := func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }

	c.TapDown(geo.NewPos(30, 60), 0)
	c.TapMove(geo.NewPos(30, 40), 0, drag)
	c.TapMove(geo.NewPos(30, 30), 0, drag)

	assert.Equal(t, 2, chart.dragged)
	assert.Zero(t, page.Offset.Y)
	assert.Empty(t, chart.scrolls)
}

// A flicked pan carries on after the release, and a finger landing stops it.
func Test_canvas_TwoFingerFlickMomentumStopsOnTouch(t *testing.T) {
	c, page, _ := panPage(t)
	drag := func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }

	c.TapDown(geo.NewPos(30, 90), 0)
	c.TapDown(geo.NewPos(60, 90), 1)
	for y := float32(80); y >= 40; y -= 10 { // 10 per frame: a flick
		c.TapMove(geo.NewPos(30, y), 0, drag)
		c.TapMove(geo.NewPos(60, y), 1, drag)
	}
	c.TapUp(geo.NewPos(30, 40), 0, nil, nil, nil, nil, drag)
	c.TapUp(geo.NewPos(60, 40), 1, nil, nil, nil, nil, drag)
	released := offsetOnMain(page)

	time.Sleep(80 * time.Millisecond)
	moving := offsetOnMain(page)
	assert.Greater(t, moving, released, "the flick carried on after the release")

	bellui.DoAndWait(func() { c.TapDown(geo.NewPos(30, 100), 2) })
	stopped := offsetOnMain(page)
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, stopped, offsetOnMain(page), "a finger landing stopped the flick")
}

// offsetOnMain reads the scroll offset on the main goroutine, where the
// momentum writes it.
func offsetOnMain(s *container.Scroll) float32 {
	var y float32
	bellui.DoAndWait(func() { y = s.Offset.Y })
	return y
}

// A finger that lands on a page coasting from a two-finger flick stops it
// and taps nothing under it; once the page is at rest, a tap taps (#792).
func Test_canvas_TouchThatStopsPanMomentumDoesNotTap(t *testing.T) {
	var taps int
	button := widget.NewButton("under the finger", func() {})
	filler := belluicanvas.NewRectangle(color.Transparent)
	filler.SetMinSize(geo.NewSize(100, 400))
	page := container.NewVScroll(container.NewVBox(container.NewGridWrap(geo.NewSize(120, 600), button), filler))
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(page)
	c.Resize(geo.NewSize(120, 200))
	drag := func(wid event.Draggable, ev *event.DragEvent) { wid.Dragged(ev) }
	tap := func(event.Tappable, *event.PointEvent) { taps++ }

	c.TapDown(geo.NewPos(30, 190), 0)
	c.TapDown(geo.NewPos(60, 190), 1)
	for y := float32(180); y >= 140; y -= 10 { // 10 per frame: a flick
		c.TapMove(geo.NewPos(30, y), 0, drag)
		c.TapMove(geo.NewPos(60, y), 1, drag)
	}
	c.TapUp(geo.NewPos(30, 140), 0, tap, nil, nil, nil, drag)
	c.TapUp(geo.NewPos(60, 140), 1, tap, nil, nil, nil, drag)
	released := offsetOnMain(page)
	time.Sleep(80 * time.Millisecond)
	require.Greater(t, offsetOnMain(page), released, "the flick carries on after the release")

	bellui.DoAndWait(func() {
		c.TapDown(geo.NewPos(60, 100), 2)
		c.TapUp(geo.NewPos(60, 100), 2, tap, nil, nil, nil, drag)
	})
	assert.Zero(t, taps, "the touch that stopped the flick tapped the button")

	time.Sleep(80 * time.Millisecond)
	bellui.DoAndWait(func() {
		c.TapDown(geo.NewPos(60, 100), 3)
		c.TapUp(geo.NewPos(60, 100), 3, tap, nil, nil, nil, drag)
	})
	assert.Equal(t, 1, taps, "a tap on the page at rest taps the button")
}
