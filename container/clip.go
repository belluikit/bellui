package container

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// Declare conformity with Widget interface
var _ core.Widget = (*Clip)(nil)

// Clip describes a rectangular region  that will clip anything outside its bounds.
//
// Since: 2.7
type Clip struct {
	widget.BaseWidget
	Content core.CanvasObject
}

// NewClip returns a new rectangular clipping object.
//
// Since: 2.7
func NewClip(content core.CanvasObject) *Clip {
	c := &Clip{Content: content}
	c.ExtendBaseWidget(c)
	return c
}

func (c *Clip) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	return newClipRenderer(c)
}

// MinSize for a Clip simply returns Size{1, 1} as there is no
// explicit content
func (c *Clip) MinSize() geo.Size {
	c.ExtendBaseWidget(c)
	return geo.NewSize(1, 1)
}

type clipRenderer struct {
	c       *Clip
	objects []core.CanvasObject
}

func newClipRenderer(c *Clip) *clipRenderer {
	return &clipRenderer{c: c, objects: []core.CanvasObject{c.Content}}
}

func (r *clipRenderer) Destroy() {
}

func (r *clipRenderer) Layout(s geo.Size) {
	o := r.objects[0]
	o.Resize(s.Max(o.MinSize()))
}

func (r *clipRenderer) MinSize() geo.Size {
	return r.objects[0].MinSize()
}

func (r *clipRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *clipRenderer) Refresh() {
	r.objects[0] = r.c.Content
	r.Layout(r.c.Size())
	r.objects[0].Refresh()
}

// IsClip marks this widget as clipping. It is on the renderer to avoid a public API addition.
func (r *clipRenderer) IsClip() {}
