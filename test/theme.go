package test

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

var defaultTheme bellui.Theme

// Try to keep these in sync with the existing color names at theme/color.go.
var knownColorNames = [...]bellui.ThemeColorName{
	theme.ColorNameBackground,
	theme.ColorNameButton,
	theme.ColorNameDisabled,
	theme.ColorNameDisabledButton,
	theme.ColorNameError,
	theme.ColorNameFocus,
	theme.ColorNameForeground,
	theme.ColorNameForegroundOnError,
	theme.ColorNameForegroundOnPrimary,
	theme.ColorNameForegroundOnSuccess,
	theme.ColorNameForegroundOnWarning,
	theme.ColorNameHeaderBackground,
	theme.ColorNameHover,
	theme.ColorNameHyperlink,
	theme.ColorNameInputBackground,
	theme.ColorNameInputBorder,
	theme.ColorNameMenuBackground,
	theme.ColorNameOverlayBackground,
	theme.ColorNamePlaceHolder,
	theme.ColorNamePressed,
	theme.ColorNamePrimary,
	theme.ColorNameScrollBar,
	theme.ColorNameScrollBarBackground,
	theme.ColorNameSelection,
	theme.ColorNameSeparator,
	theme.ColorNameShadow,
	theme.ColorNameSuccess,
	theme.ColorNameWarning,
}

// KnownThemeVariants returns the known theme variants mapped by a descriptive key.
func KnownThemeVariants() map[string]bellui.ThemeVariant {
	// Try to keep this in sync with the existing variants at theme/theme.go
	return map[string]bellui.ThemeVariant{
		"dark":  theme.VariantDark,
		"light": theme.VariantLight,
	}
}

