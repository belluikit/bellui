package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// SimpleWidget creates a widget from canvas objects and a layout function,
// avoiding the need to implement CreateRenderer and a separate renderer type.
//
// Usage:
//
//	bg := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
//	label := canvas.NewText("Hello", color.White)
//
//	w := widget.NewSimpleWidget(
//	    func() []core.CanvasObject { return []core.CanvasObject{bg, label} },
//	    func(size geo.Size) {
//	        bg.Resize(size)
//	        label.Move(geo.NewPos(8, 4))
//	    },
//	)
//
// For even simpler cases where you just want to stack objects:
//
//	w := widget.NewSimpleWidget(
//	    func() []core.CanvasObject { return []core.CanvasObject{bg, label} },
//	    nil, // nil layout = all objects fill the widget size
//	)
type SimpleWidget struct {
	BaseWidget

	objects   func() []core.CanvasObject
	layout    func(geo.Size)
	minSize   func() geo.Size
	onRefresh func()
}

// NewSimpleWidget creates a widget from an objects function and an optional layout function.
// The objects function returns the canvas objects to render.
// The layout function positions them when the widget is resized (nil = fill).
func NewSimpleWidget(objects func() []core.CanvasObject, layout func(geo.Size)) *SimpleWidget {
	w := &SimpleWidget{
		objects: objects,
		layout:  layout,
	}
	w.ExtendBaseWidget(w)
	return w
}

// SetMinSizeFunc sets a custom minimum size calculator.
func (w *SimpleWidget) SetMinSizeFunc(fn func() geo.Size) *SimpleWidget {
	w.minSize = fn
	return w
}

// SetRefreshFunc sets a function called when the widget is refreshed.
// Use this to update canvas object properties (colors, text, etc.) based on state.
func (w *SimpleWidget) SetRefreshFunc(fn func()) *SimpleWidget {
	w.onRefresh = fn
	return w
}

// CreateRenderer returns the widget renderer.
func (w *SimpleWidget) CreateRenderer() core.WidgetRenderer {
	return &simpleRenderer{widget: w}
}

type simpleRenderer struct {
	widget *SimpleWidget
}

func (r *simpleRenderer) Destroy() {}

func (r *simpleRenderer) Layout(size geo.Size) {
	if r.widget.layout != nil {
		r.widget.layout(size)
		return
	}
	// Default: all objects fill the widget size.
	for _, obj := range r.widget.objects() {
		obj.Resize(size)
	}
}

func (r *simpleRenderer) MinSize() geo.Size {
	if r.widget.minSize != nil {
		return r.widget.minSize()
	}
	// Default: largest child's MinSize.
	var min geo.Size
	for _, obj := range r.widget.objects() {
		objMin := obj.MinSize()
		if objMin.Width > min.Width {
			min.Width = objMin.Width
		}
		if objMin.Height > min.Height {
			min.Height = objMin.Height
		}
	}
	return min
}

func (r *simpleRenderer) Objects() []core.CanvasObject {
	return r.widget.objects()
}

func (r *simpleRenderer) Refresh() {
	if r.widget.onRefresh != nil {
		r.widget.onRefresh()
	}
}
