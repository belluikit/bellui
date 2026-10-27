package chart

import (
	"testing"

	"github.com/belluikit/bellui"
	fynecanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// drawnTexts counts the canvas texts the chart's renderer draws.
func drawnTexts(t *testing.T, c *Chart) map[string]int {
	t.Helper()
	r := test.TempWidgetRenderer(t, c)
	r.Refresh()
	out := map[string]int{}
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		switch v := o.(type) {
		case *fynecanvas.Text:
			out[v.Text]++
		case *bellui.Container:
			for _, child := range v.Objects {
				walk(child)
			}
		}
	}
	for _, o := range r.Objects() {
		walk(o)
	}
	return out
}

// The first label typed after choosing the Text tool stays, in the store and
// on screen, while later labels are added (#795). The chart is shaped like the
// gallery's Drawing Tools demo, where it was seen: a category axis, an axis
// tooltip and zoom.
func TestChart_FirstTextLabelStays(t *testing.T) {
	old := isMobile
	isMobile = func() bool { return true }
	t.Cleanup(func() { isMobile = old })
	test.NewTempApp(t)
	c := New(Option{
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairBoth},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.1, MaxSpan: 1.0, ZoomSensitivity: 0.1},
		Grid:     &Grid{Left: 50, Right: 20, Top: 50, Bottom: 40},
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}}},
		YAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series:   []series.Series{&series.LineSeries{SeriesName: "Price", Data: []float64{100, 105, 103, 108, 112, 110, 115, 118, 116, 120}}},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 300))
	drawnTexts(t, c) // lays out the plot, so taps map to data

	c.SetDrawingTool(interact.DrawingToolTextLabel)
	for i, label := range []string{"first", "second", "third"} {
		c.Tapped(&event.PointEvent{Position: geo.NewPos(float32(300+100*i), 150)})
		for _, r := range label {
			c.TypedRune(r)
		}
		c.TypedKey(&event.KeyEvent{Name: event.KeyReturn})

		if n := len(c.Drawings()); n != i+1 {
			t.Fatalf("after label %q: %d drawings in the store, want %d", label, n, i+1)
		}
		drawn := drawnTexts(t, c)
		if drawn["first"] != 1 {
			t.Errorf("after label %q: the first label is drawn %d times, want once", label, drawn["first"])
		}
		if drawn[label] != 1 {
			t.Errorf("label %q is drawn %d times, want once", label, drawn[label])
		}
	}
}
