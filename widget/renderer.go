package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// Renderer builds a WidgetRenderer using a fluent API, eliminating the need
// for a separate renderer struct. Each method is optional with sensible defaults.
//
// Usage inside CreateRenderer:
//
//	func (w *MyBadge) CreateRenderer() core.WidgetRenderer {
//	    bg := canvas.NewRectangle(color.NRGBA{R: 200, A: 255})
//	    label := canvas.NewText(w.Text, color.White)
//
//	    return widget.NewRenderer(bg, label).
//	        Layout(func(size geo.Size) {
//	            bg.Resize(size)
//	            label.Move(geo.NewPos(8, 4))
//	        }).
//	        MinSizeFunc(func() geo.Size {
//	            ts := label.MinSize()
//	            return geo.NewSize(ts.Width+16, ts.Height+8)
//	        }).
//	        Refresh(func() {
//	            label.Text = w.Text
//	            canvas.Refresh(label)
//	        })
//	}
//
// Defaults:
//   - Layout: all objects resize to fill the widget
//   - MinSize: largest child's MinSize
//   - Refresh: no-op
//   - Destroy: no-op
type Renderer struct {
	objects   []core.CanvasObject
	layoutFn  func(geo.Size)
	minSizeFn func() geo.Size
	refreshFn func()
	destroyFn func()
}

// NewRenderer creates a renderer with the given canvas objects.
func NewRenderer(objects ...core.CanvasObject) *Renderer {
	return &Renderer{objects: objects}
}

// Layout sets the layout function that positions child objects.
func (r *Renderer) Layout(fn func(geo.Size)) *Renderer {
	r.layoutFn = fn
	return r
}

// MinSizeFunc sets a custom minimum size calculator.
func (r *Renderer) MinSizeFunc(fn func() geo.Size) *Renderer {
	r.minSizeFn = fn
	return r
}

// Refresh sets the function called when the widget state changes
// and canvas objects need updating (colors, text, visibility).
func (r *Renderer) Refresh(fn func()) *Renderer {
	r.refreshFn = fn
	return r
}

// DestroyFunc sets a cleanup function called when the renderer is disposed.
func (r *Renderer) DestroyFunc(fn func()) *Renderer {
	r.destroyFn = fn
	return r
}

// --- WidgetRenderer interface ---

func (r *Renderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *Renderer) SetObjects(objects []core.CanvasObject) {
	r.objects = objects
}

func (r *Renderer) doLayout(size geo.Size) {
	if r.layoutFn != nil {
		r.layoutFn(size)
		return
	}
	for _, obj := range r.objects {
		obj.Resize(size)
	}
}

func (r *Renderer) doMinSize() geo.Size {
	if r.minSizeFn != nil {
		return r.minSizeFn()
	}
	var min geo.Size
	for _, obj := range r.objects {
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

func (r *Renderer) doRefresh() {
	if r.refreshFn != nil {
		r.refreshFn()
	}
}

func (r *Renderer) doDestroy() {
	if r.destroyFn != nil {
		r.destroyFn()
	}
}

// Ensure Renderer implements WidgetRenderer via the rendererAdapter.
// We can't implement the interface methods directly on Renderer because
// Layout/Refresh/Destroy are used as builder method names.
// The adapter is returned transparently via the Build method.

// Build returns the WidgetRenderer. Call this at the end of the builder chain
// if needed for explicit type conversion.
func (r *Renderer) Build() core.WidgetRenderer {
	return &rendererAdapter{r}
}

type rendererAdapter struct{ r *Renderer }

func (a *rendererAdapter) Destroy()                     { a.r.doDestroy() }
func (a *rendererAdapter) Layout(size geo.Size)         { a.r.doLayout(size) }
func (a *rendererAdapter) MinSize() geo.Size            { return a.r.doMinSize() }
func (a *rendererAdapter) Objects() []core.CanvasObject { return a.r.objects }
func (a *rendererAdapter) Refresh()                     { a.r.doRefresh() }
