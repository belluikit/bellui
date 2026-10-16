package widget

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

const defaultPlaceHolder string = "(Select one)"

var (
	_ core.Widget             = (*Select)(nil)
	_ event.Hoverable         = (*Select)(nil)
	_ event.Tappable          = (*Select)(nil)
	_ event.Focusable         = (*Select)(nil)
	_ core.Disableable        = (*Select)(nil)
	_ core.Accessible         = (*Select)(nil)
	_ core.AccessibleStateful = (*Select)(nil)
)

// Select widget has a list of options, with the current one shown, and triggers an event func when clicked
type Select struct {
	DisableableWidget

	// Alignment sets the text alignment of the select and its list of options.
	//
	// Since: 1.0
	Alignment   bellui.TextAlign
	Selected    string
	Options     []string
	PlaceHolder string
	OnChanged   func(string) `json:"-"`

	// onChangedInternal is the data-binding write-back. It is deliberately not
	// the public OnChanged field: Bind used to claim that field, so binding and
	// an app handler silently destroyed each other in both directions.
	// It runs before OnChanged so a handler reading the bound source never
	// sees a stale value.
	onChangedInternal func(string)

	binder basicBinder

	focused bool
	hovered bool
	popUp   *PopUpMenu
	tapAnim *bellui.Animation
}

// NewSelect creates a new select widget with the set list of options and changes handler
func NewSelect(options []string, changed func(string)) *Select {
	s := &Select{
		OnChanged:   changed,
		Options:     options,
		PlaceHolder: defaultPlaceHolder,
	}
	s.ExtendBaseWidget(s)
	return s
}

// NewSelectWithData returns a new select widget connected to the specified data source.
//
// Since: 1.0
func NewSelectWithData(options []string, data binding.String) *Select {
	sel := NewSelect(options, nil)
	sel.Bind(data)

	return sel
}

// AccessibleRole returns [core.RoleComboBox].
func (s *Select) AccessibleRole() core.AccessibleRole { return core.RoleComboBox }

// AccessibleLabel returns the selected value or the placeholder text.
func (s *Select) AccessibleLabel() string {
	if s.Selected != "" {
		return s.Selected
	}
	return s.PlaceHolder
}

// AccessibleState returns the disabled state.
func (s *Select) AccessibleState() core.AccessibleState {
	var st core.AccessibleState
	if s.Disabled() {
		st |= core.StateDisabled
	}
	return st
}

// Bind connects the specified data source to this select.
// The current value will be displayed and any changes in the data will cause the widget
// to update.
//
// The public [Select.OnChanged] field is left alone: binding and an app change
// handler coexist, and OnChanged may be assigned before or after Bind.
//
// Since: 1.0
func (s *Select) Bind(data binding.String) {
	s.binder.SetCallback(s.updateFromData)
	s.binder.Bind(data)

	s.onChangedInternal = func(_ string) {
		s.binder.CallWithData(s.writeData)
	}
}

// ClearSelected clears the current option of the select widget.  After
// clearing the current option, the Select widget's PlaceHolder will
// be displayed.
func (s *Select) ClearSelected() {
	s.updateSelected("")
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (s *Select) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)
	th := s.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	icon := NewIcon(th.Icon(theme.IconNameArrowDropDown))
	if s.PlaceHolder == "" {
		s.PlaceHolder = defaultPlaceHolder
	}
	txtProv := NewRichTextWithText(s.Selected)
	txtProv.inset = geo.NewSquareSize(th.Size(theme.SizeNamePadding))
	txtProv.ExtendBaseWidget(txtProv)
	txtProv.Truncation = bellui.TextTruncateEllipsis
	if s.Disabled() {
		txtProv.Segments[0].(*TextSegment).Style.ColorName = theme.ColorNameDisabled
	}

	background := &canvas.Rectangle{}
	tapBG := canvas.NewRectangle(color.Transparent)
	s.tapAnim = newButtonTapAnimation(tapBG, s, th)
	s.tapAnim.Curve = bellui.AnimationEaseOut
	objects := []core.CanvasObject{background, tapBG, txtProv, icon}
	r := &selectRenderer{icon, txtProv, background, objects, s}
	background.FillColor = r.bgColor(th, v)
	background.CornerRadius = th.Size(theme.SizeNameInputRadius)
	r.updateIcon(th)
	r.updateLabel()
	return r
}

