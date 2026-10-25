package chart

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// A drill-down from a click carries the clicked point, and every way back up,
// a breadcrumb click included, emits EventDrillUp and restores the parent's
// zoom (#343).
func TestChart_DrilldownEventsCarryHitAndFireOnBreadcrumb(t *testing.T) {
	test.NewTempApp(t)
	child := func() *Option {
		return &Option{Series: []series.Series{&series.BarSeries{SeriesName: "child", Data: []float64{1, 2}}}}
	}
	c := New(Option{
		XAxis:     []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis:     []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series:    []series.Series{&series.BarSeries{SeriesName: "s", Data: []float64{3, 5, 4}}},
		DataZoom:  &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Drilldown: &DrilldownConfig{DataProvider: func(interact.HitResult, []string) *Option { return child() }},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	test.TempWidgetRenderer(t, c).Refresh()

	var down, up []interact.DrilldownEvent
	c.Events.On(interact.EventDrilldown, func(e interface{}) { down = append(down, e.(interact.DrilldownEvent)) })
	c.Events.On(interact.EventDrillUp, func(e interface{}) { up = append(up, e.(interact.DrilldownEvent)) })

	bar := c.renderSnap.coordSystems[0].DataToPixel(1, 2.5)
	c.Tapped(&event.PointEvent{Position: bar})
	if len(down) != 1 {
		t.Fatalf("%d drill-down events after clicking a bar, want 1", len(down))
	}
	if down[0].Hit == nil || down[0].Hit.DataIndex != 1 {
		t.Errorf("drill-down Hit = %+v, want the clicked bar (data index 1)", down[0].Hit)
	}

	c.DrillInto("deeper", *child())
	if got := len(c.DrillPath()); got != 2 {
		t.Fatalf("drill path has %d levels, want 2", got)
	}

	c.drillToDepth(0) // the breadcrumb for the top-level chart
	if len(up) != 1 {
		t.Fatalf("%d drill-up events after a breadcrumb click, want 1", len(up))
	}
	if len(up[0].Path) != 0 || len(c.DrillPath()) != 0 {
		t.Errorf("after the breadcrumb click the path is %v, want the top level", c.DrillPath())
	}
	if c.zoomState == nil {
		t.Error("back at the top level the chart lost the top level's zoom")
	}

	c.DrillUp() // nothing left to go up to
	if len(up) != 1 {
		t.Errorf("DrillUp at the top level emitted an event")
	}
}
