package axis

import (
	"math"
	"testing"
)

func TestLinearScale(t *testing.T) {
	s := NewLinearScale(5, nil, nil)
	s.SetRange(0, 100)

	min, max := s.Range()
	if min > 0 || max < 100 {
		t.Errorf("Range should contain [0,100], got [%v,%v]", min, max)
	}

	// 0 should normalize to 0, max to 1
	if got := s.DataToNormalized(min); math.Abs(got) > 0.001 {
		t.Errorf("DataToNormalized(min) = %v, want ~0", got)
	}
	if got := s.DataToNormalized(max); math.Abs(got-1) > 0.001 {
		t.Errorf("DataToNormalized(max) = %v, want ~1", got)
	}

	// Round-trip
	mid := (min + max) / 2
	norm := s.DataToNormalized(mid)
	back := s.NormalizedToData(norm)
	if math.Abs(back-mid) > 0.01 {
		t.Errorf("round-trip: %v -> %v -> %v", mid, norm, back)
	}

	ticks := s.Ticks()
	if len(ticks) < 3 {
		t.Errorf("expected at least 3 ticks, got %d", len(ticks))
	}
}

func TestLinearScale_ExplicitBounds(t *testing.T) {
	lo, hi := 10.0, 50.0
	s := NewLinearScale(5, &lo, &hi)
	s.SetRange(0, 100) // data range ignored because explicit bounds set

	min, max := s.Range()
	if min > 10 || max < 50 {
		t.Errorf("explicit bounds not respected, got [%v,%v]", min, max)
	}
}

func TestCategoryScale(t *testing.T) {
	cats := []string{"Mon", "Tue", "Wed", "Thu", "Fri"}
	s := NewCategoryScale(cats)

	if s.CategoryCount() != 5 {
		t.Errorf("CategoryCount = %d, want 5", s.CategoryCount())
	}

	// Band positioning: each category centered in 1/n band
	n0 := s.DataToNormalized(0)
	n4 := s.DataToNormalized(4)
	if n0 <= 0 || n0 >= 0.2 {
		t.Errorf("category 0 norm = %v, want ~0.1", n0)
	}
	if n4 <= 0.8 || n4 >= 1.0 {
		t.Errorf("category 4 norm = %v, want ~0.9", n4)
	}

	// Round-trip
	back := s.NormalizedToData(n0)
	if math.Abs(back) > 0.01 {
		t.Errorf("round-trip category 0: norm=%v -> %v", n0, back)
	}

	ticks := s.Ticks()
	if len(ticks) != 5 {
		t.Errorf("expected 5 ticks, got %d", len(ticks))
	}
	if ticks[2].Label != "Wed" {
		t.Errorf("tick 2 label = %q, want Wed", ticks[2].Label)
	}
}

func TestCategoryScale_Zoom(t *testing.T) {
	cats := []string{"A", "B", "C", "D", "E", "F", "G"}
	s := NewCategoryScale(cats)

	// Zoom to show only B-E (indices 1-4)
	s.SetRange(1, 4)
	ticks := s.Ticks()

	// Should show B, C, D, E (4 ticks)
	if len(ticks) != 4 {
		t.Errorf("zoomed ticks: got %d, want 4", len(ticks))
	}
	if ticks[0].Label != "B" {
		t.Errorf("first zoomed tick = %q, want B", ticks[0].Label)
	}

	// Category 1 (B) should be near 0, category 4 (E) near 1
	norm1 := s.DataToNormalized(1)
	norm4 := s.DataToNormalized(4)
	if norm1 < 0 || norm1 > 0.3 {
		t.Errorf("zoomed B norm = %v, want ~0.125", norm1)
	}
	if norm4 < 0.7 || norm4 > 1 {
		t.Errorf("zoomed E norm = %v, want ~0.875", norm4)
	}
}

func TestLogScale(t *testing.T) {
	s := NewLogScale(10, 5, nil, nil)
	s.SetRange(1, 1000)

	min, max := s.Range()
	if min != 1 || max != 1000 {
		t.Errorf("LogScale range = [%v,%v], want [1, 1000]", min, max)
	}

	// 1 -> 0, 1000 -> 1
	if got := s.DataToNormalized(1); math.Abs(got) > 0.001 {
		t.Errorf("DataToNormalized(1) = %v, want ~0", got)
	}
	if got := s.DataToNormalized(1000); math.Abs(got-1) > 0.001 {
		t.Errorf("DataToNormalized(1000) = %v, want ~1", got)
	}

	// 10 should be roughly 1/3
	norm10 := s.DataToNormalized(10)
	if math.Abs(norm10-1.0/3) > 0.05 {
		t.Errorf("DataToNormalized(10) = %v, want ~0.33", norm10)
	}

	ticks := s.Ticks()
	if len(ticks) < 2 {
		t.Errorf("expected at least 2 ticks, got %d", len(ticks))
	}
}

func TestNewScale(t *testing.T) {
	cfg := DefaultConfig(TypeValue)
	s := NewScale(cfg)
	if s == nil {
		t.Fatal("NewScale(TypeValue) returned nil")
	}

	cfg = DefaultConfig(TypeCategory)
	cfg.Data = []string{"A", "B"}
	s = NewScale(cfg)
	if s == nil {
		t.Fatal("NewScale(TypeCategory) returned nil")
	}
}
