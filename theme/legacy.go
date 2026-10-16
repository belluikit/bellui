package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
)

// FromLegacy returns a 2.0 Theme created from the given LegacyTheme data.
// This is a transition path and will be removed in the future (probably version 3.0).
//
// Since: 1.0
func FromLegacy(t bellui.LegacyTheme) bellui.Theme {
	return &legacyWrapper{old: t}
}

var _ bellui.Theme = (*legacyWrapper)(nil)

type legacyWrapper struct {
	old bellui.LegacyTheme
}

func (l *legacyWrapper) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	switch n {
	case ColorNameBackground:
		return l.old.BackgroundColor()
	case ColorNameForeground:
		return l.old.TextColor()
	case ColorNameButton:
		return l.old.ButtonColor()
	case ColorNameDisabledButton:
		return l.old.DisabledButtonColor()
	case ColorNameDisabled:
		return l.old.DisabledTextColor()
	case ColorNameFocus:
		return l.old.FocusColor()
	case ColorNameHover:
		return l.old.HoverColor()
	case ColorNamePlaceHolder:
		return l.old.PlaceHolderColor()
	case ColorNamePrimary:
		return l.old.PrimaryColor()
	case ColorNameScrollBar:
		return l.old.ScrollBarColor()
	case ColorNameScrollBarBackground:
		return l.old.BackgroundColor()
	case ColorNameShadow:
		return l.old.ShadowColor()
	default:
		return DefaultTheme().Color(n, v)
	}
}

func (l *legacyWrapper) Font(s bellui.TextStyle) bellui.Resource {
	if s.Monospace {
		return l.old.TextMonospaceFont()
	}
	if s.Bold {
		if s.Italic {
			return l.old.TextBoldItalicFont()
		}
		return l.old.TextBoldFont()
	}
	if s.Italic {
		return l.old.TextItalicFont()
	}
	return l.old.TextFont()
}

func (l *legacyWrapper) Icon(n bellui.ThemeIconName) bellui.Resource {
	return DefaultTheme().Icon(n)
}

func (l *legacyWrapper) Size(n bellui.ThemeSizeName) float32 {
	switch n {
	case SizeNameInlineIcon:
		return float32(l.old.IconInlineSize())
	case SizeNamePadding:
		return float32(l.old.Padding())
	case SizeNameScrollBar:
		return float32(l.old.ScrollBarSize())
	case SizeNameScrollBarSmall:
		return float32(l.old.ScrollBarSmallSize())
	case SizeNameText:
		return float32(l.old.TextSize())
	default:
		return DefaultTheme().Size(n)
	}
}
