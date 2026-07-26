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

// FABStyle determines the color scheme of a FAB.
type FABStyle int

const (
	FABStylePrimary   FABStyle = iota // PrimaryContainer bg (default)
	FABStyleSurface                   // SurfaceContainerHigh bg
	FABStyleSecondary                 // SecondaryContainer bg
	FABStyleTertiary                  // TertiaryContainer bg
)

const (
	fabExtendedPadLeading  float32 = 16 // MD3 extended FAB leading padding
	fabExtendedIconTextGap float32 = 8  // gap between icon and label
	fabExtendedPadTrailing float32 = 20 // MD3 extended FAB trailing padding
)

var (
	_ event.Tappable          = (*FAB)(nil)
	_ event.Hoverable         = (*FAB)(nil)
	_ core.Accessible         = (*FAB)(nil)
	_ core.AccessibleActioner = (*FAB)(nil)
)

// AccessibleRole returns [core.RoleButton].
func (f *FAB) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the FAB text (empty for icon-only FABs).
func (f *FAB) AccessibleLabel() string { return f.Text }

// AccessibleActions returns the "press" action.
func (f *FAB) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Activate the button"}}
}

// PerformAccessibleAction invokes the FAB's tap handler.
func (f *FAB) PerformAccessibleAction(name string) {
	if name == core.ActionPress && !f.Disabled() && f.OnTapped != nil {
		f.OnTapped()
	}
}

// FAB is a Material Design 3 Floating Action Button.
// Set Text for an Extended FAB (icon + label), leave empty for a standard FAB (icon only).
type FAB struct {
	DisableableWidget
	Icon     bellui.Resource
	Text     string // empty = regular FAB, non-empty = Extended FAB
	Style    FABStyle
	OnTapped func() `json:"-"`

	hovered bool
}

// NewFAB creates a standard (icon-only) FAB.
func NewFAB(icon bellui.Resource, tapped func()) *FAB {
	f := &FAB{Icon: icon, OnTapped: tapped}
	f.ExtendBaseWidget(f)
	return f
}

// NewExtendedFAB creates an extended FAB with icon and label.
func NewExtendedFAB(text string, icon bellui.Resource, tapped func()) *FAB {
	f := &FAB{Text: text, Icon: icon, OnTapped: tapped}
	f.ExtendBaseWidget(f)
	return f
}

func (f *FAB) Tapped(*event.PointEvent) {
	if f.Disabled() {
		return
	}
	if f.OnTapped != nil {
		f.OnTapped()
	}
}

func (f *FAB) HoverIn(*event.HoverEvent)    { f.hovered = true; f.Refresh() }
func (f *FAB) HoverMoved(*event.HoverEvent) {}
func (f *FAB) HoverOut()                    { f.hovered = false; f.Refresh() }

func (f *FAB) MinSize() geo.Size {
	f.ExtendBaseWidget(f)
	return f.BaseWidget.MinSize()
}

func (f *FAB) CreateRenderer() core.WidgetRenderer {
	f.ExtendBaseWidget(f)
	bg := canvas.NewRectangle(color.Transparent)
	icon := canvas.NewImageFromResource(f.Icon)
	icon.FillMode = canvas.ImageFillContain

	objects := []core.CanvasObject{bg, icon}
	var label *canvas.Text
	if f.Text != "" {
		th := f.Theme()
		v := bellui.CurrentApp().Settings().ThemeVariant()
		label = canvas.NewText(f.Text, th.Color(theme.ColorNameOnPrimaryContainer, v))
		label.TextStyle.Bold = true
		objects = append(objects, label)
	}

	r := &fabRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		icon:         icon,
		label:        label,
		fab:          f,
	}
	r.Refresh()
	return r
}

type fabRenderer struct {
	internalWidget.BaseRenderer
	bg    *canvas.Rectangle
	icon  *canvas.Image
	label *canvas.Text
	fab   *FAB
}

func (r *fabRenderer) MinSize() geo.Size {
	th := r.fab.Theme()
	fabSize := th.Size(theme.SizeNameFABSize)
	iconSize := th.Size(theme.SizeNameIconSize)
	if r.fab.Text != "" {
		// Extended FAB: icon + padding + text + padding
		labelSize := th.Size(theme.SizeNameLabelLarge)
		textWidth := bellui.MeasureText(r.fab.Text, labelSize, bellui.TextStyle{Bold: true}).Width
		return geo.NewSize(fabExtendedPadLeading+iconSize+fabExtendedIconTextGap+textWidth+fabExtendedPadTrailing, fabSize)
	}
	return geo.NewSize(fabSize, fabSize) // Standard FAB
}

func (r *fabRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	th := r.fab.Theme()
	iconSize := th.Size(theme.SizeNameIconSize)
	fabSize := th.Size(theme.SizeNameFABSize)
	pad := (fabSize - iconSize) / 2

	if r.fab.Text != "" {
		// Extended: icon left, label right
		r.icon.Resize(geo.NewSize(iconSize, iconSize))
		r.icon.Move(geo.NewPos(pad, (size.Height-iconSize)/2))
		if r.label != nil {
			r.label.Move(geo.NewPos(pad+iconSize+8, (size.Height-r.label.MinSize().Height)/2))
		}
	} else {
		// Standard: centered icon
		r.icon.Resize(geo.NewSize(iconSize, iconSize))
		r.icon.Move(geo.NewPos((size.Width-iconSize)/2, (size.Height-iconSize)/2))
	}
}

func (r *fabRenderer) Refresh() {
	th := r.fab.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	var bgColor, fgColor bellui.ThemeColorName
	switch r.fab.Style {
	case FABStyleSurface:
		bgColor, fgColor = theme.ColorNameSurfaceContainerHigh, theme.ColorNamePrimary
	case FABStyleSecondary:
		bgColor, fgColor = theme.ColorNameSecondaryContainer, theme.ColorNameOnSecondaryContainer
	case FABStyleTertiary:
		bgColor, fgColor = theme.ColorNameTertiaryContainer, theme.ColorNameOnTertiaryContainer
	default:
		bgColor, fgColor = theme.ColorNamePrimaryContainer, theme.ColorNameOnPrimaryContainer
	}

	bg := th.Color(bgColor, v)
	if r.fab.hovered {
		bg = blendColor(bg, theme.StateLayerHover(th.Color(fgColor, v)))
	}

	r.bg.CornerRadius = th.Size(theme.SizeNameShapeLarge)

	r.bg.FillColor = bg
	r.bg.Refresh()

	if r.fab.Icon != nil {
		r.icon.Resource = theme.NewColoredResource(r.fab.Icon, fgColor)
		r.icon.Refresh()
	}

	if r.fab.Text != "" && r.label == nil {
		// Text was set after creation; create the label now.
		r.label = canvas.NewText(r.fab.Text, th.Color(fgColor, v))
		r.label.TextStyle.Bold = true
		r.SetObjects(append(r.Objects(), r.label))
	} else if r.fab.Text == "" && r.label != nil {
		// Text was cleared; remove the label.
		r.label = nil
		r.SetObjects([]core.CanvasObject{r.bg, r.icon})
	}
	if r.label != nil {
		r.label.Text = r.fab.Text
		r.label.Color = th.Color(fgColor, v)
		r.label.TextSize = th.Size(theme.SizeNameLabelLarge)
		r.label.Refresh()
	}

	r.Layout(r.fab.Size())
	canvas.Refresh(r.fab.super())
}
