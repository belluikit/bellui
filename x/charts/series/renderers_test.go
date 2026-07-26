package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
)

// makeNoneCoord creates a None coordinate system with a standard viewport.
func makeNoneCoord() *coord.None {
	c := coord.NewNone()
	c.SetBounds(coord.Rect{X: 0, Y: 0, W: 500, H: 400})
	return c
}

func TestRadarRenderer(t *testing.T) {
	rs := &RadarSeries{
		SeriesName: "skills",
		Indicators: []RadarIndicator{
			{Name: "Speed", Max: 100},
			{Name: "Power", Max: 100},
			{Name: "Defense", Max: 100},
			{Name: "Magic", Max: 100},
			{Name: "Stamina", Max: 100},
		},
		Data:     [][]float64{{80, 60, 70, 90, 50}},
		ShowArea: true,
	}
	r := NewRenderer(TypeRadar)
	if r == nil {
		t.Fatal("no renderer for TypeRadar")
	}
	ctx := RenderContext{
		Series:   rs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from radar renderer")
	}
}

func TestRadarRenderer_EmptyData(t *testing.T) {
	rs := &RadarSeries{SeriesName: "empty"}
	r := NewRenderer(TypeRadar)
	ctx := RenderContext{
		Series:   rs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Errorf("expected nil for empty radar, got %d objects", len(objs))
	}
}

func TestGaugeRenderer(t *testing.T) {
	gs := &GaugeSeries{
		SeriesName: "speed",
		Value:      72,
		Min:        0,
		Max:        100,
		ShowValue:  true,
	}
	r := NewRenderer(TypeGauge)
	if r == nil {
		t.Fatal("no renderer for TypeGauge")
	}
	ctx := RenderContext{
		Series:   gs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 400, H: 300},
		Color:    color.NRGBA{R: 91, G: 143, B: 249, A: 255},
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from gauge renderer")
	}
}

func TestFunnelRenderer(t *testing.T) {
	fs := &FunnelSeries{
		SeriesName: "conversion",
		Data: []FunnelData{
			{Name: "Visit", Value: 1000},
			{Name: "Inquiry", Value: 600},
			{Name: "Order", Value: 300},
			{Name: "Purchase", Value: 100},
		},
	}
	r := NewRenderer(TypeFunnel)
	if r == nil {
		t.Fatal("no renderer for TypeFunnel")
	}
	ctx := RenderContext{
		Series:   fs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 400, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from funnel renderer")
	}
}

func TestFunnelRenderer_EmptyData(t *testing.T) {
	fs := &FunnelSeries{SeriesName: "empty"}
	r := NewRenderer(TypeFunnel)
	ctx := RenderContext{
		Series:   fs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 400, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Errorf("expected nil for empty funnel, got %d", len(objs))
	}
}

func TestHeatmapRenderer(t *testing.T) {
	hs := &HeatmapSeries{
		SeriesName: "temps",
		Data:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Rows:       3,
		Cols:       3,
	}
	r := NewRenderer(TypeHeatmap)
	if r == nil {
		t.Fatal("no renderer for TypeHeatmap")
	}
	ctx := RenderContext{
		Series:   hs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 300, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from heatmap renderer")
	}
}

func TestCandlestickRenderer(t *testing.T) {
	cs := &CandlestickSeries{
		SeriesName: "stock",
		Data: []CandlestickData{
			{Open: 100, High: 120, Low: 90, Close: 110},
			{Open: 110, High: 130, Low: 100, Close: 105},
			{Open: 105, High: 115, Low: 95, Close: 112},
		},
	}
	coordSys := makeTestCoordForSeries()
	coordSys.XScale().SetRange(0, 2)
	coordSys.YScale().SetRange(90, 130)

	r := NewRenderer(TypeCandlestick)
	if r == nil {
		t.Fatal("no renderer for TypeCandlestick")
	}
	ctx := RenderContext{
		Series:   cs,
		Coord:    coordSys,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from candlestick renderer")
	}
}

func TestBoxplotRenderer(t *testing.T) {
	bs := &BoxplotSeries{
		SeriesName: "distribution",
		Data: []BoxplotData{
			{Min: 10, Q1: 25, Median: 50, Q3: 75, Max: 90},
			{Min: 15, Q1: 30, Median: 55, Q3: 70, Max: 85},
		},
	}
	coordSys := makeTestCoordForSeries()
	coordSys.XScale().SetRange(0, 1)
	coordSys.YScale().SetRange(0, 100)

	r := NewRenderer(TypeBoxplot)
	if r == nil {
		t.Fatal("no renderer for TypeBoxplot")
	}
	ctx := RenderContext{
		Series:   bs,
		Coord:    coordSys,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.NRGBA{R: 91, G: 143, B: 249, A: 255},
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from boxplot renderer")
	}
}