// NewTheme returns a new test theme using quiet ugly colors.
func NewTheme() bellui.Theme {
	blue := func(alpha uint8) color.Color {
		return &color.NRGBA{R: 0, G: 0, B: 255, A: alpha}
	}
	gray := func(level uint8) color.Color {
		return &color.Gray{Y: level}
	}
	green := func(alpha uint8) color.Color {
		return &color.NRGBA{R: 0, G: 255, B: 0, A: alpha}
	}
	red := func(alpha uint8) color.Color {
		return &color.NRGBA{R: 200, G: 0, B: 0, A: alpha}
	}

	return &configurableTheme{
		colors: map[bellui.ThemeColorName]color.Color{
			theme.ColorNameBackground:              red(255),
			theme.ColorNameButton:                  gray(100),
			theme.ColorNameDisabled:                gray(20),
			theme.ColorNameDisabledButton:          gray(230),
			theme.ColorNameError:                   blue(255),
			theme.ColorNameFocus:                   red(66),
			theme.ColorNameForeground:              gray(255),
			theme.ColorNameOnSurface:               gray(255),
			theme.ColorNameOnSurfaceVariant:        gray(255),
			theme.ColorNameForegroundOnError:       red(210),
			theme.ColorNameForegroundOnPrimary:     red(200),
			theme.ColorNameForegroundOnSuccess:     blue(201),
			theme.ColorNameForegroundOnWarning:     blue(202),
			theme.ColorNameHeaderBackground:        red(22),
			theme.ColorNameHover:                   green(200),
			theme.ColorNameHyperlink:               blue(240),
			theme.ColorNameInputBackground:         red(30),
			theme.ColorNameInputBorder:             gray(10),
			theme.ColorNameMenuBackground:          red(50),
			theme.ColorNameOverlayBackground:       red(44),
			theme.ColorNamePlaceHolder:             blue(200),
			theme.ColorNamePressed:                 blue(250),
			theme.ColorNamePrimary:                 green(255),
			theme.ColorNameScrollBar:               blue(220),
			theme.ColorNameScrollBarBackground:     red(20),
			theme.ColorNameSelection:               red(55),
			theme.ColorNameSeparator:               gray(30),
			theme.ColorNameShadow:                  blue(150),
			theme.ColorNameSuccess:                 green(150),
			theme.ColorNameWarning:                 red(100),
			theme.ColorNameOutline:                 gray(80),
			theme.ColorNameOutlineVariant:          gray(40),
			theme.ColorNameSurface:                 red(220),
			theme.ColorNameSurfaceContainerLowest:  red(200),
			theme.ColorNameSurfaceContainerLow:     red(210),
			theme.ColorNameSurfaceContainerMedium:  red(215),
			theme.ColorNameSurfaceContainerHigh:    red(220),
			theme.ColorNameSurfaceContainerHighest: red(225),
			theme.ColorNameSecondaryContainer:      blue(100),
			theme.ColorNameOnSecondaryContainer:    gray(240),
			theme.ColorNameScrim:                   blue(180),
		},
		fonts: map[bellui.TextStyle]bellui.Resource{
			{}:                         theme.DefaultTextBoldFont(),
			{Bold: true}:               theme.DefaultTextItalicFont(),
			{Bold: true, Italic: true}: theme.DefaultTextMonospaceFont(),
			{Italic: true}:             theme.DefaultTextBoldItalicFont(),
			{Monospace: true}:          theme.DefaultTextFont(),
			{Symbol: true}:             theme.DefaultSymbolFont(),
		},
		name: "Ugly Test Theme",
		sizes: map[bellui.ThemeSizeName]float32{
			theme.SizeNameInlineIcon:         float32(24),
			theme.SizeNameInnerPadding:       float32(20),
			theme.SizeNameLineSpacing:        float32(6),
			theme.SizeNamePadding:            float32(10),
			theme.SizeNameScrollBar:          float32(10),
			theme.SizeNameScrollBarMinThumb:  float32(30),
			theme.SizeNameScrollBarSmall:     float32(2),
			theme.SizeNameSeparatorThickness: float32(1),
			theme.SizeNameText:               float32(18),
			theme.SizeNameHeadingText:        float32(30.6),
			theme.SizeNameSubHeadingText:     float32(24),
			theme.SizeNameCaptionText:        float32(15),
			theme.SizeNameInputBorder:        float32(5),
			theme.SizeNameInputRadius:        float32(2),
			theme.SizeNameSelectionRadius:    float32(6),
			theme.SizeNameScrollBarRadius:    float32(2),
			theme.SizeNameShadowDepth:        float32(0),
			theme.SizeNameButtonRadius:       float32(0),
			theme.SizeNameCardRadius:         float32(0),
			theme.SizeNameDisplayLarge:       float32(57),
			theme.SizeNameDisplayMedium:      float32(45),
			theme.SizeNameDisplaySmall:       float32(36),
			theme.SizeNameHeadlineLarge:      float32(32),
			theme.SizeNameHeadlineMedium:     float32(28),
			theme.SizeNameHeadlineSmall:      float32(24),
			theme.SizeNameTitleLarge:         float32(30.6),
			theme.SizeNameTitleMedium:        float32(24),
			theme.SizeNameTitleSmall:         float32(18),
			theme.SizeNameBodyLarge:          float32(20),
			theme.SizeNameBodyMedium:         float32(18),
			theme.SizeNameBodySmall:          float32(15),
			theme.SizeNameLabelLarge:         float32(14),
			theme.SizeNameLabelMedium:        float32(12),
			theme.SizeNameLabelSmall:         float32(11),
			theme.SizeNameShapeNone:          float32(0),
			theme.SizeNameShapeExtraSmall:    float32(4),
			theme.SizeNameShapeSmall:         float32(8),
			theme.SizeNameShapeMedium:        float32(12),
			theme.SizeNameShapeLarge:         float32(16),
			theme.SizeNameShapeExtraLarge:    float32(28),
			theme.SizeNameShapeFull:          float32(9999),
			// MD3 Component Dimensions
			theme.SizeNameIconButtonSize:   float32(40),
			theme.SizeNameIconSize:         float32(24),
			theme.SizeNameFABSize:          float32(56),
			theme.SizeNameChipHeight:       float32(32),
			theme.SizeNameButtonMinHeight:  float32(40),
			theme.SizeNameNavBarHeight:     float32(80),
			theme.SizeNameNavRailWidth:     float32(80),
			theme.SizeNameTopBarHeight:     float32(64),
			theme.SizeNameBottomBarHeight:  float32(80),
			theme.SizeNameSwitchTrackWidth: float32(52),
		},
	}
}

