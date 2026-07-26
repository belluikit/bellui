package widget

import "github.com/belluikit/bellui/core"

// BaseRenderer is a renderer base that provides part of the widget.Renderer interface.
type BaseRenderer struct {
	objects []core.CanvasObject
}

// NewBaseRenderer creates a new BaseRenderer.
func NewBaseRenderer(objects []core.CanvasObject) BaseRenderer {
	return BaseRenderer{objects}
}

// Destroy does nothing in the base implementation.
func (r *BaseRenderer) Destroy() {
}

// Objects returns the objects that should be rendered.
func (r *BaseRenderer) Objects() []core.CanvasObject {
	return r.objects
}

// SetObjects updates the objects of the renderer.
func (r *BaseRenderer) SetObjects(objects []core.CanvasObject) {
	r.objects = objects
}
