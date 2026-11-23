package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// ChipStyle determines the type of chip.
type ChipStyle int

const (
	ChipStyleAssist     ChipStyle = iota // Outlined, icon+label
	ChipStyleFilter                      // Selectable; when selected, secondary-container fill and a leading check mark
	ChipStyleInput                       // Label + delete button
	ChipStyleSuggestion                  // Outlined, label only
)

var (
	_ event.Tappable          = (*Chip)(nil)
	_ event.Hoverable         = (*Chip)(nil)
	_ core.Accessible         = (*Chip)(nil)
	_ core.AccessibleStateful = (*Chip)(nil)
	_ core.Disableable        = (*Chip)(nil)
)

// AccessibleRole returns [core.RoleButton].
func (c *Chip) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the chip text. An excluded chip in a ChipGroup
// adds that it is excluded ("Rock, excluded"): core.AccessibleState has no
// state for it, and the label is what every screen reader reads (#1383).
func (c *Chip) AccessibleLabel() string {
	if c.isExcluded() {
		return lang.LocalizeKey("chip.excluded", "{Label}, excluded", map[string]any{"Label": c.Text})
	}
	return c.Text
}

// AccessibleState reports a selected filter chip as selected, so a screen
// reader can tell it from an unselected one (#1348). The tree adds the
// disabled state itself.
func (c *Chip) AccessibleState() core.AccessibleState {
	if c.Style == ChipStyleFilter && c.Selected {
		return core.StateSelected
	}
	return 0
}

// isExcluded reports whether the chip shows the excluded state: a filter
// chip a ChipGroup has excluded, and not selected.
func (c *Chip) isExcluded() bool {
	return c.Style == ChipStyleFilter && c.excluded && !c.Selected
}