func TestTreemapRenderer(t *testing.T) {
	ts := &TreemapSeries{
		SeriesName: "disk",
		Data: []TreemapNode{
			{Name: "Documents", Value: 40},
			{Name: "Photos", Value: 30},
			{Name: "Music", Value: 20, Children: []TreemapNode{
				{Name: "Rock", Value: 12},
				{Name: "Jazz", Value: 8},
			}},
		},
	}
	r := NewRenderer(TypeTreemap)
	if r == nil {
		t.Fatal("no renderer for TypeTreemap")
	}
	ctx := RenderContext{
		Series:   ts,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from treemap renderer")
	}
}

func TestSankeyRenderer(t *testing.T) {
	ss := &SankeySeries{
		SeriesName: "flow",
		Nodes: []SankeyNode{
			{Name: "A"},
			{Name: "B"},
			{Name: "C"},
		},
		Links: []SankeyLink{
			{Source: 0, Target: 2, Value: 10},
			{Source: 1, Target: 2, Value: 5},
		},
	}
	r := NewRenderer(TypeSankey)
	if r == nil {
		t.Fatal("no renderer for TypeSankey")
	}
	ctx := RenderContext{
		Series:   ss,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from sankey renderer")
	}
}

func TestSunburstRenderer(t *testing.T) {
	ss := &SunburstSeries{
		SeriesName: "hierarchy",
		Data: []SunburstNode{
			{Name: "Root1", Value: 50},
			{Name: "Root2", Value: 30, Children: []SunburstNode{
				{Name: "Child1", Value: 20},
				{Name: "Child2", Value: 10},
			}},
		},
	}
	r := NewRenderer(TypeSunburst)
	if r == nil {
		t.Fatal("no renderer for TypeSunburst")
	}
	ctx := RenderContext{
		Series:   ss,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 400, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from sunburst renderer")
	}
}

func TestGraphRenderer(t *testing.T) {
	gs := &GraphSeries{
		SeriesName: "network",
		Nodes: []GraphNode{
			{Name: "A"},
			{Name: "B"},
			{Name: "C"},
		},
		Edges: []GraphEdge{
			{Source: 0, Target: 1, Value: 5},
			{Source: 1, Target: 2, Value: 3},
		},
	}
	r := NewRenderer(TypeGraph)
	if r == nil {
		t.Fatal("no renderer for TypeGraph")
	}
	ctx := RenderContext{
		Series:   gs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from graph renderer")
	}
}

func TestTreeRenderer(t *testing.T) {
	ts := &TreeSeries{
		SeriesName: "org",
		Data: TreeNode{
			Name: "CEO",
			Children: []TreeNode{
				{Name: "CTO", Children: []TreeNode{
					{Name: "Dev1"},
					{Name: "Dev2"},
				}},
				{Name: "CFO"},
			},
		},
	}
	r := NewRenderer(TypeTree)
	if r == nil {
		t.Fatal("no renderer for TypeTree")
	}
	ctx := RenderContext{
		Series:   ts,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from tree renderer")
	}
}

func TestParallelRenderer(t *testing.T) {
	ps := &ParallelSeries{
		SeriesName: "metrics",
		Axes: []ParallelAxis{
			{Name: "CPU", Min: 0, Max: 100},
			{Name: "Memory", Min: 0, Max: 16},
			{Name: "Disk", Min: 0, Max: 500},
		},
		Data: [][]float64{
			{80, 8, 200},
			{60, 12, 350},
		},
	}
	r := NewRenderer(TypeParallel)
	if r == nil {
		t.Fatal("no renderer for TypeParallel")
	}
	ctx := RenderContext{
		Series:   ps,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from parallel renderer")
	}
}

func TestPictorialRenderer(t *testing.T) {
	ps := &PictorialSeries{
		SeriesName: "icons",
		Data:       []float64{30, 60, 90, 40},
		Symbol:     SymbolCircle,
	}
	coordSys := makeTestCoordForSeries()
	coordSys.XScale().SetRange(0, 3)
	coordSys.YScale().SetRange(0, 100)

	r := NewRenderer(TypePictorial)
	if r == nil {
		t.Fatal("no renderer for TypePictorial")
	}
	ctx := RenderContext{
		Series:   ps,
		Coord:    coordSys,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.NRGBA{R: 91, G: 143, B: 249, A: 255},
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from pictorial renderer")
	}
}

func TestEffectScatterRenderer(t *testing.T) {
	es := &EffectScatterSeries{
		SeriesName: "effect",
		Data:       []float64{1, 10, 2, 20, 3, 30},
	}
	coordSys := makeTestCoordForSeries()
	coordSys.XScale().SetRange(0, 4)
	coordSys.YScale().SetRange(0, 40)

	r := NewRenderer(TypeEffectScatter)
	if r == nil {
		t.Fatal("no renderer for TypeEffectScatter")
	}
	ctx := RenderContext{
		Series:   es,
		Coord:    coordSys,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.NRGBA{R: 91, G: 143, B: 249, A: 255},
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty output from effect scatter renderer")
	}
}

