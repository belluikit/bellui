package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"

	"github.com/belluikit/bellui/x/charts/coord"
)

func TestScatterRenderer_BasicData(t *testing.T) {
	cs := makeTestCoordForSeries()
	ss := &ScatterSeries{
		SeriesName: "test",
		Data:       []float64{1, 10, 2, 50, 3, 90}, // 3 points
	}
	r := NewRenderer(TypeScatter)
	if r == nil {
		t.Fatal("no renderer for TypeScatter")
	}
	ctx := RenderContext{
		Series:   ss,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// 3 data points -> 3 symbols (default SymbolCircle)
	if len(objs) != 3 {
		t.Errorf("expected 3 scatter symbols, got %d", len(objs))
	}
	for i, obj := range objs {
		if _, ok := obj.(*canvas.Circle); !ok {
			t.Errorf("object %d: expected *canvas.Circle (default symbol), got %T", i, obj)
		}
	}
}

func TestScatterRenderer_RectSymbol(t *testing.T) {
	cs := makeTestCoordForSeries()
	ss := &ScatterSeries{
		SeriesName: "test",
		Data:       []float64{1, 10},
		Symbol:     SymbolRect,
	}
	r := NewRenderer(TypeScatter)
	ctx := RenderContext{
		Series:   ss,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if _, ok := objs[0].(*canvas.Rectangle); !ok {
		t.Errorf("expected *canvas.Rectangle for SymbolRect, got %T", objs[0])
	}
}

func TestScatterRenderer_EmptyData(t *testing.T) {
	r := NewRenderer(TypeScatter)
	ctx := RenderContext{
		Series: &ScatterSeries{},
		Color:  color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Error("expected nil for empty data")
	}
}
