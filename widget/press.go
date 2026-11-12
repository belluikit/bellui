package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// pressTarget gives a widget the MD3 pressed state (#1091). Embedded under
// this unexported name, it adds only its event methods to a public widget.
type pressTarget = widget.PressTarget

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
