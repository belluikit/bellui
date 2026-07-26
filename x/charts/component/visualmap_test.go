package component

import (
	"image/color"
	"testing"
)

func TestVisualMap_MapColor(t *testing.T) {
	vm := &VisualMapConfig{
		Min: 0, Max: 100,
		InRange: VisualMapRange{
			Colors: []color.Color{
				color.NRGBA{R: 0, G: 0, B: 255, A: 255}, // blue at 0
				color.NRGBA{R: 255, G: 0, B: 0, A: 255}, // red at 1
			},
		},
	}

	// At min, should be blue
	c := vm.MapColor(0)
	r, _, _, _ := c.RGBA()
	if r>>8 > 10 {
		t.Errorf("at min, expected ~blue, got R=%d", r>>8)
	}

	// At max, should be red
	c = vm.MapColor(100)
	r, _, _, _ = c.RGBA()
	if r>>8 < 200 {
		t.Errorf("at max, expected ~red, got R=%d", r>>8)
	}

	// At midpoint, should be blended
	c = vm.MapColor(50)
	if c == nil {
		t.Error("midpoint color should not be nil")
	}
}

func TestVisualMap_MapSize(t *testing.T) {
	vm := &VisualMapConfig{
		Min: 0, Max: 100,
		InRange: VisualMapRange{
			SymbolSizeMin: 4,
			SymbolSizeMax: 20,
		},
	}

	if s := vm.MapSize(0); s != 4 {
		t.Errorf("min size = %v, want 4", s)
	}
	if s := vm.MapSize(100); s != 20 {
		t.Errorf("max size = %v, want 20", s)
	}
	if s := vm.MapSize(50); s < 11 || s > 13 {
		t.Errorf("mid size = %v, want ~12", s)
	}
}

func TestVisualMap_Clamp(t *testing.T) {
	vm := &VisualMapConfig{
		Min: 0, Max: 100,
		InRange: VisualMapRange{
			Colors: []color.Color{color.White, color.Black},
		},
	}

	// Below min should clamp
	c := vm.MapColor(-50)
	if c == nil {
		t.Error("below-min color should not be nil")
	}

	// Above max should clamp
	c = vm.MapColor(200)
	if c == nil {
		t.Error("above-max color should not be nil")
	}
}
