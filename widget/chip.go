package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// ChipStyle determines the type of chip.
type ChipStyle int

const (
	ChipStyleAssist     ChipStyle = iota // Outlined, icon+label
	ChipStyleFilter                      // Selectable; secondary-container fill when selected (no checkmark)
	ChipStyleInput                       // Label + delete button
	ChipStyleSuggestion                  // Outlined, label only
)

var (
	_ event.Tappable  = (*Chip)(nil)
	_ event.Hoverable = (*Chip)(nil)
	_ core.Accessible = (*Chip)(nil)
)

// AccessibleRole returns [core.RoleButton].
func (c *Chip) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the chip text.
func (c *Chip) AccessibleLabel() string { return c.Text }

// Chip is a compact MD3 element for selections, filters, or actions.
type Chip struct {
	BaseWidget
	Text      string
	Icon      bellui.Resource // optional leading icon
	Style     ChipStyle
	Selected  bool   // for Filter chips
	OnTapped  func() `json:"-"`
	OnDeleted func() `json:"-"` // for Input chips (shows X)

	hovered bool
}

// NewChip creates an assist/suggestion chip.
func NewChip(text string, tapped func()) *Chip {
	c := &Chip{Text: text, OnTapped: tapped}
	c.ExtendBaseWidget(c)
	return c
}

// NewFilterChip creates a filter chip with selection toggle.
func NewFilterChip(text string, changed func(bool)) *Chip {
	c := &Chip{Text: text, Style: ChipStyleFilter}
	c.OnTapped = func() {
		c.Selected = !c.Selected
		if changed != nil {
			changed(c.Selected)
		}
		c.Refresh()
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *Chip) Tapped(e *event.PointEvent) {
	// On an input chip a tap landing on the trailing delete (X) button fires
	// OnDeleted rather than OnTapped; otherwise the X would be inert (pre-rebrand tracker #291).
	if c.Style == ChipStyleInput && c.OnDeleted != nil && e != nil && c.inDeleteRegion(e.Position.X) {
		c.OnDeleted()
		return
	}
	if c.OnTapped != nil {
		c.OnTapped()
	}
}

// inDeleteRegion reports whether x falls on the delete button, which sits on
// the trailing edge of the chip (mirrored to the leading edge under RTL).
func (c *Chip) inDeleteRegion(x float32) bool {
	iconSz := c.Theme().Size(theme.SizeNameIconSize) * 0.75
	region := iconSz + 12 // button width plus surrounding padding
	if lang.IsRTLDirection() {
		return x < region
	}
	return x > c.Size().Width-region
}

func (c *Chip) HoverIn(*event.HoverEvent)    { c.hovered = true; c.Refresh() }
func (c *Chip) HoverMoved(*event.HoverEvent) {}
func (c *Chip) HoverOut()                    { c.hovered = false; c.Refresh() }

func (c *Chip) MinSize() geo.Size {
	c.ExtendBaseWidget(c)
	return c.BaseWidget.MinSize()
}

func (c *Chip) CreateRenderer() core.WidgetRenderer {
	c.ExtendBaseWidget(c)
	th := c.Theme()
	bg := canvas.NewRectangle(color.Transparent)
	label := canvas.NewText(c.Text, color.White)
	label.TextSize = th.Size(theme.SizeNameLabelLarge)

	var icon *canvas.Image
	objects := []core.CanvasObject{bg, label}

	if c.Icon != nil {
		icon = canvas.NewImageFromResource(c.Icon)
		icon.FillMode = canvas.ImageFillContain
		objects = append(objects, icon)
	}

	var deleteBtn *canvas.Image
	if c.Style == ChipStyleInput && c.OnDeleted != nil {
		deleteBtn = canvas.NewImageFromResource(theme.CancelIcon())
		deleteBtn.FillMode = canvas.ImageFillContain
		objects = append(objects, deleteBtn)
	}

	r := &chipRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		label:        label,
		icon:         icon,
		deleteBtn:    deleteBtn,
		chip:         c,
	}
	r.Refresh()
	return r
}

