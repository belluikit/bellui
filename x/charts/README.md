# Bellui X — charts

A native Go charting library inspired by [Apache ECharts](https://echarts.apache.org/), built on top of the [Bellui](https://bellui.com/) GUI toolkit. Charts are rendered entirely with Bellui canvas primitives -- no HTML, JavaScript, or WebView dependencies.

## Features

- **27 chart types** (the constants of `series.Type`): Line, Bar, Scatter, Pie, Candlestick, Heatmap, Radar, Gauge, Funnel, Boxplot, Treemap, Sankey, Sunburst, Graph, Tree, Parallel, Pictorial Bar, Effect Scatter, Custom, Map, Geo Scatter, Choropleth, Geo Lines, 3D Bar, 3D Scatter, 3D Surface, 3D Line
- **Declarative API**: Single `Option` struct configures the entire chart, mirroring the ECharts mental model
- **Builder / fluent API**: `builder.LineChart().Title("X").AddLine("Y", data...).Done().Widget()`
- **Stacked series**: Bar and area charts support stacking via a `Stack` group name
- **LTTB downsampling**: Automatically applied when data points exceed pixel width, preserving visual shape
- **Overlays**: ReferenceLine, ReferenceBand, and Annotation overlays with data-space positioning
- **Drilldown**: Hierarchical data navigation with breadcrumb trail and back button
- **3D charts**: Bar, Scatter, Surface and Line charts rendered via Bellui's `canvas.Scene3D` with orbit rotation and scroll zoom
- **VisualMap helpers**: `VisualMapConfig.MapColor` and `MapSize` map a value to a gradient colour or a symbol size. `Option.VisualMap` itself is not applied by any renderer yet ([#343](https://github.com/belluikit/bellui/issues/343)); call the helpers by hand
- **Toolbox**: Save-as-image and magic type (chart type switching) toolbar buttons
- **Dataset with transforms**: Column-oriented data storage, Encode mapping, Filter/Sort transforms, and Pipeline chaining
- **Animation**: Entrance animations (400ms ease-decelerate) and data transition animations (300ms ease-standard)
- **Interactive**: Hover tooltips, clickable legend toggle, scroll-to-zoom, drag-to-pan, brush selection, crosshair
- **Multi-axis**: Support for multiple Y axes (left + right) with independent scales
- **Theme-aware**: Automatically adapts to Bellui's Material Design 3 theme (light/dark mode)
- **Crosshair**: Vertical, horizontal, or both -- clipped to the plot area
- **Event system**: Central EventBus with typed events and filtered subscriptions

## Installation

```bash
go get github.com/belluikit/bellui/x/charts
```

## Quick Start

```go
package main

import (
    "github.com/belluikit/bellui"
    "github.com/belluikit/bellui/app"
    "github.com/belluikit/bellui/geo"

    "github.com/belluikit/bellui/x/charts/axis"
    "github.com/belluikit/bellui/x/charts/chart"
    "github.com/belluikit/bellui/x/charts/series"
)

func main() {
    a := app.New()
    w := a.NewWindow("My Chart")
    w.Resize(geo.NewSize(800, 500))

    c := chart.New(chart.Option{
        Title: &chart.Title{Text: "Weekly Sales"},
        XAxis: []axis.Config{{
            Type: axis.TypeCategory,
            Data: []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
        }},
        YAxis: []axis.Config{{Type: axis.TypeValue}},
        Series: []series.Series{
            &series.LineSeries{
                SeriesName: "Revenue",
                Data:       []float64{150, 230, 224, 218, 135},
                Symbol:     series.SymbolCircle,
            },
        },
    })

    w.SetContent(c)
    bellui.ShowAndRun(w)
}
```

### Builder API

```go
import "github.com/belluikit/bellui/x/charts/builder"

w := builder.LineChart().
    Title("Weekly Sales").
    CategoryX("Mon", "Tue", "Wed", "Thu", "Fri").
    ValueY().
    AddLine("Revenue", 150, 230, 224, 218, 135).
        Symbol(series.SymbolCircle).
        Done().
    Animate().
    Widget()
```

## Architecture

x/charts is organized into focused packages, each with a single responsibility. The project contains ~50 Go source files and ~10,000 lines of code across 12 packages:

```
charts/
  chart/         Main Chart widget, Option types, renderer pipeline
  series/        Series interface + per-type renderers (21 chart types)
  coord/         Coordinate system abstraction
  axis/          Axis types, scales, tick computation
  component/     UI components (title, legend, tooltip, zoom controls, overlays, breadcrumb, toolbox, visualmap)
  interact/      Hit testing, event bus, zoom/brush state
  style/         Theme resolver (Bellui MD3 -> chart styles)
  builder/       Fluent API for constructing chart Options
  dataset/       Column-oriented storage, Encode mapping, Transform pipeline
  animate/       Entrance and transition animation controllers
  internal/
    mathutil/    Nice numbers, line clipping, interpolation, LTTB downsampling
  cmd/
    demo/        24-chart scrollable demo application
```

### Package Details

#### `chart/` -- Chart Widget

The entry point. `chart.New(opt)` creates a Bellui widget that renders a chart from a declarative `Option` struct.

| File | Purpose |
|------|---------|
| `option.go` | `Option` struct with fields for Title, Legend, Tooltip, DataZoom, Brush, Drilldown, Toolbox, VisualMap, Overlays, Grid, Axes, Series, Colors, Animation |
| `chart.go` | `Chart` widget implementing `Tappable`, `Scrollable`, `Draggable`, `Hoverable` |
| `renderer.go` | Rendering pipeline: scales -> coordinates -> axes -> series -> overlays -> legend -> tooltip |

The rendering pipeline runs on every `Refresh()`:

```
Option
  -> Merge theme defaults
  -> Classify series (cartesian vs pie vs 3D)
  -> Build scales per axis config
  -> Scan visible series for data ranges (respecting zoom window)
  -> Apply LTTB downsampling when data > pixel width
  -> Build coordinate systems
  -> Render: background -> title -> breadcrumb -> axes -> grid lines -> series
       -> overlays -> legend -> zoom buttons -> toolbox -> brush overlay -> tooltip
  -> Trigger entrance animation on first render (if Animation=true)
```

#### `series/` -- Chart Series

Each chart type implements the sealed `Series` interface and has a corresponding `Renderer` registered via the factory pattern.

| Type | Struct | Key Features |
|------|--------|--------------|
| Line | `LineSeries` | Symbols, area fill, step interpolation, line clipping, stacking, LTTB downsampling |
| Bar | `BarSeries` | Corner radius, configurable width/gap, negative values, stacking |
| Scatter | `ScatterSeries` | Paired (x,y) data, multiple symbol types |
| Pie | `PieSeries` | Doughnut (via `InnerRadius`), labels, per-slice colors |
| Candlestick | `CandlestickSeries` | OHLC financial charts with bullish/bearish coloring |
| Heatmap | `HeatmapSeries` | Color matrix with its own gradient (`ColorStops`) |
| Radar | `RadarSeries` | Spider/web charts on polar coordinate system |
| Gauge | `GaugeSeries` | Arc + needle meter visualization |
| Funnel | `FunnelSeries` | Funnel/pipeline stage visualization |
| Boxplot | `BoxplotSeries` | Box-and-whisker statistical charts |
| Treemap | `TreemapSeries` | Hierarchical area-based visualization |
| Sankey | `SankeySeries` | Flow diagrams with weighted links |
| Sunburst | `SunburstSeries` | Radial hierarchical chart |
| Graph | `GraphSeries` | Node-link graph layout |
| Tree | `TreeSeries` | Tree layout visualization |
| Parallel | `ParallelSeries` | Parallel coordinates for multivariate data |
| Pictorial Bar | `PictorialSeries` | Pictorial/symbolic bar chart |
| Effect Scatter | `EffectScatterSeries` | Animated scatter with visual effects |
| Custom | `CustomSeries` | User-defined rendering callback |
| Map | `MapSeries` | Tile-based geographic map (OpenStreetMap by default) |
| Geo Scatter | `GeoScatterSeries` | Points placed by longitude/latitude on a map |
| Choropleth | `ChoroplethSeries` | Regions from GeoJSON coloured by value |
| Geo Lines | `GeoLinesSeries` | Directional lines between map coordinates |
| 3D Bar | `Bar3DSeries` | 3D bars via `canvas.Scene3D`, orbit rotation, scroll zoom |
| 3D Scatter | `Scatter3DSeries` | 3D point cloud via `canvas.Scene3D` |
| 3D Surface | `SurfaceSeries` | 3D surface mesh via `canvas.Scene3D` |
| 3D Line | `Line3DSeries` | 3D line ribbon via `canvas.Scene3D` |

Adding a new chart type:

1. Create a struct implementing `Series` (with `sealed()`)
2. Create a `Renderer` implementation
3. Register it in `init()` via `RegisterRenderer(TypeName, factory)`

#### `axis/` -- Scales and Ticks

Four scale types mapping data values to normalized `[0, 1]` space:

| Scale | Use Case |
|-------|----------|
| `LinearScale` | Continuous numeric data with nice-number ticks |
| `CategoryScale` | Discrete labels (supports zoom via `SetRange`) |
| `LogScale` | Logarithmic data (configurable base) |
| `TimeScale` | Time-based data with formatted labels |

The **nice-number algorithm** (`NiceTicks`) computes human-friendly axis ranges and tick intervals.

#### `coord/` -- Coordinate Systems

Maps data-space `(x, y)` to pixel-space `bellui.Position` and back.

| System | Use Case |
|--------|----------|
| `Cartesian2D` | Line, Bar, Scatter, Candlestick, Heatmap, Boxplot, Parallel charts |
| `Polar` | Radar, Sunburst charts |
| `None` | Pie, Gauge, Funnel, Treemap, Sankey, Graph, Tree -- series handle their own layout |

3D chart types (Bar3D, Scatter3D, Surface, Line3D) bypass the coordinate system entirely and render directly to `canvas.Scene3D`.

#### `component/` -- UI Components

| Component | File | Description |
|-----------|------|-------------|
| Title | `title.go` | Chart title + subtitle with theme-aware styling |
| Legend | `legend.go` | Color swatches + labels, click-to-toggle, dimmed hidden series |
| Tooltip | `tooltip.go` | Hover tooltip with series markers, crosshair lines, edge clamping |
| DataZoom Slider | `datazoom.go` | Range-selection slider with draggable handles |
| Zoom Buttons | `zoombuttons.go` | +/-/Reset buttons for zoom control |
| Overlay | `overlay.go` | ReferenceLine, ReferenceBand, and Annotation rendering |
| Breadcrumb | `breadcrumb.go` | Drilldown navigation trail with back button and clickable crumbs |
| VisualMap | `visualmap.go` | Data-to-color gradient and data-to-size mapping helpers (not applied by the renderer yet, #343) |
| Toolbox | `toolbox.go` | Save-as-image and magic type (chart switching) buttons |

#### `builder/` -- Fluent API

`builder.Chart()` returns a `ChartBuilder` with chainable methods for setting title, axes, series, overlays, drilldown, animation, and more. Convenience constructors `LineChart()`, `BarChart()`, `ScatterChart()`, `PieChart()` pre-configure sensible defaults.

Series-specific sub-builders (`LineBuilder`, `BarBuilder`, `ScatterBuilder`, `PieBuilder`) expose type-specific options (symbols, area fill, stacking, corner radius, doughnut). Overlay builders (`ReferenceLineBuilder`, `ReferenceBandBuilder`, `AnnotationBuilder`) configure annotation properties inline.

#### `dataset/` -- Dataset and Transforms

Column-oriented data storage supporting float64, string, and time.Time dimensions. Key types:

- **`Dataset`**: tabular storage with `AppendRow` and column accessors
- **`Encode`**: maps dimension indices to visual roles (X, Y, Size, Color, Label)
- **`Transform`** interface: `FilterTransform`, `SortTransform`
- **`Pipeline`**: chains multiple transforms in sequence

#### `animate/` -- Animation

`Controller` drives entrance and transition animations using Bellui's animation system:

- **Entrance**: animates from 0 to 1 over 400ms with `EaseStandardDecelerate`
- **Transition**: morphs from current progress to 1 over 300ms with `EaseStandard`
- Series renderers read `Controller.Progress` and `AnimProgress` in `RenderContext` to interpolate positions

#### `interact/` -- Interaction

| File | Description |
|------|-------------|
| `hit.go` | `HitTester` with per-type strategies: point proximity (line/scatter), rectangle containment (bar), angle sector (pie), axis-trigger mode |
| `event.go` | `EventBus` with `On`/`OnFiltered`/`Off`/`Emit`, event types: Click, Hover, LegendToggle, ZoomChange, BrushSelect |
| `zoom.go` | `DataWindow` (normalized [0,1] range), `ZoomState` with `ZoomAt` (centered zoom) and `Pan` |
| `brush.go` | `BrushState` tracking drag selection, `FindSelectedPoints` for point-in-rect queries |

#### `style/` -- Theming

`Resolver` reads from Bellui's MD3 theme at render time:

- Series palette derived from Primary/Secondary/Tertiary + defaults
- Grid lines from Separator color
- Axis labels from Foreground with caption text size
- Tooltips use inverse surface colors
- Automatically adapts to dark/light mode changes

#### `internal/mathutil/` -- Math Utilities

- `NiceNumber` / `NiceTicks` / `TickValues` -- axis tick computation
- `Clamp`, `Lerp`, `InverseLerp` -- interpolation helpers
- `ClipLineToRect` -- Cohen-Sutherland line clipping for zoomed views
- `LTTB` / `DownsampleFloat64` -- Largest-Triangle-Three-Buckets downsampling for large datasets

## Configuration Reference

### Option

```go
type Option struct {
    Title     *Title                      // chart title and subtitle
    Legend    *component.LegendConfig     // legend position and style
    Tooltip   *component.TooltipConfig    // tooltip trigger and crosshair mode
    DataZoom  *interact.ZoomConfig        // scroll/button zoom configuration
    Brush     *interact.BrushConfig       // drag-to-select configuration
    Drilldown *DrilldownConfig            // hierarchical data navigation
    Toolbox   *component.ToolboxConfig    // save, magic type buttons
    VisualMap []component.VisualMapConfig  // not applied by any renderer yet (#343)
    Overlays  []Overlay                   // ReferenceLine, ReferenceBand, Annotation
    Grid      *Grid                       // plot area margins (Left, Right, Top, Bottom)
    XAxis     []axis.Config               // X axis configurations
    YAxis     []axis.Config               // Y axis configurations (multiple = multi-axis)
    Series    []series.Series             // data series to render
    Color     []color.Color               // palette override
    Animation bool                        // enable entrance animation on first render
    Decal     *component.DecalConfig      // not applied by any renderer yet (#343)
}
```

### Axis Config

```go
type Config struct {
    Type       Type        // TypeValue, TypeCategory, TypeTime, TypeLog
    Name       string      // axis title: below an X axis's labels, above a Y axis
    Position   Position    // X: PositionBottom (default) or PositionTop;
                           // Y: PositionLeft or PositionRight (default: first Y axis left, the rest right)
    Data       []string    // category labels (for TypeCategory)
    Min, Max   *float64    // explicit range bounds (nil = auto)
    SplitCount int         // desired number of tick splits
    Inverse    bool        // reverse the axis direction
    LineColor  color.Color // axis line and tick marks (nil = theme)
    LabelColor color.Color // tick labels and axis name (nil = theme)
    TimeFormat string      // Go time layout for TypeTime labels

    Hide      Parts     // parts to hide: Hide.Line, Hide.Ticks, Hide.Labels (zero value hides nothing)
    SplitLine SplitLine // grid lines: SplitLineAuto (first Y axis only), SplitLineShow, SplitLineHide
    Offset    float32   // px outward from the plot edge (ECharts offset); the layout makes room

    // Deprecated, no effect: ShowLine, ShowTicks, ShowLabels, ShowSplitLine.
    // Use Hide and SplitLine.
}
```

A chart draws one X axis, from the first entry of `XAxis`.

Every part of an axis is drawn unless `Hide` says otherwise, so a `Config`
written as a literal draws the whole axis. Grid lines follow `SplitLine`: by
default only the first Y axis draws them; `SplitLineShow` adds them for the X
axis (vertical) or another Y axis, `SplitLineHide` removes them. A value, log or
time axis draws its grid lines at its ticks; a category axis draws them between
its categories, so in the example below each vertical line separates two days'
bars while the tick marks and labels stay under the bars. A left-hand
Y axis draws no tick marks, so `Hide.Ticks` changes nothing there.

```go
x := axis.Config{Type: axis.TypeCategory, Data: days, SplitLine: axis.SplitLineShow}
y := axis.Config{Type: axis.TypeValue, Hide: axis.Parts{Line: true, Ticks: true}}
```

The four `Show*` fields were never applied, and their zero value would have
hidden the axes of every literal config, so they are deprecated and ignored.
`DefaultConfig` still sets them as before for code that reads them.

### Tooltip Config

```go
type TooltipConfig struct {
    Trigger   TooltipTrigger // TooltipTriggerItem, TooltipTriggerAxis, TooltipTriggerNone
    Crosshair CrosshairMode  // CrosshairVertical, CrosshairHorizontal, CrosshairBoth, CrosshairNone
    Formatter func([]HitResult) string // custom tooltip text
}
```

### Zoom Config

```go
type ZoomConfig struct {
    Enabled         bool
    MinSpan         float64 // minimum zoom window (fraction), default 0.01
    MaxSpan         float64 // maximum zoom window (fraction), default 1.0
    ZoomSensitivity float64 // scroll wheel multiplier, default 0.05
}
```

### Overlay Types

```go
// ReferenceLine draws a horizontal or vertical line at a specific data value.
type ReferenceLine struct {
    Axis       OverlayAxis     // AxisHorizontal or AxisVertical
    Value      float64         // data-space value
    YAxisIndex int             // which Y axis (for horizontal lines)
    Color      color.Color     // nil = default accent
    Width      float32         // 0 = 1.5
    Dashed     bool
    Label      string
}

// ReferenceBand draws a shaded region between two data values.
type ReferenceBand struct {
    Axis       OverlayAxis
    From, To   float64
    YAxisIndex int
    Color      color.Color     // nil = semi-transparent accent
    Label      string
}

// Annotation places a text label at specific data coordinates.
type Annotation struct {
    X, Y       float64
    YAxisIndex int
    Text       string
    Color      color.Color     // nil = theme foreground
    TextSize   float32         // 0 = default label size
    Arrow      bool            // draw arrow from label to data point
}
```

### Drilldown Config

```go
type DrilldownConfig struct {
    // DataProvider is called on click. Return a child Option, or nil for no drilldown.
    DataProvider func(hit interact.HitResult, currentPath []string) *Option
}
```

### VisualMap Config

```go
type VisualMapConfig struct {
    Min, Max    float64          // data range (values outside are clamped)
    InRange     VisualMapRange   // color gradient stops and/or symbol size range
    Dimension   int              // data dimension to map (default 0 = Y value)
    SeriesIndex int              // restrict to specific series (-1 = all)
}
```

### Toolbox Config

```go
type ToolboxConfig struct {
    SaveAsImage bool     // show save-as-image button
    MagicType   []string // chart type switching (e.g., ["line", "bar"])
}
```

## Interactions

| Interaction | How | Requirements |
|-------------|-----|--------------|
| Hover tooltip | Move mouse over data points | Set `Tooltip` on Option |
| Legend toggle | Click a legend item | Multiple series (auto-shown) or set `Legend` on Option |
| Scroll zoom | Scroll wheel on chart | Set `DataZoom` on Option |
| Button zoom | Click +/-/Reset buttons | Set `DataZoom` on Option |
| Drag pan | Drag on chart when zoomed | Set `DataZoom` on Option, zoom in first |
| Brush select | Drag on chart | Set `Brush` on Option |
| Click event | Click on data point | Set `OnClick` callback on Chart |
| Drilldown | Click on data point | Set `Drilldown` with a `DataProvider` on Option |
| Toolbox save | Click save button | Set `Toolbox` with `SaveAsImage: true` |
| Magic type switch | Click chart type button | Set `Toolbox` with `MagicType: ["line", "bar"]` |

### Event Bus

```go
c := chart.New(opt)

// Listen to all clicks
c.Events.On(interact.EventClick, func(evt interface{}) {
    e := evt.(interact.DataPointEvent)
    fmt.Printf("Clicked %s point %d\n", e.SeriesName, e.DataIndex)
})

// Listen to zoom changes
c.Events.On(interact.EventZoomChange, func(evt interface{}) {
    e := evt.(interact.ZoomChangeEvent)
    fmt.Printf("Zoom: %.0f%% - %.0f%%\n", e.Start*100, e.End*100)
})
```

## Multi-Y-Axis

```go
chart.New(chart.Option{
    Grid: &chart.Grid{Left: 60, Right: 50, Top: 40, Bottom: 40},
    YAxis: []axis.Config{
        {Type: axis.TypeValue, SplitCount: 5}, // left axis
        {Type: axis.TypeValue, SplitCount: 5}, // right axis
    },
    Series: []series.Series{
        &series.LineSeries{
            SeriesName: "Revenue",
            Data:       []float64{150, 230, 224},
            YAxisIndex: 0, // uses left axis
        },
        &series.LineSeries{
            SeriesName: "Growth %",
            Data:       []float64{12, 53, -2},
            YAxisIndex: 1, // uses right axis
        },
    },
})
```

A third Y axis also goes on the right by default, and two axes on one side
are drawn in the same place unless one is moved out with `Offset` (ECharts
`yAxis.offset`): the number of pixels the axis sits outward from the plot edge.
The chart reserves room for the largest offset on each side, so the axis stays
inside it. A right-hand axis's labels reach 56 px beyond its line, so an `Offset` of 60
puts a second right axis clear of the first; on the left, an `Offset` of
`Grid.Left` does. The axis pointer tags follow the offset axis.

```go
YAxis: []axis.Config{
    {Type: axis.TypeValue},                                          // left
    {Type: axis.TypeValue},                                          // right
    {Type: axis.TypeValue, Position: axis.PositionRight, Offset: 60}, // right, beyond the second
},
```

## Stacked Bar / Area

```go
chart.New(chart.Option{
    XAxis: []axis.Config{{Type: axis.TypeCategory, Data: []string{"Q1", "Q2", "Q3"}}},
    YAxis: []axis.Config{{Type: axis.TypeValue}},
    Series: []series.Series{
        &series.BarSeries{SeriesName: "Product A", Data: []float64{40, 50, 60}, Stack: "total"},
        &series.BarSeries{SeriesName: "Product B", Data: []float64{30, 35, 45}, Stack: "total"},
    },
})
```

## Overlays

```go
chart.New(chart.Option{
    // ... axes and series ...
    Overlays: []chart.Overlay{
        &chart.ReferenceLine{Axis: chart.AxisHorizontal, Value: 200, Label: "Target", Dashed: true},
        &chart.ReferenceBand{Axis: chart.AxisHorizontal, From: 100, To: 150, Label: "Normal Range"},
        &chart.Annotation{X: 2, Y: 224, Text: "Peak", Arrow: true},
    },
})
```

## Roadmap

### Planned

- Geographic map chart type (GeoSeries with GeoJSON regions)
- Canvas object pooling for large dataset performance

## License

See [LICENSE](LICENSE) for details.
