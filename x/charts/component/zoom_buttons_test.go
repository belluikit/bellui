package component

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestRenderZoomButtons_ReturnsObjects(t *testing.T) {
	objs, layout := RenderZoomButtons(400, 10, testResolver())

	if len(objs) == 0 {
		t.Fatal("expected objects from RenderZoomButtons")
	}
	if layout == nil {
		t.Fatal("expected non-nil layout")
	}

	// Should have 3 buttons: zoom in, zoom out, reset
	// Each button has a background rect + label = 6 objects total
	if len(objs) != 6 {
		t.Errorf("expected 6 objects (3 buttons * 2), got %d", len(objs))
	}
}

func TestZoomButtonsLayout_HitTest_ZoomIn(t *testing.T) {
	_, layout := RenderZoomButtons(400, 10, testResolver())

	inCenter := geo.NewPos(
		layout.ZoomInRect.Pos.X+layout.ZoomInRect.Size.Width/2,
		layout.ZoomInRect.Pos.Y+layout.ZoomInRect.Size.Height/2,
	)
	action := layout.HitTest(inCenter)
	if action != "in" {
		t.Errorf("expected 'in', got %q", action)
	}
}

func TestZoomButtonsLayout_HitTest_ZoomOut(t *testing.T) {
	_, layout := RenderZoomButtons(400, 10, testResolver())

	outCenter := geo.NewPos(
		layout.ZoomOutRect.Pos.X+layout.ZoomOutRect.Size.Width/2,
		layout.ZoomOutRect.Pos.Y+layout.ZoomOutRect.Size.Height/2,
	)
	action := layout.HitTest(outCenter)
	if action != "out" {
		t.Errorf("expected 'out', got %q", action)
	}
}

func TestZoomButtonsLayout_HitTest_Reset(t *testing.T) {
	_, layout := RenderZoomButtons(400, 10, testResolver())

	resetCenter := geo.NewPos(
		layout.ResetRect.Pos.X+layout.ResetRect.Size.Width/2,
		layout.ResetRect.Pos.Y+layout.ResetRect.Size.Height/2,
	)
	action := layout.HitTest(resetCenter)
	if action != "reset" {
		t.Errorf("expected 'reset', got %q", action)
	}
}

func TestZoomButtonsLayout_HitTest_Miss(t *testing.T) {
	_, layout := RenderZoomButtons(400, 10, testResolver())

	missPos := geo.NewPos(0, 0)
	action := layout.HitTest(missPos)
	if action != "" {
		t.Errorf("expected empty string for miss, got %q", action)
	}
}

func TestZoomButtonsLayout_HitTest_Nil(t *testing.T) {
	var layout *ZoomButtonsLayout
	action := layout.HitTest(geo.NewPos(100, 10))
	if action != "" {
		t.Errorf("expected empty string for nil layout, got %q", action)
	}
}

func TestRenderZoomButtons_LayoutOrder(t *testing.T) {
	_, layout := RenderZoomButtons(500, 20, testResolver())

	// Zoom In should be leftmost, then Out, then Reset (rightmost)
	if layout.ZoomInRect.Pos.X >= layout.ZoomOutRect.Pos.X {
		t.Error("expected zoom in button to be left of zoom out")
	}
	if layout.ZoomOutRect.Pos.X >= layout.ResetRect.Pos.X {
		t.Error("expected zoom out button to be left of reset")
	}
}
