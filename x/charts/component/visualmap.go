package component

import (
	"image/color"
	"math"
)

// VisualMapConfig maps a data dimension to visual properties (color, size).
type VisualMapConfig struct {
	// Min and Max define the data range. Values outside are clamped.
	Min, Max float64

	// InRange defines the visual mapping for values within [Min, Max].
	InRange VisualMapRange

	// Dimension selects which data dimension to map (default 0 = Y value).
	Dimension int

	// SeriesIndex restricts the mapping to specific series (-1 = all).
	SeriesIndex int
}

// VisualMapRange defines the visual output for mapped values.
type VisualMapRange struct {
	// Color gradient stops. If set, values are mapped to colors along this gradient.
	Colors []color.Color

	// SymbolSize range [min, max]. If set, values are mapped to symbol sizes.
	SymbolSizeMin, SymbolSizeMax float32
}

// MapColor maps a data value to a color using the VisualMap configuration.
func (v *VisualMapConfig) MapColor(value float64) color.Color {
	if len(v.InRange.Colors) == 0 {
		return nil
	}
	norm := v.normalize(value)
	return interpolateGradient(v.InRange.Colors, norm)
}

// MapSize maps a data value to a symbol size using the VisualMap configuration.
func (v *VisualMapConfig) MapSize(value float64) float32 {
	if v.InRange.SymbolSizeMin == 0 && v.InRange.SymbolSizeMax == 0 {
		return 0
	}
	norm := v.normalize(value)
	return v.InRange.SymbolSizeMin + float32(norm)*(v.InRange.SymbolSizeMax-v.InRange.SymbolSizeMin)
}

func (v *VisualMapConfig) normalize(value float64) float64 {
	if v.Max <= v.Min {
		return 0
	}
	norm := (value - v.Min) / (v.Max - v.Min)
	return math.Max(0, math.Min(1, norm))
}

func interpolateGradient(colors []color.Color, norm float64) color.Color {
	if len(colors) == 0 {
		return color.White
	}
	if len(colors) == 1 || norm <= 0 {
		return colors[0]
	}
	if norm >= 1 {
		return colors[len(colors)-1]
	}

	// Map norm to a position between two colors
	pos := norm * float64(len(colors)-1)
	idx := int(pos)
	if idx >= len(colors)-1 {
		return colors[len(colors)-1]
	}
	t := pos - float64(idx)

	r1, g1, b1, a1 := colors[idx].RGBA()
	r2, g2, b2, a2 := colors[idx+1].RGBA()
	return color.NRGBA{
		R: uint8(float64(r1>>8) + t*float64(int(r2>>8)-int(r1>>8))),
		G: uint8(float64(g1>>8) + t*float64(int(g2>>8)-int(g1>>8))),
		B: uint8(float64(b1>>8) + t*float64(int(b2>>8)-int(b1>>8))),
		A: uint8(float64(a1>>8) + t*float64(int(a2>>8)-int(a1>>8))),
	}
}
