package render

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

func BenchmarkViewport_UniformRows_100K(b *testing.B) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        100000,
		TotalCols:        20,
		ColWidths:        make([]float32, 20),
	}
	for i := range vp.ColWidths {
		vp.ColWidths[i] = 100
	}
	vp.RecomputeColPositions()

	b.ResetTimer()
	for range b.N {
		vp.Update(geo.NewPos(500, 50000), geo.NewSize(800, 600))
	}
}

func BenchmarkViewport_UniformRows_1M(b *testing.B) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        1000000,
		TotalCols:        50,
		ColWidths:        make([]float32, 50),
	}
	for i := range vp.ColWidths {
		vp.ColWidths[i] = 120
	}
	vp.RecomputeColPositions()

	b.ResetTimer()
	for range b.N {
		vp.Update(geo.NewPos(1000, 500000), geo.NewSize(1920, 1080))
	}
}
