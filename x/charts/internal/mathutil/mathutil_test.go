package mathutil

import (
	"math"
	"testing"
)

func TestClamp(t *testing.T) {
	tests := []struct{ v, lo, hi, want float64 }{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
	}
	for _, tt := range tests {
		got := Clamp(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("Clamp(%v, %v, %v) = %v, want %v", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}

func TestLerp(t *testing.T) {
	if got := Lerp(0, 10, 0.5); got != 5 {
		t.Errorf("Lerp(0, 10, 0.5) = %v, want 5", got)
	}
	if got := Lerp(0, 10, 0); got != 0 {
		t.Errorf("Lerp(0, 10, 0) = %v, want 0", got)
	}
	if got := Lerp(0, 10, 1); got != 10 {
		t.Errorf("Lerp(0, 10, 1) = %v, want 10", got)
	}
}

func TestInverseLerp(t *testing.T) {
	if got := InverseLerp(0, 10, 5); got != 0.5 {
		t.Errorf("InverseLerp(0, 10, 5) = %v, want 0.5", got)
	}
	if got := InverseLerp(0, 10, 0); got != 0 {
		t.Errorf("InverseLerp(0, 10, 0) = %v, want 0", got)
	}
	if got := InverseLerp(5, 5, 5); got != 0 {
		t.Errorf("InverseLerp(5, 5, 5) = %v, want 0 (equal bounds)", got)
	}
}

func TestNiceNumber(t *testing.T) {
	tests := []struct {
		x     float64
		round bool
		want  float64
	}{
		{12, true, 10},
		{17, true, 20},
		{35, true, 50},
		{75, true, 100},
		{0, true, 0},
	}
	for _, tt := range tests {
		got := NiceNumber(tt.x, tt.round)
		if got != tt.want {
			t.Errorf("NiceNumber(%v, %v) = %v, want %v", tt.x, tt.round, got, tt.want)
		}
	}
}

func TestNiceTicks(t *testing.T) {
	niceMin, niceMax, interval := NiceTicks(0, 100, 5)
	if niceMin > 0 {
		t.Errorf("NiceTicks min = %v, want <= 0", niceMin)
	}
	if niceMax < 100 {
		t.Errorf("NiceTicks max = %v, want >= 100", niceMax)
	}
	if interval <= 0 {
		t.Errorf("NiceTicks interval = %v, want > 0", interval)
	}

	// Equal min/max should produce a valid range
	niceMin, niceMax, interval = NiceTicks(50, 50, 5)
	if niceMax <= niceMin {
		t.Errorf("NiceTicks with equal bounds: min=%v, max=%v", niceMin, niceMax)
	}
}

func TestTickValues(t *testing.T) {
	ticks := TickValues(0, 100, 20)
	if len(ticks) != 6 { // 0, 20, 40, 60, 80, 100
		t.Errorf("TickValues(0, 100, 20) produced %d ticks, want 6", len(ticks))
	}
	if ticks[0] != 0 || ticks[len(ticks)-1] != 100 {
		t.Errorf("TickValues bounds: first=%v last=%v", ticks[0], ticks[len(ticks)-1])
	}
}

func TestClipLineToRect_Inside(t *testing.T) {
	cx1, cy1, cx2, cy2, vis := ClipLineToRect(2, 2, 8, 8, 0, 0, 10, 10)
	if !vis {
		t.Fatal("expected visible")
	}
	if cx1 != 2 || cy1 != 2 || cx2 != 8 || cy2 != 8 {
		t.Errorf("inside line should be unchanged, got (%v,%v)-(%v,%v)", cx1, cy1, cx2, cy2)
	}
}

func TestClipLineToRect_Outside(t *testing.T) {
	_, _, _, _, vis := ClipLineToRect(11, 11, 15, 15, 0, 0, 10, 10)
	if vis {
		t.Fatal("expected not visible")
	}
}

func TestClipLineToRect_Crossing(t *testing.T) {
	cx1, cy1, cx2, cy2, vis := ClipLineToRect(-5, 5, 15, 5, 0, 0, 10, 10)
	if !vis {
		t.Fatal("expected visible")
	}
	if math.Abs(float64(cx1)) > 0.01 || math.Abs(float64(cx2)-10) > 0.01 {
		t.Errorf("clipped X should be 0-10, got %v-%v", cx1, cx2)
	}
	if cy1 != 5 || cy2 != 5 {
		t.Errorf("Y should remain 5, got %v-%v", cy1, cy2)
	}
}

func TestClipLineToRect_Vertical(t *testing.T) {
	_, _, _, _, vis := ClipLineToRect(5, -5, 5, 15, 0, 0, 10, 10)
	if !vis {
		t.Fatal("expected visible")
	}
}
