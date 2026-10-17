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

func pinch(scale float32) *event.PinchEvent {
	return &event.PinchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(400, 300)}, Scale: scale}
}

func wheel(dy float32) *event.ScrollEvent {
	return &event.ScrollEvent{PointEvent: event.PointEvent{Position: geo.NewPos(400, 300)}, Scrolled: geo.Delta{DY: dy}}
}

// The wheel and a pinch share the chart's zoom modes (#379): the 3D orbit
// camera moves 10% per notch or by 1/scale, a map 0.15 levels per notch or
// log2(scale) levels, clamped to 1..18.
func TestChart_WheelAndPinchZoomModes(t *testing.T) {
	_ = test.NewApp()

	t.Run("3d", func(t *testing.T) {
		c := New(Option{})
		c.is3D = true
		c.scene3D = fynecanvas.NewScene3D()
		c.orbitCam = fynecanvas.NewOrbitCamera(c.scene3D.Camera)
		d := c.orbitCam.Distance

		if !c.TryScrolled(wheel(1)) || !near(c.orbitCam.Distance, d*0.9) {
			t.Fatalf("a notch up should move the camera to 0.9x, got %v of %v", c.orbitCam.Distance, d)
		}
		c.TryScrolled(wheel(-1))
		d = c.orbitCam.Distance
		c.Pinched(pinch(1.25))
		if !near(c.orbitCam.Distance, d*0.8) {
			t.Errorf("a 1.25x pinch should move the camera to 0.8x, got %v of %v", c.orbitCam.Distance, d)
		}
	})

	t.Run("geo", func(t *testing.T) {
		c := New(Option{})
		c.isGeo = true
		c.geoCoord = &coord.GeoCoord{Zoom: 5}

		c.TryScrolled(wheel(1))
		if !near(c.geoCoord.Zoom, 5.15) {
			t.Errorf("a notch up should add 0.15 levels, got %v", c.geoCoord.Zoom)
		}
		c.TryScrolled(wheel(-1))
		c.Pinched(pinch(2))
		if !near(c.geoCoord.Zoom, 6) {
			t.Errorf("a 2x pinch should add one level, got %v", c.geoCoord.Zoom)
		}
		c.Pinched(pinch(1 << 20))
		if c.geoCoord.Zoom != 18 {
			t.Errorf("zoom is clamped to 18, got %v", c.geoCoord.Zoom)
		}
	})
}

// A pinch step that only moved the fingers changes no data window.
func TestChart_PinchWithoutScaleChangesNothing(t *testing.T) {
	_ = test.NewApp()
	c := New(Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series:   []series.Series{&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30}}},
	})
	c.Resize(fyne800x600())
	var notified int
	c.OnZoomChange = func(interact.DataWindow) { notified++ }
	c.Pinched(pinch(1))
	if notified != 0 || c.ZoomWindow().Span() != 1 {
		t.Errorf("scale 1 must not zoom, got span %v and %d notifications", c.ZoomWindow().Span(), notified)
	}
}

func near(a, b float32) bool {
	d := a - b
	return d < 1e-4 && d > -1e-4
}
