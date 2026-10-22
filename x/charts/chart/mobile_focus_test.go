package chart

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

func focusTestChart(t *testing.T, mobile bool) (*Chart, func() bool) {
	t.Helper()
	old := isMobile
	isMobile = func() bool { return mobile }
	t.Cleanup(func() { isMobile = old })

	test.NewTempApp(t)
	c := New(Option{Series: []series.Series{
		&series.LineSeries{SeriesName: "Data", Data: []float64{1, 2, 3, 4}},
	}})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 600))
	test.TempWidgetRenderer(t, c).Refresh() // lays out the plot, so taps map to data
	if _, _, ok := c.pixelToData(geo.NewPos(400, 300)); !ok {
		t.Fatal("the chart has no coordinate system to tap into")
	}
	return c, func() bool { return w.Canvas().Focused() == c }
}

func tapAt(c *Chart, x, y float32) { c.Tapped(&event.PointEvent{Position: geo.NewPos(x, y)}) }

// On a phone or tablet the chart takes the focus, and so the on-screen
// keyboard, only while its Text tool waits for text (#415).
func TestChart_MobileFocusOnlyForText(t *testing.T) {
	c, focused := focusTestChart(t, true)

	tapAt(c, 400, 300)
	if focused() {
		t.Fatal("a plain tap focused the chart on mobile")
	}

	c.SetDrawingTool(interact.DrawingToolTextLabel)
	tapAt(c, 400, 300)
	if !focused() {
		t.Fatal("a Text tool tap left the chart without the focus, so no keyboard to type with")
	}
	c.TypedRune('h')
	c.TypedRune('i')
	c.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	if focused() {
		t.Error("the chart kept the focus after Return finished the label")
	}
	if n := len(c.drawingStore.All()); n != 1 {
		t.Errorf("%d drawings after typing a label, want 1", n)
	}

	tapAt(c, 300, 300)
	if !focused() {
		t.Fatal("a second label's first tap left the chart without the focus")
	}
	c.SetDrawingTool(interact.DrawingToolNone)
	if focused() {
		t.Error("the chart kept the focus after the Text tool was switched off")
	}
}

// On desktop a tap focuses the chart, for its keyboard shortcuts.
func TestChart_DesktopTapFocuses(t *testing.T) {
	c, focused := focusTestChart(t, false)
	tapAt(c, 400, 300)
	if !focused() {
		t.Fatal("a tap did not focus the chart on desktop")
	}
}
