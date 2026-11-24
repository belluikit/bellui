package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// pressTarget gives a widget the MD3 pressed state (#1091). Embedded under
// this unexported name, it adds only its event methods to a public widget.
type pressTarget = widget.PressTarget

// PressTarget gives a widget made outside bellui the MD3 pressed state that
// bellui's own widgets show, such as Hyperlink and Chip: a ripple that
// spreads from the press and fades on release (#1449). A widget embeds it,
// which makes the widget a mobile.Touchable and a desktop.Mouseable, then:
//
//   - calls AttachPress from CreateRenderer, and places the ripple it
//     returns above the widget's background and below its content;
//   - calls TapPress from Tapped, so a tap that had no press before it (a
//     key, an accessibility action, a tap from code) shows one too;
//   - calls CancelPress from HoverOut if it is Hoverable, because a mouse
//     released elsewhere sends it no MouseUp.
//
// A widget that defines TouchDown or MouseDown itself, to refuse a press
// while disabled say, calls the PressTarget's method from its own.
//
// Since: 1.4
type PressTarget = widget.PressTarget

// PressShape is where a PressTarget's pressed state shows, in the widget's
// coordinates, and in what colour: Content is the colour of the widget's
// text or icon, and the state layer is that colour at the theme's pressed
// opacity.
//
// Since: 1.4
type PressShape = widget.PressShape

// AttachPress starts the pressed state of t for a new renderer and returns
// the ripple it draws on. shape is asked at each press for where the state
// layer shows; it returns false while the widget takes no press, such as
// when it is disabled.
//
// Since: 1.4
func AttachPress(t *PressTarget, shape func() (PressShape, bool)) *canvas.Ripple {
	return widget.AttachPress(t, shape)
}

// TapPress shows a press and its release for a tap that had no press before
// it. A pointer tap's press showed already, and this does nothing for it.
// ev is nil for a tap with no position, which shows from the shape's centre.
//
// Since: 1.4
func TapPress(t *PressTarget, ev *event.PointEvent) { widget.TapPress(t, ev) }

// CancelPress ends the pressed state of t with no feedback left: the pointer
// left the widget, or it stopped taking presses mid-press.
//
// Since: 1.4
func CancelPress(t *PressTarget) { widget.CancelPress(t) }

// rowPressShape is the pressed state of a row, a menu item or a cell: its
// whole bounds, with the corners of its selection, in OnSurface.
func rowPressShape(th bellui.Theme, size geo.Size) widget.PressShape {
	v := bellui.CurrentApp().Settings().ThemeVariant()
	return widget.PressShape{
		Size:         size,
		CornerRadius: th.Size(theme.SizeNameSelectionRadius),
		Content:      th.Color(theme.ColorNameOnSurface, v),
	}
}

// navIndicatorPressShape is the pressed state of a navigation bar or rail
// item: its indicator pill, in the colour of the icon in it.
func navIndicatorPressShape(th bellui.Theme, indicator *canvas.Rectangle, selected bool) widget.PressShape {
	content := theme.ColorNameOnSurfaceVariant
	if selected {
		content = theme.ColorNameOnSecondaryContainer
	}
	return widget.PressShape{
		Pos:          indicator.Position(),
		Size:         indicator.Size(),
		CornerRadius: indicator.CornerRadius,
		Content:      th.Color(content, bellui.CurrentApp().Settings().ThemeVariant()),
	}
}
