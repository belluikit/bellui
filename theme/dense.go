package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
	internaltheme "github.com/belluikit/bellui/internal/theme"
)

// denseTheme wraps the enhancedTheme with tighter spacing, smaller text,
// and compact sizing for information-dense desktop applications (trading
// systems, monitoring dashboards). Colors come from the MD3 system via
// the enhanced theme.
type denseTheme struct {
	enhanced *enhancedTheme
}

var (
	_ bellui.Theme                   = (*denseTheme)(nil)
	_ internaltheme.RenderStyleTheme = (*denseTheme)(nil)
)

// DenseTheme returns a theme optimized for high-density desktop layouts.
func DenseTheme() bellui.Theme {
	builtin := &builtinTheme{variant: internaltheme.VariantNameUserPreference}
	builtin.initFonts()
	return &denseTheme{enhanced: &enhancedTheme{builtin: builtin}}
}

func (t *denseTheme) RenderStyle() internaltheme.RenderStyle {
	return t.enhanced.RenderStyle()
}

func (t *denseTheme) Font(style bellui.TextStyle) bellui.Resource {
	return t.enhanced.Font(style)
}

func (t *denseTheme) Icon(name bellui.ThemeIconName) bellui.Resource {
	return t.enhanced.Icon(name)
}

// denseSizeValues maps theme size names to their dense-theme constant values.
// Names not present here fall through to the enhanced theme.
var denseSizeValues = map[bellui.ThemeSizeName]float32{
	// Compact spacing
	SizeNamePadding:          2,
	SizeNameInnerPadding:     4,
	SizeNameLineSpacing:      2,
	SizeNameTabContentMargin: 6,

	// Smaller text
	SizeNameText:           12,
	SizeNameHeadingText:    18,
	SizeNameSubHeadingText: 14,
	SizeNameCaptionText:    10,

	// Smaller icons
	SizeNameInlineIcon: 16,

	// Tighter inputs
	SizeNameInputBorder:     1,
	SizeNameInputRadius:     4,
	SizeNameSelectionRadius: 3,
	SizeNameScrollBarRadius: 2,

	// Compact scrollbars
	SizeNameScrollBar:         8,
	SizeNameScrollBarMinThumb: 24,
	SizeNameScrollBarSmall:    2,

	// Thinner separator
	SizeNameSeparatorThickness: 1,

	// Fork-specific
	SizeNameButtonRadius: 4,
	SizeNameCardRadius:   4,
	SizeNameShadowDepth:  0,

	// MD3 Typography (scaled down ~85%)
	SizeNameDisplayLarge:   48,
	SizeNameDisplayMedium:  38,
	SizeNameDisplaySmall:   30,
	SizeNameHeadlineLarge:  27,
	SizeNameHeadlineMedium: 24,
	SizeNameHeadlineSmall:  20,
	SizeNameTitleLarge:     18,
	SizeNameTitleMedium:    14,
	SizeNameTitleSmall:     12,
	SizeNameBodyLarge:      14,
	SizeNameBodyMedium:     12,
	SizeNameBodySmall:      10,
	SizeNameLabelLarge:     12,
	SizeNameLabelMedium:    10,
	SizeNameLabelSmall:     9,

	// MD3 Component Dimensions (~85% of enhanced)
	SizeNameIconButtonSize:   34,
	SizeNameIconSize:         20,
	SizeNameFABSize:          48,
	SizeNameChipHeight:       28,
	SizeNameButtonMinHeight:  34,
	SizeNameNavBarHeight:     64,
	SizeNameNavRailWidth:     64,
	SizeNameTopBarHeight:     52,
	SizeNameBottomBarHeight:  64,
	SizeNameSwitchTrackWidth: 36,
}

func (t *denseTheme) Size(s bellui.ThemeSizeName) float32 {
	if v, ok := denseSizeValues[s]; ok {
		return v
	}
	return t.enhanced.Size(s)
}

func (t *denseTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	return t.enhanced.Color(n, v)
}
