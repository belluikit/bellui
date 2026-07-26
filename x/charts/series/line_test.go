package series

import (
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/bellui/canvas"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
)

func makeTestCoordForSeries() *coord.Cartesian2D {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 4)
	yScale.SetRange(0, 100)
	c := coord.NewCartesian2D(xScale, yScale)
	c.SetBounds(coord.Rect{X: 0, Y: 0, W: 500, H: 300})
	return c
}

func TestLineRenderer_BasicData(t *testing.T) {
	cs := makeTestCoordForSeries()
	ls := &LineSeries{
		SeriesName: "test",
		Data:       []float64{10, 50, 90, 30, 70},
	}
	r := NewRenderer(TypeLine)
	if r == nil {
		t.Fatal("no renderer for TypeLine")
	}
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// 5 data points -> 4 line segments (no symbols since SymbolNone by default)
	if len(objs) != 4 {
		t.Errorf("expected 4 line segments, got %d", len(objs))
	}
	// All should be *canvas.Line
	for i, obj := range objs {
		if _, ok := obj.(*canvas.Line); !ok {
			t.Errorf("object %d: expected *canvas.Line, got %T", i, obj)
		}
	}
}

func TestLineRenderer_WithSymbols(t *testing.T) {
	cs := makeTestCoordForSeries()
	ls := &LineSeries{
		SeriesName: "test",
		Data:       []float64{10, 50, 90},
		Symbol:     SymbolCircle,
	}
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// 3 points -> 2 line segments + 3 symbols = 5
	if len(objs) != 5 {
		t.Errorf("expected 5 objects (2 lines + 3 symbols), got %d", len(objs))
	}
}

func TestLineRenderer_NullData(t *testing.T) {
	cs := makeTestCoordForSeries()
	ls := &LineSeries{
		SeriesName: "test",
		Data:       []float64{10, math.NaN(), 90},
	}
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// With NaN in middle, both segments adjacent to NaN are skipped
	// Segment 0->1: point 1 is null, skip
	// Segment 1->2: point 1 is null, skip
	if len(objs) != 0 {
		t.Errorf("expected 0 segments with NaN gap (no ConnectNulls), got %d", len(objs))
	}
}

func TestLineRenderer_EmptyData(t *testing.T) {
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series: &LineSeries{},
		Color:  color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Errorf("expected nil for empty data, got %d objects", len(objs))
	}
}

func TestLineRenderer_AreaStyle(t *testing.T) {
	cs := makeTestCoordForSeries()
	ls := &LineSeries{
		SeriesName: "test",
		Data:       []float64{10, 50, 90},
		AreaStyle:  &AreaStyle{},
	}
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// Should have area fill rectangles + line segments
	if len(objs) < 4 {
		t.Errorf("expected at least 4 objects (area + lines), got %d", len(objs))
	}
}

func TestLineRenderer_DashedLine(t *testing.T) {
	cs := makeTestCoordForSeries()
	ls := &LineSeries{
		SeriesName: "dashed",
		Data:       []float64{10, 50, 90, 30, 70},
		LineStyle:  LineStyle{Dash: []float32{6, 4}},
	}
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// Should produce dash segments instead of 4 solid lines.
	// Each segment is broken into multiple dash segments, so total > 4.
	if len(objs) <= 4 {
		t.Errorf("expected more than 4 objects for dashed line, got %d", len(objs))
	}
	// All objects should be *canvas.Line
	for i, obj := range objs {
		if _, ok := obj.(*canvas.Line); !ok {
			t.Errorf("object %d: expected *canvas.Line, got %T", i, obj)
		}
	}
}

func TestLineRenderer_DashedSingleValue(t *testing.T) {
	cs := makeTestCoordForSeries()
	ls := &LineSeries{
		SeriesName: "dashed-single",
		Data:       []float64{10, 90},
		LineStyle:  LineStyle{Dash: []float32{8}}, // single value: dashLen=gapLen=8
	}
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected dash segments, got 0")
	}
}

func TestResolveDash(t *testing.T) {
	tests := []struct {
		name  string
		dash  []float32
		wantD float32
		wantG float32
	}{
		{"nil", nil, 0, 0},
		{"empty", []float32{}, 0, 0},
		{"zero", []float32{0}, 0, 0},
		{"single", []float32{8}, 8, 8},
		{"pair", []float32{6, 4}, 6, 4},
		{"extra", []float32{6, 4, 2}, 6, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, g := resolveDash(tt.dash)
			if d != tt.wantD || g != tt.wantG {
				t.Errorf("resolveDash(%v) = (%v, %v), want (%v, %v)", tt.dash, d, g, tt.wantD, tt.wantG)
			}
		})
	}
}

func TestLineRenderer_WrongSeriesType(t *testing.T) {
	r := NewRenderer(TypeLine)
	ctx := RenderContext{
		Series: &BarSeries{Data: []float64{1, 2}},
		Color:  color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Error("expected nil when passing wrong series type")
	}
}
