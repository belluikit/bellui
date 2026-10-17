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

// ZoomBy takes a pinch-style factor: 2 shows half the span around the centre,
// 0.5 restores it, and a non-positive factor is ignored (#379).
func TestZoomState_ZoomBy(t *testing.T) {
	z := NewZoomState(DefaultZoomConfig())
	if !z.ZoomBy(2, 0.5) {
		t.Fatal("expected zoom to change")
	}
	if math.Abs(z.Window.Start-0.25) > 1e-9 || math.Abs(z.Window.End-0.75) > 1e-9 {
		t.Errorf("factor 2 at the centre should show 0.25-0.75, got %v-%v", z.Window.Start, z.Window.End)
	}
	if !z.ZoomBy(0.5, 0.5) || math.Abs(z.Window.Span()-1) > 1e-9 {
		t.Errorf("factor 0.5 should restore the full span, got %v", z.Window.Span())
	}
	if z.ZoomBy(0, 0.5) || z.ZoomBy(-1, 0.5) {
		t.Error("a non-positive factor must not change the window")
	}
}
