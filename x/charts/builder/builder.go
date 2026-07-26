// Package builder provides fluent API helpers for constructing chart Options.
package builder

import (
	"image/color"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/chart"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// ChartBuilder constructs a chart.Option via method chaining.
type ChartBuilder struct {
	opt chart.Option
}

// Chart creates a generic chart builder.
func Chart() *ChartBuilder {
	return &ChartBuilder{}
}

// Title sets the chart title.
func (b *ChartBuilder) Title(text string) *ChartBuilder {
	b.opt.Title = &chart.Title{Text: text}
	return b
}

// SubTitle sets the chart subtitle.
func (b *ChartBuilder) SubTitle(text string) *ChartBuilder {
	if b.opt.Title == nil {
		b.opt.Title = &chart.Title{}
	}
	b.opt.Title.SubText = text
	return b
}

// Legend enables the legend.
func (b *ChartBuilder) Legend() *ChartBuilder {
	b.opt.Legend = &component.LegendConfig{Position: component.LegendBottom}
	return b
}

// Tooltip sets the tooltip trigger mode.
func (b *ChartBuilder) Tooltip(trigger component.TooltipTrigger) *ChartBuilder {
	b.opt.Tooltip = &component.TooltipConfig{Trigger: trigger}
	return b
}

// Crosshair sets the crosshair mode on the tooltip.
func (b *ChartBuilder) Crosshair(mode component.CrosshairMode) *ChartBuilder {
	if b.opt.Tooltip == nil {
		b.opt.Tooltip = &component.TooltipConfig{}
	}
	b.opt.Tooltip.Crosshair = mode
	return b
}

// Zoom enables data zoom with default settings.
func (b *ChartBuilder) Zoom() *ChartBuilder {
	cfg := interact.DefaultZoomConfig()
	b.opt.DataZoom = &cfg
	return b
}

// Brush enables brush selection.
func (b *ChartBuilder) Brush() *ChartBuilder {
	b.opt.Brush = &interact.BrushConfig{Enabled: true}
	return b
}

// Animate enables entrance animation.
func (b *ChartBuilder) Animate() *ChartBuilder {
	b.opt.Animation = true
	return b
}

// Grid sets plot area margins.
func (b *ChartBuilder) Grid(left, right, top, bottom float32) *ChartBuilder {
	b.opt.Grid = &chart.Grid{Left: left, Right: right, Top: top, Bottom: bottom}
	return b
}

// CategoryX adds a category X axis with the given labels.
func (b *ChartBuilder) CategoryX(labels ...string) *ChartBuilder {
	b.opt.XAxis = append(b.opt.XAxis, axis.Config{Type: axis.TypeCategory, Data: labels})
	return b
}

// ValueX adds a value X axis.
func (b *ChartBuilder) ValueX() *ChartBuilder {
	b.opt.XAxis = append(b.opt.XAxis, axis.Config{Type: axis.TypeValue, SplitCount: 5})
	return b
}

// ValueY adds a value Y axis.
func (b *ChartBuilder) ValueY() *ChartBuilder {
	b.opt.YAxis = append(b.opt.YAxis, axis.Config{Type: axis.TypeValue, SplitCount: 5})
	return b
}

// Palette sets custom series colors.
func (b *ChartBuilder) Palette(colors ...color.Color) *ChartBuilder {
	b.opt.Color = colors
	return b
}

// AddLine adds a line series.
func (b *ChartBuilder) AddLine(name string, data ...float64) *LineBuilder {
	s := &series.LineSeries{SeriesName: name, Data: data}
	b.opt.Series = append(b.opt.Series, s)
	return &LineBuilder{parent: b, s: s}
}

// AddBar adds a bar series.
func (b *ChartBuilder) AddBar(name string, data ...float64) *BarBuilder {
	s := &series.BarSeries{SeriesName: name, Data: data}
	b.opt.Series = append(b.opt.Series, s)
	return &BarBuilder{parent: b, s: s}
}

// AddScatter adds a scatter series (data as x,y pairs: x0,y0,x1,y1,...).
func (b *ChartBuilder) AddScatter(name string, data ...float64) *ScatterBuilder {
	s := &series.ScatterSeries{SeriesName: name, Data: data}
	b.opt.Series = append(b.opt.Series, s)
	return &ScatterBuilder{parent: b, s: s}
}

// AddPie adds a pie series.
func (b *ChartBuilder) AddPie(name string) *PieBuilder {
	s := &series.PieSeries{SeriesName: name}
	b.opt.Series = append(b.opt.Series, s)
	return &PieBuilder{parent: b, s: s}
}

// Build returns the constructed Option.
func (b *ChartBuilder) Build() chart.Option {
	return b.opt
}

// Widget creates a Chart widget from the built Option.
func (b *ChartBuilder) Widget() *chart.Chart {
	return chart.New(b.Build())
}

// --- Line series builder ---

// LineBuilder configures a LineSeries within a ChartBuilder chain.
type LineBuilder struct {
	parent *ChartBuilder
	s      *series.LineSeries
}

// Smooth enables line smoothing (placeholder for future implementation).
func (lb *LineBuilder) Smooth() *LineBuilder { lb.s.Smooth = true; return lb }

// Symbol sets the data point symbol type.
func (lb *LineBuilder) Symbol(t series.SymbolType) *LineBuilder { lb.s.Symbol = t; return lb }

// AreaFill adds a semi-transparent area fill under the line.
func (lb *LineBuilder) AreaFill(opacity float32) *LineBuilder {
	lb.s.AreaStyle = &series.AreaStyle{Opacity: opacity}
	return lb
}

// Dashed sets the line dash pattern (e.g., {6, 4} for 6px dash, 4px gap).
func (lb *LineBuilder) Dashed(dashGap ...float32) *LineBuilder {
	lb.s.LineStyle.Dash = dashGap
	return lb
}

// YAxis sets the Y axis index for this series.
func (lb *LineBuilder) YAxis(idx int) *LineBuilder { lb.s.YAxisIndex = idx; return lb }

// Done returns to the parent ChartBuilder.
func (lb *LineBuilder) Done() *ChartBuilder { return lb.parent }

// --- Bar series builder ---

// BarBuilder configures a BarSeries within a ChartBuilder chain.
type BarBuilder struct {
	parent *ChartBuilder
	s      *series.BarSeries
}

// CornerRadius sets the bar corner radius.
func (bb *BarBuilder) CornerRadius(r float32) *BarBuilder { bb.s.CornerRadius = r; return bb }

// Width sets the bar width fraction.
func (bb *BarBuilder) Width(w float32) *BarBuilder { bb.s.BarWidth = w; return bb }

// Stack sets the stack group name.
func (bb *BarBuilder) Stack(name string) *BarBuilder { bb.s.Stack = name; return bb }

// YAxis sets the Y axis index for this series.
func (bb *BarBuilder) YAxis(idx int) *BarBuilder { bb.s.YAxisIndex = idx; return bb }

// Done returns to the parent ChartBuilder.
func (bb *BarBuilder) Done() *ChartBuilder { return bb.parent }

// --- Scatter series builder ---

// ScatterBuilder configures a ScatterSeries within a ChartBuilder chain.
type ScatterBuilder struct {
	parent *ChartBuilder
	s      *series.ScatterSeries
}

// Symbol sets the data point symbol type.
func (sb *ScatterBuilder) Symbol(t series.SymbolType) *ScatterBuilder { sb.s.Symbol = t; return sb }

// Size sets the symbol size.
func (sb *ScatterBuilder) Size(s float32) *ScatterBuilder { sb.s.SymbolSize = s; return sb }

// Done returns to the parent ChartBuilder.
func (sb *ScatterBuilder) Done() *ChartBuilder { return sb.parent }

// --- Pie series builder ---

// PieBuilder configures a PieSeries within a ChartBuilder chain.
type PieBuilder struct {
	parent *ChartBuilder
	s      *series.PieSeries
}

// Slice adds a data slice.
func (pb *PieBuilder) Slice(name string, value float64) *PieBuilder {
	pb.s.Data = append(pb.s.Data, series.PieDataItem{Name: name, Value: value})
	return pb
}

// Radius sets the pie radius fraction.
func (pb *PieBuilder) Radius(r float32) *PieBuilder { pb.s.Radius = r; return pb }

// Doughnut sets the inner radius fraction (0 = pie, >0 = doughnut).
func (pb *PieBuilder) Doughnut(r float32) *PieBuilder { pb.s.InnerRadius = r; return pb }

// Labels enables slice labels.
func (pb *PieBuilder) Labels() *PieBuilder { pb.s.ShowLabels = true; return pb }

// Done returns to the parent ChartBuilder.
func (pb *PieBuilder) Done() *ChartBuilder { return pb.parent }

// --- Convenience top-level builders ---

// --- Overlay builders ---

// ReferenceLine adds a reference line overlay.
func (b *ChartBuilder) ReferenceLine(axis chart.OverlayAxis, value float64) *ReferenceLineBuilder {
	rl := &chart.ReferenceLine{Axis: axis, Value: value}
	b.opt.Overlays = append(b.opt.Overlays, rl)
	return &ReferenceLineBuilder{parent: b, rl: rl}
}

// ReferenceBand adds a reference band overlay.
func (b *ChartBuilder) ReferenceBand(axis chart.OverlayAxis, from, to float64) *ReferenceBandBuilder {
	rb := &chart.ReferenceBand{Axis: axis, From: from, To: to}
	b.opt.Overlays = append(b.opt.Overlays, rb)
	return &ReferenceBandBuilder{parent: b, rb: rb}
}

// Annotate adds an annotation overlay at data coordinates.
func (b *ChartBuilder) Annotate(x, y float64, text string) *AnnotationBuilder {
	a := &chart.Annotation{X: x, Y: y, Text: text}
	b.opt.Overlays = append(b.opt.Overlays, a)
	return &AnnotationBuilder{parent: b, a: a}
}

// ReferenceLineBuilder configures a ReferenceLine.
type ReferenceLineBuilder struct {
	parent *ChartBuilder
	rl     *chart.ReferenceLine
}

func (b *ReferenceLineBuilder) Color(c color.Color) *ReferenceLineBuilder { b.rl.Color = c; return b }

func (b *ReferenceLineBuilder) Width(w float32) *ReferenceLineBuilder { b.rl.Width = w; return b }

func (b *ReferenceLineBuilder) Dashed() *ReferenceLineBuilder { b.rl.Dashed = true; return b }

// DashPattern sets a custom dash pattern (e.g., {6, 4}) and enables dashing.
func (b *ReferenceLineBuilder) DashPattern(dashGap ...float32) *ReferenceLineBuilder {
	b.rl.Dashed = true
	b.rl.DashPattern = dashGap
	return b
}
func (b *ReferenceLineBuilder) Label(l string) *ReferenceLineBuilder { b.rl.Label = l; return b }
func (b *ReferenceLineBuilder) Done() *ChartBuilder                  { return b.parent }

// ReferenceBandBuilder configures a ReferenceBand.
type ReferenceBandBuilder struct {
	parent *ChartBuilder
	rb     *chart.ReferenceBand
}

func (b *ReferenceBandBuilder) Color(c color.Color) *ReferenceBandBuilder { b.rb.Color = c; return b }

func (b *ReferenceBandBuilder) Label(l string) *ReferenceBandBuilder { b.rb.Label = l; return b }
func (b *ReferenceBandBuilder) Done() *ChartBuilder                  { return b.parent }

// AnnotationBuilder configures an Annotation.
type AnnotationBuilder struct {
	parent *ChartBuilder
	a      *chart.Annotation
}

func (ab *AnnotationBuilder) Color(c color.Color) *AnnotationBuilder { ab.a.Color = c; return ab }
func (ab *AnnotationBuilder) Arrow() *AnnotationBuilder              { ab.a.Arrow = true; return ab }
func (ab *AnnotationBuilder) Done() *ChartBuilder                    { return ab.parent }

// VisualMap adds a visual map configuration that maps data values to colors/sizes.
func (b *ChartBuilder) VisualMap(min, max float64) *VisualMapBuilder {
	vm := component.VisualMapConfig{Min: min, Max: max}
	b.opt.VisualMap = append(b.opt.VisualMap, vm)
	return &VisualMapBuilder{parent: b, idx: len(b.opt.VisualMap) - 1}
}

// VisualMapBuilder configures a VisualMapConfig within a ChartBuilder chain.
type VisualMapBuilder struct {
	parent *ChartBuilder
	idx    int
}

// Colors sets the color gradient stops for the visual map.
func (vb *VisualMapBuilder) Colors(colors ...color.Color) *VisualMapBuilder {
	vb.parent.opt.VisualMap[vb.idx].InRange.Colors = colors
	return vb
}

// SymbolSize sets the symbol size range [min, max] for the visual map.
func (vb *VisualMapBuilder) SymbolSize(min, max float32) *VisualMapBuilder {
	vb.parent.opt.VisualMap[vb.idx].InRange.SymbolSizeMin = min
	vb.parent.opt.VisualMap[vb.idx].InRange.SymbolSizeMax = max
	return vb
}

// Dimension sets which data dimension to map.
func (vb *VisualMapBuilder) Dimension(dim int) *VisualMapBuilder {
	vb.parent.opt.VisualMap[vb.idx].Dimension = dim
	return vb
}

// ForSeries restricts the mapping to a specific series index.
func (vb *VisualMapBuilder) ForSeries(idx int) *VisualMapBuilder {
	vb.parent.opt.VisualMap[vb.idx].SeriesIndex = idx
	return vb
}

// Done returns to the parent ChartBuilder.
func (vb *VisualMapBuilder) Done() *ChartBuilder { return vb.parent }

// Drilldown configures drilldown with a data provider callback.
func (b *ChartBuilder) Drilldown(provider func(interact.HitResult, []string) *chart.Option) *ChartBuilder {
	b.opt.Drilldown = &chart.DrilldownConfig{DataProvider: provider}
	return b
}

// --- Convenience top-level builders ---

// LineChart creates a builder pre-configured for a line chart.
func LineChart() *ChartBuilder {
	b := Chart()
	b.opt.Tooltip = &component.TooltipConfig{Trigger: component.TooltipTriggerAxis}
	return b
}

// BarChart creates a builder pre-configured for a bar chart.
func BarChart() *ChartBuilder {
	b := Chart()
	b.opt.Tooltip = &component.TooltipConfig{Trigger: component.TooltipTriggerItem}
	return b
}

// ScatterChart creates a builder pre-configured for a scatter chart.
func ScatterChart() *ChartBuilder {
	b := Chart()
	b.ValueX().ValueY()
	b.opt.Tooltip = &component.TooltipConfig{Trigger: component.TooltipTriggerItem}
	return b
}

// PieChart creates a builder pre-configured for a pie chart.
func PieChart() *ChartBuilder {
	b := Chart()
	b.Grid(20, 20, 40, 20)
	b.opt.Tooltip = &component.TooltipConfig{Trigger: component.TooltipTriggerItem}
	return b
}
