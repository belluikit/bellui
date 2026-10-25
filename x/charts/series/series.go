// Package series defines chart series types and their renderers.
package series

import (
	"image/color"
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/pool"
)

// Type identifies the chart series type.
type Type string

const (
	TypeLine          Type = "line"          // line/area chart
	TypeBar           Type = "bar"           // bar/column chart
	TypeScatter       Type = "scatter"       // scatter/bubble chart
	TypePie           Type = "pie"           // pie/doughnut chart
	TypeCandlestick   Type = "candlestick"   // candlestick/OHLC chart
	TypeHeatmap       Type = "heatmap"       // heatmap chart
	TypeRadar         Type = "radar"         // radar/spider chart
	TypeGauge         Type = "gauge"         // gauge/meter chart
	TypeFunnel        Type = "funnel"        // funnel chart
	TypeBoxplot       Type = "boxplot"       // box-and-whisker chart
	TypeTreemap       Type = "treemap"       // treemap chart
	TypeSankey        Type = "sankey"        // sankey flow diagram
	TypeSunburst      Type = "sunburst"      // sunburst hierarchical chart
	TypeGraph         Type = "graph"         // node-link graph
	TypeTree          Type = "tree"          // tree layout
	TypeParallel      Type = "parallel"      // parallel coordinates
	TypePictorial     Type = "pictorial"     // pictorial bar chart
	TypeEffectScatter Type = "effectScatter" // animated scatter
	TypeCustom        Type = "custom"        // user-defined rendering
	TypeMap           Type = "map"           // tile-based geographic map
	TypeGeoScatter    Type = "geoScatter"    // scatter points on map
	TypeChoropleth    Type = "choropleth"    // colored regions from GeoJSON
	TypeGeoLines      Type = "geoLines"      // directional lines on map
	TypeBar3D         Type = "bar3d"         // 3D bar chart
	TypeScatter3D     Type = "scatter3d"     // 3D scatter chart
	TypeSurface       Type = "surface"       // 3D surface chart
	TypeLine3D        Type = "line3d"        // 3D line/tube chart
)

// Series is the interface all series types implement.
// The sealed() method restricts implementations to this package.
type Series interface {
	SeriesType() Type
	Name() string
	sealed()
}

// ColoredSeries is implemented by the series types that can set one colour
// for the whole series. The legend shows that colour for the series, and the
// palette colour when SeriesColor returns nil. Series that colour each item
// on its own (pie slices, funnel stages, gauge sectors) do not implement it.
//
// Since: 1.3
type ColoredSeries interface {
	Series
	// SeriesColor returns the colour the series is drawn in, or nil when it
	// takes the palette colour for its position.
	SeriesColor() color.Color
}

// SymbolType defines data point symbol shapes.
type SymbolType int

const (
	SymbolNone     SymbolType = iota // no symbol marker
	SymbolCircle                     // filled circle
	SymbolRect                       // filled rectangle
	SymbolTriangle                   // filled triangle
	SymbolDiamond                    // filled diamond
)

// LineStyle configures line appearance.
type LineStyle struct {
	Color color.Color
	Width float32
	// Dash defines the dash pattern as alternating dash and gap lengths in pixels.
	// For example, {6, 4} draws 6px dashes with 4px gaps.
	// A single value {6} is treated as {6, 6}.
	// Nil or empty means a solid line.
	Dash []float32
}

// AreaStyle configures area fill under a line.
type AreaStyle struct {
	Color   color.Color
	Opacity float32 // 0-1
}

// StepType defines step-line interpolation mode.
type StepType string

const (
	StepNone   StepType = ""       // smooth or straight line interpolation
	StepStart  StepType = "start"  // step at the start of each segment
	StepMiddle StepType = "middle" // step at the midpoint of each segment
	StepEnd    StepType = "end"    // step at the end of each segment
)

// RenderContext provides everything a series renderer needs.
type RenderContext struct {
	Series       Series
	Coord        coord.System
	Viewport     coord.Rect
	Color        color.Color // resolved series color
	SeriesIndex  int
	Animate      bool
	AnimProgress float32 // 0.0 to 1.0 during animation

	// StackOffsets provides cumulative Y offsets per category index for stacked series.
	// Key is the stack group name. Each series reads its base offset, then the renderer
	// adds its heights after rendering so the next series in the group stacks on top.
	StackOffsets map[string][]float64

	// Pool provides reusable canvas objects to reduce GC pressure.
	// Nil means allocate normally. Set by the chart renderer.
	Pool *pool.ObjectPool

	// Invalidate requests another full render of the chart. Renderers that load
	// data asynchronously (the choropleth's GeoJSON) hand it to their loader so
	// the arriving data is drawn, instead of waiting for an unrelated refresh.
	// It is safe to call from any goroutine and may be nil.
	Invalidate func()
}

// Renderer produces canvas objects for a single series.
type Renderer interface {
	Render(ctx RenderContext) []core.CanvasObject
}

// YAxisIndexer is implemented by cartesian series that can be assigned to a specific Y axis.
type YAxisIndexer interface {
	GetYAxisIndex() int
}

// Stackable is implemented by series that support stacking (bar, line).
type Stackable interface {
	StackGroup() string
	StackData() []float64
}

// DataRanger is implemented by series that can report their data range for axis scaling.
type DataRanger interface {
	DataRange() (xMin, xMax, yMin, yMax float64)
}

// DataAppender is implemented by series that support in-place data appending.
// This enables efficient incremental updates without replacing the entire Option.
type DataAppender interface {
	// AppendValues appends float64 data points to the series.
	// For line/bar: each value is one data point.
	// For scatter: values are x,y pairs (x0, y0, x1, y1, ...).
	AppendValues(values ...float64)
}

// HitValue holds data-space values for a hit-testable data point.
type HitValue struct {
	X, Y  float64
	Label string
}

// HitPoint describes a point-based hit area (line, scatter).
type HitPoint struct {
	DataIndex int
	PixelPos  geo.Position
	Value     HitValue
}

// HitBar describes a rectangle-based hit area (bar, candlestick).
type HitBar struct {
	DataIndex int
	Rect      [4]float32   // x, y, w, h in pixels
	PixelPos  geo.Position // center for tooltip positioning
	Value     HitValue
}

// HitAreaCollector receives hit-test geometry from a series.
// interact.HitTester implements this interface.
type HitAreaCollector interface {
	AddHitPoints(seriesIndex int, name string, seriesType Type, points []HitPoint)
	AddHitBars(seriesIndex int, name string, seriesType Type, bars []HitBar)
}

// HitRegistrar is implemented by series that can register their own hit-test areas.
// This eliminates type switches in the renderer for hit-test registration.
type HitRegistrar interface {
	RegisterHitAreas(collector HitAreaCollector, seriesIndex int, coordSys coord.System, plotRect coord.Rect)
}

// renderer registry — protected by renderersMu for safe concurrent access.
var (
	renderers   = map[Type]func() Renderer{}
	renderersMu sync.RWMutex
)

// RegisterRenderer registers a renderer factory for a series type.
// Typically called from init() functions.
func RegisterRenderer(t Type, factory func() Renderer) {
	renderersMu.Lock()
	renderers[t] = factory
	renderersMu.Unlock()
}

// NewRenderer creates a renderer for the given series type.
func NewRenderer(t Type) Renderer {
	renderersMu.RLock()
	f, ok := renderers[t]
	renderersMu.RUnlock()
	if ok {
		return f()
	}
	return nil
}

func init() {
	RegisterRenderer(TypeLine, func() Renderer { return &lineRenderer{} })
}
