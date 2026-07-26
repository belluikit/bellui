package widget

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestSimpleWidget_Basic(t *testing.T) {
	_ = test.NewTempApp(t)

	bg := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
	label := canvas.NewText("Hello", color.White)

	w := NewSimpleWidget(
		func() []core.CanvasObject { return []core.CanvasObject{bg, label} },
		nil, // default layout fills all objects
	)

	min := w.MinSize()
	if min.Width == 0 || min.Height == 0 {
		t.Errorf("expected non-zero MinSize, got %v", min)
	}

	w.Resize(geo.NewSize(200, 100))

	r := test.TempWidgetRenderer(t, w)
	objs := r.Objects()
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
}

func TestSimpleWidget_CustomLayout(t *testing.T) {
	_ = test.NewTempApp(t)

	bg := canvas.NewRectangle(color.NRGBA{G: 255, A: 255})
	label := canvas.NewText("Test", color.Black)

	layoutCalled := false
	w := NewSimpleWidget(
		func() []core.CanvasObject { return []core.CanvasObject{bg, label} },
		func(size geo.Size) {
			layoutCalled = true
			bg.Resize(size)
			label.Move(geo.NewPos(10, 5))
		},
	)

	w.Resize(geo.NewSize(200, 100))
	r := test.TempWidgetRenderer(t, w)
	r.Layout(geo.NewSize(200, 100))

	if !layoutCalled {
		t.Error("custom layout function should have been called")
	}
}

func TestSimpleWidget_MinSizeFunc(t *testing.T) {
	_ = test.NewTempApp(t)

	w := NewSimpleWidget(
		func() []core.CanvasObject { return nil },
		nil,
	).SetMinSizeFunc(func() geo.Size {
		return geo.NewSize(150, 50)
	})

	min := w.MinSize()
	if min.Width != 150 || min.Height != 50 {
		t.Errorf("expected 150x50, got %v", min)
	}
}

func TestSimpleWidget_RefreshFunc(t *testing.T) {
	_ = test.NewTempApp(t)

	label := canvas.NewText("initial", color.White)
	refreshed := false

	w := NewSimpleWidget(
		func() []core.CanvasObject { return []core.CanvasObject{label} },
		nil,
	).SetRefreshFunc(func() {
		refreshed = true
		label.Text = "updated"
	})

	w.Refresh()
	if !refreshed {
		t.Error("refresh function should have been called")
	}
	if label.Text != "updated" {
		t.Error("label text should be 'updated'")
	}
}
