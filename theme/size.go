package theme

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

const (
	// SizeNameCaptionText is the name of theme lookup for helper text size, normally smaller than regular text size.
	//
	// Since: 1.0
	SizeNameCaptionText bellui.ThemeSizeName = "helperText"

	// SizeNameInlineIcon is the name of theme lookup for inline icons size.
	//
	// Since: 1.0
	SizeNameInlineIcon bellui.ThemeSizeName = "iconInline"

	// SizeNameInnerPadding is the name of theme lookup for internal widget padding size.
	//
	// Since: 1.0
	SizeNameInnerPadding bellui.ThemeSizeName = "innerPadding"

	// SizeNameLineSpacing is the name of theme lookup for between text line spacing.
	//
	// Since: 1.0
	SizeNameLineSpacing bellui.ThemeSizeName = "lineSpacing"

	// SizeNamePadding is the name of theme lookup for padding size.
	//
	// Since: 1.0
	SizeNamePadding bellui.ThemeSizeName = "padding"

	// SizeNameScrollBar is the name of theme lookup for the scrollbar size.
	//
	// Since: 1.0
	SizeNameScrollBar bellui.ThemeSizeName = "scrollBar"

	// SizeNameScrollBarMinThumb is the name of theme lookup for the minimum
	// scroll bar thumb length along its axis. Platform scrollbars keep the
	// thumb grabbable no matter how long the content is; without a floor a
	// very long document's proportional thumb shrinks to a few pixels.
	//
	// Since: 1.0
	SizeNameScrollBarMinThumb bellui.ThemeSizeName = "scrollBarMinThumb"

	// SizeNameScrollBarSmall is the name of theme lookup for the shrunk scrollbar size.
	//
	// Since: 1.0
	SizeNameScrollBarSmall bellui.ThemeSizeName = "scrollBarSmall"

	// SizeNameSeparatorThickness is the name of theme lookup for the thickness of a separator.
	//
	// Since: 1.0
	SizeNameSeparatorThickness bellui.ThemeSizeName = "separator"

	// SizeNameText is the name of theme lookup for text size.
	//
	// Since: 1.0
	SizeNameText bellui.ThemeSizeName = "text"

	// SizeNameHeadingText is the name of theme lookup for text size of a heading.
	//
	// Since: 1.0
	SizeNameHeadingText bellui.ThemeSizeName = "headingText"

	// SizeNameSubHeadingText is the name of theme lookup for text size of a sub-heading.
	//
	// Since: 1.0
	SizeNameSubHeadingText bellui.ThemeSizeName = "subHeadingText"

	// SizeNameInputBorder is the name of theme lookup for input border size.
	//
	// Since: 1.0
	SizeNameInputBorder bellui.ThemeSizeName = "inputBorder"

	// SizeNameInputRadius is the name of theme lookup for input corner radius.
	//
	// Since: 1.0
	SizeNameInputRadius bellui.ThemeSizeName = "inputRadius"

	// SizeNameSelectionRadius is the name of theme lookup for selection corner radius.
	//
	// Since: 1.0
	SizeNameSelectionRadius bellui.ThemeSizeName = "selectionRadius"

	// SizeNameScrollBarRadius is the name of theme lookup for the scroll bar corner radius.
	//
	// Since: 1.0
	SizeNameScrollBarRadius bellui.ThemeSizeName = "scrollBarRadius"

	// SizeNameWindowButtonHeight is the name of the height for an inner window titleBar button.
	//
	// Since: 1.0
	SizeNameWindowButtonHeight bellui.ThemeSizeName = "windowButtonHeight"

	// SizeNameWindowButtonRadius is the name of the radius for an inner window titleBar button.
	//
	// Since: 1.0
	SizeNameWindowButtonRadius bellui.ThemeSizeName = "windowButtonRadius"

	// SizeNameWindowButtonIcon is the name of the width of an inner window titleBar button.
	//
	// Since: 1.0
	SizeNameWindowButtonIcon bellui.ThemeSizeName = "windowButtonIcon"

	// SizeNameWindowTitleBarHeight is the height for inner window titleBars.
	//
	// Since: 1.0
	SizeNameWindowTitleBarHeight bellui.ThemeSizeName = "windowTitleBarHeight"

	// SizeNameTabContentMargin is the gap between a tab bar and the content
	// area of the selected tab in AppTabs and DocTabs.
	//
	// Since: 1.0
	SizeNameTabContentMargin bellui.ThemeSizeName = "tabContentMargin"

	// SizeNameShadowDepth is a multiplier for shadow depth. The default builtin theme
	// returns 0, which means use the standard depth of float32(elevationLevel).
	// A value of 4 means shadows are 4x deeper than the elevation level.
	SizeNameShadowDepth bellui.ThemeSizeName = "shadowDepth"

	// SizeNameButtonRadius is the corner radius for buttons. If 0, falls back to SizeNameInputRadius.
	SizeNameButtonRadius bellui.ThemeSizeName = "buttonRadius"

	// SizeNameCardRadius is the corner radius for cards. If 0, cards have no corner rounding.
	SizeNameCardRadius bellui.ThemeSizeName = "cardRadius"

	// --- MD3 Typography Scale ---

	SizeNameDisplayLarge  bellui.ThemeSizeName = "displayLarge"  // 57px
	SizeNameDisplayMedium bellui.ThemeSizeName = "displayMedium" // 45px
	SizeNameDisplaySmall  bellui.ThemeSizeName = "displaySmall"  // 36px

	SizeNameHeadlineLarge  bellui.ThemeSizeName = "headlineLarge"  // 32px
	SizeNameHeadlineMedium bellui.ThemeSizeName = "headlineMedium" // 28px
	SizeNameHeadlineSmall  bellui.ThemeSizeName = "headlineSmall"  // 24px

	SizeNameTitleLarge  bellui.ThemeSizeName = "titleLarge"  // 22px
	SizeNameTitleMedium bellui.ThemeSizeName = "titleMedium" // 16px
	SizeNameTitleSmall  bellui.ThemeSizeName = "titleSmall"  // 14px

	SizeNameBodyLarge  bellui.ThemeSizeName = "bodyLarge"  // 16px
	SizeNameBodyMedium bellui.ThemeSizeName = "bodyMedium" // 14px
	SizeNameBodySmall  bellui.ThemeSizeName = "bodySmall"  // 12px

	SizeNameLabelLarge  bellui.ThemeSizeName = "labelLarge"  // 14px
	SizeNameLabelMedium bellui.ThemeSizeName = "labelMedium" // 12px
	SizeNameLabelSmall  bellui.ThemeSizeName = "labelSmall"  // 11px

	// --- MD3 Component Dimensions ---

	// SizeNameIconButtonSize is the touch target size for icon-only buttons and action buttons in bars.
	SizeNameIconButtonSize bellui.ThemeSizeName = "iconButtonSize" // 40dp

	// SizeNameIconSize is the standard MD3 icon content size used in FABs, chips, nav items, icon buttons.
	SizeNameIconSize bellui.ThemeSizeName = "iconSize" // 24dp

	// SizeNameFABSize is the standard Floating Action Button outer dimension.
	SizeNameFABSize bellui.ThemeSizeName = "fabSize" // 56dp

	// SizeNameChipHeight is the height of chip components.
	SizeNameChipHeight bellui.ThemeSizeName = "chipHeight" // 32dp

	// SizeNameButtonMinHeight is the minimum height for MD3-style buttons and segmented button segments.
	SizeNameButtonMinHeight bellui.ThemeSizeName = "buttonMinHeight" // 40dp

	// SizeNameNavBarHeight is the height of the bottom navigation bar.
	SizeNameNavBarHeight bellui.ThemeSizeName = "navBarHeight" // 80dp

	// SizeNameNavRailWidth is the width of the side navigation rail.
	SizeNameNavRailWidth bellui.ThemeSizeName = "navRailWidth" // 80dp

	// SizeNameTopBarHeight is the height of the top app bar.
	SizeNameTopBarHeight bellui.ThemeSizeName = "topBarHeight" // 64dp

	// SizeNameBottomBarHeight is the height of the bottom app bar.
	SizeNameBottomBarHeight bellui.ThemeSizeName = "bottomBarHeight" // 80dp

	// SizeNameSwitchTrackWidth is the switch track width. Height and thumb are derived proportionally.
	SizeNameSwitchTrackWidth bellui.ThemeSizeName = "switchTrackWidth" // 52dp

	// --- MD3 Shape Scale ---

	SizeNameShapeNone       bellui.ThemeSizeName = "shapeNone"       // 0dp
	SizeNameShapeExtraSmall bellui.ThemeSizeName = "shapeExtraSmall" // 4dp
	SizeNameShapeSmall      bellui.ThemeSizeName = "shapeSmall"      // 8dp
	SizeNameShapeMedium     bellui.ThemeSizeName = "shapeMedium"     // 12dp
	SizeNameShapeLarge      bellui.ThemeSizeName = "shapeLarge"      // 16dp
	SizeNameShapeExtraLarge bellui.ThemeSizeName = "shapeExtraLarge" // 28dp
	SizeNameShapeFull       bellui.ThemeSizeName = "shapeFull"       // 9999dp (pill)
)

