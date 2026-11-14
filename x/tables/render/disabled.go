package render

import (
	"image/color"

	"github.com/belluikit/bellui/core"
)

// gridDisabled reports whether the grid behind a render widget is disabled.
// The access interfaces do not carry Disabled (a fake grid in a test need not
// implement it), so it is found by type assertion: a grid without it is
// never disabled.
//
// The drivers do not route around a disabled widget: a tap, drag or
// accessible action reaches the deepest handler under it. So every
// interactive part the grid builds (the header, the floating filters, the
// select checkboxes, the accessible disclosure and cell nodes) asks here.
func gridDisabled(grid any) bool {
	d, ok := grid.(core.Disableable)
	return ok && d.Disabled()
}

// syncDisabled puts one of the grid's own Disableable parts in the grid's
// state, touching it only when the state differs: the parts are re-synced on
// every layout pass, and Disable and Enable each refresh.
func syncDisabled(part core.Disableable, disabled bool) {
	if part.Disabled() == disabled {
		return
	}
	if disabled {
		part.Disable()
	} else {
		part.Enable()
	}
}

// resolveColors is ResolveThemeColors for the grid behind a render widget.
// While the grid is disabled the grid's own text (header titles, sort and
// filter marks, group rows, chevrons, the status bar) takes the disabled
// colour, as widget.Button's label does. Backgrounds and the selection fill
// stay: selection is state, which a disabled grid still shows.
func resolveColors(grid any) ThemeColors {
	tc := ResolveThemeColors()
	if gridDisabled(grid) {
		tc = tc.disabled()
	}
	return tc
}

func (tc ThemeColors) disabled() ThemeColors {
	tc.Foreground = tc.DisabledFg
	tc.HeaderFg = tc.DisabledFg
	tc.GroupRowFg = tc.DisabledFg
	tc.Primary = tc.DisabledFg
	tc.Hover = color.Transparent
	tc.Focus = color.Transparent
	return tc
}
