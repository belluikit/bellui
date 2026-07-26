package render

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// ThemeColors holds resolved theme colors for the current variant.
// Constructed once per Refresh to avoid repeated theme lookups in hot loops.
type ThemeColors struct {
	Background   color.Color
	Foreground   color.Color
	HeaderBg     color.Color
	HeaderFg     color.Color
	Separator    color.Color
	Selection    color.Color
	Hover        color.Color
	Focus        color.Color
	Primary      color.Color
	RowAlternate color.Color
	GroupRowBg   color.Color
	GroupRowFg   color.Color
	DisabledFg   color.Color
}

// ResolveThemeColors reads all grid-relevant colors from the current theme.
func ResolveThemeColors() ThemeColors {
	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bg := th.Color(theme.ColorNameBackground, v)
	fg := th.Color(theme.ColorNameForeground, v)
	primary := th.Color(theme.ColorNamePrimary, v)
	separator := th.Color(theme.ColorNameSeparator, v)
	selection := th.Color(theme.ColorNameSelection, v)
	hover := th.Color(theme.ColorNameHover, v)
	focus := th.Color(theme.ColorNameFocus, v)
	headerBg := th.Color(theme.ColorNameHeaderBackground, v)
	disabled := th.Color(theme.ColorNameDisabled, v)

	// Row alternate: slightly shift background
	altR, altG, altB, altA := bg.RGBA()
	// Nudge towards foreground slightly for alternation
	fgR, _, _, _ := fg.RGBA()
	blend := uint32(10)
	if fgR > 32768 { // dark bg (foreground is light)
		altR += blend * 256
		altG += blend * 256
		altB += blend * 256
	} else { // light bg
		altR -= blend * 256
		altG -= blend * 256
		altB -= blend * 256
	}
	rowAlt := color.NRGBA{
		R: uint8(altR >> 8), G: uint8(altG >> 8), B: uint8(altB >> 8), A: uint8(altA >> 8),
	}

	// Group row: use header-like bg
	groupBg := headerBg

	return ThemeColors{
		Background:   bg,
		Foreground:   fg,
		HeaderBg:     headerBg,
		HeaderFg:     fg,
		Separator:    separator,
		Selection:    selection,
		Hover:        hover,
		Focus:        focus,
		Primary:      primary,
		RowAlternate: rowAlt,
		GroupRowBg:   groupBg,
		GroupRowFg:   fg,
		DisabledFg:   disabled,
	}
}
