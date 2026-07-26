package component

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestRenderLegend_Empty(t *testing.T) {
	objs, layout, height := RenderLegend(nil, nil, 10, 10, 500, testResolver())
	if objs != nil {
		t.Errorf("expected nil objects for nil items, got %d", len(objs))
	}
	if layout != nil {
		t.Error("expected nil layout for nil items")
	}
	if height != 0 {
		t.Errorf("expected zero height, got %f", height)
	}
}

func TestRenderLegend_SingleItem(t *testing.T) {
	items := []LegendItem{
		{Name: "Series A", Color: color.NRGBA{R: 255, A: 255}, Visible: true},
	}
	objs, layout, height := RenderLegend(items, nil, 10, 100, 500, testResolver())

	if len(objs) == 0 {
		t.Fatal("expected objects for single legend item")
	}
	if layout == nil {
		t.Fatal("expected non-nil layout")
	}
	if height <= 0 {
		t.Errorf("expected positive height, got %f", height)
	}
	if len(layout.Items) != 1 {
		t.Errorf("expected 1 layout item, got %d", len(layout.Items))
	}
}

func TestRenderLegend_MultipleItems(t *testing.T) {
	items := []LegendItem{
		{Name: "A", Color: color.NRGBA{R: 255, A: 255}, Visible: true},
		{Name: "B", Color: color.NRGBA{G: 255, A: 255}, Visible: true},
		{Name: "C", Color: color.NRGBA{B: 255, A: 255}, Visible: false},
	}
	objs, layout, _ := RenderLegend(items, nil, 0, 0, 800, testResolver())

	// Each item produces a swatch + label = 2 objects each = 6 total
	if len(objs) != 6 {
		t.Errorf("expected 6 objects (3 items * 2), got %d", len(objs))
	}
	if len(layout.Items) != 3 {
		t.Errorf("expected 3 layout items, got %d", len(layout.Items))
	}
}

func TestLegendLayout_HitTest(t *testing.T) {
	items := []LegendItem{
		{Name: "First", Color: color.White, Visible: true},
		{Name: "Second", Color: color.White, Visible: true},
	}
	_, layout, _ := RenderLegend(items, nil, 0, 0, 800, testResolver())

	if layout == nil {
		t.Fatal("layout is nil")
	}

	// Test hit on first item
	firstItem := layout.Items[0]
	hitPos := geo.NewPos(
		firstItem.Rect.X+firstItem.Size.Width/2,
		firstItem.Rect.Y+firstItem.Size.Height/2,
	)
	idx := layout.HitTest(hitPos)
	if idx != 0 {
		t.Errorf("expected hit on item 0, got %d", idx)
	}

	// Test hit on second item
	secondItem := layout.Items[1]
	hitPos2 := geo.NewPos(
		secondItem.Rect.X+secondItem.Size.Width/2,
		secondItem.Rect.Y+secondItem.Size.Height/2,
	)
	idx2 := layout.HitTest(hitPos2)
	if idx2 != 1 {
		t.Errorf("expected hit on item 1, got %d", idx2)
	}

	// Test miss (way below)
	missPos := geo.NewPos(0, 9999)
	idx3 := layout.HitTest(missPos)
	if idx3 != -1 {
		t.Errorf("expected -1 for miss, got %d", idx3)
	}
}

func TestLegendLayout_HitTestNil(t *testing.T) {
	var layout *LegendLayout
	// Should not panic
	if layout != nil {
		idx := layout.HitTest(geo.NewPos(0, 0))
		if idx != -1 {
			t.Errorf("expected -1 for nil layout, got %d", idx)
		}
	}
}

func TestRenderLegend_HiddenItemDimmed(t *testing.T) {
	items := []LegendItem{
		{Name: "Hidden", Color: color.NRGBA{R: 255, G: 0, B: 0, A: 255}, Visible: false},
	}
	objs, _, _ := RenderLegend(items, nil, 0, 0, 500, testResolver())

	// Should still render objects (dimmed, not omitted)
	if len(objs) == 0 {
		t.Error("expected objects for hidden item (dimmed)")
	}
}
