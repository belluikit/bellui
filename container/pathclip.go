package container

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Declare conformity with Widget interface
var _ core.Widget = (*PathClip)(nil)

// PathClip clips its content to an arbitrary path shape using stencil-based
// clipping. The ClipPath defines the region inside which content is visible.
type PathClip struct {
	widget.BaseWidget
	ClipPath *canvas.Path
	Content  core.CanvasObject
}

// NewPathClip returns a new path-clipping container.
func NewPathClip(clipPath *canvas.Path, content core.CanvasObject) *PathClip {
	pc := &PathClip{ClipPath: clipPath, Content: content}
	pc.ExtendBaseWidget(pc)
	return pc
}

// CreateRenderer returns a new renderer for PathClip.
func (pc *PathClip) CreateRenderer() core.WidgetRenderer {
	pc.ExtendBaseWidget(pc)
	return &pathClipRenderer{pc: pc, objects: []core.CanvasObject{pc.Content}}
}

// MinSize returns the minimum size — delegates to content.
func (pc *PathClip) MinSize() geo.Size {
	pc.ExtendBaseWidget(pc)
	if pc.Content == nil {
		return geo.NewSize(1, 1)
	}
	return pc.Content.MinSize()
}

type pathClipRenderer struct {
	pc      *PathClip
	objects []core.CanvasObject
}

func (r *pathClipRenderer) Destroy() {}

func (r *pathClipRenderer) Layout(s geo.Size) {
	if r.pc.Content != nil {
		r.pc.Content.Resize(s.Max(r.pc.Content.MinSize()))
	}
}

func (r *pathClipRenderer) MinSize() geo.Size {
	if r.pc.Content == nil {
		return geo.NewSize(1, 1)
	}
	return r.pc.Content.MinSize()
}

func (r *pathClipRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *pathClipRenderer) Refresh() {
	r.objects[0] = r.pc.Content
	r.Layout(r.pc.Size())
	if r.pc.Content != nil {
		r.pc.Content.Refresh()
	}
}

// ClipPath returns the path used for clipping. Returns any to avoid import
// cycles in the internal/driver package. This is a marker method detected
// by the paint loop to enable stencil-based path clipping.
func (r *pathClipRenderer) ClipPath() any {
	return r.pc.ClipPath
}
