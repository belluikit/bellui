package widget

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	col "github.com/belluikit/bellui/internal/color"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// ButtonAlign represents the horizontal alignment of a button.
type ButtonAlign int

// ButtonIconPlacement represents the ordering of icon & text within a button.
type ButtonIconPlacement int

// ButtonStyle determines the visual variant of a button per MD3 spec.
type ButtonStyle int

const (
	// ButtonStyleDefault follows the Importance field for color scheme.
	ButtonStyleDefault ButtonStyle = iota
	// ButtonStyleFilled uses Primary background with OnPrimary text (MD3 Filled).
	ButtonStyleFilled
	// ButtonStyleFilledTonal uses SecondaryContainer background (MD3 Filled Tonal).
	ButtonStyleFilledTonal
	// ButtonStyleElevated uses surface color with shadow and Primary text (MD3 Elevated).
	ButtonStyleElevated
	// ButtonStyleOutlined uses transparent background with border (MD3 Outlined).
	ButtonStyleOutlined
	// ButtonStyleText uses transparent background with Primary text (MD3 Text).
	ButtonStyleText
)

const (
	// ButtonAlignCenter aligns the icon and the text centrally.
	ButtonAlignCenter ButtonAlign = iota
	// ButtonAlignLeading aligns the icon and the text with the leading edge.
	ButtonAlignLeading
	// ButtonAlignTrailing aligns the icon and the text with the trailing edge.
	ButtonAlignTrailing
)

const (
	// ButtonIconLeadingText aligns the icon on the leading edge of the text.
	ButtonIconLeadingText ButtonIconPlacement = iota
	// ButtonIconTrailingText aligns the icon on the trailing edge of the text.
	ButtonIconTrailingText
)

var (
	_ event.Focusable         = (*Button)(nil)
	_ event.Activatable       = (*Button)(nil)
	_ core.Accessible         = (*Button)(nil)
	_ core.AccessibleActioner = (*Button)(nil)
)

// Button widget has a text label and triggers an event func when clicked
type Button struct {
	DisableableWidget
	Text string
	Icon bellui.Resource
	// Specify how prominent the button should be, High will highlight the button and Low will remove some decoration.
	//
	// Since: 1.0
	Importance Importance
	// Style sets the MD3 visual variant. When ButtonStyleDefault, the Importance
	// field controls colors. Other styles override Importance for color selection.
	Style         ButtonStyle
	Alignment     ButtonAlign
	IconPlacement ButtonIconPlacement

	OnTapped func() `json:"-"`

	hovered, focused bool
	tapAnim          *bellui.Animation
	isAnimating      bool
}

// NewButton creates a new button widget with the set label and tap handler
func NewButton(label string, tapped func()) *Button {
	button := &Button{
		Text:     label,
		OnTapped: tapped,
	}

	button.ExtendBaseWidget(button)
	return button
}

// NewButtonWithIcon creates a new button widget with the specified label, themed icon and tap handler
func NewButtonWithIcon(label string, icon bellui.Resource, tapped func()) *Button {
	button := &Button{
		Text:     label,
		Icon:     icon,
		OnTapped: tapped,
	}

	button.ExtendBaseWidget(button)
	return button
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (b *Button) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	th := b.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	seg := &TextSegment{Text: b.Text, Style: RichTextStyleStrong}
	seg.Style.Alignment = bellui.TextAlignCenter
	text := NewRichText(seg)
	text.inset = geo.NewSquareSize(th.Size(theme.SizeNameInnerPadding))

	background := canvas.NewRectangle(th.Color(theme.ColorNameButton, v))
	background.CornerRadius = buttonRadius(th)

	ripple := canvas.NewRipple(th.Color(theme.ColorNamePressed, v))
	ripple.Hide()

	// Always create the gradient overlay; show/hide based on render style.
	grad := canvas.NewLinearGradient(color.Transparent, color.Transparent, 90)
	grad.Hide()

	r := &buttonRenderer{
		BaseRenderer: widget.NewBaseRenderer([]core.CanvasObject{background, grad, ripple, text}),
		background:   background,
		gradientBG:   grad,
		ripple:       ripple,
		button:       b,
		label:        text,
		layout:       layout.NewHBoxLayout(),
	}
	r.updateIconAndText()
	r.applyTheme()
	return r
}

