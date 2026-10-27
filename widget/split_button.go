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

var (
	_ event.Tappable          = (*SplitButton)(nil)
	_ event.Hoverable         = (*SplitButton)(nil)
	_ core.Accessible         = (*SplitButton)(nil)
	_ core.AccessibleActioner = (*SplitButton)(nil)
)

// AccessibleRole returns [core.RoleButton].
func (s *SplitButton) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the button text.
func (s *SplitButton) AccessibleLabel() string { return s.Text }

// AccessibleActions returns the "press" action.
func (s *SplitButton) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Activate the button"}}
}

// PerformAccessibleAction invokes the button's primary tap handler.
func (s *SplitButton) PerformAccessibleAction(name string) {
	if name == core.ActionPress && !s.Disabled() && s.OnTapped != nil {
		s.OnTapped()
	}
}

// SplitButton is an MD3 button with a primary action and a dropdown arrow
// that opens a menu of secondary actions.
type SplitButton struct {
	DisableableWidget
	Text      string
	Icon      bellui.Resource
	OnTapped  func() `json:"-"`
	MenuItems []*bellui.MenuItem

	hovered bool
	popUp   *PopUpMenu
}

// NewSplitButton creates a split button with a primary action and menu items.
func NewSplitButton(text string, tapped func(), items []*bellui.MenuItem) *SplitButton {
	s := &SplitButton{Text: text, OnTapped: tapped, MenuItems: items}
	s.ExtendBaseWidget(s)
	return s
}

func (s *SplitButton) Tapped(e *event.PointEvent) {
	if s.Disabled() {
		return
	}
	// Check if tap is on the dropdown area: the trailing end, which the
	// layout mirrors to the left under RTL (#835).
	dropWidth := float32(36)
	onDrop := e != nil && e.Position.X > s.Size().Width-dropWidth
	if e != nil && isRTLAt(s) {
		onDrop = e.Position.X < dropWidth
	}
	if onDrop {
		s.showMenu()
		return
	}
	if s.OnTapped != nil {
		s.OnTapped()
	}
}

func (s *SplitButton) HoverIn(*event.HoverEvent)    { s.hovered = true; s.Refresh() }
func (s *SplitButton) HoverMoved(*event.HoverEvent) {}
func (s *SplitButton) HoverOut()                    { s.hovered = false; s.Refresh() }

func (s *SplitButton) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

func (s *SplitButton) showMenu() {
	c := bellui.CurrentApp().Driver().CanvasForObject(s)
	if c == nil || len(s.MenuItems) == 0 {
		return
	}
	menu := bellui.NewMenu("", s.MenuItems...)
	absPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(s)
	s.popUp = NewPopUpMenu(menu, c)
	inheritDirection(s.popUp, s) // #749
	// The menu lines up with the button's start edge: the right one under
	// RTL, kept on the canvas when the menu is wider than the room (#835).
	x := absPos.X
	if isRTLAt(s) {
		x = max(0, absPos.X+s.Size().Width-s.popUp.MinSize().Width)
	}
	s.popUp.ShowAtPosition(geo.NewPos(x, absPos.Y+s.Size().Height))
}

func (s *SplitButton) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	bg := canvas.NewRectangle(color.Transparent)
	divider := canvas.NewRectangle(color.Transparent)
	label := canvas.NewText(s.Text, color.White)
	label.TextStyle.Bold = true
	label.TextSize = 14

	var icon *canvas.Image
	if s.Icon != nil {
		icon = canvas.NewImageFromResource(s.Icon)
		icon.FillMode = canvas.ImageFillContain
	}

	dropIcon := canvas.NewImageFromResource(theme.MenuDropDownIcon())
	dropIcon.FillMode = canvas.ImageFillContain

	objects := []core.CanvasObject{bg, label, divider, dropIcon}
	if icon != nil {
		objects = append(objects, icon)
	}

	r := &splitButtonRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		divider:      divider,
		label:        label,
		icon:         icon,
		dropIcon:     dropIcon,
		btn:          s,
	}
	r.Refresh()
	return r
}

type splitButtonRenderer struct {
	internalWidget.BaseRenderer
	bg       *canvas.Rectangle
	divider  *canvas.Rectangle
	label    *canvas.Text
	icon     *canvas.Image
	dropIcon *canvas.Image
	btn      *SplitButton
}

func (r *splitButtonRenderer) MinSize() geo.Size {
	textW := bellui.MeasureText(r.btn.Text, 14, bellui.TextStyle{Bold: true}).Width
	w := float32(16) + textW + 16 + 1 + 36 // pad + text + pad + divider + dropdown
	if r.btn.Icon != nil {
		w += 24 + 8 // icon + gap
	}
	return geo.NewSize(w, 40)
}

func (r *splitButtonRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)

	dropW := float32(36)
	dividerX := size.Width - dropW
	x := float32(16)

	if r.icon != nil {
		r.icon.Resize(geo.NewSize(18, 18))
		r.icon.Move(geo.NewPos(x, (size.Height-18)/2))
		x += 26
	}

	labelSize := r.label.MinSize()
	r.label.Move(geo.NewPos(x, (size.Height-labelSize.Height)/2))
	r.label.Resize(labelSize) // tight box so the RTL reflection lands correctly

	r.divider.Resize(geo.NewSize(1, size.Height*0.6))
	r.divider.Move(geo.NewPos(dividerX, size.Height*0.2))

	r.dropIcon.Resize(geo.NewSize(20, 20))
	r.dropIcon.Move(geo.NewPos(dividerX+(dropW-20)/2, (size.Height-20)/2))

	// Under RTL reflect the row so the dropdown section moves to the leading
	// (left) edge and the primary label/icon to the trailing (right) edge.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *splitButtonRenderer) Refresh() {
	th := r.btn.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bgColor := th.Color(theme.ColorNamePrimary, v)
	if r.btn.hovered {
		bgColor = blendColor(bgColor, theme.StateLayerHover(th.Color(theme.ColorNameForegroundOnPrimary, v)))
	}
	if r.btn.Disabled() {
		bgColor = th.Color(theme.ColorNameDisabledButton, v)
	}

	r.bg.FillColor = bgColor
	r.bg.CornerRadius = 20 // pill shape
	r.bg.Refresh()

	r.divider.FillColor = th.Color(theme.ColorNameForegroundOnPrimary, v)
	r.divider.Refresh()

	fgName := theme.ColorNameForegroundOnPrimary
	if r.btn.Disabled() {
		fgName = theme.ColorNameDisabled
	}

	r.label.Text = r.btn.Text
	r.label.Color = th.Color(fgName, v)
	r.label.Refresh()

	if r.icon != nil && r.btn.Icon != nil {
		r.icon.Resource = theme.NewColoredResource(r.btn.Icon, fgName)
		r.icon.Refresh()
	}

	r.dropIcon.Resource = theme.NewColoredResource(theme.MenuDropDownIcon(), fgName)
	r.dropIcon.Refresh()

	r.Layout(r.btn.Size())
	canvas.Refresh(r.btn.super())
}
