package bellui_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

func BenchmarkPosition_Add(b *testing.B) {
	b.Run("Add()", benchmarkPositionAdd)
	b.Run("AddXY()", benchmarkPositionAddXY)
}

func BenchmarkPosition_Subtract(b *testing.B) {
	b.Run("Subtract()", benchmarkPositionSubtract)
	b.Run("SubtractXY()", benchmarkPositionSubtractXY)
}

func BenchmarkSize_Add(b *testing.B) {
	b.Run("Add()", benchmarkSizeAdd)
	b.Run("AddWidthHeight()", benchmarkSizeAddWidthHeight)
}

func BenchmarkSize_Subtract(b *testing.B) {
	b.Run("Subtract()", benchmarkSizeSubtract)
	b.Run("SubtractWidthHeight()", benchmarkSizeSubtractWidthHeight)
}

var (
	benchmarkSize geo.Size
	benchmarkPos  geo.Position
)

func benchmarkPositionAdd(b *testing.B) {
	pos := geo.NewPos(10, 10)
	for n := 0; n < b.N; n++ {
		pos = pos.Add(geo.NewPos(float32(n), float32(n)))
	}
	benchmarkPos = pos
}

func benchmarkPositionAddXY(b *testing.B) {
	pos := geo.NewPos(10, 10)
	for n := 0; n < b.N; n++ {
		pos = pos.AddXY(float32(n), float32(n))
	}
	benchmarkPos = pos
}

func benchmarkPositionSubtract(b *testing.B) {
	pos := geo.NewPos(10, 10)
	for n := 0; n < b.N; n++ {
		pos = pos.Subtract(geo.NewPos(float32(n), float32(n)))
	}
	benchmarkPos = pos
}

func benchmarkPositionSubtractXY(b *testing.B) {
	pos := geo.NewPos(10, 10)
	for n := 0; n < b.N; n++ {
		pos = pos.SubtractXY(float32(n), float32(n))
	}
	benchmarkPos = pos
}

func benchmarkSizeAdd(b *testing.B) {
	size := geo.NewSize(10, 10)
	for n := 0; n < b.N; n++ {
		size = size.Add(geo.NewPos(float32(n), float32(n)))
	}
	benchmarkSize = size
}

func benchmarkSizeAddWidthHeight(b *testing.B) {
	size := geo.NewSize(10, 10)
	for n := 0; n < b.N; n++ {
		size = size.AddWidthHeight(float32(n), float32(n))
	}
	benchmarkSize = size
}

func benchmarkSizeSubtract(b *testing.B) {
	size := geo.NewSize(10, 10)
	for n := 0; n < b.N; n++ {
		size = size.Subtract(geo.NewSize(float32(n), float32(n)))
	}
	benchmarkSize = size
}

func benchmarkSizeSubtractWidthHeight(b *testing.B) {
	size := geo.NewSize(10, 10)
	for n := 0; n < b.N; n++ {
		size = size.SubtractWidthHeight(float32(n), float32(n))
	}
	benchmarkSize = size
}