// AccessibleRole returns [core.RoleButton].
func (b *Button) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the button text.
func (b *Button) AccessibleLabel() string { return b.Text }

// AccessibleActions returns the "press" action.
func (b *Button) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Activate the button"}}
}

// PerformAccessibleAction invokes the button's tap handler.
func (b *Button) PerformAccessibleAction(name string) {
	if name == core.ActionPress && !b.Disabled() && b.OnTapped != nil {
		b.OnTapped()
	}
}

// Cursor returns the cursor type of this widget
func (b *Button) Cursor() desktop.Cursor {
	return desktop.DefaultCursor
}

// FocusGained is a hook called by the focus handling logic after this object gained the focus.
func (b *Button) FocusGained() {
	b.focused = true
	b.Refresh()
}

// FocusLost is a hook called by the focus handling logic after this object lost the focus.
func (b *Button) FocusLost() {
	b.focused = false
	b.Refresh()
}

// MinSize returns the size that this widget should not shrink below
func (b *Button) MinSize() geo.Size {
	b.ExtendBaseWidget(b)
	return b.BaseWidget.MinSize()
}

// HoverIn is called when a desktop pointer enters the widget
func (b *Button) HoverIn(*event.HoverEvent) {
	b.hovered = true
	b.Refresh()
}

// HoverMoved is called when a desktop pointer hovers over the widget
func (b *Button) HoverMoved(*event.HoverEvent) {
}

// HoverOut is called when a desktop pointer exits the widget
func (b *Button) HoverOut() {
	b.hovered = false
	b.Refresh()
}

// SetIcon updates the icon on a label - pass nil to hide an icon.
// It is safe to call from any goroutine.
func (b *Button) SetIcon(icon bellui.Resource) {
	onMain(func() {
		b.Icon = icon
		b.Refresh()
	})
}

// SetText allows the button label to be changed.
// It is safe to call from any goroutine.
func (b *Button) SetText(text string) {
	onMain(func() {
		b.Text = text
		b.Refresh()
	})
}

// Tapped is called when a pointer tapped event is captured and triggers any tap handler
func (b *Button) Tapped(ev *event.PointEvent) {
	impl := b.super()
	if c := bellui.CurrentApp().Driver().CanvasForObject(impl); c != nil {
		c.Focus(nil) // the focus manager won't get this Tap and we are not focusable
	}

	if b.Disabled() {
		return
	}

	b.tapAnimation(ev)

	if onTapped := b.OnTapped; onTapped != nil {
		onTapped()
	}
}

// TypedRune is a hook called by the input handling logic on text input events if this object is focused.
func (b *Button) TypedRune(rune) {
}

// TypedKey is a hook called by the input handling logic on key events if this object is focused.
func (b *Button) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeySpace {
		b.Tapped(nil)
	}
}

// ActivatedByKey reports that Space activates the button, so the driver fires the
// keyboard-activation signal in parity with a pointer tap (#700).
func (b *Button) ActivatedByKey(ev *event.KeyEvent) bool {
	return ev.Name == event.KeySpace
}

func (b *Button) tapAnimation(ev *event.PointEvent) {
	if b.tapAnim != nil {
		b.tapAnim.Stop()
	}
	b.isAnimating = false

	if !bellui.CurrentApp().Settings().ShowAnimations() {
		return
	}

	r := cache.Renderer(b.super())
	if r == nil {
		return
	}
	br, ok := r.(*buttonRenderer)
	if !ok || br.ripple == nil {
		return
	}

	// Set ripple center to touch point (or center if no event).
	if ev != nil {
		br.ripple.Center = ev.Position
	} else {
		br.ripple.Center = geo.NewPos(b.Size().Width/2, b.Size().Height/2)
	}
	br.ripple.CornerRadius = buttonRadius(b.Theme())

	// Calculate max radius (distance from center to farthest corner).
	size := b.Size()
	cx, cy := br.ripple.Center.X, br.ripple.Center.Y
	dx := geo.Max(cx, size.Width-cx)
	dy := geo.Max(cy, size.Height-cy)
	maxRadius := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	br.ripple.Resize(size)
	br.ripple.Show()

	b.isAnimating = true
	b.tapAnim = bellui.NewAnimation(canvas.DurationStandard, func(done float32) {
		br.ripple.Radius = maxRadius * done
		br.ripple.Opacity = 0.12 * (1 - done)
		br.ripple.Refresh()
		if done >= 1.0 {
			br.ripple.Hide()
			b.isAnimating = false
		}
	})
	b.tapAnim.Curve = bellui.AnimationEaseOut
	b.tapAnim.Start()
}