// Theme returns a test theme useful for image based tests.
func Theme() bellui.Theme {
	if defaultTheme == nil {
		defaultTheme = &configurableTheme{
			colors: map[bellui.ThemeColorName]color.Color{
				theme.ColorNameBackground:          color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xff},
				theme.ColorNameButton:              color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff},
				theme.ColorNameDisabled:            color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff},
				theme.ColorNameDisabledButton:      color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff},
				theme.ColorNameError:               color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0xff},
				theme.ColorNameFocus:               color.NRGBA{R: 0x78, G: 0x3a, B: 0x3a, A: 0xff},
				theme.ColorNameForeground:          color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
				theme.ColorNameForegroundOnError:   color.NRGBA{R: 0x08, G: 0x0a, B: 0x0f, A: 0xff},
				theme.ColorNameForegroundOnPrimary: color.NRGBA{R: 0x08, G: 0x0c, B: 0x0f, A: 0xff},
				theme.ColorNameForegroundOnSuccess: color.NRGBA{R: 0x0a, G: 0x0c, B: 0x0f, A: 0xff},
				theme.ColorNameForegroundOnWarning: color.NRGBA{R: 0x08, G: 0x0c, B: 0x0a, A: 0xff},
				theme.ColorNameHeaderBackground:    color.NRGBA{R: 0x25, G: 0x25, B: 0x25, A: 0xff},
				theme.ColorNameHover:               color.NRGBA{R: 0x88, G: 0xff, B: 0xff, A: 0x22},
				theme.ColorNameHyperlink:           color.NRGBA{R: 0xff, G: 0xcc, B: 0x80, A: 0xff},
				theme.ColorNameInputBackground:     color.NRGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff},
				theme.ColorNameInputBorder:         color.NRGBA{R: 0x86, G: 0x86, B: 0x86, A: 0xff},
				theme.ColorNameMenuBackground:      color.NRGBA{R: 0x56, G: 0x56, B: 0x56, A: 0xff},
				theme.ColorNameOverlayBackground:   color.NRGBA{R: 0x28, G: 0x28, B: 0x28, A: 0xff},
				theme.ColorNamePlaceHolder:         color.NRGBA{R: 0xaa, G: 0xaa, B: 0xaa, A: 0xff},
				theme.ColorNamePressed:             color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x33},
				theme.ColorNamePrimary:             color.NRGBA{R: 0xff, G: 0xc0, B: 0x80, A: 0xff},
				theme.ColorNameScrollBar:           color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xaa},
				theme.ColorNameScrollBarBackground: color.NRGBA{R: 0x67, G: 0x66, B: 0x66, A: 0xff},
				theme.ColorNameSelection:           color.NRGBA{R: 0x78, G: 0x3a, B: 0x3a, A: 0x99},
				theme.ColorNameSeparator:           color.NRGBA{R: 0x90, G: 0x90, B: 0x90, A: 0xff},
				theme.ColorNameShadow:              color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x88},
				theme.ColorNameSuccess:             color.NRGBA{R: 0x00, G: 0x99, B: 0x00, A: 0xff},
				theme.ColorNameWarning:             color.NRGBA{R: 0xee, G: 0xee, B: 0x00, A: 0xff},
				theme.ColorNameCardBackground:      color.Transparent,
				theme.ColorNameElevatedBackground:  color.Transparent,
				// MD3 color roles
				theme.ColorNamePrimaryContainer:        color.NRGBA{R: 0x40, G: 0x30, B: 0x60, A: 0xff},
				theme.ColorNameOnPrimaryContainer:      color.NRGBA{R: 0xf0, G: 0xe0, B: 0xff, A: 0xff},
				theme.ColorNameSecondary:               color.NRGBA{R: 0x80, G: 0x70, B: 0x90, A: 0xff},
				theme.ColorNameOnSecondary:             color.NRGBA{R: 0x20, G: 0x10, B: 0x30, A: 0xff},
				theme.ColorNameSecondaryContainer:      color.NRGBA{R: 0x40, G: 0x35, B: 0x50, A: 0xff},
				theme.ColorNameOnSecondaryContainer:    color.NRGBA{R: 0xe0, G: 0xd0, B: 0xf0, A: 0xff},
				theme.ColorNameTertiary:                color.NRGBA{R: 0x90, G: 0x80, B: 0x70, A: 0xff},
				theme.ColorNameOnTertiary:              color.NRGBA{R: 0x30, G: 0x20, B: 0x10, A: 0xff},
				theme.ColorNameTertiaryContainer:       color.NRGBA{R: 0x50, G: 0x40, B: 0x30, A: 0xff},
				theme.ColorNameOnTertiaryContainer:     color.NRGBA{R: 0xf0, G: 0xe0, B: 0xd0, A: 0xff},
				theme.ColorNameErrorContainer:          color.NRGBA{R: 0x60, G: 0x20, B: 0x20, A: 0xff},
				theme.ColorNameOnErrorContainer:        color.NRGBA{R: 0xff, G: 0xd0, B: 0xd0, A: 0xff},
				theme.ColorNameSurface:                 color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff},
				theme.ColorNameSurfaceDim:              color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
				theme.ColorNameSurfaceBright:           color.NRGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff},
				theme.ColorNameSurfaceContainerLowest:  color.NRGBA{R: 0x0c, G: 0x0c, B: 0x0c, A: 0xff},
				theme.ColorNameSurfaceContainerLow:     color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff},
				theme.ColorNameSurfaceContainerMedium:  color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff},
				theme.ColorNameSurfaceContainerHigh:    color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff},
				theme.ColorNameSurfaceContainerHighest: color.NRGBA{R: 0x35, G: 0x35, B: 0x35, A: 0xff},
				theme.ColorNameOnSurface:               color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
				theme.ColorNameOnSurfaceVariant:        color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
				theme.ColorNameOutline:                 color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff},
				theme.ColorNameOutlineVariant:          color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xff},
				theme.ColorNameInverseSurface:          color.NRGBA{R: 0xe6, G: 0xe6, B: 0xe6, A: 0xff},
				theme.ColorNameInverseOnSurface:        color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff},
				theme.ColorNameInversePrimary:          color.NRGBA{R: 0x60, G: 0x50, B: 0xa0, A: 0xff},
				theme.ColorNameScrim:                   color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff},
			},
			fonts: map[bellui.TextStyle]bellui.Resource{
				{}:                         theme.DefaultTextFont(),
				{Bold: true}:               theme.DefaultTextBoldFont(),
				{Bold: true, Italic: true}: theme.DefaultTextBoldItalicFont(),
				{Italic: true}:             theme.DefaultTextItalicFont(),
				{Monospace: true}:          theme.DefaultTextMonospaceFont(),
				{Symbol: true}:             theme.DefaultSymbolFont(),
			},
			name: "Default Test Theme",
			sizes: map[bellui.ThemeSizeName]float32{
				theme.SizeNameInlineIcon:           float32(20),
				theme.SizeNameInnerPadding:         float32(8),
				theme.SizeNameLineSpacing:          float32(4),
				theme.SizeNamePadding:              float32(4),
				theme.SizeNameScrollBar:            float32(16),
				theme.SizeNameScrollBarMinThumb:    float32(48),
				theme.SizeNameScrollBarSmall:       float32(3),
				theme.SizeNameSeparatorThickness:   float32(1),
				theme.SizeNameText:                 float32(14),
				theme.SizeNameHeadingText:          float32(23.8),
				theme.SizeNameSubHeadingText:       float32(18),
				theme.SizeNameCaptionText:          float32(11),
				theme.SizeNameInputBorder:          float32(2),
				theme.SizeNameInputRadius:          float32(4),
				theme.SizeNameSelectionRadius:      float32(4),
				theme.SizeNameScrollBarRadius:      float32(3),
				theme.SizeNameWindowTitleBarHeight: float32(20),
				theme.SizeNameWindowButtonHeight:   float32(10),
				theme.SizeNameWindowButtonIcon:     float32(8),
				theme.SizeNameWindowButtonRadius:   float32(5),
				theme.SizeNameShadowDepth:          float32(0),
				theme.SizeNameButtonRadius:         float32(0),
				theme.SizeNameCardRadius:           float32(0),
				// MD3 Typography
				theme.SizeNameDisplayLarge:    float32(57),
				theme.SizeNameDisplayMedium:   float32(45),
				theme.SizeNameDisplaySmall:    float32(36),
				theme.SizeNameHeadlineLarge:   float32(32),
				theme.SizeNameHeadlineMedium:  float32(28),
				theme.SizeNameHeadlineSmall:   float32(24),
				theme.SizeNameTitleLarge:      float32(22),
				theme.SizeNameTitleMedium:     float32(16),
				theme.SizeNameTitleSmall:      float32(14),
				theme.SizeNameBodyLarge:       float32(16),
				theme.SizeNameBodyMedium:      float32(14),
				theme.SizeNameBodySmall:       float32(12),
				theme.SizeNameLabelLarge:      float32(14),
				theme.SizeNameLabelMedium:     float32(12),
				theme.SizeNameLabelSmall:      float32(11),
				theme.SizeNameShapeNone:       float32(0),
				theme.SizeNameShapeExtraSmall: float32(4),
				theme.SizeNameShapeSmall:      float32(8),
				theme.SizeNameShapeMedium:     float32(12),
				theme.SizeNameShapeLarge:      float32(16),
				theme.SizeNameShapeExtraLarge: float32(28),
				theme.SizeNameShapeFull:       float32(9999),
				// MD3 Component Dimensions
				theme.SizeNameIconButtonSize:   float32(40),
				theme.SizeNameIconSize:         float32(24),
				theme.SizeNameFABSize:          float32(56),
				theme.SizeNameChipHeight:       float32(32),
				theme.SizeNameButtonMinHeight:  float32(40),
				theme.SizeNameNavBarHeight:     float32(80),
				theme.SizeNameNavRailWidth:     float32(80),
				theme.SizeNameTopBarHeight:     float32(64),
				theme.SizeNameBottomBarHeight:  float32(80),
				theme.SizeNameSwitchTrackWidth: float32(52),
			},
		}
	}
	return defaultTheme
}

