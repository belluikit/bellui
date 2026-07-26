package widget_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// --- OLD WAY: ~40 lines for a colored badge ---

type oldBadge struct {
	widget.BaseWidget
	Text  string
	Color color.Color
}

func newOldBadge(text string, c color.Color) *oldBadge {
	w := &oldBadge{Text: text, Color: c}
	w.ExtendBaseWidget(w)
	return w
}

func (w *oldBadge) CreateRenderer() core.WidgetRenderer {
	return &oldBadgeRenderer{widget: w}
}

type oldBadgeRenderer struct {
	widget *oldBadge
	bg     *canvas.Rectangle
	label  *canvas.Text
}

func (r *oldBadgeRenderer) Destroy() {}
func (r *oldBadgeRenderer) Layout(size geo.Size) {
	if r.bg == nil {
		r.bg = canvas.NewRectangle(r.widget.Color)
		r.label = canvas.NewText(r.widget.Text, color.White)
	}
	r.bg.Resize(size)
	r.label.Move(geo.NewPos(8, 4))
}

func (r *oldBadgeRenderer) MinSize() geo.Size {
	if r.label == nil {
		return geo.NewSize(20, 20)
	}
	ts := r.label.MinSize()
	return geo.NewSize(ts.Width+16, ts.Height+8)
}

func (r *oldBadgeRenderer) Objects() []core.CanvasObject {
	if r.bg == nil {
		return nil
	}
	return []core.CanvasObject{r.bg, r.label}
}

func (r *oldBadgeRenderer) Refresh() {
	if r.bg == nil {
		return
	}
	r.bg.FillColor = r.widget.Color
	r.label.Text = r.widget.Text
	canvas.Refresh(r.bg)
	canvas.Refresh(r.label)
}

// --- NEW WAY: ~15 lines with the renderer builder ---

type newBadge struct {
	widget.BaseWidget
	Text  string
	Color color.Color
}

func newNewBadge(text string, c color.Color) *newBadge {
	w := &newBadge{Text: text, Color: c}
	w.ExtendBaseWidget(w)
	return w
}

func (w *newBadge) CreateRenderer() core.WidgetRenderer {
	bg := canvas.NewRectangle(w.Color)
	label := canvas.NewText(w.Text, color.White)

	return widget.NewRenderer(bg, label).
		Layout(func(size geo.Size) {
			bg.Resize(size)
			label.Move(geo.NewPos(8, 4))
		}).
		MinSizeFunc(func() geo.Size {
			ts := label.MinSize()
			return geo.NewSize(ts.Width+16, ts.Height+8)
		}).
		Refresh(func() {
			bg.FillColor = w.Color
			label.Text = w.Text
			canvas.Refresh(bg)
			canvas.Refresh(label)
		}).
		Build()
}

// --- Tests ---

func TestRenderer_OldBadge(t *testing.T) {
	_ = test.NewTempApp(t)

	b := newOldBadge("OK", color.NRGBA{G: 200, A: 255})
	min := b.MinSize()
	if min.Width == 0 {
		t.Error("expected non-zero width")
	}
}

func TestRenderer_NewBadge(t *testing.T) {
	_ = test.NewTempApp(t)

	b := newNewBadge("OK", color.NRGBA{G: 200, A: 255})
	min := b.MinSize()
	if min.Width == 0 {
		t.Error("expected non-zero width")
	}

	r := test.TempWidgetRenderer(t, b)
	if len(r.Objects()) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(r.Objects()))
	}

	// Update state and refresh.
	b.Text = "ERROR"
	b.Color = color.NRGBA{R: 200, A: 255}
	b.Refresh()

	// Verify the renderer updated.
	objs := r.Objects()
	bg := objs[0].(*canvas.Rectangle)
	label := objs[1].(*canvas.Text)

	if label.Text != "ERROR" {
		t.Errorf("expected 'ERROR', got %q", label.Text)
	}
	if bg.FillColor != b.Color {
		t.Error("background color not updated")
	}
}

func TestRenderer_DefaultLayout(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)

	w := widget.NewSimpleWidget(
		func() []core.CanvasObject { return []core.CanvasObject{rect} },
		nil, // default layout fills
	)
	w.Resize(geo.NewSize(200, 100))

	r := test.TempWidgetRenderer(t, w)
	r.Layout(geo.NewSize(200, 100))

	if rect.Size().Width != 200 || rect.Size().Height != 100 {
		t.Errorf("expected rect to fill 200x100, got %v", rect.Size())
	}
}
