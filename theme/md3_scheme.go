package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
)

// MD3Scheme holds all Material Design 3 color roles for a single variant (dark or light).
type MD3Scheme struct {
	Primary, OnPrimary, PrimaryContainer, OnPrimaryContainer         color.NRGBA
	Secondary, OnSecondary, SecondaryContainer, OnSecondaryContainer color.NRGBA
	Tertiary, OnTertiary, TertiaryContainer, OnTertiaryContainer     color.NRGBA
	Error, OnError, ErrorContainer, OnErrorContainer                 color.NRGBA

	Surface, SurfaceDim, SurfaceBright                            color.NRGBA
	SurfaceContainerLowest, SurfaceContainerLow, SurfaceContainer color.NRGBA
	SurfaceContainerHigh, SurfaceContainerHighest                 color.NRGBA
	OnSurface, OnSurfaceVariant                                   color.NRGBA
	Outline, OutlineVariant                                       color.NRGBA
	InverseSurface, InverseOnSurface, InversePrimary              color.NRGBA
	Shadow, Scrim                                                 color.NRGBA
}

// toneMap holds the tone values that differ between dark and light MD3 schemes.
type toneMap struct {
	role, onRole, container, onContainer int // shared pattern for primary/secondary/tertiary/error

	surface, surfaceDim, surfaceBright                            int
	surfaceContainerLowest, surfaceContainerLow, surfaceContainer int
	surfaceContainerHigh, surfaceContainerHighest                 int

	onSurface, onSurfaceVariant int
	outline, outlineVariant     int

	inverseSurface, inverseOnSurface, inversePrimary int
}

var darkTones = toneMap{
	role: 80, onRole: 20, container: 30, onContainer: 90,

	surface: 6, surfaceDim: 6, surfaceBright: 24,
	surfaceContainerLowest: 4, surfaceContainerLow: 10, surfaceContainer: 12,
	surfaceContainerHigh: 17, surfaceContainerHighest: 22,

	onSurface: 90, onSurfaceVariant: 80,
	outline: 60, outlineVariant: 30,

	inverseSurface: 90, inverseOnSurface: 20, inversePrimary: 40,
}

var lightTones = toneMap{
	role: 40, onRole: 100, container: 90, onContainer: 10,

	surface: 98, surfaceDim: 87, surfaceBright: 98,
	surfaceContainerLowest: 100, surfaceContainerLow: 96, surfaceContainer: 94,
	surfaceContainerHigh: 92, surfaceContainerHighest: 90,

	onSurface: 10, onSurfaceVariant: 30,
	outline: 50, outlineVariant: 80,

	inverseSurface: 20, inverseOnSurface: 95, inversePrimary: 80,
}

func schemeFromTones(p *CorePalettes, t toneMap) *MD3Scheme {
	return &MD3Scheme{
		Primary:            p.Primary.Tone(t.role),
		OnPrimary:          p.Primary.Tone(t.onRole),
		PrimaryContainer:   p.Primary.Tone(t.container),
		OnPrimaryContainer: p.Primary.Tone(t.onContainer),

		Secondary:            p.Secondary.Tone(t.role),
		OnSecondary:          p.Secondary.Tone(t.onRole),
		SecondaryContainer:   p.Secondary.Tone(t.container),
		OnSecondaryContainer: p.Secondary.Tone(t.onContainer),

		Tertiary:            p.Tertiary.Tone(t.role),
		OnTertiary:          p.Tertiary.Tone(t.onRole),
		TertiaryContainer:   p.Tertiary.Tone(t.container),
		OnTertiaryContainer: p.Tertiary.Tone(t.onContainer),

		Error:            p.Error.Tone(t.role),
		OnError:          p.Error.Tone(t.onRole),
		ErrorContainer:   p.Error.Tone(t.container),
		OnErrorContainer: p.Error.Tone(t.onContainer),

		Surface:                 p.Neutral.Tone(t.surface),
		SurfaceDim:              p.Neutral.Tone(t.surfaceDim),
		SurfaceBright:           p.Neutral.Tone(t.surfaceBright),
		SurfaceContainerLowest:  p.Neutral.Tone(t.surfaceContainerLowest),
		SurfaceContainerLow:     p.Neutral.Tone(t.surfaceContainerLow),
		SurfaceContainer:        p.Neutral.Tone(t.surfaceContainer),
		SurfaceContainerHigh:    p.Neutral.Tone(t.surfaceContainerHigh),
		SurfaceContainerHighest: p.Neutral.Tone(t.surfaceContainerHighest),

		OnSurface:        p.Neutral.Tone(t.onSurface),
		OnSurfaceVariant: p.NeutralVariant.Tone(t.onSurfaceVariant),
		Outline:          p.NeutralVariant.Tone(t.outline),
		OutlineVariant:   p.NeutralVariant.Tone(t.outlineVariant),

		InverseSurface:   p.Neutral.Tone(t.inverseSurface),
		InverseOnSurface: p.Neutral.Tone(t.inverseOnSurface),
		InversePrimary:   p.Primary.Tone(t.inversePrimary),

		Shadow: color.NRGBA{R: 0, G: 0, B: 0, A: 0xff},
		Scrim:  color.NRGBA{R: 0, G: 0, B: 0, A: 0x80},
	}
}

