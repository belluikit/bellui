package mathutil

import (
	"math"
	"testing"
)

func BenchmarkLTTB_10000(b *testing.B) {
	n := 10000
	dataX := make([]float64, n)
	dataY := make([]float64, n)
	for i := 0; i < n; i++ {
		dataX[i] = float64(i)
		dataY[i] = math.Sin(float64(i)*0.01) * 100
	}

	threshold := 500

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LTTB(dataX, dataY, threshold)
	}
}