type chipRenderer struct {
	internalWidget.BaseRenderer
	bg        *canvas.Rectangle
	label     *canvas.Text
	icon      *canvas.Image
	deleteBtn *canvas.Image
	chip      *Chip
}

func (r *chipRenderer) MinSize() geo.Size {
	th := r.chip.Theme()
	chipH := th.Size(theme.SizeNameChipHeight)
	textSize := th.Size(theme.SizeNameLabelLarge)
	iconSz := th.Size(theme.SizeNameIconSize) * 0.75
	hPad := chipH * 0.375 // horizontal padding derived from chip height

	textWidth := bellui.MeasureText(r.chip.Text, textSize, bellui.TextStyle{}).Width
	w := hPad + textWidth + hPad // left pad + text + right pad
	if r.chip.Icon != nil {
		w += iconSz + 8 // icon + gap
	}
	if r.deleteBtn != nil {
		w += iconSz + 4 // delete + gap
	}
	return geo.NewSize(w, chipH)
}

func (r *chipRenderer) Layout(size geo.Size) {
	th := r.chip.Theme()
	chipH := th.Size(theme.SizeNameChipHeight)
	iconSz := th.Size(theme.SizeNameIconSize) * 0.75
	hPad := chipH * 0.375

	r.bg.Resize(size)
	x := hPad

	if r.icon != nil {
		r.icon.Resize(geo.NewSize(iconSz, iconSz))
		r.icon.Move(geo.NewPos(x, (size.Height-iconSz)/2))
		x += iconSz + 8
	}

	labelSize := r.label.MinSize()
	r.label.Move(geo.NewPos(x, (size.Height-labelSize.Height)/2))
	r.label.Resize(labelSize) // tight box so the RTL reflection lands correctly
	x += labelSize.Width + 4

	if r.deleteBtn != nil {
		r.deleteBtn.Resize(geo.NewSize(iconSz, iconSz))
		r.deleteBtn.Move(geo.NewPos(size.Width-iconSz-8, (size.Height-iconSz)/2))
	}

	// Under RTL the element row (icon | label | delete) reflects so the leading
	// icon sits on the right; the full-width bg maps onto itself.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *chipRenderer) Refresh() {
	th := r.chip.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.bg.CornerRadius = th.Size(theme.SizeNameShapeSmall) // MD3: small shape for chips
	r.label.Text = r.chip.Text
	r.label.TextSize = th.Size(theme.SizeNameLabelLarge)

	if r.chip.Style == ChipStyleFilter && r.chip.Selected {
		bgColor := th.Color(theme.ColorNameSecondaryContainer, v)
		if r.chip.hovered {
			bgColor = blendColor(bgColor, theme.StateLayerHover(th.Color(theme.ColorNameOnSecondaryContainer, v)))
		}
		r.bg.FillColor = bgColor
		r.bg.StrokeWidth = 0
		r.label.Color = th.Color(theme.ColorNameOnSecondaryContainer, v)
	} else {
		r.bg.FillColor = color.Transparent
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = th.Color(theme.ColorNameOutline, v)
		r.label.Color = th.Color(theme.ColorNameOnSurface, v)
		if r.chip.hovered {
			r.bg.FillColor = theme.StateLayerHover(th.Color(theme.ColorNameOnSurface, v))
		}
	}

	r.bg.Refresh()
	r.label.Refresh()

	if r.icon != nil && r.chip.Icon != nil {
		r.icon.Resource = theme.NewColoredResource(r.chip.Icon, theme.ColorNamePrimary)
		r.icon.Refresh()
	}
	if r.deleteBtn != nil {
		r.deleteBtn.Resource = theme.NewColoredResource(theme.CancelIcon(), theme.ColorNameOnSurfaceVariant)
		r.deleteBtn.Refresh()
	}

	r.Layout(r.chip.Size())
	canvas.Refresh(r.chip.super())
}
