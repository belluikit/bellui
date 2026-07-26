package series

import "image/color"

// Default rendering constants used across series renderers.
// These replace hardcoded magic numbers scattered throughout the codebase.
const (
	DefaultBarWidth       float32 = 0.6
	DefaultBoxWidth       float32 = 0.5
	DefaultLineWidth      float32 = 2
	DefaultLineSymbolSize float32 = 6
	DefaultScatterSize    float32 = 8
	DefaultPieRadius      float32 = 0.75
	DefaultGaugeRadius    float32 = 0.85
	DefaultAreaOpacity    float32 = 0.3
	DefaultRippleScale    float32 = 2.5
	DefaultPictorialSize  float32 = 12
	DefaultGraphNodeSize  float32 = 8
	DefaultTreeNodeSize   float32 = 6
	DefaultParallelWidth  float32 = 1.5
	DefaultBar3DSize      float32 = 0.4
	DefaultScatter3DSize  float32 = 0.08
)

// DefaultPalette provides a set of visually distinct colors for chart series.
// Used by pie, radar, bar3d, treemap, sankey, sunburst, graph, tree, parallel, and funnel renderers.
var DefaultPalette = []color.Color{
	color.NRGBA{R: 91, G: 143, B: 249, A: 255},  // blue
	color.NRGBA{R: 250, G: 200, B: 88, A: 255},  // amber
	color.NRGBA{R: 116, G: 211, B: 169, A: 255}, // teal
	color.NRGBA{R: 235, G: 96, B: 128, A: 255},  // pink
	color.NRGBA{R: 149, G: 134, B: 244, A: 255}, // purple
	color.NRGBA{R: 246, G: 152, B: 96, A: 255},  // orange
	color.NRGBA{R: 115, G: 192, B: 222, A: 255}, // cyan
	color.NRGBA{R: 59, G: 179, B: 113, A: 255},  // green
}
