package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// controlPressShape is the pressed state of a checkbox, a radio button or a
// switch's thumb: the circle of its hover and focus indicator, spreading from
// its centre wherever on the widget the press lands. It is Primary while the
// control is selected and OnSurface while it is not, per MD3.
func controlPressShape(th bellui.Theme, indicator core.CanvasObject, selected bool) widget.PressShape {
	content := theme.ColorNameOnSurface
	if selected {
		content = theme.ColorNamePrimary
	}
	size := indicator.Size()
	return widget.PressShape{
		Pos:          indicator.Position(),
		Size:         size,
		CornerRadius: geo.Min(size.Width, size.Height) / 2,
		Content:      th.Color(content, bellui.CurrentApp().Settings().ThemeVariant()),
		FromCenter:   true,
	}
}