// FocusGained is called after this Select has gained focus.
func (s *Select) FocusGained() {
	s.focused = true
	s.Refresh()
}

// FocusLost is called after this Select has lost focus.
func (s *Select) FocusLost() {
	s.focused = false
	s.Refresh()
}

// Hide hides the select.
func (s *Select) Hide() {
	if s.popUp != nil {
		s.popUp.Hide()
		s.popUp = nil
	}
	s.BaseWidget.Hide()
}

// MinSize returns the size that this widget should not shrink below
func (s *Select) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

// HoverIn is called when a desktop pointer enters the widget
func (s *Select) HoverIn(*event.HoverEvent) {
	s.hovered = true
	s.Refresh()
}

// HoverMoved is called when a desktop pointer hovers over the widget
func (s *Select) HoverMoved(*event.HoverEvent) {
}

// HoverOut is called when a desktop pointer exits the widget
func (s *Select) HoverOut() {
	s.hovered = false
	s.Refresh()
}

// Move changes the relative position of the select.
func (s *Select) Move(pos geo.Position) {
	s.BaseWidget.Move(pos)

	if s.popUp != nil {
		s.popUp.Move(s.popUpPos())
	}
}

// Resize sets a new size for a widget.
// Note this should not be used if the widget is being managed by a Layout within a Container.
func (s *Select) Resize(size geo.Size) {
	s.BaseWidget.Resize(size)

	if s.popUp != nil {
		s.popUp.Resize(geo.NewSize(size.Width, s.popUp.MinSize().Height))
	}
}

// SelectedIndex returns the index value of the currently selected item in Options list.
// It will return -1 if there is no selection.
func (s *Select) SelectedIndex() int {
	for i, option := range s.Options {
		if s.Selected == option {
			return i
		}
	}
	return -1 // not selected/found
}

// SetOptions updates the list of options available and refreshes the widget
//
// Since: 1.0
func (s *Select) SetOptions(options []string) {
	onMain(func() {
		s.Options = options
		s.Refresh()
	})
}

// SetSelected sets the current option of the select widget.
// It is safe to call from any goroutine.
func (s *Select) SetSelected(text string) {
	onMain(func() {
		for _, option := range s.Options {
			if text == option {
				s.updateSelected(text)
			}
		}
	})
}

// SetSelectedIndex will set the Selected option from the value in Options list at index position.
// It is safe to call from any goroutine.
func (s *Select) SetSelectedIndex(index int) {
	onMain(func() {
		if index < 0 || index >= len(s.Options) {
			return
		}
		s.updateSelected(s.Options[index])
	})
}

// Tapped is called when a pointer tapped event is captured and triggers any tap handler
func (s *Select) Tapped(*event.PointEvent) {
	if s.Disabled() {
		return
	}

	if !s.focused {
		focusIfNotMobile(s.super())
	}

	s.tapAnimation()
	s.Refresh()

	s.showPopUp()
}

// TypedKey is called if a key event happens while this Select is focused.
func (s *Select) TypedKey(ev *event.KeyEvent) {
	switch ev.Name {
	case event.KeySpace, event.KeyUp, event.KeyDown:
		s.showPopUp()
	case event.KeyRight:
		// Under a right-to-left layout the visual reading order is reversed, so
		// the right arrow steps to the previous option rather than the next.
		if lang.IsRTLDirection() {
			s.cycleSelection(-1)
		} else {
			s.cycleSelection(1)
		}
	case event.KeyLeft:
		if lang.IsRTLDirection() {
			s.cycleSelection(1)
		} else {
			s.cycleSelection(-1)
		}
	}
}