type buttonRenderer struct {
	widget.BaseRenderer

	icon       *canvas.Image
	label      *RichText
	background *canvas.Rectangle
	gradientBG *canvas.LinearGradient // non-nil for modern render style
	ripple     *canvas.Ripple
	button     *Button
	layout     core.Layout
}

// Layout the components of the button widget
func (r *buttonRenderer) Layout(size geo.Size) {
	r.background.Resize(size)
	if r.gradientBG != nil {
		r.gradientBG.Resize(size)
	}
	if r.ripple != nil && !r.button.isAnimating {
		r.ripple.Resize(size)
	}

	th := r.button.Theme()
	padding := r.padding(th)
	hasIcon := r.icon != nil
	hasLabel := r.label.Segments[0].(*TextSegment).Text != ""
	if !hasIcon && !hasLabel {
		// Nothing to layout
		return
	}
	iconSize := geo.NewSquareSize(th.Size(theme.SizeNameInlineIcon))
	labelSize := r.label.MinSize()

	if hasLabel {
		if hasIcon {
			// Both
			var objects []core.CanvasObject
			if r.button.IconPlacement == ButtonIconLeadingText {
				objects = append(objects, r.icon, r.label)
			} else {
				objects = append(objects, r.label, r.icon)
			}
			r.icon.SetMinSize(iconSize)
			min := r.layout.MinSize(objects)
			r.layout.Layout(objects, min)
			pos := alignedPosition(r.button.Alignment, padding, min, size)
			labelOff := (min.Height - labelSize.Height) / 2
			r.label.Move(r.label.Position().Add(pos).AddXY(0, labelOff))
			r.icon.Move(r.icon.Position().Add(pos))
		} else {
			// Label Only
			r.label.Move(alignedPosition(r.button.Alignment, padding, labelSize, size))
			r.label.Resize(labelSize)
		}
	} else {
		// Icon Only
		r.icon.Move(alignedPosition(r.button.Alignment, padding, iconSize, size))
		r.icon.Resize(iconSize)
	}
}

// MinSize calculates the minimum size of a button.
// This is based on the contained text, any icon that is set and a standard
// amount of padding added.
func (r *buttonRenderer) MinSize() (size geo.Size) {
	th := r.button.Theme()
	hasIcon := r.icon != nil
	hasLabel := r.label.Segments[0].(*TextSegment).Text != ""
	iconSize := geo.NewSquareSize(th.Size(theme.SizeNameInlineIcon))
	labelSize := r.label.MinSize()
	if hasLabel {
		size.Width = labelSize.Width
	}
	if hasIcon {
		if hasLabel {
			size.Width += th.Size(theme.SizeNamePadding)
		}
		size.Width += iconSize.Width
	}
	size.Height = geo.Max(labelSize.Height, iconSize.Height)
	size = size.Add(r.padding(th))
	// MD3 buttons have a minimum height per theme token.
	minH := th.Size(theme.SizeNameButtonMinHeight)
	if size.Height < minH {
		size.Height = minH
	}
	return size
}

func (r *buttonRenderer) Refresh() {
	th := r.button.Theme()
	r.label.inset = geo.NewSquareSize(th.Size(theme.SizeNameInnerPadding))

	r.label.Segments[0].(*TextSegment).Text = r.button.Text
	r.updateIconAndText()
	r.applyTheme()

	r.background.Refresh()
	r.Layout(r.button.Size())
	canvas.Refresh(r.button.super())
}

