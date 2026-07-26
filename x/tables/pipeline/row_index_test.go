package pipeline

import "testing"

// DisplayRow inverts SourceRow for rows in view and returns -1 otherwise.
func TestRowIndex_DisplayRoundTrip(t *testing.T) {
	// Display order [2,0,1]: display 0->source 2, display 1->source 0, etc.
	idx := NewRowIndex([]int{2, 0, 1})

	for d := 0; d < idx.Length(); d++ {
		s := idx.SourceRow(d)
		if got := idx.DisplayRow(s); got != d {
			t.Errorf("DisplayRow(SourceRow(%d)=%d) = %d, want %d", d, s, got, d)
		}
	}
}

// A source row that is not in the current view (filtered out) maps to -1, as do
// negative inputs.
func TestRowIndex_DisplayRowNotInView(t *testing.T) {
	idx := NewRowIndex([]int{0, 2}) // source 1 is filtered out

	if got := idx.DisplayRow(1); got != -1 {
		t.Errorf("DisplayRow(1) for filtered-out row = %d, want -1", got)
	}
	if got := idx.DisplayRow(-1); got != -1 {
		t.Errorf("DisplayRow(-1) = %d, want -1", got)
	}
	if got := idx.DisplayRow(99); got != -1 {
		t.Errorf("DisplayRow(out-of-range) = %d, want -1", got)
	}
}

// The reverse map is not built until the first DisplayRow call, so forward-only
// use pays nothing.
func TestRowIndex_ReverseBuiltLazily(t *testing.T) {
	idx := NewRowIndex([]int{2, 0, 1})

	// Forward-only access must not allocate the reverse map.
	_ = idx.SourceRow(0)
	_ = idx.Indices()
	if idx.reverse != nil {
		t.Fatal("reverse map built without a DisplayRow call")
	}

	if idx.DisplayRow(0) != 1 {
		t.Fatalf("DisplayRow(0) = %d, want 1", idx.DisplayRow(0))
	}
	if idx.reverse == nil {
		t.Error("reverse map not built after DisplayRow call")
	}
}

// Synthetic rows (source < 0, e.g. group headers) are skipped and never resolve.
func TestRowIndex_SyntheticRowsSkipped(t *testing.T) {
	idx := NewRowIndex([]int{-1, 0, -1, 1}) // group-header sentinels interleaved

	if got := idx.DisplayRow(0); got != 1 {
		t.Errorf("DisplayRow(0) = %d, want 1", got)
	}
	if got := idx.DisplayRow(1); got != 3 {
		t.Errorf("DisplayRow(1) = %d, want 3", got)
	}
	// The negative sentinels must not be resolvable.
	if got := idx.DisplayRow(-1); got != -1 {
		t.Errorf("DisplayRow(-1) = %d, want -1", got)
	}
}
