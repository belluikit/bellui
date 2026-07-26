package widget_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestInteractive_Tapped(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.NRGBA{R: 200, A: 255})
	var tapped bool
	w := widget.NewInteractive(rect)
	w.OnTapped = func(*event.PointEvent) {
		tapped = true
	}

	w.Tapped(&event.PointEvent{})
	if !tapped {
		t.Error("expected OnTapped to be called")
	}
}

func TestInteractive_NilCallbackSafe(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)
	w := widget.NewInteractive(rect)

	// None of these should panic with nil callbacks.
	w.Tapped(&event.PointEvent{})
	w.DoubleTapped(&event.PointEvent{})
	w.TappedSecondary(&event.PointEvent{})
	w.Dragged(&event.DragEvent{})
	w.DragEnd()
	w.Scrolled(&event.ScrollEvent{})
	w.HoverIn(&event.HoverEvent{})
	w.HoverMoved(&event.HoverEvent{})
	w.HoverOut()
}

func TestInteractive_Dragged(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)
	var dragged bool
	var ended bool
	w := widget.NewInteractive(rect)
	w.OnDragged = func(*event.DragEvent) { dragged = true }
	w.OnDragEnd = func() { ended = true }

	w.Dragged(&event.DragEvent{})
	w.DragEnd()

	if !dragged {
		t.Error("expected OnDragged to be called")
	}
	if !ended {
		t.Error("expected OnDragEnd to be called")
	}
}

func TestInteractive_Renderer(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.NRGBA{B: 200, A: 255})
	w := widget.NewInteractive(rect)

	r := test.TempWidgetRenderer(t, w)
	objs := r.Objects()
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if objs[0] != rect {
		t.Error("expected renderer to contain the wrapped rect")
	}
}

func TestInteractive_MinSize(t *testing.T) {
	_ = test.NewTempApp(t)

	text := canvas.NewText("Hello", color.Black)
	w := widget.NewInteractive(text)

	min := w.MinSize()
	textMin := text.MinSize()
	if min.Width < textMin.Width || min.Height < textMin.Height {
		t.Errorf("expected MinSize >= text MinSize, got %v vs %v", min, textMin)
	}
}

func TestInteractive_CursorDefault(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)
	w := widget.NewInteractive(rect)

	if w.Cursor() != desktop.DefaultCursor {
		t.Error("expected default cursor")
	}
}

func TestInteractive_CursorCustom(t *testing.T) {
	_ = test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)
	w := widget.NewInteractive(rect)
	w.CursorStyle = desktop.PointerCursor

	if w.Cursor() != desktop.PointerCursor {
		t.Error("expected pointer cursor")
	}
}
