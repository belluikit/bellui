package render

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// LoadingOverlay displays a semi-transparent overlay with a loading message.
type LoadingOverlay struct {
	widget.BaseWidget

	Message string
	visible bool
}

// NewLoadingOverlay creates a loading overlay.
func NewLoadingOverlay() *LoadingOverlay {
	o := &LoadingOverlay{Message: "Loading..."}
	o.ExtendBaseWidget(o)
	return o
}

// Show displays the overlay.
func (o *LoadingOverlay) Show() {
	o.visible = true
	o.BaseWidget.Show()
	o.Refresh()
}

// Hide hides the overlay.
func (o *LoadingOverlay) Hide() {
	o.visible = false
	o.BaseWidget.Hide()
}

// SetMessage updates the loading message.
func (o *LoadingOverlay) SetMessage(msg string) {
	o.Message = msg
	o.Refresh()
}

func (o *LoadingOverlay) CreateRenderer() core.WidgetRenderer {
	o.ExtendBaseWidget(o)
	tc := ResolveThemeColors()
	// Semi-transparent background overlay
	r, g, b, _ := tc.Background.RGBA()
	overlayBg := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 180}
	bg := canvas.NewRectangle(overlayBg)
	label := canvas.NewText("Loading...", tc.Foreground)
	label.TextSize = 14
	label.Alignment = bellui.TextAlignCenter
	return &loadingRenderer{
		overlay: o,
		bg:      bg,
		label:   label,
	}
}

type loadingRenderer struct {
	overlay *LoadingOverlay
	bg      *canvas.Rectangle
	label   *canvas.Text
}

func (r *loadingRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	r.label.Move(geo.NewPos(0, size.Height/2-10))
	r.label.Resize(geo.NewSize(size.Width, 20))
}

func (r *loadingRenderer) MinSize() geo.Size {
	return geo.NewSize(100, 50)
}

func (r *loadingRenderer) Refresh() {
	r.label.Text = r.overlay.Message
	r.label.Refresh()
}

func (r *loadingRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.bg, r.label}
}

func (r *loadingRenderer) Destroy() {}