// cycleSelection advances the selected option by step (+1 next, -1 previous),
// wrapping around the ends.
func (s *Select) cycleSelection(step int) {
	count := len(s.Options)
	if count == 0 {
		return
	}
	i := s.SelectedIndex() + step
	if i >= count {
		i = 0
	} else if i < 0 {
		i = count - 1
	}
	s.SetSelectedIndex(i)
}

// TypedRune is called if a text event happens while this Select is focused.
func (s *Select) TypedRune(_ rune) {
	// intentionally left blank
}

// Unbind disconnects any configured data source from this Select.
// The current value will remain at the last value of the data source.
//
// Any [Select.OnChanged] handler the app installed is left in place -- Unbind
// only clears the write-back it installed itself.
//
// Since: 1.0
func (s *Select) Unbind() {
	s.onChangedInternal = nil
	s.binder.Unbind()
}

func (s *Select) popUpPos() geo.Position {
	buttonPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(s.super())
	return buttonPos.Add(geo.NewPos(0, s.Size().Height-s.Theme().Size(theme.SizeNameInputBorder)))
}

func (s *Select) showPopUp() {
	items := make([]*bellui.MenuItem, len(s.Options))
	for i := range s.Options {
		text := s.Options[i] // capture
		items[i] = bellui.NewMenuItem(text, func() {
			s.updateSelected(text)
			s.popUp = nil
		})
	}

	c := bellui.CurrentApp().Driver().CanvasForObject(s.super())
	if c == nil {
		return // widget not yet attached to a canvas (e.g., hidden tab)
	}
	pop := NewPopUpMenu(bellui.NewMenu("", items...), c)
	pop.alignment = s.Alignment
	pop.ShowAtPosition(s.popUpPos())
	pop.Resize(geo.NewSize(s.Size().Width, pop.MinSize().Height))
	pop.OnDismiss = func() {
		pop.Hide()
		if s.popUp == pop {
			s.popUp = nil
		}
	}
	s.popUp = pop
}

func (s *Select) tapAnimation() {
	if s.tapAnim == nil {
		return
	}
	s.tapAnim.Stop()

	if bellui.CurrentApp().Settings().ShowAnimations() {
		s.tapAnim.Start()
	}
}

func (s *Select) updateFromData(data binding.DataItem) {
	if data == nil {
		return
	}
	stringSource, ok := data.(binding.String)
	if !ok {
		return
	}

	val, err := stringSource.Get()
	if err != nil {
		return
	}
	if val == s.Selected {
		// SetSelected has no equality guard of its own (unlike Check.SetChecked),
		// so without this the write-back's own notification re-enters and reports
		// a single change twice.
		return
	}
	s.SetSelected(val)
}

func (s *Select) updateSelected(text string) {
	s.Selected = text

	if s.onChangedInternal != nil {
		s.onChangedInternal(s.Selected)
	}
	if s.OnChanged != nil {
		s.OnChanged(s.Selected)
	}

	s.Refresh()
}

func (s *Select) writeData(data binding.DataItem) {
	if data == nil {
		return
	}
	stringTarget, ok := data.(binding.String)
	if !ok {
		return
	}
	currentValue, err := stringTarget.Get()
	if err != nil {
		return
	}
	if currentValue != s.Selected {
		err := stringTarget.Set(s.Selected)
		if err != nil {
			bellui.LogError(fmt.Sprintf("Failed to set binding value to %s", s.Selected), err)
		}
	}
}

type selectRenderer struct {
	icon       *Icon
	label      *RichText
	background *canvas.Rectangle

	objects []core.CanvasObject
	combo   *Select
}

func (s *selectRenderer) Objects() []core.CanvasObject {
	return s.objects
}

// Destroy has nothing to release: the renderer's objects are torn down by the
// framework's destroy walk, the dropdown popup is owned by the Select widget and
// dismissed in Hide, and tapAnim is a one-shot ease-out that unregisters when it
// completes (the same reason Button leaves its tap animation to BaseRenderer).
func (s *selectRenderer) Destroy() {}