func TestCustomRenderer(t *testing.T) {
	called := false
	cs := &CustomSeries{
		SeriesName: "custom",
		RenderFunc: func(ctx RenderContext) []core.CanvasObject {
			called = true
			return []core.CanvasObject{container.NewWithoutLayout()}
		},
	}
	r := NewRenderer(TypeCustom)
	if r == nil {
		t.Fatal("no renderer for TypeCustom")
	}
	ctx := RenderContext{
		Series:   cs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if !called {
		t.Error("custom render function was not called")
	}
	if len(objs) == 0 {
		t.Error("expected non-empty output from custom renderer")
	}
}

func TestCustomRenderer_NilFunc(t *testing.T) {
	cs := &CustomSeries{SeriesName: "nil-func"}
	r := NewRenderer(TypeCustom)
	ctx := RenderContext{
		Series: cs,
		Coord:  makeNoneCoord(),
		Color:  color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Errorf("expected nil for nil RenderFunc, got %d", len(objs))
	}
}

func TestBar3DRenderer(t *testing.T) {
	bs := &Bar3DSeries{
		SeriesName: "bars3d",
		Data: []Bar3DData{
			{X: 0, Z: 0, Value: 5},
			{X: 1, Z: 0, Value: 10},
			{X: 0, Z: 1, Value: 7},
		},
	}
	r := NewRenderer(TypeBar3D)
	if r == nil {
		t.Fatal("no renderer for TypeBar3D")
	}
	ctx := RenderContext{
		Series:   bs,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) != 1 {
		t.Errorf("expected 1 object (Scene3D), got %d", len(objs))
	}
}

func TestScatter3DRenderer(t *testing.T) {
	ss := &Scatter3DSeries{
		SeriesName: "scatter3d",
		Data:       []float64{1, 2, 3, 4, 5, 6},
	}
	r := NewRenderer(TypeScatter3D)
	if r == nil {
		t.Fatal("no renderer for TypeScatter3D")
	}
	ctx := RenderContext{
		Series:   ss,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.NRGBA{R: 91, G: 143, B: 249, A: 255},
	}
	objs := r.Render(ctx)
	if len(objs) != 1 {
		t.Errorf("expected 1 object (Scene3D), got %d", len(objs))
	}
}

func TestSurfaceRenderer(t *testing.T) {
	ss := &SurfaceSeries{
		SeriesName: "surface",
		Rows:       3,
		Cols:       3,
		Data:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	r := NewRenderer(TypeSurface)
	if r == nil {
		t.Fatal("no renderer for TypeSurface")
	}
	ctx := RenderContext{
		Series:   ss,
		Coord:    makeNoneCoord(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) != 1 {
		t.Errorf("expected 1 object (Scene3D), got %d", len(objs))
	}
}

// TestAllRegisteredRenderers verifies that every registered type has a renderer.
func TestAllRegisteredRenderers(t *testing.T) {
	types := []Type{
		TypeLine, TypeBar, TypeScatter, TypePie, TypeCandlestick, TypeHeatmap,
		TypeRadar, TypeGauge, TypeFunnel, TypeBoxplot, TypeTreemap, TypeSankey,
		TypeSunburst, TypeGraph, TypeTree, TypeParallel, TypePictorial,
		TypeEffectScatter, TypeCustom, TypeBar3D, TypeScatter3D, TypeSurface,
	}
	for _, tp := range types {
		r := NewRenderer(tp)
		if r == nil {
			t.Errorf("no renderer registered for type %q", tp)
		}
	}
}

// TestCandlestickDataRange verifies DataRange computation.
func TestCandlestickDataRange(t *testing.T) {
	cs := &CandlestickSeries{
		Data: []CandlestickData{
			{Open: 100, High: 120, Low: 90, Close: 110},
			{Open: 110, High: 130, Low: 85, Close: 105},
		},
	}
	xMin, xMax, yMin, yMax := cs.DataRange()
	if xMin != 0 || xMax != 1 {
		t.Errorf("xRange: got [%f, %f], want [0, 1]", xMin, xMax)
	}
	if yMin != 85 || yMax != 130 {
		t.Errorf("yRange: got [%f, %f], want [85, 130]", yMin, yMax)
	}
}

// TestBoxplotDataRange verifies DataRange computation.
func TestBoxplotDataRange(t *testing.T) {
	bs := &BoxplotSeries{
		Data: []BoxplotData{
			{Min: 10, Q1: 25, Median: 50, Q3: 75, Max: 90},
			{Min: 5, Q1: 20, Median: 45, Q3: 70, Max: 95},
		},
	}
	xMin, xMax, yMin, yMax := bs.DataRange()
	if xMin != 0 || xMax != 1 {
		t.Errorf("xRange: got [%f, %f], want [0, 1]", xMin, xMax)
	}
	if yMin != 5 || yMax != 95 {
		t.Errorf("yRange: got [%f, %f], want [5, 95]", yMin, yMax)
	}
}

// makeTestCoordForSeriesWithBounds is a helper for cartesian series tests.
func makeTestCoordForSeriesWithBounds(xMin, xMax, yMin, yMax float64) *coord.Cartesian2D {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(xMin, xMax)
	yScale.SetRange(yMin, yMax)
	c := coord.NewCartesian2D(xScale, yScale)
	c.SetBounds(coord.Rect{X: 0, Y: 0, W: 500, H: 300})
	return c
}
