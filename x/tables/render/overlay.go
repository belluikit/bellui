package render

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// OverlayLayer manages selection, hover, and focus indicators above the cells.
type OverlayLayer struct {
	SelectionRects []*canvas.Rectangle
	HoverRect      *canvas.Rectangle
	FocusRect      *canvas.Rectangle
}

// NewOverlayLayer creates a new overlay layer.
func NewOverlayLayer() *OverlayLayer {
	hover := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 20})
	hover.Hide()

	focus := &canvas.Rectangle{
		FillColor:   color.Transparent,
		StrokeColor: color.NRGBA{R: 33, G: 150, B: 243, A: 200},
		StrokeWidth: 2,
	}
	focus.Hide()

	return &OverlayLayer{
		HoverRect: hover,
		FocusRect: focus,
	}
}

// ShowHover shows the hover overlay at the given position and size.
func (o *OverlayLayer) ShowHover(pos geo.Position, size geo.Size) {
	o.HoverRect.Move(pos)
	o.HoverRect.Resize(size)
	o.HoverRect.Show()
}

// HideHover hides the hover overlay.
func (o *OverlayLayer) HideHover() {
	o.HoverRect.Hide()
}

// ShowFocus shows the focus ring at the given position and size.
func (o *OverlayLayer) ShowFocus(pos geo.Position, size geo.Size) {
	o.FocusRect.Move(pos)
	o.FocusRect.Resize(size)
	o.FocusRect.Show()
}

// HideFocus hides the focus ring.
func (o *OverlayLayer) HideFocus() {
	o.FocusRect.Hide()
}

// SetSelections creates/updates selection rectangles for the given positions.
func (o *OverlayLayer) SetSelections(positions []selectionRect, selColor color.Color) {
	// Grow pool if needed
	for len(o.SelectionRects) < len(positions) {
		r := canvas.NewRectangle(selColor)
		o.SelectionRects = append(o.SelectionRects, r)
	}

	for i, pos := range positions {
		r := o.SelectionRects[i]
		r.FillColor = selColor
		r.Move(pos.Pos)
		r.Resize(pos.Size)
		r.Show()
	}

	// Hide unused rects
	for i := len(positions); i < len(o.SelectionRects); i++ {
		o.SelectionRects[i].Hide()
	}
}

type selectionRect struct {
	Pos  geo.Position
	Size geo.Size
}

// Objects returns all overlay CanvasObjects for rendering.
func (o *OverlayLayer) Objects() []core.CanvasObject {
	objs := make([]core.CanvasObject, 0, len(o.SelectionRects)+2)
	for _, r := range o.SelectionRects {
		objs = append(objs, r)
	}
	objs = append(objs, o.HoverRect, o.FocusRect)
	return objs
}
