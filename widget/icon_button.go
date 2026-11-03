package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// IconButtonStyle determines the visual variant of an icon button.
type IconButtonStyle int

const (
	// IconButtonStyleStandard has no background, icon color OnSurfaceVariant.
	IconButtonStyleStandard IconButtonStyle = iota
	// IconButtonStyleFilled has Primary background, OnPrimary icon.
	IconButtonStyleFilled
	// IconButtonStyleFilledTonal has SecondaryContainer background.
	IconButtonStyleFilledTonal
	// IconButtonStyleOutlined has border, no fill, OnSurfaceVariant icon.
	IconButtonStyleOutlined
)

// IconButtonSize selects one of the Material 3 Expressive icon button sizes.
// The zero value, [IconButtonSizeDefault], takes both the container and the
// icon size from the theme, so dense themes stay dense. The other sizes are
// fixed by the spec, in dp: container / icon.
type IconButtonSize int

const (
	// IconButtonSizeDefault uses the theme's [theme.SizeNameIconButtonSize]
	// container and [theme.SizeNameIconSize] icon (40 / 24 in the default theme).
	IconButtonSizeDefault IconButtonSize = iota
	// IconButtonSizeExtraSmall is 32 / 20.
	IconButtonSizeExtraSmall
	// IconButtonSizeSmall is 40 / 24.
	IconButtonSizeSmall
	// IconButtonSizeMedium is 56 / 24.
	IconButtonSizeMedium
	// IconButtonSizeLarge is 96 / 32.
	IconButtonSizeLarge
	// IconButtonSizeExtraLarge is 136 / 40.
	IconButtonSizeExtraLarge
)

// dims returns the container and icon size for s; the default size reads th.
func (s IconButtonSize) dims(th bellui.Theme) (container, icon float32) {
	switch s {
	case IconButtonSizeExtraSmall:
		return 32, 20
	case IconButtonSizeSmall:
		return 40, 24
	case IconButtonSizeMedium:
		return 56, 24
	case IconButtonSizeLarge:
		return 96, 32
	case IconButtonSizeExtraLarge:
		return 136, 40
	default:
		return th.Size(theme.SizeNameIconButtonSize), th.Size(theme.SizeNameIconSize)
	}
}

var (
	_ event.Tappable          = (*IconButton)(nil)
	_ event.Hoverable         = (*IconButton)(nil)
	_ core.Accessible         = (*IconButton)(nil)
	_ core.AccessibleActioner = (*IconButton)(nil)
)

// AccessibleRole returns [core.RoleButton].
func (b *IconButton) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns an empty string; icon buttons have no intrinsic text label.
// Use [AccessibleDescriber] to provide a description for screen readers.
func (b *IconButton) AccessibleLabel() string { return "" }

// AccessibleActions returns the "press" action.
func (b *IconButton) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Activate the button"}}
}

// PerformAccessibleAction invokes the button's tap handler.
func (b *IconButton) PerformAccessibleAction(name string) {
	if name == core.ActionPress && !b.Disabled() && b.OnTapped != nil {
		b.OnTapped()
	}
}

// IconButton is an icon-only button with MD3 visual variants.
type IconButton struct {
	DisableableWidget
	Icon  bellui.Resource
	Style IconButtonStyle
	// ButtonSize picks the button's size variant. It is not called Size
	// because Size returns the widget's current size.
	ButtonSize IconButtonSize
	OnTapped   func() `json:"-"`

	hovered bool
}

// NewIconButton creates a standard icon button.
func NewIconButton(icon bellui.Resource, tapped func()) *IconButton {
	b := &IconButton{Icon: icon, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

func (b *IconButton) Tapped(*event.PointEvent) {
	if b.Disabled() {
		return
	}
	if b.OnTapped != nil {
		b.OnTapped()
	}
}

func (b *IconButton) HoverIn(*event.HoverEvent)    { b.hovered = true; b.Refresh() }
func (b *IconButton) HoverMoved(*event.HoverEvent) {}
func (b *IconButton) HoverOut()                    { b.hovered = false; b.Refresh() }

func (b *IconButton) MinSize() geo.Size {
	b.ExtendBaseWidget(b)
	return b.BaseWidget.MinSize()
}

func (b *IconButton) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	bg := canvas.NewRectangle(color.Transparent)
	icon := canvas.NewImageFromResource(b.Icon)
	icon.FillMode = canvas.ImageFillContain

	r := &iconButtonRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer([]core.CanvasObject{bg, icon}),
		bg:           bg,
		icon:         icon,
		btn:          b,
	}
	r.Refresh()
	return r
}

type iconButtonRenderer struct {
	internalWidget.BaseRenderer
	bg   *canvas.Rectangle
	icon *canvas.Image
	btn  *IconButton
}

func (r *iconButtonRenderer) MinSize() geo.Size {
	s, _ := r.btn.ButtonSize.dims(r.btn.Theme())
	return geo.NewSize(s, s)
}

func (r *iconButtonRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	_, iconSize := r.btn.ButtonSize.dims(r.btn.Theme())
	r.icon.Resize(geo.NewSize(iconSize, iconSize))
	r.icon.Move(geo.NewPos((size.Width-iconSize)/2, (size.Height-iconSize)/2))
}

func (r *iconButtonRenderer) Refresh() {
	th := r.btn.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	container, _ := r.btn.ButtonSize.dims(th)
	r.bg.CornerRadius = container / 2

	var bgColorName, fgColorName bellui.ThemeColorName
	switch r.btn.Style {
	case IconButtonStyleFilled:
		bgColorName = theme.ColorNamePrimary
		fgColorName = theme.ColorNameForegroundOnPrimary
	case IconButtonStyleFilledTonal:
		bgColorName = theme.ColorNameSecondaryContainer
		fgColorName = theme.ColorNameOnSecondaryContainer
	case IconButtonStyleOutlined:
		fgColorName = theme.ColorNameOnSurfaceVariant
		r.bg.FillColor = color.Transparent
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = th.Color(theme.ColorNameOutlineVariant, v)
	default: // Standard — visible background for contrast on light surfaces.
		bgColorName = theme.ColorNameButton
		fgColorName = theme.ColorNameOnSurfaceVariant
	}

	if bgColorName != "" {
		bg := th.Color(bgColorName, v)
		if r.btn.hovered {
			bg = blendColor(bg, theme.StateLayerHover(th.Color(fgColorName, v)))
		}
		r.bg.FillColor = bg
	} else if r.btn.hovered {
		r.bg.FillColor = theme.StateLayerHover(th.Color(fgColorName, v))
	}

	if r.btn.Disabled() {
		fgColorName = theme.ColorNameDisabled
		r.bg.FillColor = color.Transparent
	}

	r.bg.Refresh()

	if r.btn.Icon != nil {
		r.icon.Resource = theme.NewColoredResource(r.btn.Icon, fgColorName)
		r.icon.Refresh()
	}

	r.Layout(r.btn.Size())
	canvas.Refresh(r.btn.super())
}
