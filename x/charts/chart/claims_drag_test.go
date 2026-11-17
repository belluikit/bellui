package chart

import (
	"testing"

	fynecanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// The chart claims a touch drag exactly when resolveDrag acts on it (#1266):
// claiming one it ignores keeps a swipe from the page, and refusing one it
// uses breaks the gesture. Each mode is checked both ways, up and sideways.
func TestChart_ClaimsDragMatchesWhatItActsOn(t *testing.T) {
	test.NewTempApp(t)
	modes := []struct {
		name  string
		setUp func(c *Chart)
	}{
		{"plain", func(*Chart) {}},
		{"zoomed", func(c *Chart) { c.SetZoomWindow(interact.DataWindow{Start: 0.25, End: 0.5}) }},
		{"measure tool", func(c *Chart) { c.SetDrawingTool(interact.DrawingToolMeasure) }},
		{"trend line tool", func(c *Chart) { c.SetDrawingTool(interact.DrawingToolTrendLine) }},
		{"brush", func(c *Chart) {
			opt := c.option
			opt.Brush = &interact.BrushConfig{Enabled: true}
			c.SetOption(opt)
		}},
		{"geo", func(c *Chart) {
			c.isGeo = true
			c.geoCoord = &coord.GeoCoord{Zoom: 5}
		}},
		{"3d", func(c *Chart) {
			c.is3D = true
			c.scene3D = fynecanvas.NewScene3D()
			c.orbitCam = fynecanvas.NewOrbitCamera(c.scene3D.Camera)
		}},
	}
	// Straight along each axis: a zoomed chart's pan would also follow the
	// small DX of a mostly upward drag, which the claim leaves to the page on
	// purpose (TestChart_ZoomedChartLeavesMostlyUpwardDrags).
	drags := map[string]geo.Delta{"up": {DY: -20}, "sideways": {DX: -20}}

	for _, m := range modes {
		for dir, d := range drags {
			c := New(Option{
				DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
				Series:   []series.Series{&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30, 40}}},
			})
			c.Resize(geo.NewSize(400, 300))
			m.setUp(c)

			claims := c.ClaimsDrag(d)
			acts := c.resolveDrag(&event.DragEvent{
				PointEvent: event.PointEvent{Position: geo.NewPos(200, 150)},
				Dragged:    d,
			}).kind != ""
			if claims != acts {
				t.Errorf("%s, %s: ClaimsDrag %v, but the chart acts on the drag: %v", m.name, dir, claims, acts)
			}
		}
	}
}

// A zoomed chart leaves a mostly upward drag to the page even though its pan
// would have followed the drag's small sideways part: the finger is scrolling
// the page, not panning the data.
func TestChart_ZoomedChartLeavesMostlyUpwardDrags(t *testing.T) {
	test.NewTempApp(t)
	c := New(Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series:   []series.Series{&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30, 40}}},
	})
	c.SetZoomWindow(interact.DataWindow{Start: 0.25, End: 0.5})
	if c.ClaimsDrag(geo.Delta{DX: 3, DY: -20}) {
		t.Error("a zoomed chart claimed a mostly upward drag")
	}
	if !c.ClaimsDrag(geo.Delta{DX: -20, DY: 3}) {
		t.Error("a zoomed chart left a mostly sideways drag to the page")
	}
}

// A selected drawing moves only under a drag that starts on it, and the claim
// cannot see where a drag starts, so the chart claims every drag while a
// drawing is selected, as it took every drag before #1266.
func TestChart_ClaimsEveryDragWhileADrawingIsSelected(t *testing.T) {
	test.NewTempApp(t)
	c := New(Option{})
	c.selectedID = "d1"
	if !c.ClaimsDrag(geo.Delta{DY: -20}) || !c.ClaimsDrag(geo.Delta{DX: -20}) {
		t.Error("a chart with a selected drawing left a drag to the page")
	}
}
