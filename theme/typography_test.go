package theme

import (
	"testing"

	"github.com/belluikit/bellui"

	"github.com/stretchr/testify/assert"
)

func TestTypography_DisplayWeights(t *testing.T) {
	assert.Equal(t, bellui.WeightNormal, TypographyDisplayLarge.ResolvedWeight())
	assert.Equal(t, bellui.WeightNormal, TypographyDisplayMedium.ResolvedWeight())
	assert.Equal(t, bellui.WeightNormal, TypographyDisplaySmall.ResolvedWeight())
}

func TestTypography_HeadlineWeights(t *testing.T) {
	assert.Equal(t, bellui.WeightNormal, TypographyHeadlineLarge.ResolvedWeight())
	assert.Equal(t, bellui.WeightNormal, TypographyHeadlineMedium.ResolvedWeight())
	assert.Equal(t, bellui.WeightNormal, TypographyHeadlineSmall.ResolvedWeight())
}

func TestTypography_TitleWeights(t *testing.T) {
	assert.Equal(t, bellui.WeightNormal, TypographyTitleLarge.ResolvedWeight())
	assert.Equal(t, bellui.WeightMedium, TypographyTitleMedium.ResolvedWeight())
	assert.Equal(t, bellui.WeightMedium, TypographyTitleSmall.ResolvedWeight())
}

func TestTypography_BodyWeights(t *testing.T) {
	assert.Equal(t, bellui.WeightNormal, TypographyBodyLarge.ResolvedWeight())
	assert.Equal(t, bellui.WeightNormal, TypographyBodyMedium.ResolvedWeight())
	assert.Equal(t, bellui.WeightNormal, TypographyBodySmall.ResolvedWeight())
}

func TestTypography_LabelWeights(t *testing.T) {
	assert.Equal(t, bellui.WeightMedium, TypographyLabelLarge.ResolvedWeight())
	assert.Equal(t, bellui.WeightMedium, TypographyLabelMedium.ResolvedWeight())
	assert.Equal(t, bellui.WeightMedium, TypographyLabelSmall.ResolvedWeight())
}

func TestTypography_LineHeightScalePositive(t *testing.T) {
	presets := []bellui.TextStyle{
		TypographyDisplayLarge, TypographyDisplayMedium, TypographyDisplaySmall,
		TypographyHeadlineLarge, TypographyHeadlineMedium, TypographyHeadlineSmall,
		TypographyTitleLarge, TypographyTitleMedium, TypographyTitleSmall,
		TypographyBodyLarge, TypographyBodyMedium, TypographyBodySmall,
		TypographyLabelLarge, TypographyLabelMedium, TypographyLabelSmall,
	}
	for i, p := range presets {
		assert.Greater(t, p.ResolvedLineHeightScale(), float32(1.0),
			"preset %d should have LineHeightScale > 1.0", i)
	}
}
