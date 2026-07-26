package theme

import "github.com/belluikit/bellui"

// MD3 Typography Scale presets. Each bundles the Weight, LineHeightScale, and
// LetterSpacing for a specific MD3 type token. Use with the corresponding
// SizeName constant for the font size:
//
//	text := canvas.NewText("Hello", color.White)
//	text.TextSize = theme.Size(theme.SizeNameBodyMedium) // 14px
//	text.TextStyle = theme.TypographyBodyMedium
var (
	TypographyDisplayLarge = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(64.0 / 57.0)), LetterSpacing: bellui.Ptr(float32(-0.25)),
	}
	TypographyDisplayMedium = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(52.0 / 45.0)),
	}
	TypographyDisplaySmall = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(44.0 / 36.0)),
	}

	TypographyHeadlineLarge = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(40.0 / 32.0)),
	}
	TypographyHeadlineMedium = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(36.0 / 28.0)),
	}
	TypographyHeadlineSmall = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(32.0 / 24.0)),
	}

	TypographyTitleLarge = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(28.0 / 22.0)),
	}
	TypographyTitleMedium = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightMedium), LineHeightScale: bellui.Ptr(float32(24.0 / 16.0)), LetterSpacing: bellui.Ptr(float32(0.15)),
	}
	TypographyTitleSmall = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightMedium), LineHeightScale: bellui.Ptr(float32(20.0 / 14.0)), LetterSpacing: bellui.Ptr(float32(0.1)),
	}

	TypographyBodyLarge = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(24.0 / 16.0)), LetterSpacing: bellui.Ptr(float32(0.5)),
	}
	TypographyBodyMedium = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(20.0 / 14.0)), LetterSpacing: bellui.Ptr(float32(0.25)),
	}
	TypographyBodySmall = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightNormal), LineHeightScale: bellui.Ptr(float32(16.0 / 12.0)), LetterSpacing: bellui.Ptr(float32(0.4)),
	}

	TypographyLabelLarge = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightMedium), LineHeightScale: bellui.Ptr(float32(20.0 / 14.0)), LetterSpacing: bellui.Ptr(float32(0.1)),
	}
	TypographyLabelMedium = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightMedium), LineHeightScale: bellui.Ptr(float32(16.0 / 12.0)), LetterSpacing: bellui.Ptr(float32(0.5)),
	}
	TypographyLabelSmall = bellui.TextStyle{
		Weight: bellui.Ptr(bellui.WeightMedium), LineHeightScale: bellui.Ptr(float32(16.0 / 11.0)), LetterSpacing: bellui.Ptr(float32(0.5)),
	}
)
