package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// LoadingShape determines the shape of a loading indicator.
type LoadingShape int

const (
	LoadingShapeRect   LoadingShape = iota // Rectangular placeholder (cards, images)
	LoadingShapeCircle                     // Circular placeholder (avatars)
	LoadingShapeText                       // Thin rectangles (text lines)
)

// LoadingIndicator is an MD3 skeleton/shimmer placeholder shown while content loads.
type LoadingIndicator struct {
	BaseWidget
	Shape LoadingShape

	anim    *bellui.Animation
	opacity float32
}

// NewLoadingIndicator creates a loading indicator with the given shape.
func NewLoadingIndicator(shape LoadingShape) *LoadingIndicator {
	l := &LoadingIndicator{Shape: shape, opacity: 0.3}
	l.ExtendBaseWidget(l)
	return l
}

func (l *LoadingIndicator) MinSize() geo.Size {
	l.ExtendBaseWidget(l)
	switch l.Shape {
	case LoadingShapeCircle:
		return geo.NewSize(40, 40)
	case LoadingShapeText:
		return geo.NewSize(200, 14)
	default:
		return geo.NewSize(200, 100)
	}
}

func (l *LoadingIndicator) CreateRenderer() core.WidgetRenderer {
	l.ExtendBaseWidget(l)

	var shape core.CanvasObject
	switch l.Shape {
	case LoadingShapeCircle:
		c := &canvas.Circle{}
		shape = c
	case LoadingShapeText:
		r := canvas.NewRectangle(nil)
		r.CornerRadius = 4
		shape = r
	default:
		r := canvas.NewRectangle(nil)
		r.CornerRadius = 8
		shape = r
	}

	r := &loadingRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{shape}),
		shape:        shape,
		indicator:    l,
	}
	r.Refresh()

	// Start pulse animation.
	l.anim = &bellui.Animation{
		Duration:    canvas.DurationStandard * 3,
		RepeatCount: bellui.AnimationRepeatForever,
		AutoReverse: true,
		Curve:       bellui.AnimationEaseInOut,
		Tick: func(done float32) {
			l.opacity = 0.12 + done*0.24 // oscillate between 0.12 and 0.36
			r.updateOpacity()
		},
	}
	l.anim.Start()

	return r
}

type loadingRenderer struct {
	internalWidget.BaseRenderer
	shape     core.CanvasObject
	indicator *LoadingIndicator
}

func (r *loadingRenderer) MinSize() geo.Size {
	return r.indicator.MinSize()
}

func (r *loadingRenderer) Layout(size geo.Size) {
	r.shape.Resize(size)
}

func (r *loadingRenderer) Refresh() {
	r.updateOpacity()
	r.Layout(r.indicator.Size())
	canvas.Refresh(r.indicator.super())
}

func (r *loadingRenderer) updateOpacity() {
	th := r.indicator.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	baseColor := th.Color(theme.ColorNameOnSurface, v)

	// Apply opacity to base color.
	nr, ng, nb, _ := baseColor.RGBA()
	a := uint8(r.indicator.opacity * 255)

	c := &nrgbaColor{uint8(nr >> 8), uint8(ng >> 8), uint8(nb >> 8), a}

	switch s := r.shape.(type) {
	case *canvas.Circle:
		s.FillColor = c
		s.Refresh()
	case *canvas.Rectangle:
		s.FillColor = c
		s.Refresh()
	}
}

func (r *loadingRenderer) Destroy() {
	if r.indicator.anim != nil {
		r.indicator.anim.Stop()
	}
}

type nrgbaColor struct {
	r, g, b, a uint8
}

func (c *nrgbaColor) RGBA() (r, g, b, a uint32) {
	return uint32(c.r) * 257, uint32(c.g) * 257, uint32(c.b) * 257, uint32(c.a) * 257
}
