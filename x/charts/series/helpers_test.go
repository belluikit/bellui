package series

import (
	"image/color"
	"testing"
)

func TestResolveColor(t *testing.T) {
	fallback := color.NRGBA{R: 255, A: 255}
	override := color.NRGBA{G: 255, A: 255}

	if got := ResolveColor(nil, fallback); got != fallback {
		t.Error("nil override should return fallback")
	}
	if got := ResolveColor(override, fallback); got != override {
		t.Error("non-nil override should return override")
	}
}

func TestResolveFloat32(t *testing.T) {
	if got := ResolveFloat32(0, 5); got != 5 {
		t.Errorf("zero value should return default, got %v", got)
	}
	if got := ResolveFloat32(-1, 5); got != 5 {
		t.Errorf("negative value should return default, got %v", got)
	}
	if got := ResolveFloat32(3, 5); got != 3 {
		t.Errorf("positive value should be kept, got %v", got)
	}
}
