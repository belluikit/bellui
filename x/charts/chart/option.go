package chart

import (
	"image/color"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// Option is the top-level declarative configuration for a chart (analogous to ECharts option).
type Option struct {
	Title     *Title
	Legend    *component.LegendConfig
	Tooltip   *component.TooltipConfig
	DataZoom  *interact.ZoomConfig
	Brush     *interact.BrushConfig
	Drilldown *DrilldownConfig
	Toolbox   *component.ToolboxConfig
	VisualMap []component.VisualMapConfig
	Overlays  []Overlay
	Grid      *Grid
	XAxis     []axis.Config
	YAxis     []axis.Config
	Series    []series.Series
	Color     []color.Color          // palette override
	Animation bool                   // enable entrance animation on first render
	Decal     *component.DecalConfig // accessibility decal patterns
}

// DrilldownConfig configures hierarchical data navigation.
// DataProvider is called when a data point is clicked. It receives the hit
// and the current breadcrumb path, and returns a child Option to drill into.
// Return nil to indicate no drilldown for that click.
type DrilldownConfig struct {
	DataProvider func(hit interact.HitResult, currentPath []string) *Option
}

// Overlay is the sealed interface for chart overlay annotations.
type Overlay interface {
	overlayType() string
}

// OverlayAxis specifies the orientation of a reference line or band.
type OverlayAxis int

const (
	AxisHorizontal OverlayAxis = iota // constant Y value, spans full width
	AxisVertical                      // constant X value, spans full height
)

// ReferenceLine draws a horizontal or vertical line at a specific data value.
type ReferenceLine struct {
	Axis        OverlayAxis
	Value       float64     // data-space value
	YAxisIndex  int         // which Y axis (for horizontal lines)
	Color       color.Color // nil = default accent
	Width       float32     // 0 = 1.5
	Dashed      bool
	DashPattern []float32 // overrides default dash/gap lengths when Dashed is true
	Label       string
}

func (r *ReferenceLine) overlayType() string { return "referenceLine" }

// ReferenceBand draws a shaded region between two data values.
type ReferenceBand struct {
	Axis       OverlayAxis
	From, To   float64
	YAxisIndex int
	Color      color.Color // nil = semi-transparent accent
	Label      string
}

func (r *ReferenceBand) overlayType() string { return "referenceBand" }

// Annotation places a text label at specific data coordinates.
type Annotation struct {
	X, Y       float64
	YAxisIndex int
	Text       string
	Color      color.Color // nil = theme foreground
	TextSize   float32     // 0 = default label size
	Arrow      bool        // draw arrow from label to data point
}

func (a *Annotation) overlayType() string { return "annotation" }

// Title configures the chart title.
type Title struct {
	Text    string
	SubText string
}

// Grid configures the chart plot area margins.
type Grid struct {
	Left   float32
	Right  float32
	Top    float32
	Bottom float32
}

// DefaultGrid returns a Grid with sensible default margins.
func DefaultGrid() Grid {
	return Grid{
		Left:   60,
		Right:  20,
		Top:    40,
		Bottom: 40,
	}
}