// applyTheme updates this button to match the current theme
// must be called with the button propertyLock RLocked
func (r *buttonRenderer) applyTheme() {
	th := r.button.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	fgColorName, bgColorName, bgBlendName := r.buttonColorNames()

	bgColor := color.Color(color.Transparent)
	if bgColorName != "" {
		bgColor = th.Color(bgColorName, v)
	}

	// MD3 state layers: compute from content color, not fixed theme color.
	if bgBlendName != "" {
		fgColor := th.Color(fgColorName, v)
		var stateColor color.Color
		if bgBlendName == theme.ColorNameFocus {
			stateColor = theme.StateLayerFocus(fgColor)
		} else {
			stateColor = theme.StateLayerHover(fgColor)
		}
		bgColor = blendColor(bgColor, stateColor)
	}

	if bg := r.background; bg != nil {
		bg.FillColor = bgColor

		// MD3 pill shape (height/2).
		minH := th.Size(theme.SizeNameButtonMinHeight)
		h := r.button.Size().Height
		if h < minH {
			h = minH
		}
		bg.CornerRadius = h / 2

		// MD3 button style-specific border handling.
		switch r.button.Style {
		case ButtonStyleOutlined:
			bg.StrokeWidth = 1
			bg.StrokeColor = th.Color(theme.ColorNameOutline, v)
		case ButtonStyleText:
			bg.StrokeWidth = 0
			bg.StrokeColor = color.Transparent
		case ButtonStyleElevated:
			bg.StrokeWidth = 0
			bg.StrokeColor = color.Transparent
		default:
			bg.StrokeWidth = 0
			bg.StrokeColor = color.Transparent
		}
		bg.Refresh()
	}

	// Hide gradient overlay — all styles now use MD3 flat fill.
	if r.gradientBG != nil {
		r.gradientBG.Hide()
	}

	r.label.Segments[0].(*TextSegment).Style.ColorName = fgColorName
	r.label.Refresh()
	if r.icon != nil && r.icon.Resource != nil {
		icon := r.icon.Resource
		if r.button.Importance != MediumImportance && r.button.Importance != LowImportance {
			if thRes, ok := icon.(bellui.ThemedResource); ok {
				if thRes.ThemeColorName() != fgColorName {
					icon = theme.NewColoredResource(icon, fgColorName)
				}
			}
		}
		r.icon.Resource = icon
		r.icon.Refresh()
	}
}

func (r *buttonRenderer) buttonColorNames() (foreground, background, backgroundBlend bellui.ThemeColorName) {
	foreground = theme.ColorNameForeground
	b := r.button

	if b.Disabled() {
		foreground = theme.ColorNameDisabled
		if b.Importance != LowImportance && b.Style != ButtonStyleText {
			background = theme.ColorNameDisabledButton
		}
		return
	}

	if b.focused {
		backgroundBlend = theme.ColorNameFocus
	} else if b.hovered {
		backgroundBlend = theme.ColorNameHover
	}

	// MD3 Style color selection. Default uses FilledTonal (no legacy Importance-based colors).
	style := b.Style
	if style == ButtonStyleDefault {
		// Map Importance to MD3 styles for backwards compatibility.
		switch b.Importance {
		case DangerImportance:
			foreground = theme.ColorNameForegroundOnError
			background = theme.ColorNameError
			return
		case HighImportance:
			style = ButtonStyleFilled
		case SuccessImportance:
			foreground = theme.ColorNameForegroundOnSuccess
			background = theme.ColorNameSuccess
			return
		case WarningImportance:
			foreground = theme.ColorNameForegroundOnWarning
			background = theme.ColorNameWarning
			return
		default:
			style = ButtonStyleFilledTonal
		}
	}
	switch style {
	case ButtonStyleFilled:
		foreground = theme.ColorNameForegroundOnPrimary
		background = theme.ColorNamePrimary
	case ButtonStyleFilledTonal:
		foreground = theme.ColorNameOnSecondaryContainer
		background = theme.ColorNameSecondaryContainer
	case ButtonStyleElevated:
		foreground = theme.ColorNamePrimary
		background = theme.ColorNameSurfaceContainerLow
	case ButtonStyleOutlined:
		foreground = theme.ColorNamePrimary
		background = theme.ColorNameButton
	case ButtonStyleText:
		foreground = theme.ColorNamePrimary
		if backgroundBlend != "" {
			background = theme.ColorNameSurface
		}
	}
	return foreground, background, backgroundBlend
}

func (r *buttonRenderer) padding(th bellui.Theme) geo.Size {
	return geo.NewSquareSize(th.Size(theme.SizeNameInnerPadding) * 2)
}