type configurableTheme struct {
	colors map[bellui.ThemeColorName]color.Color
	fonts  map[bellui.TextStyle]bellui.Resource
	name   string
	sizes  map[bellui.ThemeSizeName]float32
}

var _ bellui.Theme = (*configurableTheme)(nil)

func (t *configurableTheme) Color(n bellui.ThemeColorName, _ bellui.ThemeVariant) color.Color {
	// Namespaced names ("." or ":") belong to extension packages (x/editor,
	// x/tables); asking the app theme first and treating nil as "use your
	// built-in default" is their supported pattern (#150), so an undefined one
	// is not an error worth logging on every paint.
	if t.colors[n] == nil && !strings.ContainsAny(string(n), ".:") {
		bellui.LogError(fmt.Sprintf("color %s not defined in theme %s", n, t.name), nil)
	}

	return t.colors[n]
}

func (t *configurableTheme) Font(style bellui.TextStyle) bellui.Resource {
	if t.fonts[style] == nil {
		bellui.LogError(fmt.Sprintf("font for style %#v not defined in theme %s", style, t.name), nil)
	}

	return t.fonts[style]
}

func (t *configurableTheme) Icon(n bellui.ThemeIconName) bellui.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (t *configurableTheme) Size(s bellui.ThemeSizeName) float32 {
	if _, ok := t.sizes[s]; !ok {
		bellui.LogError(fmt.Sprintf("size %s not defined in theme %s", s, t.name), nil)
		return 0
	}

	return t.sizes[s]
}
