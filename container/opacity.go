package container

import (
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

var (
	_ core.Widget       = (*Opacity)(nil)
	_ core.OpacityGroup = (*Opacity)(nil)
)

// Opacity draws its content translucent: everything in it is drawn with its
// alpha multiplied by the Opacity, between 0 (not drawn) and 1 (unchanged).
// Opacity containers nest, multiplying. Each draw is faded on its own, so
// where two objects in the content overlap, the lower one shows through.
//
// It is a [core.OpacityGroup], which both the desktop and the mobile drivers
// honour. The software painter behind test images does not, so a rendered
// test image shows the content at full opacity.
type Opacity struct {
	widget.BaseWidget
	Content core.CanvasObject
	// Opacity is the content's opacity. Change it with SetOpacity, or set it
	// and Refresh, which also refreshes the content.
	Opacity float32
}

// NewOpacity wraps content to draw it at the given opacity.
func NewOpacity(content core.CanvasObject, opacity float32) *Opacity {
	o := &Opacity{Content: content, Opacity: opacity}
	o.ExtendBaseWidget(o)
	return o
}

// SetOpacity changes the content's opacity and redraws it. The content is not
// refreshed: the painters apply the opacity per draw, so nothing in it changes,
// and an animated opacity would otherwise refresh every object under it on
// every frame, an image re-reading its file each time (#1138).
func (o *Opacity) SetOpacity(opacity float32) {
	o.Opacity = opacity
	canvas.Refresh(o)
}

// GroupOpacity returns the Opacity clamped to [0, 1].
func (o *Opacity) GroupOpacity() float32 {
	return max(0, min(1, o.Opacity))
}

func (o *Opacity) CreateRenderer() core.WidgetRenderer {
	o.ExtendBaseWidget(o)
	return &opacityRenderer{o: o, objects: []core.CanvasObject{o.Content}}
}

// MinSize is the content's.
func (o *Opacity) MinSize() geo.Size {
	o.ExtendBaseWidget(o)
	return o.BaseWidget.MinSize()
}

type opacityRenderer struct {
	o       *Opacity
	objects []core.CanvasObject
}

func (r *opacityRenderer) Destroy() {}

func (r *opacityRenderer) Layout(s geo.Size) {
	r.objects[0].Move(geo.Position{})
	r.objects[0].Resize(s)
}

func (r *opacityRenderer) MinSize() geo.Size { return r.objects[0].MinSize() }

func (r *opacityRenderer) Objects() []core.CanvasObject { return r.objects }

func (r *opacityRenderer) Refresh() {
	r.objects[0] = r.o.Content
	r.Layout(r.o.Size())
	r.objects[0].Refresh()
}