// DarkSchemeFromPalettes generates an MD3 dark color scheme from core palettes.
func DarkSchemeFromPalettes(p *CorePalettes) *MD3Scheme {
	return schemeFromTones(p, darkTones)
}

// LightSchemeFromPalettes generates an MD3 light color scheme from core palettes.
func LightSchemeFromPalettes(p *CorePalettes) *MD3Scheme {
	return schemeFromTones(p, lightTones)
}

// md3SchemeColor returns the color for an MD3-specific color name from the scheme.
// Returns nil if the name is not an MD3-specific role.
func md3SchemeColor(s *MD3Scheme, name bellui.ThemeColorName) *color.NRGBA {
	switch name {
	case ColorNamePrimaryContainer:
		return &s.PrimaryContainer
	case ColorNameOnPrimaryContainer:
		return &s.OnPrimaryContainer
	case ColorNameSecondary:
		return &s.Secondary
	case ColorNameOnSecondary:
		return &s.OnSecondary
	case ColorNameSecondaryContainer:
		return &s.SecondaryContainer
	case ColorNameOnSecondaryContainer:
		return &s.OnSecondaryContainer
	case ColorNameTertiary:
		return &s.Tertiary
	case ColorNameOnTertiary:
		return &s.OnTertiary
	case ColorNameTertiaryContainer:
		return &s.TertiaryContainer
	case ColorNameOnTertiaryContainer:
		return &s.OnTertiaryContainer
	case ColorNameErrorContainer:
		return &s.ErrorContainer
	case ColorNameOnErrorContainer:
		return &s.OnErrorContainer
	case ColorNameSurface:
		return &s.Surface
	case ColorNameSurfaceDim:
		return &s.SurfaceDim
	case ColorNameSurfaceBright:
		return &s.SurfaceBright
	case ColorNameSurfaceContainerLowest:
		return &s.SurfaceContainerLowest
	case ColorNameSurfaceContainerLow:
		return &s.SurfaceContainerLow
	case ColorNameSurfaceContainerMedium:
		return &s.SurfaceContainer
	case ColorNameSurfaceContainerHigh:
		return &s.SurfaceContainerHigh
	case ColorNameSurfaceContainerHighest:
		return &s.SurfaceContainerHighest
	case ColorNameOnSurface:
		return &s.OnSurface
	case ColorNameOnSurfaceVariant:
		return &s.OnSurfaceVariant
	case ColorNameOutline:
		return &s.Outline
	case ColorNameOutlineVariant:
		return &s.OutlineVariant
	case ColorNameInverseSurface:
		return &s.InverseSurface
	case ColorNameInverseOnSurface:
		return &s.InverseOnSurface
	case ColorNameInversePrimary:
		return &s.InversePrimary
	case ColorNameScrim:
		return &s.Scrim
	}
	return nil
}
