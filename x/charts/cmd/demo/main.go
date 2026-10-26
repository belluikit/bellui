// Command demo showcases the x/charts capabilities: its chart types organized in
// 9 tabbed categories (Basic, Financial, Hierarchical, Specialized, Advanced,
// Drilldown, 3D, Trading, Geo) with interactions, overlays, builder API, dataset,
// animation, and real-time trading chart features.
package main

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/builder"
	"github.com/belluikit/bellui/x/charts/chart"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/dataset"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

func main() {
	a := app.NewWithID("com.trendvidia.charts.demo")
	w := a.NewWindow("x/charts demo — all capabilities")
	w.Resize(geo.NewSize(1400, 900))

	legend := &component.LegendConfig{Position: component.LegendBottom}
	itemTooltip := &component.TooltipConfig{Trigger: component.TooltipTriggerItem}
	itemTooltipNoCrosshair := &component.TooltipConfig{Trigger: component.TooltipTriggerItem, Crosshair: component.CrosshairNone}

	// ── 1. Line chart with overlays + toolbox ──
	// Features: dual Y axes, area fill, zoom, overlays, toolbox (save + magic type)
	lineChart := chart.New(chart.Option{
		Title:    &chart.Title{Text: "Line + Overlays", SubText: "Ref Line · Band · Toolbox"},
		Legend:   legend,
		Toolbox:  &component.ToolboxConfig{SaveAsImage: true, MagicType: []string{"line", "bar"}},
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairBoth},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.1, MaxSpan: 1.0, ZoomSensitivity: 0.1},
		Grid:     &chart.Grid{Left: 60, Right: 50, Top: 50, Bottom: 40},
		Overlays: []chart.Overlay{
			&chart.ReferenceLine{Axis: chart.AxisHorizontal, Value: 200, Dashed: true, Label: "Target", Color: color.NRGBA{R: 255, G: 180, B: 0, A: 200}},
			&chart.ReferenceBand{Axis: chart.AxisHorizontal, From: 130, To: 180, Label: "Low Zone", Color: color.NRGBA{R: 255, G: 80, B: 80, A: 30}},
			&chart.Annotation{X: 6, Y: 260, Text: "Peak!", Arrow: true},
		},
		XAxis: []axis.Config{{
			Type: axis.TypeCategory,
			Data: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		}},
		YAxis: []axis.Config{
			{Type: axis.TypeValue, SplitCount: 5},
			{Type: axis.TypeValue, SplitCount: 5},
		},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "Revenue",
				Data:       []float64{150, 230, 224, 218, 135, 147, 260},
				Symbol:     series.SymbolCircle,
				YAxisIndex: 0,
				AreaStyle:  &series.AreaStyle{Opacity: 0.15},
			},
			&series.LineSeries{
				SeriesName: "Growth %",
				Data:       []float64{12, 53, -2, -3, -38, 9, 77},
				Symbol:     series.SymbolCircle,
				YAxisIndex: 1,
			},
		},
		Animation: true,
	})
	lineChart.OnClick = func(hit interact.HitResult) {
		fmt.Printf("Clicked %s point %d: %.1f\n", hit.SeriesName, hit.DataIndex, hit.Value.Y)
	}

	// ── 2. Bar chart with drilldown ──
	// Features: click a bar to drill into monthly data, breadcrumb navigation back
	months := []string{"Jan", "Feb", "Mar"}
	quarterData := map[string][]float64{
		"Q1": {120, 100, 100},
		"Q2": {150, 180, 170},
		"Q3": {110, 130, 150},
		"Q4": {200, 210, 200},
	}
	barChart := chart.New(chart.Option{
		Title:     &chart.Title{Text: "Bar (Drilldown)", SubText: "Click bar to drill"},
		Legend:    legend,
		Tooltip:   itemTooltip,
		Animation: true,
		XAxis:     []axis.Config{{Type: axis.TypeCategory, Data: []string{"Q1", "Q2", "Q3", "Q4"}}},
		YAxis:     []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.BarSeries{SeriesName: "Revenue", Data: []float64{320, 480, 390, 510}, CornerRadius: 4},
		},
		Drilldown: &chart.DrilldownConfig{
			DataProvider: func(hit interact.HitResult, path []string) *chart.Option {
				labels := []string{"Q1", "Q2", "Q3", "Q4"}
				if hit.DataIndex >= len(labels) {
					return nil
				}
				q := labels[hit.DataIndex]
				data, ok := quarterData[q]
				if !ok {
					return nil
				}
				return &chart.Option{
					Title:     &chart.Title{Text: q + " Monthly", SubText: "Drilled down"},
					Animation: true,
					XAxis:     []axis.Config{{Type: axis.TypeCategory, Data: months}},
					YAxis:     []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
					Series:    []series.Series{&series.BarSeries{SeriesName: q, Data: data, CornerRadius: 4}},
				}
			},
		},
	})

	// ── 3. Scatter chart (with Dataset) ──
	// Features: dataset package, brush selection, EventBus
	ds := dataset.FromFloat64([][]float64{
		{10, 8.04},
		{8, 6.95},
		{13, 7.58},
		{9, 8.81},
		{11, 8.33},
		{14, 9.96},
		{6, 7.24},
		{4, 4.26},
		{12, 10.84},
		{7, 4.82},
		{5, 5.68},
	})
	dsXs, dsYs := dataset.ExtractXY(ds, dataset.DefaultEncode())
	scatterData := make([]float64, 0, len(dsXs)*2)
	for i := range dsXs {
		scatterData = append(scatterData, dsXs[i], dsYs[i])
	}

	scatterChart := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Scatter", SubText: "Dataset + Brush"},
		Legend:  legend,
		Tooltip: itemTooltip,
		Brush:   &interact.BrushConfig{Enabled: true},
		Grid:    &chart.Grid{Left: 60, Right: 20, Top: 50, Bottom: 40},
		XAxis:   []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		YAxis:   []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.ScatterSeries{SeriesName: "From Dataset", Data: scatterData, Symbol: series.SymbolCircle, SymbolSize: 10},
			&series.ScatterSeries{
				SeriesName: "Cluster B",
				Data:       []float64{8, 15.5, 10, 16.2, 12, 14.8, 9, 17.1, 11, 15.9, 7, 13.2, 13, 16.8, 6, 12.1},
				Symbol:     series.SymbolRect, SymbolSize: 10,
			},
		},
	})
	scatterChart.Events.On(interact.EventBrushSelect, func(evt interface{}) {
		e := evt.(interact.BrushSelectEvent)
		fmt.Printf("EventBus: brush selected %d points\n", len(e.Selected))
	})

	// ── 4. Doughnut chart ──
	// Features: doughnut (InnerRadius), labels, pie tooltip
	pieChart := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Doughnut Chart", SubText: "InnerRadius: 0.5"},
		Tooltip: itemTooltipNoCrosshair,
		Grid:    &chart.Grid{Left: 20, Right: 20, Top: 50, Bottom: 20},
		Series: []series.Series{
			&series.PieSeries{
				SeriesName:  "Browser Share",
				InnerRadius: 0.5,
				Data: []series.PieDataItem{
					{Name: "Chrome", Value: 65},
					{Name: "Safari", Value: 18},
					{Name: "Firefox", Value: 8},
					{Name: "Edge", Value: 5},
					{Name: "Other", Value: 4},
				},
				Radius: 1.0, ShowLabels: true,
			},
		},
	})

	// ── 5. Candlestick chart ──
	// Features: OHLC, up/down colors, legend
	candlestickChart := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Candlestick", SubText: "OHLC Financial"},
		Legend:  legend,
		Tooltip: itemTooltip,
		XAxis:   []axis.Config{{Type: axis.TypeCategory, Data: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul"}}},
		YAxis:   []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.CandlestickSeries{
				SeriesName: "Stock",
				Data: []series.CandlestickData{
					{Open: 100, High: 115, Low: 95, Close: 110},
					{Open: 110, High: 130, Low: 105, Close: 125},
					{Open: 125, High: 135, Low: 110, Close: 112},
					{Open: 112, High: 120, Low: 100, Close: 105},
					{Open: 105, High: 118, Low: 98, Close: 115},
					{Open: 115, High: 140, Low: 112, Close: 135},
					{Open: 135, High: 145, Low: 125, Close: 128},
				},
			},
		},
	})

	// ── 5b. Builder API showcase (replaces heatmap slot — heatmap moves to #6) ──
	// Features: fluent builder API, custom palette, overlays via builder
	builderChart := builder.LineChart().
		Title("Builder API").SubTitle("Fluent + Custom Palette").
		CategoryX("A", "B", "C", "D", "E").
		ValueY().
		Legend().
		Palette(
			color.NRGBA{R: 255, G: 99, B: 132, A: 255},
			color.NRGBA{R: 54, G: 162, B: 235, A: 255},
		).
		AddLine("Sales", 40, 65, 50, 80, 55).Symbol(series.SymbolCircle).Done().
		AddLine("Cost", 30, 45, 35, 50, 40).AreaFill(0.2).Done().
		ReferenceLine(chart.AxisHorizontal, 60).Label("Avg").Dashed().Done().
		Animate().
		Widget()

	// ── 6. Heatmap chart ──
	// Features: raster rendering, color gradient, tooltip
	heatmapChart := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Heatmap", SubText: "Raster + Tooltip"},
		Tooltip: itemTooltip,
		Grid:    &chart.Grid{Left: 20, Right: 20, Top: 50, Bottom: 20},
		Series: []series.Series{
			&series.HeatmapSeries{
				SeriesName: "Activity",
				Rows:       5, Cols: 7,
				Data: []float64{
					2, 5, 8, 3, 1, 6, 9,
					4, 7, 2, 8, 5, 3, 1,
					9, 3, 6, 1, 7, 8, 4,
					1, 8, 4, 9, 2, 5, 7,
					6, 2, 7, 5, 8, 1, 3,
				},
			},
		},
	})

	// ── 7. Radar chart ──
	// Features: spider grid, multi-series, filled area, indicators
	radarChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Radar", SubText: "Multi-Series + Area"},
		Grid:  &chart.Grid{Left: 20, Right: 20, Top: 50, Bottom: 20},
		Series: []series.Series{
			&series.RadarSeries{
				SeriesName: "Skills",
				Indicators: []series.RadarIndicator{
					{Name: "Attack", Max: 100},
					{Name: "Defense", Max: 100},
					{Name: "Speed", Max: 100},
					{Name: "Stamina", Max: 100},
					{Name: "Magic", Max: 100},
				},
				Data:      [][]float64{{85, 70, 90, 60, 75}, {65, 90, 50, 85, 60}},
				DataNames: []string{"Warrior", "Mage"},
				ShowArea:  true,
			},
		},
	})

	// ── 8. Gauge chart ──
	// Features: arc gauge, colored sectors, needle, value display
	gaugeChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Gauge", SubText: "Sectors + Needle"},
		Grid:  &chart.Grid{Left: 20, Right: 20, Top: 50, Bottom: 20},
		Series: []series.Series{
			&series.GaugeSeries{
				SeriesName: "Speed",
				Value:      72.5, Min: 0, Max: 100,
				ShowValue: true,
				Sectors: []series.GaugeSector{
					{Min: 0, Max: 30, Color: color.NRGBA{R: 38, G: 186, B: 121, A: 100}},
					{Min: 30, Max: 70, Color: color.NRGBA{R: 250, G: 200, B: 88, A: 100}},
					{Min: 70, Max: 100, Color: color.NRGBA{R: 239, G: 83, B: 80, A: 100}},
				},
			},
		},
	})

	// ── 9. 3D Bar chart ──
	// Features: Scene3D canvas object, 3D box meshes, perspective, lighting
	bar3DChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "3D Bar", SubText: "Scene3D + Lighting"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.Bar3DSeries{
				SeriesName: "Sales",
				Data: []series.Bar3DData{
					{X: 0, Z: 0, Value: 80},
					{X: 1, Z: 0, Value: 120},
					{X: 2, Z: 0, Value: 60},
					{X: 0, Z: 1, Value: 100},
					{X: 1, Z: 1, Value: 150},
					{X: 2, Z: 1, Value: 90},
				},
			},
		},
	})

	// ── 10. 3D Scatter chart ──
	// Features: octahedron point markers, 3D positioning
	scatter3DChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "3D Scatter", SubText: "Octahedron Points"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.Scatter3DSeries{
				SeriesName: "Particles",
				Data: []float64{
					0, 1, 0, 1, 2, 1, -1, 0.5, 1, 0.5, 1.5, -0.5,
					-0.5, 0.8, -1, 1.5, 0.3, 0.5, -1, 1.8, 0, 0, 0.5, 1.5,
				},
				PointSize: 0.12,
				Color:     color.NRGBA{R: 91, G: 143, B: 249, A: 255},
			},
		},
	})

	// ── 11. 3D Surface chart ──
	// Features: height-mapped surface mesh, per-face color gradient
	surfaceRows, surfaceCols := 15, 15
	surfaceData := make([]float64, surfaceRows*surfaceCols)
	for r := 0; r < surfaceRows; r++ {
		for c := 0; c < surfaceCols; c++ {
			x := float64(c)/float64(surfaceCols-1)*4 - 2
			z := float64(r)/float64(surfaceRows-1)*4 - 2
			surfaceData[r*surfaceCols+c] = math.Sin(x)*math.Cos(z) + 1
		}
	}

	surfaceChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "3D Surface", SubText: "sin(x)·cos(z)"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.SurfaceSeries{
				SeriesName: "Terrain",
				Rows:       surfaceRows,
				Cols:       surfaceCols,
				Data:       surfaceData,
			},
		},
	})

	// ── 3D Line chart ── (Excel-style multi-series at different Z depths)
	// Values scaled to 0-5 range so X, Y, Z are comparable for 3D viewing.
	zPos := func(z float64) *float64 { return &z }
	line3DChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "3D Line", SubText: "Multi-Series Depth"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.Line3DSeries{
				SeriesName: "Revenue",
				Data:       []float64{0, 0.5, 1, 2.3, 2, 1.8, 3, 3.1, 4, 2.5, 5, 4.0, 6, 3.5},
				ZPosition:  zPos(0),
				LineWidth:  0.25,
				Color:      color.NRGBA{R: 91, G: 143, B: 249, A: 255},
			},
			&series.Line3DSeries{
				SeriesName: "Profit",
				Data:       []float64{0, 0.3, 1, 1.2, 2, 0.8, 3, 2.0, 4, 1.5, 5, 2.8, 6, 2.2},
				ZPosition:  zPos(1.5),
				LineWidth:  0.25,
				Color:      color.NRGBA{R: 255, G: 99, B: 132, A: 255},
			},
			&series.Line3DSeries{
				SeriesName: "Cost",
				Data:       []float64{0, 0.2, 1, 1.1, 2, 1.0, 3, 1.1, 4, 1.0, 5, 1.2, 6, 1.3},
				ZPosition:  zPos(3),
				LineWidth:  0.25,
				Color:      color.NRGBA{R: 50, G: 205, B: 50, A: 255},
			},
		},
	})

	// ── 13. Funnel chart ──
	funnelChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Funnel", SubText: "Process Stages"},
		Grid:  &chart.Grid{Left: 20, Right: 20, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.FunnelSeries{
				SeriesName: "Sales",
				Data: []series.FunnelData{
					{Name: "Visits", Value: 1200},
					{Name: "Signups", Value: 800},
					{Name: "Trials", Value: 400},
					{Name: "Purchases", Value: 200},
					{Name: "Renewals", Value: 100},
				},
			},
		},
	})

	// ── 14. Boxplot chart ──
	boxplotChart := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Boxplot", SubText: "Statistical Distribution"},
		Tooltip: itemTooltip,
		XAxis:   []axis.Config{{Type: axis.TypeCategory, Data: []string{"A", "B", "C", "D"}}},
		YAxis:   []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.BoxplotSeries{
				SeriesName: "Distribution",
				Data: []series.BoxplotData{
					{Min: 5, Q1: 15, Median: 25, Q3: 35, Max: 50},
					{Min: 10, Q1: 20, Median: 30, Q3: 45, Max: 60},
					{Min: 8, Q1: 18, Median: 22, Q3: 30, Max: 42},
					{Min: 12, Q1: 25, Median: 35, Q3: 48, Max: 65},
				},
			},
		},
	})

	// ── 15. Treemap chart ──
	treemapChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Treemap", SubText: "Hierarchical Data"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.TreemapSeries{
				SeriesName: "Disk Usage",
				Data: []series.TreemapNode{
					{Name: "System", Value: 30},
					{Name: "Apps", Value: 25, Children: []series.TreemapNode{
						{Name: "Chrome", Value: 12},
						{Name: "Slack", Value: 8},
						{Name: "Other", Value: 5},
					}},
					{Name: "Documents", Value: 20},
					{Name: "Media", Value: 15},
					{Name: "Downloads", Value: 10},
				},
			},
		},
	})

	// ── 16. Sankey chart ──
	sankeyChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Sankey", SubText: "Flow Diagram"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.SankeySeries{
				SeriesName: "Energy",
				Nodes: []series.SankeyNode{
					{Name: "Solar"},
					{Name: "Wind"},
					{Name: "Coal"},
					{Name: "Grid"},
					{Name: "Homes"},
					{Name: "Industry"},
				},
				Links: []series.SankeyLink{
					{Source: 0, Target: 3, Value: 40},
					{Source: 1, Target: 3, Value: 30},
					{Source: 2, Target: 3, Value: 50},
					{Source: 3, Target: 4, Value: 60},
					{Source: 3, Target: 5, Value: 60},
				},
			},
		},
	})

	// ── 17. Sunburst chart ──
	sunburstChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Sunburst", SubText: "Nested Hierarchy"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.SunburstSeries{
				SeriesName: "Revenue",
				Data: []series.SunburstNode{
					{Name: "US", Value: 50, Children: []series.SunburstNode{
						{Name: "East", Value: 20},
						{Name: "West", Value: 30},
					}},
					{Name: "EU", Value: 30, Children: []series.SunburstNode{
						{Name: "DE", Value: 15},
						{Name: "FR", Value: 15},
					}},
					{Name: "Asia", Value: 20},
				},
			},
		},
	})

	// ── 18. Graph chart ──
	graphChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Graph", SubText: "Network Diagram"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.GraphSeries{
				SeriesName: "Network",
				Nodes: []series.GraphNode{
					{Name: "Server"},
					{Name: "DB"},
					{Name: "Cache"},
					{Name: "App1"},
					{Name: "App2"},
					{Name: "LB"},
				},
				Edges: []series.GraphEdge{
					{Source: 5, Target: 0, Value: 10},
					{Source: 5, Target: 3, Value: 8},
					{Source: 5, Target: 4, Value: 8},
					{Source: 0, Target: 1, Value: 5},
					{Source: 0, Target: 2, Value: 5},
					{Source: 3, Target: 1, Value: 3},
					{Source: 4, Target: 2, Value: 3},
				},
			},
		},
	})

	// ── 19. Tree chart ──
	treeChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Tree", SubText: "Org Structure"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.TreeSeries{
				SeriesName: "Org",
				Data: series.TreeNode{
					Name: "CEO",
					Children: []series.TreeNode{
						{Name: "CTO", Children: []series.TreeNode{
							{Name: "Eng1"}, {Name: "Eng2"}, {Name: "Eng3"},
						}},
						{Name: "CFO", Children: []series.TreeNode{
							{Name: "Acct"}, {Name: "Fin"},
						}},
					},
				},
			},
		},
	})

	// ── 20. Parallel coordinates ──
	parallelChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Parallel", SubText: "Multi-Dimensional"},
		Grid:  &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.ParallelSeries{
				SeriesName: "Cars",
				Axes: []series.ParallelAxis{
					{Name: "HP"}, {Name: "MPG"}, {Name: "Weight"}, {Name: "0-60"},
				},
				Data: [][]float64{
					{130, 28, 3200, 8.5},
					{250, 18, 3800, 6.2},
					{180, 24, 3400, 7.1},
					{320, 15, 4200, 5.4},
					{150, 32, 2900, 9.0},
				},
			},
		},
	})

	// ── 21. Stacked Bar ──
	stackedBarChart := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Stacked Bar", SubText: "Stack + Decal Patterns"},
		Legend:  legend,
		Tooltip: itemTooltip,
		Decal:   &component.DecalConfig{Enabled: true, Patterns: []component.DecalPattern{component.DecalStripe, component.DecalDot, component.DecalCross}},
		XAxis:   []axis.Config{{Type: axis.TypeCategory, Data: []string{"Mon", "Tue", "Wed", "Thu", "Fri"}}},
		YAxis:   []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.BarSeries{SeriesName: "Email", Data: []float64{120, 132, 101, 134, 90}, Stack: "total", CornerRadius: 0},
			&series.BarSeries{SeriesName: "Ads", Data: []float64{220, 182, 191, 234, 290}, Stack: "total", CornerRadius: 0},
			&series.BarSeries{SeriesName: "Video", Data: []float64{150, 232, 201, 154, 190}, Stack: "total", CornerRadius: 2},
		},
	})

	// ── 22. Pictorial Bar ──
	pictorialChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Pictorial Bar", SubText: "Symbol Fill"},
		XAxis: []axis.Config{{Type: axis.TypeCategory, Data: []string{"US", "EU", "CN", "JP"}}},
		YAxis: []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.PictorialSeries{
				SeriesName: "Population",
				Data:       []float64{330, 450, 1400, 125},
				Symbol:     series.SymbolCircle,
				SymbolSize: 10,
			},
		},
	})

	// ── 23. Effect Scatter ──
	effectChart := chart.New(chart.Option{
		Title:     &chart.Title{Text: "Effect Scatter", SubText: "Ripple Animation"},
		Animation: true,
		XAxis:     []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		YAxis:     []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.EffectScatterSeries{
				SeriesName:  "Events",
				Data:        []float64{5, 28, 8, 35, 12, 42, 15, 20, 3, 50, 10, 15, 7, 38},
				SymbolSize:  10,
				RippleScale: 2.5,
			},
		},
	})

	// ── 24. Custom Series ──
	customChart := chart.New(chart.Option{
		Title: &chart.Title{Text: "Custom Series", SubText: "User-Defined"},
		Grid:  &chart.Grid{Left: 20, Right: 20, Top: 50, Bottom: 20},
		Series: []series.Series{
			&series.CustomSeries{
				SeriesName: "Custom",
				RenderFunc: func(ctx series.RenderContext) []core.CanvasObject {
					vp := ctx.Viewport
					cx, cy := vp.X+vp.W/2, vp.Y+vp.H/2
					var objs []core.CanvasObject
					colors := []color.NRGBA{
						{R: 91, G: 143, B: 249, A: 200},
						{R: 250, G: 200, B: 88, A: 200},
						{R: 116, G: 211, B: 169, A: 200},
					}
					for i := 0; i < 3; i++ {
						r := float32(30 + i*25)
						c := canvas.NewCircle(color.Transparent)
						c.StrokeColor = colors[i]
						c.StrokeWidth = 3
						c.Position1 = geo.NewPos(cx-r, cy-r)
						c.Position2 = geo.NewPos(cx+r, cy+r)
						objs = append(objs, c)
					}
					label := canvas.NewText("Custom!", colors[0])
					label.TextSize = 14
					label.TextStyle = bellui.TextStyle{Bold: true}
					label.Alignment = bellui.TextAlignCenter
					label.Move(geo.NewPos(cx-30, cy-8))
					label.Resize(geo.NewSize(60, 16))
					objs = append(objs, label)
					return objs
				},
			},
		},
	})

	// ── Geo: World map with scatter + flight routes ──
	mapCenter := [2]float64{10, 30} // lon, lat centered on Atlantic
	mapZoom := float32(2)

	// API key entry + tile URL builder
	apiKeyEntry := widget.NewEntry()
	apiKeyEntry.SetPlaceHolder("Enter tile API key (or leave empty for OSM)")

	buildTileURL := func(key string) string {
		if key == "" {
			return "" // default OSM
		}
		// Common patterns: Mapbox, Thunderforest, Stadia, etc.
		// User can paste a full URL template with {z}/{x}/{y} or just a key
		if len(key) > 20 && (key[0] == 'h' || key[0] == '/') {
			return key // treat as full URL template
		}
		// Default: assume Thunderforest-style key appended to OSM
		return "https://tile.thunderforest.com/atlas/{z}/{x}/{y}.png?apikey=" + key
	}

	worldMap := chart.New(chart.Option{
		Title:   &chart.Title{Text: "World Map", SubText: "Tiles + Cities + Routes"},
		Tooltip: itemTooltipNoCrosshair,
		Grid:    &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.MapSeries{SeriesName: "Map", Center: mapCenter, Zoom: mapZoom},
			&series.GeoScatterSeries{
				SeriesName: "Cities",
				MapCenter:  mapCenter, MapZoom: mapZoom,
				Data: []float64{
					-74.0, 40.7, // New York
					-0.1, 51.5, // London
					2.3, 48.9, // Paris
					139.7, 35.7, // Tokyo
					116.4, 39.9, // Beijing
					-43.2, -22.9, // Rio
					28.0, -26.2, // Johannesburg
					151.2, -33.9, // Sydney
				},
				SymbolSize: 8,
				Color:      color.NRGBA{R: 255, G: 100, B: 100, A: 255},
			},
			&series.GeoLinesSeries{
				SeriesName: "Routes",
				MapCenter:  mapCenter, MapZoom: mapZoom,
				Data: []series.GeoLine{
					{From: [2]float64{-74.0, 40.7}, To: [2]float64{-0.1, 51.5}},
					{From: [2]float64{-0.1, 51.5}, To: [2]float64{139.7, 35.7}},
					{From: [2]float64{139.7, 35.7}, To: [2]float64{151.2, -33.9}},
				},
				Curved: false,
				Width:  2,
				Color:  color.NRGBA{R: 91, G: 143, B: 249, A: 200},
			},
		},
	})

	choroplethMap := chart.New(chart.Option{
		Title:   &chart.Title{Text: "Choropleth", SubText: "World GDP by Country"},
		Tooltip: itemTooltipNoCrosshair,
		Grid:    &chart.Grid{Left: 10, Right: 10, Top: 50, Bottom: 10},
		Series: []series.Series{
			&series.MapSeries{SeriesName: "Base", Center: mapCenter, Zoom: mapZoom},
			&series.ChoroplethSeries{
				SeriesName: "GDP",
				GeoJSONURL: "https://raw.githubusercontent.com/datasets/geo-countries/master/data/countries.geojson",
				MapCenter:  mapCenter,
				MapZoom:    mapZoom,
				RegionData: map[string]float64{
					"United States of America": 25.5, "China": 18.3, "Japan": 4.2,
					"Germany": 4.1, "India": 3.7, "United Kingdom": 3.1,
					"France": 2.8, "Canada": 2.1, "Italy": 2.0, "Brazil": 1.9,
					"Russia": 1.8, "Australia": 1.7, "South Korea": 1.7,
					"Mexico": 1.3, "Spain": 1.4, "Indonesia": 1.3,
				},
				Min: 0, Max: 26,
			},
		},
	})

	// Update tile URL when API key changes
	apiKeyEntry.OnChanged = func(key string) {
		tileURL := buildTileURL(key)
		opt := worldMap.Option()
		for _, s := range opt.Series {
			if ms, ok := s.(*series.MapSeries); ok {
				ms.TileURL = tileURL
			}
		}
		worldMap.SetOption(opt)

		opt2 := choroplethMap.Option()
		for _, s := range opt2.Series {
			if ms, ok := s.(*series.MapSeries); ok {
				ms.TileURL = tileURL
			}
		}
		choroplethMap.SetOption(opt2)
	}

	// ── Trading Features Demo ──
	// Showcases: dashed lines, multi-pane sync, incremental append, follow-latest, keyboard

	// 1. Dashed line styles — MACD-style indicator overlay with dashed EMA lines
	dashedChart := chart.New(chart.Option{
		Title:    &chart.Title{Text: "Dashed Lines", SubText: "MACD-Style Indicators"},
		Legend:   legend,
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairVertical},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.1, MaxSpan: 1.0, ZoomSensitivity: 0.1},
		Grid:     &chart.Grid{Left: 50, Right: 20, Top: 50, Bottom: 40},
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: []string{"9:30", "9:35", "9:40", "9:45", "9:50", "9:55", "10:00", "10:05", "10:10", "10:15"}}},
		YAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Overlays: []chart.Overlay{
			&chart.ReferenceLine{Axis: chart.AxisHorizontal, Value: 55, Dashed: true, DashPattern: []float32{8, 3}, Label: "Support", Color: color.NRGBA{R: 50, G: 205, B: 50, A: 200}},
			&chart.ReferenceLine{Axis: chart.AxisHorizontal, Value: 72, Dashed: true, DashPattern: []float32{4, 4}, Label: "Resistance", Color: color.NRGBA{R: 255, G: 80, B: 80, A: 200}},
		},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "Price",
				Data:       []float64{52, 58, 55, 62, 68, 65, 70, 67, 72, 69},
				Symbol:     series.SymbolCircle,
			},
			&series.LineSeries{
				SeriesName: "Fast EMA",
				Data:       []float64{52, 55, 55, 58, 63, 64, 67, 67, 69, 69},
				LineStyle:  series.LineStyle{Dash: []float32{6, 4}, Color: color.NRGBA{R: 0, G: 200, B: 200, A: 200}},
			},
			&series.LineSeries{
				SeriesName: "Slow EMA",
				Data:       []float64{52, 54, 54, 56, 59, 61, 63, 64, 66, 67},
				LineStyle:  series.LineStyle{Dash: []float32{10, 5}, Color: color.NRGBA{R: 200, G: 100, B: 255, A: 200}},
			},
		},
		Animation: true,
	})

	// 2. Multi-pane chart group — price + volume synchronized
	priceChart := chart.New(chart.Option{
		Title:    &chart.Title{Text: "Price (Multi-Pane)", SubText: "Zoom syncs with volume"},
		Legend:   legend,
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairVertical},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.05, MaxSpan: 1.0, ZoomSensitivity: 0.08},
		Grid:     &chart.Grid{Left: 50, Right: 20, Top: 50, Bottom: 10},
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: []string{"9:30", "9:35", "9:40", "9:45", "9:50", "9:55", "10:00", "10:05", "10:10", "10:15", "10:20", "10:25", "10:30"}}},
		YAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 4}},
		Series: []series.Series{
			&series.CandlestickSeries{
				SeriesName: "AAPL",
				Data: []series.CandlestickData{
					{Open: 150, High: 155, Low: 148, Close: 153},
					{Open: 153, High: 158, Low: 151, Close: 156},
					{Open: 156, High: 160, Low: 154, Close: 155},
					{Open: 155, High: 157, Low: 150, Close: 152},
					{Open: 152, High: 159, Low: 151, Close: 158},
					{Open: 158, High: 165, Low: 157, Close: 163},
					{Open: 163, High: 168, Low: 161, Close: 166},
					{Open: 166, High: 170, Low: 163, Close: 165},
					{Open: 165, High: 167, Low: 160, Close: 162},
					{Open: 162, High: 166, Low: 159, Close: 164},
					{Open: 164, High: 169, Low: 162, Close: 168},
					{Open: 168, High: 172, Low: 166, Close: 170},
					{Open: 170, High: 175, Low: 168, Close: 173},
				},
			},
		},
	})
	volumeChart := chart.New(chart.Option{
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairVertical},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.05, MaxSpan: 1.0, ZoomSensitivity: 0.08},
		Grid:     &chart.Grid{Left: 50, Right: 20, Top: 5, Bottom: 30},
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: []string{"9:30", "9:35", "9:40", "9:45", "9:50", "9:55", "10:00", "10:05", "10:10", "10:15", "10:20", "10:25", "10:30"}}},
		YAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 3}},
		Series: []series.Series{
			&series.BarSeries{
				SeriesName: "Volume",
				Data:       []float64{1200, 1800, 900, 2100, 1500, 2400, 1900, 1100, 1700, 2000, 1400, 2200, 1600},
			},
		},
	})
	chartGroup := chart.NewChartGroup(
		chart.Pane{Chart: priceChart, Height: 3},
		chart.Pane{Chart: volumeChart, Height: 1},
	)

	// 3. Real-time streaming with AppendData + FollowLatest
	streamChart := chart.New(chart.Option{
		Title:    &chart.Title{Text: "Live Streaming", SubText: "AppendData + FollowLatest"},
		Legend:   legend,
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairBoth},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.05, MaxSpan: 1.0, ZoomSensitivity: 0.08},
		Grid:     &chart.Grid{Left: 50, Right: 20, Top: 50, Bottom: 40},
		XAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 6}},
		YAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "Bid",
				Data:       []float64{100, 102, 101, 103, 105, 104, 106, 108, 107, 109},
				LineStyle:  series.LineStyle{Color: color.NRGBA{R: 60, G: 180, B: 75, A: 255}},
			},
			&series.LineSeries{
				SeriesName: "Ask",
				Data:       []float64{101, 103, 102, 104, 106, 105, 107, 109, 108, 110},
				LineStyle:  series.LineStyle{Color: color.NRGBA{R: 255, G: 100, B: 80, A: 255}, Dash: []float32{4, 3}},
			},
		},
	})
	streamChart.SetFollowLatest(true)
	// Simulate live data feed — append a new tick every 500ms
	go func() {
		tick := 10
		bidPrice := 109.0
		askPrice := 110.0
		for {
			time.Sleep(500 * time.Millisecond)
			tick++
			bidPrice += (float64(tick%3) - 1) * 0.5
			askPrice = bidPrice + 1
			streamChart.AppendData(0, bidPrice)
			streamChart.AppendData(1, askPrice)
			if tick > 200 {
				break
			}
		}
	}()

	// 4. Drawing tools — interactive drawing on chart
	drawChart := chart.New(chart.Option{
		Title:    &chart.Title{Text: "Drawing Tools", SubText: "Select tool → click/drag on chart"},
		Legend:   legend,
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairBoth},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.1, MaxSpan: 1.0, ZoomSensitivity: 0.1},
		Grid:     &chart.Grid{Left: 50, Right: 20, Top: 50, Bottom: 40},
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: []string{"9:30", "9:35", "9:40", "9:45", "9:50", "9:55", "10:00", "10:05", "10:10", "10:15", "10:20", "10:25", "10:30"}}},
		YAxis:    []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "Price",
				Data:       []float64{100, 105, 103, 108, 112, 110, 115, 118, 116, 120, 117, 122, 125},
				Symbol:     series.SymbolCircle,
			},
		},
	})

	// How to use each tool, shown when it is chosen.
	toolHints := map[interact.DrawingToolMode]string{
		interact.DrawingToolNone:           "click a drawing to select it, drag to move",
		interact.DrawingToolHLine:          "click on chart",
		interact.DrawingToolVLine:          "click on chart",
		interact.DrawingToolTrendLine:      "2 clicks",
		interact.DrawingToolRay:            "2 clicks",
		interact.DrawingToolRectangle:      "2 clicks",
		interact.DrawingToolChannel:        "3 clicks",
		interact.DrawingToolFibRetracement: "2 clicks",
		interact.DrawingToolTextLabel:      "click, type, Enter",
		interact.DrawingToolMeasure:        "drag",
	}
	toolStatus := func(tool interact.DrawingToolMode) string {
		return fmt.Sprintf("Tool: %s — %s", tool, toolHints[tool])
	}
	drawStatusLabel := canvas.NewText(toolStatus(interact.DrawingToolNone), color.NRGBA{R: 180, G: 180, B: 180, A: 220})
	drawStatusLabel.TextSize = 12
	drawChart.Events.On(interact.EventDrawingCreated, func(evt interface{}) {
		e := evt.(interact.DrawingEvent)
		drawStatusLabel.Text = fmt.Sprintf("Created: %s", e.DrawingID)
		drawStatusLabel.Refresh()
	})
	drawChart.Events.On(interact.EventDrawingDeleted, func(evt interface{}) {
		e := evt.(interact.DrawingEvent)
		drawStatusLabel.Text = fmt.Sprintf("Deleted: %s", e.DrawingID)
		drawStatusLabel.Refresh()
	})

	// The default drawing tools. The status line follows the active tool
	// through the chart's event, so it also tracks changes the toolbar makes.
	drawChart.Events.On(interact.EventDrawingToolChanged, func(evt interface{}) {
		drawStatusLabel.Text = toolStatus(evt.(interact.DrawingToolEvent).Tool)
		drawStatusLabel.Refresh()
	})
	toolButtons := chart.NewDrawingToolbar(drawChart)
	drawChartWithTools := container.NewBorder(
		container.NewVBox(toolButtons, drawStatusLabel),
		nil, nil, nil,
		drawChart,
	)

	// Tabbed layout — grouped by category, 2x2 grids per tab
	tabs := container.NewAppTabs(
		container.NewTabItem("Basic",
			container.NewGridWithColumns(2, lineChart, stackedBarChart, scatterChart, pieChart)),
		container.NewTabItem("Financial",
			container.NewGridWithColumns(2, candlestickChart, boxplotChart, heatmapChart, parallelChart)),
		container.NewTabItem("Hierarchical",
			container.NewGridWithColumns(2, treemapChart, sunburstChart, sankeyChart, treeChart)),
		container.NewTabItem("Specialized",
			container.NewGridWithColumns(2, radarChart, gaugeChart, funnelChart, graphChart)),
		container.NewTabItem("Advanced",
			container.NewGridWithColumns(2, pictorialChart, effectChart, customChart, builderChart)),
		container.NewTabItem("Drilldown",
			container.NewGridWithColumns(1, barChart)),
		container.NewTabItem("3D",
			container.NewGridWithColumns(2, bar3DChart, scatter3DChart, surfaceChart, line3DChart)),
		container.NewTabItem("Trading",
			container.NewGridWithColumns(2, dashedChart, chartGroup, streamChart, drawChartWithTools)),
		container.NewTabItem("Geo",
			container.NewBorder(
				container.NewBorder(
					nil, nil,
					widget.NewLabel("Tile API Key / URL:"), nil,
					apiKeyEntry,
				),
				nil, nil, nil,
				container.NewGridWithColumns(2, worldMap, choroplethMap),
			)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	bellui.ShowAndRun(w)
}
