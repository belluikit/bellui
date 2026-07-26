package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/pool"
)

func benchmarkLineRender(b *testing.B, n int) {
	cs := makeTestCoordForSeries()
	data := make([]float64, n)
	for i := range data {
		data[i] = float64(i%100) + 10
	}
	cs.XScale().SetRange(0, float64(n-1))
	cs.YScale().SetRange(0, 120)

	ls := &LineSeries{
		SeriesName: "bench",
		Data:       data,
		Symbol:     SymbolNone,
	}
	r := NewRenderer(TypeLine)
	p := pool.New()
	ctx := RenderContext{
		Series:   ls,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
		Pool:     p,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Reset()
		r.Render(ctx)
	}
}

func BenchmarkLineRender_100(b *testing.B)   { benchmarkLineRender(b, 100) }
func BenchmarkLineRender_10000(b *testing.B) { benchmarkLineRender(b, 10000) }

func BenchmarkBarRender_1000(b *testing.B) {
	cs := makeTestCoordForSeries()
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i%50) + 5
	}
	cs.XScale().SetRange(0, 999)
	cs.YScale().SetRange(0, 60)

	bs := &BarSeries{
		SeriesName: "bench",
		Data:       data,
	}
	r := NewRenderer(TypeBar)
	p := pool.New()
	ctx := RenderContext{
		Series:   bs,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
		Pool:     p,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Reset()
		r.Render(ctx)
	}
}

func BenchmarkBarRender_1000_NoPool(b *testing.B) {
	cs := makeTestCoordForSeries()
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i%50) + 5
	}
	cs.XScale().SetRange(0, 999)
	cs.YScale().SetRange(0, 60)

	bs := &BarSeries{
		SeriesName: "bench",
		Data:       data,
	}
	r := NewRenderer(TypeBar)
	ctx := RenderContext{
		Series:   bs,
		Coord:    cs,
		Viewport: coord.Rect{X: 0, Y: 0, W: 500, H: 300},
		Color:    color.White,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Render(ctx)
	}
}
