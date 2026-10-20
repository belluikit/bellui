package common

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

// TestClampRegionToCanvas covers the dirty-region clamp that keeps a partial
// repaint's clip in bounds when a refreshed object sits (partly) off-canvas —
// e.g. content scrolled out of a container (pre-rebrand tracker #238). Both
// drivers use it (#591).
func TestClampRegionToCanvas(t *testing.T) {
	canvas := geo.NewSize(100, 100)
	for _, tt := range []struct {
		name         string
		pos          geo.Position
		sz           geo.Size
		wantPos      geo.Position
		wantSz       geo.Size
		wantEmptySet bool // expect a zero-size result
	}{
		{"fully inside", geo.NewPos(10, 10), geo.NewSize(30, 30), geo.NewPos(10, 10), geo.NewSize(30, 30), false},
		{"overhang right/bottom", geo.NewPos(80, 80), geo.NewSize(40, 40), geo.NewPos(80, 80), geo.NewSize(20, 20), false},
		{"negative origin", geo.NewPos(-10, -10), geo.NewSize(30, 30), geo.NewPos(0, 0), geo.NewSize(20, 20), false},
		{"fully below (scrolled out)", geo.NewPos(20, 160), geo.NewSize(40, 40), geo.NewPos(20, 100), geo.Size{}, true},
		{"fully right", geo.NewPos(160, 20), geo.NewSize(40, 40), geo.NewPos(100, 20), geo.Size{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pos, sz := ClampRegionToCanvas(tt.pos, tt.sz, canvas)
			empty := sz.Width <= 0 || sz.Height <= 0
			if empty != tt.wantEmptySet {
				t.Fatalf("empty = %v (sz=%v), want empty=%v", empty, sz, tt.wantEmptySet)
			}
			if tt.wantEmptySet {
				return
			}
			if pos != tt.wantPos || sz != tt.wantSz {
				t.Errorf("clamp = (%v, %v), want (%v, %v)", pos, sz, tt.wantPos, tt.wantSz)
			}
		})
	}
}
