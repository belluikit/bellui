package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget     = (*radioItem)(nil)
	_ event.Hoverable = (*radioItem)(nil)
	_ event.Tappable  = (*radioItem)(nil)
	_ event.Focusable = (*radioItem)(nil)
)

func newRadioItem(label string, onTap func(*radioItem)) *radioItem {
	i := &radioItem{Label: label, onTap: onTap}
	i.ExtendBaseWidget(i)
	return i
}

// radioItem is a single radio item to be used by RadioGroup.
type radioItem struct {
	DisableableWidget

	Label    string
	Selected bool

	focused bool
	hovered bool
	onTap   func(item *radioItem)
}

// AccessibleRole returns [core.RoleRadioButton].
func (i *radioItem) AccessibleRole() core.AccessibleRole { return core.RoleRadioButton }

// AccessibleLabel returns the item's text.
func (i *radioItem) AccessibleLabel() string { return i.Label }

// AccessibleState reports the selected, disabled and focused states.
func (i *radioItem) AccessibleState() core.AccessibleState {
	var state core.AccessibleState
	if i.Selected {
		state |= core.StateChecked
	}
	if i.Disabled() {
		state |= core.StateDisabled
	}
	if i.focused {
		state |= core.StateFocused
	}
	return state
}

// AccessibleActions returns the "select" action.
func (i *radioItem) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this option"}}
}

// PerformAccessibleAction toggles the item like a tap would.
func (i *radioItem) PerformAccessibleAction(name string) {
	if name == core.ActionSelect && !i.Disabled() {
		i.toggle()
	}
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer.
func (i *radioItem) CreateRenderer() core.WidgetRenderer {
	txt := canvas.Text{Alignment: bellui.TextAlignLeading}
	txt.TextSize = i.Theme().Size(theme.SizeNameText)
	r := &radioItemRenderer{item: i, label: &txt}
	r.SetObjects([]core.CanvasObject{&r.focusIndicator, &r.icon, &r.over, &txt})
	r.update()
	return r
}

// FocusGained is called when this item gained the focus.
func (i *radioItem) FocusGained() {
	i.focused = true
	i.Refresh()
}

// FocusLost is called when this item lost the focus.
func (i *radioItem) FocusLost() {
	i.focused = false
	i.Refresh()
}

// HoverIn is called when a desktop pointer enters the widget.
func (i *radioItem) HoverIn(_ *event.HoverEvent) {
	if i.Disabled() {
		return
	}

	i.hovered = true
	i.Refresh()
}

// HoverMoved is called when a desktop pointer hovers over the widget.
func (i *radioItem) HoverMoved(_ *event.HoverEvent) {
}

// HoverOut is called when a desktop pointer exits the widget
func (i *radioItem) HoverOut() {
	if i.Disabled() {
		return
	}

	i.hovered = false
	i.Refresh()
}

// SetSelected sets whether this radio item is selected or not.
func (i *radioItem) SetSelected(selected bool) {
	if i.Disabled() || i.Selected == selected {
		return
	}

	i.Selected = selected
	i.Refresh()
}

// Tapped is called when a pointer tapped event is captured and triggers any change handler
func (i *radioItem) Tapped(_ *event.PointEvent) {
	if !i.focused {
		focusIfNotMobile(i.super())
	}
	i.toggle()
}

// TypedKey is called when this item receives a key event.
func (i *radioItem) TypedKey(_ *event.KeyEvent) {
}

// TypedRune is called when this item receives a char event.
func (i *radioItem) TypedRune(r rune) {
	if r == ' ' {
		i.toggle()
	}
}

func (i *radioItem) toggle() {
	if i.Disabled() || i.onTap == nil {
		return
	}

	i.onTap(i)
}

type radioItemRenderer struct {
	widget.BaseRenderer
	item *radioItem

	focusIndicator canvas.Circle
	icon, over     canvas.Image
	label          *canvas.Text
}

func (r *radioItemRenderer) Layout(size geo.Size) {
	th := r.item.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	borderSize := th.Size(theme.SizeNameInputBorder)
	iconInlineSize := th.Size(theme.SizeNameInlineIcon)

	focusIndicatorSize := geo.NewSquareSize(iconInlineSize + innerPadding)
	r.focusIndicator.Resize(focusIndicatorSize)
	r.focusIndicator.Move(geo.NewPos(borderSize, (size.Height-focusIndicatorSize.Height)/2))

	labelSize := geo.NewSize(size.Width, size.Height)
	r.label.Resize(labelSize)
	r.label.Move(geo.NewPos(focusIndicatorSize.Width+th.Size(theme.SizeNamePadding), 0))

	iconPos := geo.NewPos(innerPadding/2+borderSize, (size.Height-iconInlineSize)/2)
	iconSize := geo.NewSquareSize(iconInlineSize)
	r.icon.Resize(iconSize)
	r.icon.Move(iconPos)
	r.over.Resize(iconSize)
	r.over.Move(iconPos)
}

func (r *radioItemRenderer) MinSize() geo.Size {
	th := r.item.Theme()
	inPad := th.Size(theme.SizeNameInnerPadding) * 2

	return r.label.MinSize().
		AddWidthHeight(inPad+th.Size(theme.SizeNameInlineIcon)+th.Size(theme.SizeNamePadding), inPad)
}

func (r *radioItemRenderer) Refresh() {
	r.update()
	canvas.Refresh(r.item.super())
}

func (r *radioItemRenderer) update() {
	th := r.item.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.label.Text = r.item.Label
	r.label.TextSize = th.Size(theme.SizeNameText)
	if r.item.Disabled() {
		r.label.Color = th.Color(theme.ColorNameDisabled, v)
	} else {
		r.label.Color = th.Color(theme.ColorNameOnSurface, v)
	}

	// MD3: an unselected radio is an empty ring; a selected one is a Primary
	// ring around a Primary dot (#494).
	out := theme.NewThemedResource(th.Icon(theme.IconNameRadioButton))
	out.ColorName = theme.ColorNameOnSurfaceVariant
	in := theme.NewThemedResource(th.Icon(theme.IconNameRadioButtonFill))
	in.ColorName = theme.ColorNamePrimary
	if r.item.Selected {
		out.ColorName = theme.ColorNamePrimary
	}
	if r.item.Disabled() {
		in.ColorName = theme.ColorNameDisabled
		out.ColorName = theme.ColorNameDisabled
	}
	r.icon.Resource = in
	if r.item.Selected {
		r.icon.Show()
	} else {
		r.icon.Hide()
	}
	r.icon.Refresh()
	r.over.Resource = out
	r.over.Refresh()

	if r.item.Disabled() {
		r.focusIndicator.FillColor = color.Transparent
	} else if r.item.focused {
		r.focusIndicator.FillColor = theme.StateLayerFocus(th.Color(theme.ColorNamePrimary, v))
	} else if r.item.hovered {
		r.focusIndicator.FillColor = theme.StateLayerHover(th.Color(theme.ColorNameOnSurface, v))
	} else {
		r.focusIndicator.FillColor = color.Transparent
	}
}
