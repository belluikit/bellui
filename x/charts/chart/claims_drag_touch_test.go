//go:build (ci || android || ios || mobile) && !wasm

// The page Scroll is Draggable only where bellui's scroller_mobile.go is
// built, and these tests are about which Draggable a touch drag reaches.

package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// A touch drag goes to the innermost Draggable at its start that claims it
// (event.DragClaimer, #662). These run the drag through the touch handler the
// mobile, embedded and desktop-touch drivers share, over a chart in a page
// (#1266).

// pageFinder finds objects in content the way the drivers' finders do.
type pageFinder struct{ content core.CanvasObject }

func (f *pageFinder) FindObjectAtPositionMatching(pos geo.Position,
	matches func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, int) {
	return intdriver.FindObjectAtPositionMatching(pos, matches, nil, f.content)
}

func (f *pageFinder) CanvasForTouch() core.Canvas { return nil }

// touchSwipe drags contact id from one point to another in ten steps through
// the shared touch handler, then lifts it.
func touchSwipe(h *common.TouchHandler, id int, from, to geo.Position) {
	drag := func(d event.Draggable, ev *event.DragEvent) { d.Dragged(ev) }
	h.TapDown(from, id)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		h.TapMove(geo.NewPos(from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f), id, drag)
	}
	h.TapUp(to, id, func(event.Tappable, *event.PointEvent) {}, func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {}, func(event.DoubleTappable, *event.PointEvent) {},
		func(d event.Draggable, _ *event.DragEvent) { d.DragEnd() })
}

// chartPage puts a 400x200 line chart with DataZoom at the top of a vertical
// page 300 high whose content is 800 high, and returns a touch handler over it.
func chartPage(t *testing.T) (*Chart, *container.Scroll, *common.TouchHandler) {
	t.Helper()
	test.NewTempApp(t)
	c := New(Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series:   []series.Series{&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30, 40, 50, 60, 70, 80}}},
	})
	rest := canvas.NewRectangle(color.Transparent)
	rest.SetMinSize(geo.NewSize(400, 600))
	page := container.NewVScroll(container.NewVBox(container.NewGridWrap(geo.NewSize(400, 200), c), rest))
	w := test.NewTempWindow(t, page)
	w.Resize(geo.NewSize(400, 300))
	page.Resize(geo.NewSize(400, 300))
	if c.Size().Height < 150 {
		t.Fatalf("the chart was laid out %v high, too small for the swipes", c.Size())
	}
	h := &common.TouchHandler{}
	h.InitTouchHandler(&pageFinder{content: page})
	return c, page, h
}

// An upward swipe that starts on a plain chart scrolls the page (#1266): the
// chart does nothing with a drag unless it is zoomed, brushing, drawing, a map
// or 3D, so it leaves the drag to the scroll around it.
func TestChart_SwipeOnPlainChartScrollsThePage(t *testing.T) {
	c, page, h := chartPage(t)
	on := geo.NewPos(200, 100)

	touchSwipe(h, 1, on, on.SubtractXY(0, 80))
	if page.Offset.Y != 80 {
		t.Errorf("the page scrolled to %v, want 80: the chart kept the swipe", page.Offset.Y)
	}
	if c.ZoomWindow() != interact.FullWindow() {
		t.Errorf("the chart's window moved to %v", c.ZoomWindow())
	}

	// A sideways swipe on a plain chart is nobody's: the page scrolls only
	// vertically and the chart has nothing to pan.
	page.ScrollToTop()
	touchSwipe(h, 2, on, on.AddXY(-80, 0))
	if page.Offset.Y != 0 || c.ZoomWindow() != interact.FullWindow() {
		t.Errorf("a sideways swipe moved the page to %v and the window to %v", page.Offset.Y, c.ZoomWindow())
	}
}

// A zoomed chart still pans on a sideways swipe, and leaves an upward one to
// the page, since its pan follows DX only.
func TestChart_ZoomedChartPansSidewaysAndLeavesUpwardSwipes(t *testing.T) {
	c, page, h := chartPage(t)
	c.SetZoomWindow(interact.DataWindow{Start: 0.25, End: 0.5})
	on := geo.NewPos(200, 100)

	touchSwipe(h, 1, on, on.AddXY(-80, 0))
	if got := c.ZoomWindow(); got.Start <= 0.25 {
		t.Errorf("a sideways swipe left the window at %v: the zoomed chart did not pan", got)
	}
	if page.Offset.Y != 0 {
		t.Errorf("the sideways swipe scrolled the page to %v", page.Offset.Y)
	}

	panned := c.ZoomWindow()
	touchSwipe(h, 2, on, on.SubtractXY(0, 80))
	if page.Offset.Y != 80 {
		t.Errorf("an upward swipe on a zoomed chart scrolled the page to %v, want 80", page.Offset.Y)
	}
	if c.ZoomWindow() != panned {
		t.Errorf("the upward swipe moved the window from %v to %v", panned, c.ZoomWindow())
	}
}

// In brush mode the chart takes a drag in any direction: an upward swipe
// draws a brush and leaves the page where it was.
func TestChart_BrushModeKeepsUpwardSwipes(t *testing.T) {
	c, page, h := chartPage(t)
	opt := c.option
	opt.Brush = &interact.BrushConfig{Enabled: true}
	c.SetOption(opt)
	var brushed int
	c.OnBrushSelect = func(interact.BrushSelectEvent) { brushed++ }
	on := geo.NewPos(200, 150)

	touchSwipe(h, 1, on, on.SubtractXY(0, 80))
	if page.Offset.Y != 0 {
		t.Errorf("the swipe scrolled the page to %v; the brushing chart should keep it", page.Offset.Y)
	}
	if brushed != 1 {
		t.Errorf("the swipe brushed %d times, want 1", brushed)
	}
}
