// Package style resolves chart visual properties from the Fyne theme.
package style

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// defaultPalette provides a set of visually distinct colors for chart series.
var defaultPalette = []color.Color{
	color.NRGBA{R: 91, G: 143, B: 249, A: 255},  // blue
	color.NRGBA{R: 250, G: 200, B: 88, A: 255},  // amber
	color.NRGBA{R: 116, G: 211, B: 169, A: 255}, // teal
	color.NRGBA{R: 235, G: 96, B: 128, A: 255},  // pink
	color.NRGBA{R: 149, G: 134, B: 244, A: 255}, // purple
	color.NRGBA{R: 246, G: 152, B: 96, A: 255},  // orange
	color.NRGBA{R: 115, G: 192, B: 222, A: 255}, // cyan
	color.NRGBA{R: 59, G: 179, B: 113, A: 255},  // green
}

// Resolver computes visual properties for chart elements from a Fyne theme.
type Resolver struct {
	th      bellui.Theme
	variant bellui.ThemeVariant
	palette []color.Color
}

// NewResolver creates a style Resolver for the given theme and variant.
func NewResolver(th bellui.Theme, variant bellui.ThemeVariant) *Resolver {
	return &Resolver{th: th, variant: variant}
}

// SetPalette overrides the default series color palette.
func (r *Resolver) SetPalette(colors []color.Color) {
	r.palette = colors
}

// SeriesColor returns the color for a series at the given index.
func (r *Resolver) SeriesColor(index int) color.Color {
	pal := r.palette
	if len(pal) == 0 {
		pal = r.themePalette()
	}
	if len(pal) == 0 {
		return color.White
	}
	return pal[index%len(pal)]
}

func (r *Resolver) themePalette() []color.Color {
	primary := r.th.Color(theme.ColorNamePrimary, r.variant)
	if primary == nil {
		return defaultPalette
	}
	// Build palette starting with theme primary, then use defaults for variety
	pal := make([]color.Color, 0, len(defaultPalette)+1)
	pal = append(pal, primary)
	pal = append(pal, defaultPalette...)
	return pal
}

// BackgroundColor returns the chart background color.
func (r *Resolver) BackgroundColor() color.Color {
	return r.th.Color(theme.ColorNameBackground, r.variant)
}

// GridLineColor returns the color for grid/split lines.
func (r *Resolver) GridLineColor() color.Color {
	c := r.th.Color(theme.ColorNameSeparator, r.variant)
	if c == nil {
		return color.NRGBA{R: 200, G: 200, B: 200, A: 50}
	}
	return c
}

// AxisLineColor returns the color for axis lines.
func (r *Resolver) AxisLineColor() color.Color {
	c := r.th.Color(theme.ColorNameForeground, r.variant)
	if c == nil {
		return color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	}
	return c
}

// AxisLabelColor returns the color for axis tick labels.
func (r *Resolver) AxisLabelColor() color.Color {
	c := r.th.Color(theme.ColorNameForeground, r.variant)
	if c == nil {
		return color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	}
	return c
}

// TitleColor returns the color for chart titles.
func (r *Resolver) TitleColor() color.Color {
	return r.th.Color(theme.ColorNameForeground, r.variant)
}

// LabelTextSize returns the text size for axis labels.
func (r *Resolver) LabelTextSize() float32 {
	return r.th.Size(theme.SizeNameCaptionText)
}

// TitleTextSize returns the text size for chart titles.
func (r *Resolver) TitleTextSize() float32 {
	return r.th.Size(theme.SizeNameSubHeadingText)
}

// Padding returns the standard padding used in charts.
func (r *Resolver) Padding() float32 {
	return r.th.Size(theme.SizeNamePadding)
}

// TooltipBackgroundColor returns the background color for tooltips.
func (r *Resolver) TooltipBackgroundColor() color.Color {
	return color.NRGBA{R: 40, G: 40, B: 40, A: 230}
}

// TooltipBorderColor returns the border color for tooltips.
func (r *Resolver) TooltipBorderColor() color.Color {
	return color.NRGBA{R: 80, G: 80, B: 80, A: 200}
}

// TooltipTextColor returns the text color for tooltip content.
func (r *Resolver) TooltipTextColor() color.Color {
	return color.NRGBA{R: 230, G: 230, B: 230, A: 255}
}

