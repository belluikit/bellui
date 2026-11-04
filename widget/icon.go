package widget

import (
	"image/color"

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
	if i.image.IconSize > 0 {
		return geo.NewSquareSize(i.image.IconSize)
	}
	return geo.NewSquareSize(i.image.Theme().Size(theme.SizeNameInlineIcon))
}

func (i *iconRenderer) Layout(size geo.Size) {
	if len(i.Objects()) == 0 {
		return
	}

	i.Objects()[0].Resize(size)
}

func (i *iconRenderer) Refresh() {
	if i.image.Resource == i.image.cachedRes && sameColor(i.image.Color, i.image.cachedColor) {
		return
	}

	i.raster.Resource = i.image.drawn()
	i.image.cachedRes = i.image.Resource
	i.image.cachedColor = i.image.Color

	if i.image.Resource == nil {
		i.raster.Image = nil // reset the internal caching too...
	}

	i.raster.Refresh()
}

// Icon widget is a basic image component that load's its resource to match the theme.
type Icon struct {
	BaseWidget

	Resource bellui.Resource // The resource for this icon

	// IconSize is the icon's width and height in dp. Zero, the default,
	// keeps the theme's inline icon size (#937). Call Refresh after
	// changing it.
	IconSize float32

	// Color tints the icon with a fixed colour, whatever the theme (#952).
	// nil, the default, draws Resource as given, so a themed resource keeps
	// following the theme. The tint is theme.NewColorizedResource's: an SVG is
	// recoloured, and any other resource is drawn unchanged. Call Refresh after
	// changing it.
	Color color.Color

	cachedRes   bellui.Resource
	cachedColor color.Color
}

// drawn is the resource the renderer draws: Resource, tinted when Color is set.
func (i *Icon) drawn() bellui.Resource {
	if i.Resource == nil || i.Color == nil {
		return i.Resource
	}
	return theme.NewColorizedResource(i.Resource, i.Color)
}

// sameColor reports whether a and b are both nil or the same colour.
func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return color.NRGBAModel.Convert(a) == color.NRGBAModel.Convert(b)
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

	img := canvas.NewImageFromResource(i.drawn())
	img.FillMode = canvas.ImageFillContain

	r := &iconRenderer{image: i, raster: img}
	r.SetObjects([]core.CanvasObject{img})
	i.cachedRes = i.Resource
	i.cachedColor = i.Color
	return r
}

// NewIcon returns a new icon widget that displays a themed icon resource
func NewIcon(res bellui.Resource) *Icon {
	icon := &Icon{}
	icon.ExtendBaseWidget(icon)
	icon.SetResource(res) // force the image conversion

	return icon
}
