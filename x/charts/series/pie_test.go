package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/x/charts/coord"
)

func TestPieRenderer_BasicSlices(t *testing.T) {
	ps := &PieSeries{
		SeriesName: "test",
		Data: []PieDataItem{
			{Name: "A", Value: 60},
			{Name: "B", Value: 40},
		},
	}
	r := NewRenderer(TypePie)
	if r == nil {
		t.Fatal("no renderer for TypePie")
	}
	ctx := RenderContext{
		Series:   ps,
		Coord:    coord.NewNone(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 400, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected non-empty objects for pie chart")
	}
}

func TestPieRenderer_EmptyData(t *testing.T) {
	r := NewRenderer(TypePie)
	ctx := RenderContext{
		Series: &PieSeries{},
		Color:  color.White,
	}
	objs := r.Render(ctx)
	if objs != nil {
		t.Error("expected nil for empty pie data")
	}
}

func TestPieRenderer_SingleSlice(t *testing.T) {
	ps := &PieSeries{
		SeriesName: "single",
		Data: []PieDataItem{
			{Name: "Only", Value: 100},
		},
	}
	r := NewRenderer(TypePie)
	ctx := RenderContext{
		Series:   ps,
		Coord:    coord.NewNone(),
		Viewport: coord.Rect{X: 0, Y: 0, W: 400, H: 400},
		Color:    color.White,
	}
	objs := r.Render(ctx)
	if len(objs) == 0 {
		t.Error("expected objects for single-slice pie")
	}
}