// Chip is a compact MD3 element for selections, filters, or actions.
//
// A disabled chip keeps its place in the disabled look and takes no tap, on
// the chip or on its delete X, and shows no hover or pressed state (#1212).
type Chip struct {
	DisableableWidget
	Text      string
	Icon      bellui.Resource // optional leading icon
	Style     ChipStyle
	Selected  bool   // for Filter chips
	OnTapped  func() `json:"-"`
	OnDeleted func() `json:"-"` // for Input chips (shows X)

	pressTarget
	deletePress internalWidget.PressTarget // the delete X's own, on an input chip

	hovered bool
	// excluded is set by an excludable ChipGroup for an option it excludes
	// (#1383).
	excluded bool
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

// Tapped fires OnTapped, or OnDeleted for a tap on an input chip's delete X.
// It does nothing while the chip is disabled.
func (c *Chip) Tapped(e *event.PointEvent) {
	if c.Disabled() {
		return
	}
	// On an input chip a tap landing on the trailing delete (X) button fires
	// OnDeleted rather than OnTapped; otherwise the X would be inert (pre-rebrand tracker #291).
	if c.hasDelete() && e != nil && c.inDeleteRegion(e.Position.X) {
		internalWidget.TapPress(&c.deletePress, nil)
		c.OnDeleted()
		return
	}
	if c.OnTapped != nil {
		internalWidget.TapPress(&c.pressTarget, e)
		c.OnTapped()
	}
}

// hasDelete reports whether the chip shows a delete X.
func (c *Chip) hasDelete() bool { return c.Style == ChipStyleInput && c.OnDeleted != nil }

// TouchDown shows the pressed state of the chip, or of its delete X for a
// touch on the X (#1091).
func (c *Chip) TouchDown(ev *mobile.TouchEvent) {
	if c.hasDelete() && c.inDeleteRegion(ev.Position.X) {
		c.deletePress.TouchDown(ev)
		return
	}
	c.pressTarget.TouchDown(ev)
}

// TouchUp ends the pressed state.
func (c *Chip) TouchUp(ev *mobile.TouchEvent) {
	c.deletePress.TouchUp(ev)
	c.pressTarget.TouchUp(ev)
}

// TouchCancel ends the pressed state with no feedback left.
func (c *Chip) TouchCancel(ev *mobile.TouchEvent) {
	c.deletePress.TouchCancel(ev)
	c.pressTarget.TouchCancel(ev)
}

// MouseDown shows the pressed state of the chip, or of its delete X for a
// press on the X.
func (c *Chip) MouseDown(ev *desktop.MouseEvent) {
	if c.hasDelete() && c.inDeleteRegion(ev.Position.X) {
		c.deletePress.MouseDown(ev)
		return
	}
	c.pressTarget.MouseDown(ev)
}

// MouseUp ends the pressed state.
func (c *Chip) MouseUp(ev *desktop.MouseEvent) {
	c.deletePress.MouseUp(ev)
	c.pressTarget.MouseUp(ev)
}

// pressShape is the whole chip, in its label's colour, while it has a tap
// handler and is enabled.
func (c *Chip) pressShape() (internalWidget.PressShape, bool) {
	if c.OnTapped == nil || c.Disabled() {
		return internalWidget.PressShape{}, false
	}
	th := c.Theme()
	content := theme.ColorNameOnSurface
	switch {
	case c.Style == ChipStyleFilter && c.Selected:
		content = theme.ColorNameOnSecondaryContainer
	case c.isExcluded():
		content = theme.ColorNameError
	}
	return internalWidget.PressShape{
		Size:         c.Size(),
		CornerRadius: th.Size(theme.SizeNameShapeSmall),
		Content:      th.Color(content, bellui.CurrentApp().Settings().ThemeVariant()),
	}, true
}

// deletePressShape is a circle around the delete X, as for an icon button.
func (c *Chip) deletePressShape() (internalWidget.PressShape, bool) {
	if c.Disabled() {
		return internalWidget.PressShape{}, false
	}
	r, ok := cache.Renderer(c.super()).(*chipRenderer)
	if !ok || r.deleteBtn == nil {
		return internalWidget.PressShape{}, false
	}
	th := c.Theme()
	x := r.deleteBtn.Size()
	d := geo.Min(c.Size().Height, x.Width+12)
	centre := r.deleteBtn.Position().Add(geo.NewPos(x.Width/2, x.Height/2))
	return internalWidget.PressShape{
		Pos:          centre.SubtractXY(d/2, d/2),
		Size:         geo.NewSquareSize(d),
		CornerRadius: d / 2,
		Content:      th.Color(theme.ColorNameOnSurfaceVariant, bellui.CurrentApp().Settings().ThemeVariant()),
		FromCenter:   true,
	}, true
}

// inDeleteRegion reports whether x falls on the delete button, which sits on
// the trailing edge of the chip (mirrored to the leading edge under RTL).
func (c *Chip) inDeleteRegion(x float32) bool {
	iconSz := c.Theme().Size(theme.SizeNameIconSize) * 0.75
	region := iconSz + 12 // button width plus surrounding padding
	if isRTLAt(c.super()) {
		return x < region
	}
	return x > c.Size().Width-region
}

// HoverIn shows the hover state layer, unless the chip is disabled.
func (c *Chip) HoverIn(*event.HoverEvent) {
	if c.Disabled() {
		return
	}
	c.hovered = true
	c.Refresh()
}

// HoverMoved does nothing.
func (c *Chip) HoverMoved(*event.HoverEvent) {}

// HoverOut ends the hover state and any press the pointer took off the chip.
// It runs while disabled too, so a chip disabled under the pointer does not
// show a hover when it is enabled again.
func (c *Chip) HoverOut() {
	internalWidget.CancelPress(&c.deletePress)
	internalWidget.CancelPress(&c.pressTarget)
	c.hovered = false
	c.Refresh()
}

// Disable makes the chip inert: no tap on it or on its delete X, no hover or
// pressed state, and the disabled look (#1212).
func (c *Chip) Disable() {
	internalWidget.CancelPress(&c.deletePress)
	internalWidget.CancelPress(&c.pressTarget)
	c.hovered = false
	c.DisableableWidget.Disable()
}

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
	objects := []core.CanvasObject{bg, internalWidget.AttachPress(&c.pressTarget, c.pressShape)}
	var deleteRipple *canvas.Ripple
	if c.hasDelete() {
		deleteRipple = internalWidget.AttachPress(&c.deletePress, c.deletePressShape)
		objects = append(objects, deleteRipple)
	}
	objects = append(objects, label)

	if c.Icon != nil {
		icon = canvas.NewImageFromResource(c.Icon)
		icon.FillMode = canvas.ImageFillContain
		objects = append(objects, icon)
	}

	// MD3's filter chip shows a check mark while selected, in the leading
	// icon's place (#1364): the fill alone was hard to tell from unselected.
	// An excluded chip shows a cross in the same place (#1383).
	var check, cross *canvas.Image
	if c.Style == ChipStyleFilter {
		check = canvas.NewImageFromResource(theme.ConfirmIcon())
		check.FillMode = canvas.ImageFillContain
		check.Hide()
		cross = canvas.NewImageFromResource(theme.CancelIcon())
		cross.FillMode = canvas.ImageFillContain
		cross.Hide()
		objects = append(objects, check, cross)
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
		check:        check,
		cross:        cross,
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
	check     *canvas.Image // a filter chip's check mark, shown while selected
	cross     *canvas.Image // a filter chip's cross, shown while excluded
	deleteBtn *canvas.Image
	chip      *Chip
}

// checked is whether the chip shows its check mark: a selected filter chip.
func (r *chipRenderer) checked() bool {
	return r.check != nil && r.chip.Style == ChipStyleFilter && r.chip.Selected
}

// crossed is whether the chip shows its cross: an excluded filter chip.
func (r *chipRenderer) crossed() bool {
	return r.cross != nil && r.chip.isExcluded()
}

// leading is the icon at the chip's start: the check mark while checked or
// the cross while crossed, in place of the chip's own icon, else that icon,
// or nil.
func (r *chipRenderer) leading() *canvas.Image {
	switch {
	case r.checked():
		return r.check
	case r.crossed():
		return r.cross
	}
	return r.icon
}

func (r *chipRenderer) MinSize() geo.Size {
	th := r.chip.Theme()
	chipH := th.Size(theme.SizeNameChipHeight)
	textSize := th.Size(theme.SizeNameLabelLarge)
	iconSz := th.Size(theme.SizeNameIconSize) * 0.75
	hPad := chipH * 0.375 // horizontal padding derived from chip height

	textWidth := bellui.MeasureText(r.chip.Text, textSize, bellui.TextStyle{}).Width
	w := hPad + textWidth + hPad // left pad + text + right pad
	if r.chip.Icon != nil || r.checked() || r.crossed() {
		w += iconSz + 8 // leading icon, check mark or cross + gap
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

	if lead := r.leading(); lead != nil {
		lead.Resize(geo.NewSize(iconSz, iconSz))
		lead.Move(geo.NewPos(x, (size.Height-iconSz)/2))
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
	// icon sits on the right; the full-width bg maps onto itself. The press
	// ripples are left out: a press places them.
	mirrored := []core.CanvasObject{r.bg, r.label}
	if lead := r.leading(); lead != nil {
		mirrored = append(mirrored, lead)
	}
	if r.deleteBtn != nil {
		mirrored = append(mirrored, r.deleteBtn)
	}
	mirrorIfRTL(mirrored, size.Width)
}

func (r *chipRenderer) Refresh() {
	th := r.chip.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.bg.CornerRadius = th.Size(theme.SizeNameShapeSmall) // MD3: small shape for chips
	r.label.Text = r.chip.Text
	r.label.TextSize = th.Size(theme.SizeNameLabelLarge)
	// Struck through while excluded, so the state does not rest on colour
	// alone (#1383).
	r.label.TextStyle.Strikethrough = r.crossed()

	if r.chip.Disabled() {
		r.refreshDisabled(th, v)
		return
	}

	if r.chip.Style == ChipStyleFilter && r.chip.Selected {
		bgColor := th.Color(theme.ColorNameSecondaryContainer, v)
		if r.chip.hovered {
			bgColor = blendColor(bgColor, theme.StateLayerHover(th.Color(theme.ColorNameOnSecondaryContainer, v)))
		}
		r.bg.FillColor = bgColor
		r.bg.StrokeWidth = 0
		r.label.Color = th.Color(theme.ColorNameOnSecondaryContainer, v)
	} else if r.crossed() {
		// BellUI's own look: MD3 has no excluded filter chip. An
		// error-coloured outline and cross around the struck-through label.
		errColor := th.Color(theme.ColorNameError, v)
		r.bg.FillColor = color.Transparent
		if r.chip.hovered {
			r.bg.FillColor = theme.StateLayerHover(errColor)
		}
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = errColor
		r.label.Color = th.Color(theme.ColorNameOnSurface, v)
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
	r.showCheck(theme.NewColoredResource(theme.ConfirmIcon(), theme.ColorNameOnSecondaryContainer),
		theme.NewColoredResource(theme.CancelIcon(), theme.ColorNameError))
	if r.deleteBtn != nil {
		r.deleteBtn.Resource = theme.NewColoredResource(theme.CancelIcon(), theme.ColorNameOnSurfaceVariant)
		r.deleteBtn.Refresh()
	}

	r.relayout()
}

// refreshDisabled draws the MD3 disabled chip: label and icons in the
// disabled content colour, a selected filter chip's fill and the others'
// outline in the disabled container colour, and no state layer (#1212).
func (r *chipRenderer) refreshDisabled(th bellui.Theme, v bellui.ThemeVariant) {
	container := th.Color(theme.ColorNameDisabledButton, v)
	if r.chip.Style == ChipStyleFilter && r.chip.Selected {
		r.bg.FillColor = container
		r.bg.StrokeWidth = 0
	} else {
		r.bg.FillColor = color.Transparent
		r.bg.StrokeWidth = 1
		r.bg.StrokeColor = container
	}
	r.label.Color = th.Color(theme.ColorNameDisabled, v)
	r.bg.Refresh()
	r.label.Refresh()

	if r.icon != nil && r.chip.Icon != nil {
		r.icon.Resource = theme.NewDisabledResource(r.chip.Icon)
		r.icon.Refresh()
	}
	r.showCheck(theme.NewDisabledResource(theme.ConfirmIcon()), theme.NewDisabledResource(theme.CancelIcon()))
	if r.deleteBtn != nil {
		r.deleteBtn.Resource = theme.NewDisabledResource(theme.CancelIcon())
		r.deleteBtn.Refresh()
	}
	r.relayout()
}

// showCheck shows the check mark, drawn as checkRes, while the chip is
// checked, the cross, drawn as crossRes, while it is crossed, and the chip's
// own icon only while neither is: the mark takes its place.
func (r *chipRenderer) showCheck(checkRes, crossRes bellui.Resource) {
	if r.check == nil {
		return
	}
	showMark(r.check, checkRes, r.checked())
	showMark(r.cross, crossRes, r.crossed())
	if r.icon != nil {
		if r.checked() || r.crossed() {
			r.icon.Hide()
		} else {
			r.icon.Show()
		}
	}
}

// showMark shows img drawn as res while on, and hides it otherwise.
func showMark(img *canvas.Image, res bellui.Resource, on bool) {
	if !on {
		img.Hide()
		return
	}
	img.Resource = res
	img.Show()
	img.Refresh()
}

func (r *chipRenderer) relayout() {
	// Outside a layout walk (hover, SetText), so lay out in the chip's own
	// direction (#696).
	inDirectionOf(r.chip.super(), func() { r.Layout(r.chip.Size()) })
	canvas.Refresh(r.chip.super())
}