// CaptionTextSize returns the size for caption text.
func CaptionTextSize() float32 {
	return Current().Size(SizeNameCaptionText)
}

// IconInlineSize is the standard size of icons which appear within buttons, labels etc.
func IconInlineSize() float32 {
	return Current().Size(SizeNameInlineIcon)
}

// InnerPadding is the standard gap between element content and the outside edge of a widget.
//
// Since: 1.0
func InnerPadding() float32 {
	return Current().Size(SizeNameInnerPadding)
}

// InputBorderSize returns the input border size (or underline size for an entry).
//
// Since: 1.0
func InputBorderSize() float32 {
	return Current().Size(SizeNameInputBorder)
}

// InputRadiusSize returns the input radius size.
//
// Since: 1.0
func InputRadiusSize() float32 {
	return Current().Size(SizeNameInputRadius)
}

// LineSpacing is the default gap between multiple lines of text.
//
// Since: 1.0
func LineSpacing() float32 {
	return Current().Size(SizeNameLineSpacing)
}

// Padding is the standard gap between elements and the border around interface elements.
func Padding() float32 {
	return Current().Size(SizeNamePadding)
}

// ScrollBarSize is the width (or height) of the bars on a ScrollContainer.
func ScrollBarSize() float32 {
	return Current().Size(SizeNameScrollBar)
}

