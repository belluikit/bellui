package widget

import (
	"image/color"
	"net/url"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var (
	_ event.Focusable         = (*Hyperlink)(nil)
	_ event.Activatable       = (*Hyperlink)(nil)
	_ core.Widget             = (*Hyperlink)(nil)
	_ core.Accessible         = (*Hyperlink)(nil)
	_ core.AccessibleActioner = (*Hyperlink)(nil)
)

// Hyperlink widget is a text component with appropriate padding and layout.
// When clicked, the default web browser should open with a URL
type Hyperlink struct {
	BaseWidget
	Text      string
	URL       *url.URL
	Alignment bellui.TextAlign // The alignment of the Text
	Wrapping  bellui.TextWrap  // The wrapping of the Text
	TextStyle bellui.TextStyle // The style of the hyperlink text

	// The truncation mode of the hyperlink
	//
	// Since: 1.0
	Truncation bellui.TextTruncation

	// The theme size name for the text size of the hyperlink
	//
	// Since: 1.0
	SizeName bellui.ThemeSizeName

	// ColorName is the theme color used to render the link text, its hover
	// underline and its focus ring. When empty (the zero value) the link uses
	// the theme's primary color, so existing links are unchanged.
	//
	// Since: 1.0
	ColorName bellui.ThemeColorName

	// FontSource is an optional custom font resource used to render the link
	// text, matching TextSegment.Style.FontSource (#494). When nil (the zero
	// value) the theme font is used, so existing links are unchanged. The chosen
	// font is honoured for measurement as well as rendering.
	//
	// Since: 1.0
	FontSource bellui.Resource

	// OnTapped overrides the default `bellui.OpenURL` call when the link is tapped
	//
	// Since: 1.0
	OnTapped func() `json:"-"`

	textSize         geo.Size // updated in syncSegments
	focused, hovered bool
	provider         RichText
}

// NewHyperlink creates a new hyperlink widget with the set text content
func NewHyperlink(text string, url *url.URL) *Hyperlink {
	return NewHyperlinkWithStyle(text, url, bellui.TextAlignLeading, bellui.TextStyle{})
}

// NewHyperlinkWithStyle creates a new hyperlink widget with the set text content
func NewHyperlinkWithStyle(text string, url *url.URL, alignment bellui.TextAlign, style bellui.TextStyle) *Hyperlink {
	hl := &Hyperlink{
		Text:      text,
		URL:       url,
		Alignment: alignment,
		TextStyle: style,
	}
	hl.ExtendBaseWidget(hl)
	return hl
}

// AccessibleRole returns [core.RoleLink].
func (hl *Hyperlink) AccessibleRole() core.AccessibleRole { return core.RoleLink }

// AccessibleLabel returns the hyperlink text.
func (hl *Hyperlink) AccessibleLabel() string { return hl.Text }

// AccessibleActions returns the "press" action.
func (hl *Hyperlink) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Open the link"}}
}

