package mathutil

import "testing"

func TestLTTB_SmallData(t *testing.T) {
	data := []float64{1, 2, 3}
	_, indices := DownsampleFloat64(data, 10)
	if len(indices) != 3 {
		t.Errorf("small data should return all points, got %d", len(indices))
	}
}

func TestLTTB_Reduction(t *testing.T) {
	n := 100
	dataX := make([]float64, n)
	dataY := make([]float64, n)
	for i := 0; i < n; i++ {
		dataX[i] = float64(i)
		dataY[i] = float64(i * i)
	}
	indices := LTTB(dataX, dataY, 10)
	if len(indices) != 10 {
		t.Errorf("LTTB should return exactly threshold points, got %d", len(indices))
	}
	// First and last must be preserved
	if indices[0] != 0 {
		t.Error("first index should be 0")
	}
	if indices[len(indices)-1] != n-1 {
		t.Errorf("last index should be %d, got %d", n-1, indices[len(indices)-1])
	}
	// Indices should be sorted
	for i := 1; i < len(indices); i++ {
		if indices[i] <= indices[i-1] {
			t.Errorf("indices not sorted at position %d: %d <= %d", i, indices[i], indices[i-1])
		}
	}
}

func TestDownsampleFloat64(t *testing.T) {
	data := make([]float64, 50)
	for i := range data {
		data[i] = float64(i)
	}
	result, indices := DownsampleFloat64(data, 10)
	if len(result) != 10 || len(indices) != 10 {
		t.Errorf("expected 10 points, got %d results, %d indices", len(result), len(indices))
	}
	for i, idx := range indices {
		if result[i] != data[idx] {
			t.Errorf("result[%d]=%v doesn't match data[%d]=%v", i, result[i], idx, data[idx])
		}
	}
}
