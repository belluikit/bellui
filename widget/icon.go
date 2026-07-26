package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

type iconRenderer struct {
	widget.BaseRenderer
	raster *canvas.Image

	image *Icon
}

func (i *iconRenderer) MinSize() geo.Size {
	return geo.NewSquareSize(i.image.Theme().Size(theme.SizeNameInlineIcon))
}

func (i *iconRenderer) Layout(size geo.Size) {
	if len(i.Objects()) == 0 {
		return
	}

	i.Objects()[0].Resize(size)
}

func (i *iconRenderer) Refresh() {
	if i.image.Resource == i.image.cachedRes {
		return
	}

	i.raster.Resource = i.image.Resource
	i.image.cachedRes = i.image.Resource

	if i.image.Resource == nil {
		i.raster.Image = nil // reset the internal caching too...
	}

	i.raster.Refresh()
}

// Icon widget is a basic image component that load's its resource to match the theme.
type Icon struct {
	BaseWidget

	Resource  bellui.Resource // The resource for this icon
	cachedRes bellui.Resource
}

// SetResource updates the resource rendered in this icon widget
func (i *Icon) SetResource(res bellui.Resource) {
	i.Resource = res
	i.Refresh()
}

// MinSize returns the size that this widget should not shrink below
func (i *Icon) MinSize() geo.Size {
	i.ExtendBaseWidget(i)
	return i.BaseWidget.MinSize()
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (i *Icon) CreateRenderer() core.WidgetRenderer {
	i.ExtendBaseWidget(i)

	img := canvas.NewImageFromResource(i.Resource)
	img.FillMode = canvas.ImageFillContain

	r := &iconRenderer{image: i, raster: img}
	r.SetObjects([]core.CanvasObject{img})
	i.cachedRes = i.Resource
	return r
}

// NewIcon returns a new icon widget that displays a themed icon resource
func NewIcon(res bellui.Resource) *Icon {
	icon := &Icon{}
	icon.ExtendBaseWidget(icon)
	icon.SetResource(res) // force the image conversion

	return icon
}
