package interact

import (
	"math"
	"testing"
)

func TestZoomState_ZoomIn(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())

	changed := z.ZoomAt(1, 0.5) // zoom in, centered
	if !changed {
		t.Fatal("expected zoom to change")
	}
	if z.Window.Start <= 0 || z.Window.End >= 1 {
		t.Errorf("window should narrow: %v-%v", z.Window.Start, z.Window.End)
	}
	if !z.IsZoomed() {
		t.Error("should be zoomed")
	}
}

func TestZoomState_ZoomOut(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())
	z.ZoomAt(1, 0.5) // zoom in first
	start := z.Window.Start

	z.ZoomAt(-1, 0.5) // zoom out
	if z.Window.Start >= start {
		t.Errorf("zoom out should widen window, start: %v >= %v", z.Window.Start, start)
	}
}

func TestZoomState_ZoomAtMax(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())
	// Already at max (full window), zooming out should not change
	changed := z.ZoomAt(-1, 0.5)
	if changed {
		t.Error("should not change when already at max span")
	}
}

func TestZoomState_Pan(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())
	z.ZoomAt(1, 0.5) // zoom in
	start := z.Window.Start

	z.Pan(0.05)
	if z.Window.Start <= start {
		t.Errorf("pan should shift start right: %v <= %v", z.Window.Start, start)
	}
}

func TestZoomState_PanClamp(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())
	z.SetWindow(0.8, 1.0) // near right edge

	z.Pan(0.5) // try to pan way right
	if z.Window.End > 1.001 {
		t.Errorf("window end should clamp to 1, got %v", z.Window.End)
	}
	if z.Window.Start < 0 {
		t.Errorf("window start should be >= 0, got %v", z.Window.Start)
	}
}

func TestZoomState_SetWindow(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())
	z.SetWindow(0.2, 0.8)
	if z.Window.Start != 0.2 || z.Window.End != 0.8 {
		t.Errorf("SetWindow: got %v-%v, want 0.2-0.8", z.Window.Start, z.Window.End)
	}
}

func TestDataWindow_Span(t *testing.T) {
	w := DataWindow{Start: 0.2, End: 0.8}
	if math.Abs(w.Span()-0.6) > 0.001 {
		t.Errorf("Span = %v, want ~0.6", w.Span())
	}
}

func TestFullWindow(t *testing.T) {
	w := FullWindow()
	if w.Start != 0 || w.End != 1 {
		t.Errorf("FullWindow = %v-%v, want 0-1", w.Start, w.End)
	}
}
