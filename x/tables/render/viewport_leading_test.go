package render

import "testing"

// TestViewport_Leading checks that the width before the first column, the
// checkbox-selection column (#1498), counts in column positions and in the
// content width, also when there are no columns after it.
func TestViewport_Leading(t *testing.T) {
	v := &Viewport{Leading: 32, Padding: 8, ColWidths: []float32{100, 50}, TotalCols: 2}
	v.RecomputeColPositions()
	if x, _ := v.ColX(0); x != 32 {
		t.Errorf("ColX(0) = %v, want 32", x)
	}
	if x, _ := v.ColX(1); x != 140 {
		t.Errorf("ColX(1) = %v, want 140", x)
	}
	if w := v.ContentSize().Width; w != 190 {
		t.Errorf("content width = %v, want 190", w)
	}

	empty := &Viewport{Leading: 32}
	empty.RecomputeColPositions()
	if w := empty.ContentSize().Width; w != 32 {
		t.Errorf("content width with no columns = %v, want 32", w)
	}
}
