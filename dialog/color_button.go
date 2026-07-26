package dialog

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

var (
	_ core.Widget     = (*colorButton)(nil)
	_ event.Hoverable = (*colorButton)(nil)
)

// colorButton displays a color and triggers the callback when tapped.
type colorButton struct {
	widget.BaseWidget
	color   color.Color
	onTap   func(color.Color)
	hovered bool
}

// newColorButton creates a colorButton with the given color and callback.
func newColorButton(color color.Color, onTap func(color.Color)) *colorButton {
	b := &colorButton{
		color: color,
		onTap: onTap,
	}
	b.ExtendBaseWidget(b)
	return b
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (b *colorButton) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	background := newCheckeredBackground(false)
	rectangle := &canvas.Rectangle{
		FillColor: b.color,
	}
	return &colorButtonRenderer{
		BaseRenderer: internalwidget.NewBaseRenderer([]core.CanvasObject{background, rectangle}),
		button:       b,
		background:   background,
		rectangle:    rectangle,
	}
}

// HoverIn is called when a pointer enters the widget
func (b *colorButton) HoverIn(*event.HoverEvent) {
	b.hovered = true
	b.Refresh()
}

// HoverOut is called when a pointer exits the widget
func (b *colorButton) HoverOut() {
	b.hovered = false
	b.Refresh()
}

// HoverMoved is called when a pointer hovers over the widget
func (b *colorButton) HoverMoved(*event.HoverEvent) {
}

// MinSize returns the size that this widget should not shrink below
func (b *colorButton) MinSize() geo.Size {
	return b.BaseWidget.MinSize()
}

// SetColor updates the color selected in this color widget
func (b *colorButton) SetColor(color color.Color) {
	if b.color == color {
		return
	}
	b.color = color
	b.Refresh()
}

// Tapped is called when a pointer tapped event is captured and triggers any change handler
func (b *colorButton) Tapped(*event.PointEvent) {
	if f := b.onTap; f != nil {
		f(b.color)
	}
}

type colorButtonRenderer struct {
	internalwidget.BaseRenderer
	button     *colorButton
	background *canvas.Raster
	rectangle  *canvas.Rectangle
}

func (r *colorButtonRenderer) Layout(size geo.Size) {
	r.rectangle.Move(geo.NewPos(0, 0))
	r.rectangle.Resize(size)
	r.background.Resize(size)
}

func (r *colorButtonRenderer) MinSize() geo.Size {
	return r.rectangle.MinSize().Max(geo.NewSize(32, 32))
}

func (r *colorButtonRenderer) Refresh() {
	if r.button.hovered {
		r.rectangle.StrokeColor = theme.Color(theme.ColorNameHover)
		r.rectangle.StrokeWidth = theme.Padding()
	} else {
		r.rectangle.StrokeWidth = 0
	}
	r.rectangle.FillColor = r.button.color
	r.background.Refresh()
	canvas.Refresh(r.button)
}
