package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/series"
)

// Series renderers that load asynchronously (the choropleth's GeoJSON, #35)
// depend on being handed a repaint seam; without it a completed download would
// sit unseen until some unrelated refresh happened.
func TestChartRenderer_RenderContextCarriesInvalidate(t *testing.T) {
	_ = test.NewApp()

	c := New(Option{Series: []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{1, 2, 3}}}})
	c.Resize(geo.NewSize(400, 300))
	r := &chartRenderer{chart: c}

	ctx := r.seriesRenderContext(nil, nil, coord.Rect{W: 400, H: 300}, color.White, 0, false, 0)
	if ctx.Invalidate == nil {
		t.Fatal("RenderContext.Invalidate must be set for every series render")
	}
}

// invalidate must actually reach the chart and drive a full rebuild — that is
// what turns an arriving download into a visible map.
func TestChartRenderer_InvalidateRebuildsTheChart(t *testing.T) {
	_ = test.NewApp()

	c := New(Option{Series: []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{1, 2, 3}}}})
	c.Resize(geo.NewSize(400, 300))

	r, ok := test.TempWidgetRenderer(t, c).(*chartRenderer)
	if !ok {
		t.Fatal("expected the chart's own renderer")
	}

	r.hasCachedBase = false // only a full rebuild sets this again
	r.invalidate()

	if !r.hasCachedBase {
		t.Error("invalidate() did not rebuild the chart")
	}
}