// PerformAccessibleAction opens the link URL.
func (hl *Hyperlink) PerformAccessibleAction(name string) {
	if name == core.ActionPress {
		hl.openURL()
	}
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (hl *Hyperlink) CreateRenderer() core.WidgetRenderer {
	hl.ExtendBaseWidget(hl)
	hl.provider.ExtendBaseWidget(&hl.provider)
	hl.syncSegments()

	th := hl.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	focus := canvas.NewRectangle(color.Transparent)
	focus.StrokeColor = th.Color(theme.ColorNamePrimary, v)
	focus.StrokeWidth = 2
	focus.Hide()
	under := canvas.NewRectangle(th.Color(theme.ColorNamePrimary, v))
	under.Hide()
	return &hyperlinkRenderer{hl: hl, objects: []core.CanvasObject{&hl.provider, focus, under}, focus: focus, under: under}
}

// Cursor returns the cursor type of this widget
func (hl *Hyperlink) Cursor() desktop.Cursor {
	if hl.hovered {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

// FocusGained is a hook called by the focus handling logic after this object gained the focus.
func (hl *Hyperlink) FocusGained() {
	hl.focused = true
	hl.BaseWidget.Refresh()
}

// FocusLost is a hook called by the focus handling logic after this object lost the focus.
func (hl *Hyperlink) FocusLost() {
	hl.focused = false
	hl.BaseWidget.Refresh()
}

// HoverIn is a hook that is called if the pointer enters the element.
func (hl *Hyperlink) HoverIn(e *event.HoverEvent) {
	hl.HoverMoved(e)
}

// HoverMoved is a hook that is called if the pointer moved over the element.
func (hl *Hyperlink) HoverMoved(e *event.HoverEvent) {
	oldHovered := hl.hovered
	hl.hovered = hl.isPosOverText(e.Position)
	if hl.hovered != oldHovered {
		hl.BaseWidget.Refresh()
	}
}

// HoverOut is a hook that is called if the pointer leaves the element.
func (hl *Hyperlink) HoverOut() {
	changed := hl.hovered
	hl.hovered = false
	if changed {
		hl.BaseWidget.Refresh()
	}
}

func (hl *Hyperlink) focusWidth() float32 {
	th := hl.Theme()

	innerPad := th.Size(theme.SizeNameInnerPadding)
	return geo.Min(hl.Size().Width, hl.textSize.Width+innerPad+th.Size(theme.SizeNamePadding)*2) - innerPad
}

func (hl *Hyperlink) focusXPos() float32 {
	innerPad := hl.Theme().Size(theme.SizeNameInnerPadding)

	// Resolve against the active layout direction so the focus box tracks the
	// text edge under RTL (no-op for the default LTR layout).
	switch lang.ResolveAlign(hl.Alignment) {
	case bellui.TextAlignLeading:
		return innerPad / 2
	case bellui.TextAlignCenter:
		return (hl.Size().Width - hl.focusWidth()) / 2
	case bellui.TextAlignTrailing:
		return (hl.Size().Width - hl.focusWidth()) - innerPad/2
	default:
		return 0 // unreached
	}
}

func (hl *Hyperlink) isPosOverText(pos geo.Position) bool {
	th := hl.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	pad := th.Size(theme.SizeNamePadding)
	lineCount := geo.Max(1, float32(len(hl.provider.rowBounds)))

	xpos := hl.focusXPos()
	return pos.X >= xpos && pos.X <= xpos+hl.focusWidth() &&
		pos.Y >= innerPad/2 && pos.Y <= hl.textSize.Height*lineCount+pad*2+innerPad/2
}

// Refresh triggers a redraw of the hyperlink.
func (hl *Hyperlink) Refresh() {
	if len(hl.provider.Segments) == 0 {
		return // Not initialized yet.
	}

	hl.syncSegments()
	hl.provider.Refresh()
	hl.BaseWidget.Refresh()
}

// MinSize returns the smallest size this widget can shrink to
func (hl *Hyperlink) MinSize() geo.Size {
	hl.ExtendBaseWidget(hl)
	return hl.BaseWidget.MinSize()
}

// Resize sets a new size for the hyperlink.
// Note this should not be used if the widget is being managed by a Layout within a Container.
func (hl *Hyperlink) Resize(size geo.Size) {
	hl.BaseWidget.Resize(size)

	if len(hl.provider.Segments) == 0 {
		return // Not initialized yet.
	}
	hl.provider.Resize(size)
}

// SetText sets the text of the hyperlink.
// It is safe to call from any goroutine.
func (hl *Hyperlink) SetText(text string) {
	onMain(func() {
		hl.Text = text
		if len(hl.provider.Segments) == 0 {
			return
		}
		hl.syncSegments()
		hl.provider.Refresh()
	})
}

// SetURL sets the URL of the hyperlink, taking in a URL type
func (hl *Hyperlink) SetURL(url *url.URL) {
	hl.URL = url
}

// SetURLFromString sets the URL of the hyperlink, taking in a string type
func (hl *Hyperlink) SetURLFromString(str string) error {
	u, err := url.Parse(str)
	if err != nil {
		return err
	}
	hl.SetURL(u)
	return nil
}

// Tapped is called when a pointer tapped event is captured and triggers any change handler
func (hl *Hyperlink) Tapped(e *event.PointEvent) {
	if len(hl.provider.Segments) != 0 && !hl.isPosOverText(e.Position) {
		return // tapped outside text area
	}
	hl.invokeAction()
}

func (hl *Hyperlink) invokeAction() {
	onTapped := hl.OnTapped

	if onTapped != nil {
		onTapped()
		return
	}
	hl.openURL()
}

// TypedRune is a hook called by the input handling logic on text input events if this object is focused.
func (hl *Hyperlink) TypedRune(rune) {
}

// TypedKey is a hook called by the input handling logic on key events if this object is focused.
func (hl *Hyperlink) TypedKey(ev *event.KeyEvent) {
	if ev.Name == event.KeySpace {
		hl.invokeAction()
	}
}

// ActivatedByKey reports that Space activates the link, so the driver fires the
// keyboard-activation signal in parity with a pointer tap (#700).
func (hl *Hyperlink) ActivatedByKey(ev *event.KeyEvent) bool {
	return ev.Name == event.KeySpace
}

func (hl *Hyperlink) openURL() {
	url := hl.URL

	if url != nil {
		err := bellui.OpenURL(url)
		if err != nil {
			bellui.LogError("Failed to open url", err)
		}
	}
}

func (hl *Hyperlink) syncSegments() {
	th := hl.Theme()

	hl.provider.Wrapping = hl.Wrapping
	hl.provider.Truncation = hl.Truncation

	if len(hl.provider.Segments) == 0 {
		hl.provider.Scroll = widget.ScrollNone
		hl.provider.Segments = []RichTextSegment{
			&TextSegment{
				Style: RichTextStyle{
					Alignment:  hl.Alignment,
					ColorName:  hl.colorName(),
					Inline:     true,
					TextStyle:  hl.TextStyle,
					FontSource: hl.FontSource,
				},
				Text: hl.Text,
			},
		}
	} else {
		segment := hl.provider.Segments[0].(*TextSegment)
		segment.Style.Alignment = hl.Alignment
		segment.Style.ColorName = hl.colorName()
		segment.Style.TextStyle = hl.TextStyle
		segment.Style.FontSource = hl.FontSource
		segment.Text = hl.Text
	}

	sizeName := hl.SizeName
	if sizeName == "" {
		sizeName = theme.SizeNameText
	}
	hl.provider.Segments[0].(*TextSegment).Style.SizeName = sizeName
	hl.textSize, _ = bellui.MeasureTextFull(hl.Text, th.Size(sizeName), hl.TextStyle, hl.FontSource)
}

// colorName is the theme color used for the link text, hover underline and
// focus ring — the explicit ColorName override when set, else the theme's
// primary color (the historical default).
func (hl *Hyperlink) colorName() bellui.ThemeColorName {
	if hl.ColorName != "" {
		return hl.ColorName
	}
	return theme.ColorNamePrimary
}

var _ core.WidgetRenderer = (*hyperlinkRenderer)(nil)

type hyperlinkRenderer struct {
	hl    *Hyperlink
	focus *canvas.Rectangle
	under *canvas.Rectangle

	objects []core.CanvasObject
}

func (r *hyperlinkRenderer) Destroy() {
}

func (r *hyperlinkRenderer) Layout(s geo.Size) {
	th := r.hl.Theme()
	textSize := r.hl.textSize
	innerPad := th.Size(theme.SizeNameInnerPadding)
	w := r.hl.focusWidth()
	xposFocus := r.hl.focusXPos()

	xposUnderline := xposFocus + innerPad/2
	lineCount := float32(len(r.hl.provider.rowBounds))

	r.hl.provider.Resize(s)
	r.focus.Move(geo.NewPos(xposFocus, innerPad/2))
	r.focus.Resize(geo.NewSize(w, textSize.Height*lineCount+innerPad))
	r.under.Move(geo.NewPos(xposUnderline, textSize.Height*lineCount+th.Size(theme.SizeNamePadding)*2))
	r.under.Resize(geo.NewSize(w-innerPad, 1))
}

func (r *hyperlinkRenderer) MinSize() geo.Size {
	return r.hl.provider.MinSize()
}

func (r *hyperlinkRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *hyperlinkRenderer) Refresh() {
	r.hl.provider.Refresh()
	th := r.hl.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	linkColor := th.Color(r.hl.colorName(), v)
	r.focus.StrokeColor = linkColor
	r.focus.Hidden = !r.hl.focused
	r.focus.Refresh()
	r.under.FillColor = linkColor
	r.under.Hidden = !r.hl.hovered
	r.under.Refresh()
}
