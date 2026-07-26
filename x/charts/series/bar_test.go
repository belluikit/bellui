package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"

	"github.com/belluikit/bellui/x/charts/coord"
)

func TestBarRenderer_BasicData(t *testing.T) {
	cs := makeTestCoordForSeries()
	bs := &BarSeries{
		SeriesName: "test",
		Data:       []float64{10, 50, 90},
	}
	r := NewRenderer(TypeBar)
	if r == nil {
		t.Fatal("no renderer for TypeBar")
	}
	ctx := RenderContext{
		Series:   bs,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	// 3 data points -> 3 bar rectangles
	if len(objs) != 3 {
		t.Errorf("expected 3 bar rectangles, got %d", len(objs))
	}
	for i, obj := range objs {
		rect, ok := obj.(*canvas.Rectangle)
		if !ok {
			t.Errorf("object %d: expected *canvas.Rectangle, got %T", i, obj)
			continue
		}
		size := rect.Size()
		if size.Width <= 0 || size.Height <= 0 {
			t.Errorf("bar %d has invalid size: %v", i, size)
		}
	}
}

func TestBarRenderer_NegativeValues(t *testing.T) {
	cs := makeTestCoordForSeries()
	bs := &BarSeries{
		SeriesName: "test",
		Data:       []float64{-10, 50},
	}
	r := NewRenderer(TypeBar)
	ctx := RenderContext{
		Series:   bs,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) != 2 {
		t.Errorf("expected 2 bars, got %d", len(objs))
	}
	// Both should have positive height (negative bars flip)
	for i, obj := range objs {
		rect := obj.(*canvas.Rectangle)
		if rect.Size().Height <= 0 {
			t.Errorf("bar %d has non-positive height", i)
		}
	}
}

func TestBarRenderer_EmptyData(t *testing.T) {
	r := NewRenderer(TypeBar)
	ctx := RenderContext{
		Series: &BarSeries{},
		Color:  color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Error("expected nil for empty data")
	}
}

func TestBarRenderer_Stacking(t *testing.T) {
	cs := makeTestCoordForSeries()
	offsets := map[string][]float64{
		"stack1": {10, 20, 30},
	}
	bs := &BarSeries{
		SeriesName: "test",
		Data:       []float64{5, 10, 15},
		Stack:      "stack1",
	}
	r := NewRenderer(TypeBar)
	ctx := RenderContext{
		Series:       bs,
		Coord:        cs,
		Viewport:     coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:        color.White,
		StackOffsets: offsets,
	}
	objs := r.Render(ctx)
	if len(objs) != 3 {
		t.Errorf("expected 3 stacked bars, got %d", len(objs))
	}
}
