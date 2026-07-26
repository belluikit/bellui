package painter_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter"
)

func TestPaint_GetCornerRadius(t *testing.T) {
	cases := []struct {
		perCorner, base, want float32
	}{
		{0, 5, 5},
		{3, 5, 3},
		{0, 0, 0},
		{7, 0, 7},
	}
	for _, c := range cases {
		got := painter.GetCornerRadius(c.perCorner, c.base)
		if got != c.want {
			t.Errorf("GetCornerRadius(%v, %v) = %v, want %v", c.perCorner, c.base, got, c.want)
		}
	}
}

func TestPaint_GetMaximumRadius(t *testing.T) {
	cases := []struct {
		size geo.Size
		want float32
	}{
		{geo.NewSize(10, 20), 5},
		{geo.NewSize(30, 30), 15},
		{geo.NewSize(0, 0), 0},
		{geo.NewSize(100, 50), 25},
	}
	for _, c := range cases {
		got := painter.GetMaximumRadius(c.size)
		if got != c.want {
			t.Errorf("GetMaximumRadius(%v) = %v, want %v", c.size, got, c.want)
		}
	}
}

func TestPaint_GetMaximumCornerRadius(t *testing.T) {
	cases := []struct {
		radius, adjW, adjH float32
		size               geo.Size
		want               float32
	}{
		{5, 5, 5, geo.NewSquareSize(10), 5},
		{10, 2, 2, geo.NewSquareSize(10), 8},
		{20, 0, 0, geo.NewSquareSize(20), 20},
		{15, 5, 10, geo.NewSquareSize(20), 10},
		{30, 2, 10, geo.NewSquareSize(20), 10},
		{8, 4, 4, geo.NewSquareSize(16), 8},
		{12, 6, 6, geo.NewSize(24, 16), 10},
		{16, 8, 8, geo.NewSize(32, 16), 8},
		{20, 10, 5, geo.NewSize(40, 20), 15},
		{10, 5, 5, geo.NewSize(40, 20), 10},
		{10, 20, 5, geo.NewSize(20, 40), 10},
		{13, 5, 2, geo.NewSize(17, 20), 12},
		{13, 2, 5, geo.NewSize(17, 20), 13},
		{canvas.RadiusMaximum, 3, 5, geo.NewSize(36, 16), 11},
		{canvas.RadiusMaximum, canvas.RadiusMaximum, canvas.RadiusMaximum, geo.NewSize(36, 16), 8},
		{canvas.RadiusMaximum, 0, 0, geo.NewSize(10, 14), 10},
		{canvas.RadiusMaximum, 1, 0, geo.NewSize(10, 14), 9},
		{canvas.RadiusMaximum, canvas.RadiusMaximum, 4, geo.NewSize(30, 20), 15},
	}
	for _, c := range cases {
		got := painter.GetMaximumCornerRadius(c.radius, c.adjW, c.adjH, c.size)
		if got != c.want {
			t.Errorf("GetMaximumCornerRadius(%v, %v, %v, %v) = %v, want %v", c.radius, c.adjW, c.adjH, c.size, got, c.want)
		}
	}
}

func TestPaint_GetMaximumRadiusArc(t *testing.T) {
	cases := []struct {
		outer, inner, sweep, want float32
	}{
		{10, 0, 180, 3.75},
		{10, 8, 180, 1},
		{20, 10, 90, 5},
		{30, 20, 360, 5},
		{50, 45, 45, 2.5},
	}
	for _, c := range cases {
		got := painter.GetMaximumRadiusArc(c.outer, c.inner, c.sweep)
		if got != c.want {
			t.Errorf("GetMaximumRadiusArc(%v, %v, %v) = %v, want %v", c.outer, c.inner, c.sweep, got, c.want)
		}
	}
}

func TestPaint_NormalizeArcAngles(t *testing.T) {
	cases := []struct {
		start, end, wantStart, wantEnd float32
	}{
		{0, 90, 90, 0},
		{180, 270, -90, -180},
		{360, 0, -270, 90},
		{-90, -180, 180, 270},
		{45, 45, 45, 45},
		{-360, 0, 450, 90},
		{-50, -220, 140, 310},
		{-180, 0, 270, 90},
		{90, -90, 0, 180},
		{270, 90, -180, 0},
		{0, -90, 90, 180},
		{110, 150, -20, -60},
	}
	for _, c := range cases {
		gotStart, gotEnd := painter.NormalizeArcAngles(c.start, c.end)
		if gotStart != c.wantStart || gotEnd != c.wantEnd {
			t.Errorf("NormalizeArcAngles(%v, %v) = (%v, %v), want (%v, %v)", c.start, c.end, gotStart, gotEnd, c.wantStart, c.wantEnd)
		}
	}
}
