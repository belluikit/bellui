package component

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/interact"
)

func TestRenderTooltip_Hidden(t *testing.T) {
	resolver := testResolver()
	state := &TooltipState{Visible: false}
	objs := RenderTooltip(state, nil, resolver, nil, geo.NewSize(800, 600), PlotArea{})
	if objs != nil {
		t.Errorf("expected nil objects for hidden tooltip, got %d", len(objs))
	}
}

func TestRenderTooltip_NilState(t *testing.T) {
	resolver := testResolver()
	objs := RenderTooltip(nil, nil, resolver, nil, geo.NewSize(800, 600), PlotArea{})
	if objs != nil {
		t.Errorf("expected nil objects for nil state, got %d", len(objs))
	}
}

func TestRenderTooltip_NoHits(t *testing.T) {
	resolver := testResolver()
	state := &TooltipState{Visible: true, Hits: nil}
	objs := RenderTooltip(state, nil, resolver, nil, geo.NewSize(800, 600), PlotArea{})
	if objs != nil {
		t.Errorf("expected nil objects for empty hits, got %d", len(objs))
	}
}

func TestRenderTooltip_Visible(t *testing.T) {
	resolver := testResolver()
	hits := []interact.HitResult{
		{
			SeriesIndex: 0,
			DataIndex:   2,
			SeriesName:  "Sales",
			Value:       interact.DataValue{X: 2, Y: 100},
			PixelPos:    geo.NewPos(200, 150),
		},
	}
	seriesColors := map[int]color.Color{
		0: color.NRGBA{R: 255, A: 255},
	}
	state := &TooltipState{
		Visible:  true,
		Hits:     hits,
		MousePos: geo.NewPos(200, 150),
	}
	plotArea := PlotArea{X: 60, Y: 40, W: 680, H: 520}
	objs := RenderTooltip(state, nil, resolver, seriesColors, geo.NewSize(800, 600), plotArea)

	// Should have: bg + border + (marker + text) per line + crosshair
	if len(objs) < 4 {
		t.Errorf("expected at least 4 objects (bg, border, marker, text), got %d", len(objs))
	}
}

func TestRenderTooltip_MultipleHits(t *testing.T) {
	resolver := testResolver()
	hits := []interact.HitResult{
		{SeriesIndex: 0, SeriesName: "A", Value: interact.DataValue{Y: 10}, PixelPos: geo.NewPos(100, 100)},
		{SeriesIndex: 1, SeriesName: "B", Value: interact.DataValue{Y: 20}, PixelPos: geo.NewPos(100, 80)},
	}
	seriesColors := map[int]color.Color{
		0: color.NRGBA{R: 255, A: 255},
		1: color.NRGBA{G: 255, A: 255},
	}
	state := &TooltipState{Visible: true, Hits: hits, MousePos: geo.NewPos(100, 90)}
	plotArea := PlotArea{X: 0, Y: 0, W: 400, H: 300}
	objs := RenderTooltip(state, nil, resolver, seriesColors, geo.NewSize(400, 300), plotArea)

	// bg + border + 2*(marker + text) + crosshair = at least 7
	if len(objs) < 7 {
		t.Errorf("expected at least 7 objects for 2 hits, got %d", len(objs))
	}
}

func TestRenderTooltip_NoCrosshair(t *testing.T) {
	resolver := testResolver()
	hits := []interact.HitResult{
		{SeriesIndex: 0, SeriesName: "A", Value: interact.DataValue{Y: 10}, PixelPos: geo.NewPos(100, 100)},
	}
	seriesColors := map[int]color.Color{0: color.White}
	state := &TooltipState{Visible: true, Hits: hits, MousePos: geo.NewPos(100, 100)}
	cfg := &TooltipConfig{Crosshair: CrosshairNone}
	plotArea := PlotArea{X: 0, Y: 0, W: 400, H: 300}
	objs := RenderTooltip(state, cfg, resolver, seriesColors, geo.NewSize(400, 300), plotArea)

	// bg + border + marker + text = 4 (no crosshair)
	if len(objs) != 4 {
		t.Errorf("expected 4 objects with no crosshair, got %d", len(objs))
	}
}

func TestRenderTooltip_CustomFormatter(t *testing.T) {
	resolver := testResolver()
	hits := []interact.HitResult{
		{SeriesIndex: 0, SeriesName: "A", Value: interact.DataValue{Y: 42}, PixelPos: geo.NewPos(100, 100)},
	}
	seriesColors := map[int]color.Color{0: color.White}
	state := &TooltipState{Visible: true, Hits: hits, MousePos: geo.NewPos(100, 100)}
	cfg := &TooltipConfig{
		Crosshair: CrosshairNone,
		Formatter: func(hits []interact.HitResult) string {
			return "custom text"
		},
	}
	plotArea := PlotArea{X: 0, Y: 0, W: 400, H: 300}
	objs := RenderTooltip(state, cfg, resolver, seriesColors, geo.NewSize(400, 300), plotArea)

	if len(objs) == 0 {
		t.Error("expected objects with custom formatter")
	}
}