// ScrollBarSmallSize is the width (or height) of the minimized bars on a ScrollContainer.
func ScrollBarSmallSize() float32 {
	return Current().Size(SizeNameScrollBarSmall)
}

// ScrollBarMinThumbSize is the minimum length of a scroll bar thumb along its
// axis, keeping it grabbable on very long content.
//
// Since: 1.0
func ScrollBarMinThumbSize() float32 {
	return Current().Size(SizeNameScrollBarMinThumb)
}

// SelectionRadiusSize returns the selection highlight radius size.
//
// Since: 1.0
func SelectionRadiusSize() float32 {
	return Current().Size(SizeNameSelectionRadius)
}

// SeparatorThicknessSize is the standard thickness of the separator widget.
//
// Since: 1.0
func SeparatorThicknessSize() float32 {
	return Current().Size(SizeNameSeparatorThickness)
}

// Size looks up the specified size for current theme.
//
// Since: 1.0
func Size(name bellui.ThemeSizeName) float32 {
	return Current().Size(name)
}

// SizeForWidget looks up the specified size for the requested widget using the current theme.
// If the widget theme has been overridden that theme will be used.
//
// Since: 1.0
func SizeForWidget(name bellui.ThemeSizeName, w core.Widget) float32 {
	return CurrentForWidget(w).Size(name)
}

// TextHeadingSize returns the text size for header text.
//
// Since: 1.0
func TextHeadingSize() float32 {
	return Current().Size(SizeNameHeadingText)
}

// TextSize returns the standard text size.
func TextSize() float32 {
	return Current().Size(SizeNameText)
}

// TextSubHeadingSize returns the text size for sub-header text.
//
// Since: 1.0
func TextSubHeadingSize() float32 {
	return Current().Size(SizeNameSubHeadingText)
}

// builtinThemeSizes maps each known theme size name to its literal value.
var builtinThemeSizes = map[bellui.ThemeSizeName]float32{
	SizeNameSeparatorThickness:   1,
	SizeNameInlineIcon:           20,
	SizeNameInnerPadding:         8,
	SizeNameLineSpacing:          4,
	SizeNamePadding:              4,
	SizeNameScrollBar:            12,
	SizeNameScrollBarMinThumb:    36,
	SizeNameScrollBarSmall:       3,
	SizeNameText:                 14,
	SizeNameHeadingText:          24,
	SizeNameSubHeadingText:       18,
	SizeNameCaptionText:          11,
	SizeNameInputBorder:          1,
	SizeNameInputRadius:          5,
	SizeNameSelectionRadius:      3,
	SizeNameScrollBarRadius:      3,
	SizeNameWindowButtonHeight:   16,
	SizeNameWindowButtonRadius:   8,
	SizeNameWindowButtonIcon:     14,
	SizeNameWindowTitleBarHeight: 26,
	SizeNameTabContentMargin:     12,

	// MD3 Component Dimensions (defaults for any theme).
	SizeNameIconButtonSize:   40,
	SizeNameIconSize:         24,
	SizeNameFABSize:          56,
	SizeNameChipHeight:       32,
	SizeNameButtonMinHeight:  40,
	SizeNameNavBarHeight:     80,
	SizeNameNavRailWidth:     80,
	SizeNameTopBarHeight:     64,
	SizeNameBottomBarHeight:  80,
	SizeNameSwitchTrackWidth: 52,
}

func (t *builtinTheme) Size(s bellui.ThemeSizeName) float32 {
	if v, ok := builtinThemeSizes[s]; ok {
		return v
	}

	warnUnknownSize(s)
	return 0
}