// must be called with r.button.propertyLock RLocked
func (r *buttonRenderer) updateIconAndText() {
	if r.button.Icon != nil && !r.button.Hidden {
		icon := r.button.Icon
		if r.icon == nil {
			r.icon = canvas.NewImageFromResource(icon)
			r.icon.FillMode = canvas.ImageFillContain
			r.SetObjects([]core.CanvasObject{r.background, r.gradientBG, r.ripple, r.label, r.icon})
		}
		if r.button.Disabled() {
			icon = theme.NewDisabledResource(icon)
		}
		r.icon.Resource = icon
		r.icon.Refresh()
		r.icon.Show()
	} else if r.icon != nil {
		r.icon.Hide()
	}
	if r.button.Text == "" {
		r.label.Hide()
	} else {
		r.label.Show()
	}
	r.label.Refresh()
}

func alignedPosition(align ButtonAlign, padding, objectSize, layoutSize geo.Size) (pos geo.Position) {
	pos.Y = (layoutSize.Height - objectSize.Height) / 2
	switch align {
	case ButtonAlignCenter:
		pos.X = (layoutSize.Width - objectSize.Width) / 2
	case ButtonAlignLeading:
		pos.X = padding.Width / 2
	case ButtonAlignTrailing:
		pos.X = layoutSize.Width - objectSize.Width - padding.Width/2
	}
	return pos
}

func blendColor(under, over color.Color) color.Color {
	// This alpha blends with the over operator, and accounts for RGBA() returning alpha-premultiplied values
	dstR, dstG, dstB, dstA := under.RGBA()
	srcR, srcG, srcB, srcA := over.RGBA()

	srcAlpha := float32(srcA) / 0xFFFF
	dstAlpha := float32(dstA) / 0xFFFF

	outAlpha := srcAlpha + dstAlpha*(1-srcAlpha)
	outR := srcR + uint32(float32(dstR)*(1-srcAlpha))
	outG := srcG + uint32(float32(dstG)*(1-srcAlpha))
	outB := srcB + uint32(float32(dstB)*(1-srcAlpha))
	// We create an RGBA64 here because the color components are already alpha-premultiplied 16-bit values (they're just stored in uint32s).
	return color.RGBA64{R: uint16(outR), G: uint16(outG), B: uint16(outB), A: uint16(outAlpha * 0xFFFF)}
}

// buttonRadius returns the corner radius for buttons. It uses the dedicated
// SizeNameButtonRadius if non-zero, otherwise falls back to SizeNameInputRadius.
func buttonRadius(th bellui.Theme) float32 {
	if r := th.Size(theme.SizeNameButtonRadius); r > 0 {
		return r
	}
	return th.Size(theme.SizeNameInputRadius)
}

// lightenColor returns a color that is brightened by the given factor (0.0–1.0).
// Each RGB channel is moved toward 255 by factor * (255 - channel).
func lightenColor(c color.Color, factor float64) color.Color {
	r, g, b, a := col.ToNRGBA(c)
	return &color.NRGBA{
		R: uint8(float64(r) + factor*(255-float64(r))),
		G: uint8(float64(g) + factor*(255-float64(g))),
		B: uint8(float64(b) + factor*(255-float64(b))),
		A: uint8(a),
	}
}

func newButtonTapAnimation(bg *canvas.Rectangle, w core.Widget, th bellui.Theme) *bellui.Animation {
	v := bellui.CurrentApp().Settings().ThemeVariant()
	return bellui.NewAnimation(canvas.DurationStandard, func(done float32) {
		mid := w.Size().Width / 2
		size := mid * done
		bg.Resize(geo.NewSize(size*2, w.Size().Height))
		bg.Move(geo.NewPos(mid-size, 0))

		r, g, bb, a := col.ToNRGBA(th.Color(theme.ColorNamePressed, v))
		aa := uint8(a)
		fade := aa - uint8(float32(aa)*done)
		if fade > 0 {
			bg.FillColor = &color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(bb), A: fade}
		} else {
			bg.FillColor = color.Transparent
		}
		canvas.Refresh(bg)
		if done == 1.0 {
			if btn, ok := w.(*Button); ok {
				btn.isAnimating = false
			}
		}
	})
}