// CrosshairColor returns the color for crosshair lines.
func (r *Resolver) CrosshairColor() color.Color {
	return color.NRGBA{R: 150, G: 150, B: 150, A: 100}
}

// TooltipPadding returns the internal padding for tooltips.
func (r *Resolver) TooltipPadding() float32 {
	return 8
}

// TooltipLineHeight returns the line height for tooltip text.
func (r *Resolver) TooltipLineHeight() float32 {
	return 16
}

// TooltipMarkerSize returns the size of the series color marker in tooltips.
func (r *Resolver) TooltipMarkerSize() float32 {
	return 8
}

// OverlayDefaultLineColor returns the default color for reference lines when none is specified.
func (r *Resolver) OverlayDefaultLineColor() color.Color {
	return color.NRGBA{R: 255, G: 100, B: 100, A: 200}
}

// OverlayDefaultBandColor returns the default color for reference bands when none is specified.
func (r *Resolver) OverlayDefaultBandColor() color.Color {
	return color.NRGBA{R: 91, G: 143, B: 249, A: 40}
}

// OverlayLabelColor returns the color for reference band labels.
func (r *Resolver) OverlayLabelColor() color.Color {
	return color.NRGBA{R: 150, G: 150, B: 150, A: 200}
}

// AnnotationBackgroundColor returns the background color for annotation labels.
func (r *Resolver) AnnotationBackgroundColor() color.Color {
	return color.NRGBA{R: 50, G: 50, B: 50, A: 200}
}

// BreadcrumbBackgroundColor returns the background color for breadcrumb buttons.
func (r *Resolver) BreadcrumbBackgroundColor() color.Color {
	return color.NRGBA{R: 70, G: 70, B: 70, A: 220}
}

// BreadcrumbTextColor returns the text color for breadcrumb labels.
func (r *Resolver) BreadcrumbTextColor() color.Color {
	return color.NRGBA{R: 220, G: 220, B: 220, A: 255}
}

// BreadcrumbAccentColor returns the accent color for clickable breadcrumb segments.
func (r *Resolver) BreadcrumbAccentColor() color.Color {
	return color.NRGBA{R: 91, G: 143, B: 249, A: 255}
}

// BreadcrumbSeparatorColor returns the color for breadcrumb separators.
func (r *Resolver) BreadcrumbSeparatorColor() color.Color {
	return color.NRGBA{R: 120, G: 120, B: 120, A: 200}
}

// LegendTextColor returns the color for legend item labels.
func (r *Resolver) LegendTextColor() color.Color {
	return color.NRGBA{R: 200, G: 200, B: 200, A: 255}
}

// LegendDimmedTextColor returns the color for hidden/toggled-off legend items.
func (r *Resolver) LegendDimmedTextColor() color.Color {
	return color.NRGBA{R: 120, G: 120, B: 120, A: 255}
}

// LegendSwatchSize returns the size of the color swatch in legend items.
func (r *Resolver) LegendSwatchSize() float32 { return 10 }

// LegendTextSize returns the text size for legend labels.
func (r *Resolver) LegendTextSize() float32 { return 11 }

// LegendItemGap returns the horizontal gap between legend items.
func (r *Resolver) LegendItemGap() float32 { return 16 }

// ButtonBackgroundColor returns the background color for UI buttons (toolbox, zoom).
func (r *Resolver) ButtonBackgroundColor() color.Color {
	return color.NRGBA{R: 60, G: 60, B: 60, A: 200}
}

// ButtonTextColor returns the text color for UI buttons.
func (r *Resolver) ButtonTextColor() color.Color {
	return color.NRGBA{R: 200, G: 200, B: 200, A: 255}
}

// ButtonTextSize returns the text size for UI button labels.
func (r *Resolver) ButtonTextSize() float32 { return 10 }

// AccentColor returns the accent/highlight color used for interactive elements.
func (r *Resolver) AccentColor() color.Color {
	return color.NRGBA{R: 91, G: 143, B: 249, A: 255}
}

// ZoomButtonBackgroundColor returns the background for zoom buttons (slightly lighter than toolbox).
func (r *Resolver) ZoomButtonBackgroundColor() color.Color {
	return color.NRGBA{R: 70, G: 70, B: 70, A: 220}
}
