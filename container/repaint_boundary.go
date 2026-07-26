package container

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Declare conformity with Widget interface
var _ core.Widget = (*RepaintBoundary)(nil)

// RepaintBoundary isolates its content subtree for repaint purposes.
// When the subtree is unchanged, it is rendered from a cached framebuffer
// object (FBO) as a single textured quad, avoiding the cost of walking
// and drawing the entire subtree each frame.
//
// Use this to wrap independently-updating regions of the UI (e.g. charts,
// video players, animated panels) so that refreshing one region does not
// force a full repaint of the entire canvas.
//
// Since: 2.7
type RepaintBoundary struct {
	widget.BaseWidget
	Content core.CanvasObject
}

// NewRepaintBoundary creates a new RepaintBoundary wrapping the given content.
//
// Since: 2.7
func NewRepaintBoundary(content core.CanvasObject) *RepaintBoundary {
	rb := &RepaintBoundary{Content: content}
	rb.ExtendBaseWidget(rb)
	return rb
}

// CreateRenderer returns a renderer for the RepaintBoundary.
func (rb *RepaintBoundary) CreateRenderer() core.WidgetRenderer {
	rb.ExtendBaseWidget(rb)
	return &repaintBoundaryRenderer{
		boundary: rb,
		objects:  []core.CanvasObject{rb.Content},
	}
}

type repaintBoundaryRenderer struct {
	boundary *RepaintBoundary
	objects  []core.CanvasObject
}

func (r *repaintBoundaryRenderer) Destroy() {}

func (r *repaintBoundaryRenderer) Layout(s geo.Size) {
	if r.objects[0] != nil {
		r.objects[0].Resize(s)
	}
}

func (r *repaintBoundaryRenderer) MinSize() geo.Size {
	if r.objects[0] != nil {
		return r.objects[0].MinSize()
	}
	return geo.Size{}
}

func (r *repaintBoundaryRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *repaintBoundaryRenderer) Refresh() {
	r.objects[0] = r.boundary.Content
	if r.objects[0] != nil {
		r.Layout(r.boundary.Size())
		r.objects[0].Refresh()
	}
}

// IsRepaintBoundary is a marker method detected by the paint walk to enable
// FBO-cached rendering of this widget's subtree.
func (r *repaintBoundaryRenderer) IsRepaintBoundary() {}
