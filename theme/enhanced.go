package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
	internaltheme "github.com/belluikit/bellui/internal/theme"
)

// enhancedTheme wraps the builtinTheme with MD3-compliant colors generated
// from tonal palettes, larger sizes, and the "modern" render style.
// It is the default theme for all apps built with this fork.
type enhancedTheme struct {
	builtin *builtinTheme

	// MD3 palette cache (lazy, regenerated when primary color changes).
	cachedPrimary string
	palettes      *CorePalettes
	darkScheme    *MD3Scheme
	lightScheme   *MD3Scheme
	// Warning palette (not part of MD3, kept for compatibility).
	warningPalette *TonalPalette
}

var (
	_ bellui.Theme                   = (*enhancedTheme)(nil)
	_ internaltheme.RenderStyleTheme = (*enhancedTheme)(nil)
)

// RenderStyle implements internal/theme.RenderStyleTheme.
func (t *enhancedTheme) RenderStyle() internaltheme.RenderStyle {
	return internaltheme.RenderStyleModern
}

func (t *enhancedTheme) Font(style bellui.TextStyle) bellui.Resource {
	return t.builtin.Font(style)
}

func (t *enhancedTheme) Icon(name bellui.ThemeIconName) bellui.Resource {
	return t.builtin.Icon(name)
}

func (t *enhancedTheme) Size(s bellui.ThemeSizeName) float32 {
	switch s {
	// Enhanced spacing overrides
	case SizeNameInputRadius:
		return 8
	case SizeNameSelectionRadius:
		return 6
	case SizeNameInnerPadding:
		return 10
	case SizeNamePadding:
		return 6
	case SizeNameShadowDepth:
		return 2
	case SizeNameButtonRadius:
		return 8
	case SizeNameCardRadius:
		return 10

	// MD3 Typography Scale
	case SizeNameDisplayLarge:
		return 57
	case SizeNameDisplayMedium:
		return 45
	case SizeNameDisplaySmall:
		return 36
	case SizeNameHeadlineLarge:
		return 32
	case SizeNameHeadlineMedium:
		return 28
	case SizeNameHeadlineSmall:
		return 24
	case SizeNameTitleLarge:
		return 22
	case SizeNameTitleMedium:
		return 16
	case SizeNameTitleSmall:
		return 14
	case SizeNameBodyLarge:
		return 16
	case SizeNameBodyMedium:
		return 14
	case SizeNameBodySmall:
		return 12
	case SizeNameLabelLarge:
		return 14
	case SizeNameLabelMedium:
		return 12
	case SizeNameLabelSmall:
		return 11

	// MD3 Shape Scale
	case SizeNameShapeNone:
		return 0
	case SizeNameShapeExtraSmall:
		return 4
	case SizeNameShapeSmall:
		return 8
	case SizeNameShapeMedium:
		return 12
	case SizeNameShapeLarge:
		return 16
	case SizeNameShapeExtraLarge:
		return 28
	case SizeNameShapeFull:
		return 9999
	}
	return t.builtin.Size(s)
}

func (t *enhancedTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if t.builtin.variant != internaltheme.VariantNameUserPreference {
		v = t.builtin.variant
	}

	scheme := t.md3Scheme(v)

	if c := ColorFromMD3Scheme(scheme, n, v); c != nil {
		return c
	}

	// Warning uses a separate palette, not the primary-derived scheme.
	switch n {
	case ColorNameWarning:
		return t.warningColor(v)
	case ColorNameForegroundOnWarning:
		return t.warningOnColor(v)
	}

	return t.builtin.Color(n, v)
}

// ColorFromMD3Scheme resolves a Bellui color name against the given MD3 scheme.
// It handles both MD3-specific names (PrimaryContainer, OnSurface, etc.) and
// legacy Bellui names (Background, Button, Disabled, etc.) mapped to their MD3
// equivalents. Returns nil for names not mapped to the scheme (e.g., Warning
// which uses a separate palette). Callers should fall back to their base theme
// for nil results.
func ColorFromMD3Scheme(s *MD3Scheme, n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	// MD3-specific color names (PrimaryContainer, SecondaryContainer, Surface variants, etc.)
	if c := md3SchemeColor(s, n); c != nil {
		return *c
	}

	// Legacy Bellui color names that map directly to a single scheme field.
	if field := legacyMD3ColorFields[n]; field != nil {
		return field(s)
	}

	// Legacy Bellui color names requiring a transform or variant-dependent logic.
	switch n {
	case ColorNameFocus:
		return withAlpha(s.Primary, 0x4D) // ~30%
	case ColorNameSelection:
		return withAlpha(s.Primary, 0x40) // ~25%
	case ColorNameDisabled:
		return withAlpha(s.OnSurface, 0x61) // 38%
	case ColorNameMenuBackground:
		if v == VariantDark {
			return SurfaceWithElevation(s.SurfaceContainer, s.Primary, 2)
		}
		return s.SurfaceContainer
	case ColorNameOverlayBackground:
		if v == VariantDark {
			return SurfaceWithElevation(s.SurfaceContainerLow, s.Primary, 3)
		}
		return s.SurfaceContainerLow
	case ColorNameCardBackground:
		if v == VariantDark {
			return SurfaceWithElevation(s.SurfaceContainer, s.Primary, 1)
		}
		return s.SurfaceContainer
	case ColorNameElevatedBackground:
		if v == VariantDark {
			return SurfaceWithElevation(s.SurfaceContainerLow, s.Primary, 1)
		}
		return s.SurfaceContainerLow
	case ColorNameShadow:
		return withAlpha(s.Shadow, 0xAA)
	case ColorNameHover:
		return withAlpha(s.OnSurface, 0x14) // 8%
	case ColorNamePressed:
		return withAlpha(s.OnSurface, 0x1F) // 12%
	case ColorNameScrollBar:
		return withAlpha(s.OnSurface, 0x99) // 60%
	}

	return nil
}

