package theme

import (
	"image/color"
	"testing"

	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/stretchr/testify/assert"
)

func Test_DenseTheme_Constructor(t *testing.T) {
	th := DenseTheme()
	_, ok := th.(*denseTheme)
	assert.True(t, ok, "DenseTheme() should return a denseTheme")
}

func Test_DenseTheme_RenderStyle(t *testing.T) {
	th := DenseTheme()
	rs, ok := th.(internaltheme.RenderStyleTheme)
	assert.True(t, ok, "denseTheme should implement RenderStyleTheme")

	assert.Equal(t, internaltheme.RenderStyleModern, rs.RenderStyle())
}

func Test_DenseTheme_Size_Overrides(t *testing.T) {
	th := DenseTheme()

	assert.Equal(t, float32(12), th.Size(SizeNameText))
	assert.Equal(t, float32(18), th.Size(SizeNameHeadingText))
	assert.Equal(t, float32(14), th.Size(SizeNameSubHeadingText))
	assert.Equal(t, float32(10), th.Size(SizeNameCaptionText))
	assert.Equal(t, float32(2), th.Size(SizeNamePadding))
	assert.Equal(t, float32(4), th.Size(SizeNameInnerPadding))
	assert.Equal(t, float32(2), th.Size(SizeNameLineSpacing))
	assert.Equal(t, float32(16), th.Size(SizeNameInlineIcon))
	assert.Equal(t, float32(4), th.Size(SizeNameInputRadius))
	assert.Equal(t, float32(3), th.Size(SizeNameSelectionRadius))
	assert.Equal(t, float32(2), th.Size(SizeNameScrollBarRadius))
	assert.Equal(t, float32(8), th.Size(SizeNameScrollBar))
	assert.Equal(t, float32(2), th.Size(SizeNameScrollBarSmall))
	assert.Equal(t, float32(4), th.Size(SizeNameButtonRadius))
	assert.Equal(t, float32(4), th.Size(SizeNameCardRadius))
	assert.Equal(t, float32(0), th.Size(SizeNameShadowDepth))
}

func Test_DenseTheme_MD3_Typography(t *testing.T) {
	th := DenseTheme()

	// Dense typography should be smaller than enhanced
	assert.Equal(t, float32(48), th.Size(SizeNameDisplayLarge))
	assert.Equal(t, float32(12), th.Size(SizeNameBodyMedium))
	assert.Equal(t, float32(9), th.Size(SizeNameLabelSmall))
}

func Test_DenseTheme_Color_MD3(t *testing.T) {
	th := DenseTheme()

	// Dense should use MD3 colors from enhanced theme
	bg := th.Color(ColorNameBackground, VariantDark)
	assert.NotNil(t, bg)
	assert.NotEqual(t, color.Transparent, bg)

	primary := th.Color(ColorNamePrimary, VariantDark)
	assert.NotEqual(t, color.Transparent, primary)

	// MD3 roles should work
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameSecondary, VariantDark))
	assert.NotEqual(t, color.Transparent, th.Color(ColorNameSurface, VariantDark))
}

func Test_DenseTheme_SmallerThanEnhanced(t *testing.T) {
	dense := DenseTheme()
	enhanced := DefaultTheme()

	assert.Less(t, dense.Size(SizeNameText), enhanced.Size(SizeNameText))
	assert.Less(t, dense.Size(SizeNamePadding), enhanced.Size(SizeNamePadding))
	assert.Less(t, dense.Size(SizeNameInnerPadding), enhanced.Size(SizeNameInnerPadding))
	assert.Less(t, dense.Size(SizeNameInlineIcon), enhanced.Size(SizeNameInlineIcon))
	assert.Less(t, dense.Size(SizeNameInputRadius), enhanced.Size(SizeNameInputRadius))
}
