package theme

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/stretchr/testify/assert"
)

func Test_EnhancedTheme_IsDefault(t *testing.T) {
	th := DefaultTheme()
	_, ok := th.(*enhancedTheme)
	assert.True(t, ok, "DefaultTheme() should return an enhancedTheme")
}

func Test_EnhancedTheme_RenderStyle(t *testing.T) {
	th := DefaultTheme()
	rs, ok := th.(internaltheme.RenderStyleTheme)
	assert.True(t, ok, "enhancedTheme should implement RenderStyleTheme")

	assert.Equal(t, internaltheme.RenderStyleModern, rs.RenderStyle())
}

func Test_EnhancedTheme_Size_Overrides(t *testing.T) {
	th := DefaultTheme()

	assert.Equal(t, float32(8), th.Size(SizeNameInputRadius))
	assert.Equal(t, float32(6), th.Size(SizeNameSelectionRadius))
	assert.Equal(t, float32(10), th.Size(SizeNameInnerPadding))
	assert.Equal(t, float32(6), th.Size(SizeNamePadding))
	assert.Equal(t, float32(2), th.Size(SizeNameShadowDepth))
	assert.Equal(t, float32(8), th.Size(SizeNameButtonRadius))
	assert.Equal(t, float32(10), th.Size(SizeNameCardRadius))
}

func Test_EnhancedTheme_Size_Delegates(t *testing.T) {
	th := DefaultTheme()
	assert.Equal(t, float32(14), th.Size(SizeNameText))
	assert.Equal(t, float32(20), th.Size(SizeNameInlineIcon))
}

func Test_EnhancedTheme_MD3_Typography(t *testing.T) {
	th := DefaultTheme()

	assert.Equal(t, float32(57), th.Size(SizeNameDisplayLarge))
	assert.Equal(t, float32(45), th.Size(SizeNameDisplayMedium))
	assert.Equal(t, float32(36), th.Size(SizeNameDisplaySmall))
	assert.Equal(t, float32(32), th.Size(SizeNameHeadlineLarge))
	assert.Equal(t, float32(28), th.Size(SizeNameHeadlineMedium))
	assert.Equal(t, float32(24), th.Size(SizeNameHeadlineSmall))
	assert.Equal(t, float32(22), th.Size(SizeNameTitleLarge))
	assert.Equal(t, float32(16), th.Size(SizeNameTitleMedium))
	assert.Equal(t, float32(14), th.Size(SizeNameTitleSmall))
	assert.Equal(t, float32(16), th.Size(SizeNameBodyLarge))
	assert.Equal(t, float32(14), th.Size(SizeNameBodyMedium))
	assert.Equal(t, float32(12), th.Size(SizeNameBodySmall))
	assert.Equal(t, float32(14), th.Size(SizeNameLabelLarge))
	assert.Equal(t, float32(12), th.Size(SizeNameLabelMedium))
	assert.Equal(t, float32(11), th.Size(SizeNameLabelSmall))
}

func Test_EnhancedTheme_MD3_Colors_Dark(t *testing.T) {
	th := DefaultTheme()

	// Background should map to MD3 Surface (non-transparent, dark tone)
	bg := th.Color(ColorNameBackground, VariantDark)
	assert.NotNil(t, bg)
	assert.NotEqual(t, color.Transparent, bg)

	// Primary should be non-transparent
	primary := th.Color(ColorNamePrimary, VariantDark)
	assert.NotEqual(t, color.Transparent, primary)

	// New MD3 roles should return non-transparent
	assert.NotEqual(t, color.Transparent, th.Color(ColorNamePrimaryContainer, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameSecondary, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameTertiary, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameSurface, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameOnSurface, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameOutline, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameOutlineVariant, VariantDark))
}

func Test_EnhancedTheme_MD3_Colors_Light(t *testing.T) {
	th := DefaultTheme()

	bg := th.Color(ColorNameBackground, VariantLight)
	assert.NotNil(t, bg)
	assert.NotEqual(t, color.Transparent, bg)

	assert.NotEqual(t, color.Transparent, th.Color(ColorNamePrimaryContainer, VariantLight))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameSecondary, VariantLight))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameSurface, VariantLight))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameOnSurface, VariantLight))
}

func Test_EnhancedTheme_MD3_DarkLightDifference(t *testing.T) {
	th := DefaultTheme()

	darkBg := th.Color(ColorNameBackground, VariantDark)
	lightBg := th.Color(ColorNameBackground, VariantLight)
	assert.NotEqual(t, darkBg, lightBg, "dark and light backgrounds should differ")

	darkPrimary := th.Color(ColorNamePrimary, VariantDark)
	lightPrimary := th.Color(ColorNamePrimary, VariantLight)
	assert.NotEqual(t, darkPrimary, lightPrimary, "dark and light primary should differ")
}

func Test_EnhancedTheme_LegacyColorsMapped(t *testing.T) {
	th := DefaultTheme()

	// All legacy color names should return non-transparent values
	legacyColors := []bellui.ThemeColorName{
		ColorNameBackground, ColorNameForeground, ColorNameButton,
		ColorNameDisabledButton, ColorNameDisabled, ColorNameError,
		ColorNameForegroundOnError, ColorNameInputBackground,
		ColorNameInputBorder, ColorNameMenuBackground,
		ColorNameOverlayBackground, ColorNameSeparator,
		ColorNameShadow, ColorNameHover, ColorNamePressed,
		ColorNamePlaceHolder, ColorNameScrollBar,
	}
	for _, name := range legacyColors {
		c := th.Color(name, VariantDark)
		assert.NotNil(t, c, "color %s should not be nil", name)
		assert.NotEqual(t, color.Transparent, c, "color %s should not be transparent in dark", name)

		c = th.Color(name, VariantLight)
		assert.NotNil(t, c, "color %s should not be nil", name)
		assert.NotEqual(t, color.Transparent, c, "color %s should not be transparent in light", name)
	}
}

func Test_BuiltinTheme_BackwardCompat(t *testing.T) {
	dark := DarkTheme()
	_, ok := dark.(*builtinTheme)
	assert.True(t, ok, "DarkTheme() should return builtinTheme")

	light := LightTheme()
	_, ok = light.(*builtinTheme)
	assert.True(t, ok, "LightTheme() should return builtinTheme")

	// builtinTheme returns 0 for new size constants.
	assert.Equal(t, float32(0), dark.Size(SizeNameShadowDepth))
	assert.Equal(t, float32(0), dark.Size(SizeNameButtonRadius))

	// builtinTheme returns transparent for new color constants.
	assert.Equal(t, color.Transparent, dark.Color(ColorNameCardBackground, VariantDark))
	assert.Equal(t, color.Transparent, dark.Color(ColorNameSecondary, VariantDark))
}