// legacyMD3ColorFields maps legacy Bellui color names that resolve directly to a
// single MD3 scheme field (no transform, no variant dependence).
var legacyMD3ColorFields = map[bellui.ThemeColorName]func(*MD3Scheme) color.Color{
	ColorNameBackground:          func(s *MD3Scheme) color.Color { return s.Surface },
	ColorNameForeground:          func(s *MD3Scheme) color.Color { return s.OnSurface },
	ColorNamePrimary:             func(s *MD3Scheme) color.Color { return s.Primary },
	ColorNameHyperlink:           func(s *MD3Scheme) color.Color { return s.Primary },
	ColorNameForegroundOnPrimary: func(s *MD3Scheme) color.Color { return s.OnPrimary },
	ColorNameButton:              func(s *MD3Scheme) color.Color { return s.SurfaceContainerHigh },
	ColorNameDisabledButton:      func(s *MD3Scheme) color.Color { return s.SurfaceContainerLow },
	ColorNameError:               func(s *MD3Scheme) color.Color { return s.Error },
	ColorNameForegroundOnError:   func(s *MD3Scheme) color.Color { return s.OnError },
	ColorNameSuccess:             func(s *MD3Scheme) color.Color { return s.Tertiary },
	ColorNameForegroundOnSuccess: func(s *MD3Scheme) color.Color { return s.OnTertiary },
	ColorNameInputBackground:     func(s *MD3Scheme) color.Color { return s.SurfaceContainerHighest },
	ColorNameInputBorder:         func(s *MD3Scheme) color.Color { return s.Outline },
	ColorNameHeaderBackground:    func(s *MD3Scheme) color.Color { return s.SurfaceContainerLow },
	ColorNameSeparator:           func(s *MD3Scheme) color.Color { return s.OutlineVariant },
	ColorNamePlaceHolder:         func(s *MD3Scheme) color.Color { return s.OnSurfaceVariant },
	ColorNameScrollBarBackground: func(s *MD3Scheme) color.Color { return s.SurfaceContainerHighest },
}

func (t *enhancedTheme) md3Scheme(v bellui.ThemeVariant) *MD3Scheme {
	primary := ""
	if app := bellui.CurrentApp(); app != nil {
		primary = app.Settings().PrimaryColor()
	}
	if primary == "" {
		primary = ColorBlue
	}

	if primary != t.cachedPrimary || t.palettes == nil {
		primaryColor, ok := internaltheme.PrimaryColorNamed(primary).(color.NRGBA)
		if !ok {
			primaryColor = color.NRGBA{R: 0x29, G: 0x6F, B: 0xF6, A: 0xff} // fallback blue
		}
		t.palettes = CorePalettesFromPrimary(primaryColor)
		t.darkScheme = DarkSchemeFromPalettes(t.palettes)
		t.lightScheme = LightSchemeFromPalettes(t.palettes)
		t.warningPalette = NewTonalPalette(40, 80) // orange hue
		t.cachedPrimary = primary
	}

	if v == VariantLight {
		return t.lightScheme
	}
	return t.darkScheme
}

func (t *enhancedTheme) warningColor(v bellui.ThemeVariant) color.NRGBA {
	t.md3Scheme(v) // ensure palette is initialized
	if v == VariantLight {
		return t.warningPalette.Tone(40)
	}
	return t.warningPalette.Tone(80)
}

func (t *enhancedTheme) warningOnColor(v bellui.ThemeVariant) color.NRGBA {
	t.md3Scheme(v)
	if v == VariantLight {
		return t.warningPalette.Tone(100)
	}
	return t.warningPalette.Tone(20)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
}