// Layout the components of the button widget
func (s *selectRenderer) Layout(size geo.Size) {
	th := s.combo.Theme()
	pad := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	innerPad := th.Size(theme.SizeNameInnerPadding)
	s.background.Resize(geo.NewSize(size.Width, size.Height))
	s.label.inset = geo.NewSquareSize(pad)

	iconPos := geo.NewPos(size.Width-iconSize-innerPad, (size.Height-iconSize)/2)
	labelSize := geo.NewSize(iconPos.X-pad, s.label.MinSize().Height)

	s.label.Resize(labelSize)
	s.label.Move(geo.NewPos(pad, (size.Height-labelSize.Height)/2))

	s.icon.Resize(geo.NewSquareSize(iconSize))
	s.icon.Move(iconPos)
}

// MinSize calculates the minimum size of a select button.
// This is based on the selected text, the drop icon and a standard amount of padding added.
func (s *selectRenderer) MinSize() geo.Size {
	th := s.combo.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)

	minPlaceholderWidth := bellui.MeasureText(s.combo.PlaceHolder, th.Size(theme.SizeNameText), bellui.TextStyle{}).Width
	min := s.label.MinSize()
	min.Width = minPlaceholderWidth
	min = min.Add(geo.NewSize(innerPad*3, innerPad))
	return min.Add(geo.NewSize(th.Size(theme.SizeNameInlineIcon)+innerPad, 0))
}

func (s *selectRenderer) Refresh() {
	th := s.combo.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	style := internaltheme.RenderStyleForWidget(s.combo)

	s.updateLabel()
	s.updateIcon(th)
	bgColor := s.bgColor(th, v)
	s.background.FillColor = bgColor
	s.background.CornerRadius = buttonRadius(th)

	if style == internaltheme.RenderStyleModern {
		s.background.StrokeWidth = 1
		s.background.StrokeColor = lightenColor(bgColor, 0.15)
	} else {
		s.background.StrokeWidth = 0
		s.background.StrokeColor = color.Transparent
	}

	s.Layout(s.combo.Size())
	if s.combo.popUp != nil {
		s.combo.popUp.alignment = s.combo.Alignment
		s.combo.popUp.Move(s.combo.popUpPos())
		s.combo.popUp.Resize(geo.NewSize(s.combo.Size().Width, s.combo.popUp.MinSize().Height))
		s.combo.popUp.Refresh()
	}
	s.background.Refresh()
	canvas.Refresh(s.combo.super())
}

func (s *selectRenderer) bgColor(th bellui.Theme, v bellui.ThemeVariant) color.Color {
	base := th.Color(theme.ColorNameInputBackground, v)
	if s.combo.Disabled() {
		return th.Color(theme.ColorNameDisabledButton, v)
	}
	onSurface := th.Color(theme.ColorNameOnSurface, v)
	if s.combo.focused {
		return blendColor(base, theme.StateLayerFocus(onSurface))
	}
	if s.combo.hovered {
		return blendColor(base, theme.StateLayerHover(onSurface))
	}
	return base
}

func (s *selectRenderer) updateIcon(th bellui.Theme) {
	icon := th.Icon(theme.IconNameArrowDropDown)
	if s.combo.Disabled() {
		s.icon.Resource = theme.NewDisabledResource(icon)
	} else {
		s.icon.Resource = icon
	}
	s.icon.Refresh()
}

func (s *selectRenderer) updateLabel() {
	if s.combo.PlaceHolder == "" {
		s.combo.PlaceHolder = defaultPlaceHolder
	}

	segment := s.label.Segments[0].(*TextSegment)
	segment.Style.Alignment = s.combo.Alignment
	if s.combo.Disabled() {
		segment.Style.ColorName = theme.ColorNameDisabled
	} else {
		segment.Style.ColorName = theme.ColorNameForeground
	}
	if s.combo.Selected == "" {
		segment.Text = s.combo.PlaceHolder
	} else {
		segment.Text = s.combo.Selected
	}
	s.label.Refresh()
}
