package component

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
)

func makeTestCoordSystem() coord.System {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 100)
	yScale.SetRange(0, 100)
	cs := coord.NewCartesian2D(xScale, yScale)
	cs.SetBounds(coord.Rect{X: 50, Y: 20, W: 400, H: 300})
	return cs
}

func TestRenderOverlays_ReferenceLine(t *testing.T) {
	cs := makeTestCoordSystem()
	plotRect := coord.Rect{X: 50, Y: 20, W: 400, H: 300}
	// Use a line without label to avoid resolver dependency
	lines := []OverlayReferenceLine{
		{Axis: AxisHorizontal, Value: 50, Color: color.White},
	}
	objs := RenderOverlays(lines, nil, nil, []coord.System{cs}, plotRect, nil)
	if len(objs) == 0 {
		t.Error("expected objects for reference line")
	}
}

func TestRenderOverlays_ReferenceBand(t *testing.T) {
	cs := makeTestCoordSystem()
	plotRect := coord.Rect{X: 50, Y: 20, W: 400, H: 300}
	bands := []OverlayReferenceBand{
		{Axis: AxisHorizontal, From: 20, To: 80},
	}
	objs := RenderOverlays(nil, bands, nil, []coord.System{cs}, plotRect, nil)
	if len(objs) == 0 {
		t.Error("expected objects for reference band")
	}
}

func TestRenderOverlays_Annotation(t *testing.T) {
	cs := makeTestCoordSystem()
	plotRect := coord.Rect{X: 50, Y: 20, W: 400, H: 300}
	anns := []OverlayAnnotation{
		{X: 50, Y: 50, Text: "Hello", Arrow: true, Color: color.White, TextSize: 12},
	}
	objs := RenderOverlays(nil, nil, anns, []coord.System{cs}, plotRect, nil)
	if len(objs) < 3 { // bg rect + text + arrow + dot
		t.Errorf("expected >= 3 objects for annotation with arrow, got %d", len(objs))
	}
}

func TestRenderOverlays_OutOfBounds(t *testing.T) {
	cs := makeTestCoordSystem()
	plotRect := coord.Rect{X: 50, Y: 20, W: 400, H: 300}
	// Value 200 is outside the Y range [0, 100] so pixel position outside plotRect
	lines := []OverlayReferenceLine{
		{Axis: AxisHorizontal, Value: 200, Color: color.White},
	}
	objs := RenderOverlays(lines, nil, nil, []coord.System{cs}, plotRect, nil)
	if len(objs) != 0 {
		t.Error("expected no objects for out-of-bounds reference line")
	}
}

func TestDashedLine(t *testing.T) {
	objs := renderDashedLine(
		geo.NewPos(0, 0), geo.NewPos(100, 0),
		color.White, 1, 10, 5,
	)
	// 100 pixels / (10+5) = ~6-7 segments
	if len(objs) < 5 || len(objs) > 10 {
		t.Errorf("expected 5-10 dash segments, got %d", len(objs))
	}
}
