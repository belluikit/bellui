package chart

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/animate"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/internal/pool"
	"github.com/belluikit/bellui/x/charts/series"
	"github.com/belluikit/bellui/x/charts/style"
)

// renderState holds per-render interaction state that is atomically published
// from the renderer to the Chart under a single lock acquisition.
type renderState struct {
	hitTester        *interact.HitTester
	legendLayout     *component.LegendLayout
	seriesColors     map[int]color.Color
	zoomButtons      *component.ZoomButtonsLayout
	toolboxLayout    *component.ToolboxLayout
	breadcrumbLayout *component.BreadcrumbLayout
	coordSystems     []coord.System // for PixelToData in gesture handlers
	plotRect         coord.Rect     // plot area bounds
}

// layoutResult holds the outputs of the layout-computation phase of rebuild().
type layoutResult struct {
	plotRect         coord.Rect
	grid             Grid // the margins plotRect was laid out in
	titleObjs        []core.CanvasObject
	breadcrumbObjs   []core.CanvasObject
	legendObjs       []core.CanvasObject
	legendLayout     *component.LegendLayout
	breadcrumbLayout *component.BreadcrumbLayout
}

// cachedRanges stores pre-computed data ranges to avoid re-scanning
// all series data on every render (animation frames, hover, etc.).
type cachedRanges struct {
	xMin, xMax  float64
	yRanges     []yRange
	stackTotals map[string][]float64
}

type yRange struct{ min, max float64 }

// chartRenderer implements core.WidgetRenderer for the Chart widget.
type chartRenderer struct {
	// laidOut and layoutSize remember the last Layout, so a Layout at the
	// same size does not rebuild (see Layout).
	laidOut    bool
	layoutSize geo.Size

	chart      *Chart
	objects    []core.CanvasObject
	textColor  color.Color // the theme's foreground, for series labels (RenderContext.TextColor)
	rebuilding bool        // re-entrancy guard to prevent refresh feedback loops
	pool       *pool.ObjectPool

	// Cached state for partial re-rendering (tooltip/brush-only updates).
	// On mouse-move events, only the interactive layer is rebuilt — the
	// base objects (background, axes, series, overlays, legend, controls)
	// are reused from the last full rebuild.
	cachedBaseObjs     []core.CanvasObject
	cachedSeriesColors map[int]color.Color
	cachedTooltipCfg   *component.TooltipConfig
	cachedPlotArea     component.PlotArea
	cachedYAxisSeries  map[int]pointerYAxis // where each series' Y axis is drawn, when not at the plot's left edge (see pointerPlotArea)
	cachedPalette      []color.Color
	cachedSize         geo.Size
	hasCachedBase      bool
}

func newChartRenderer(c *Chart) *chartRenderer {
	r := &chartRenderer{chart: c, pool: pool.New()}
	c.mu.Lock()
	c.pendingDirty = dirtyFull
	c.mu.Unlock()
	r.Refresh()
	return r
}

// Layout rebuilds the chart only when its size changed. A rebuild replaces
// every child object, and the canvas lays the chart out again when its
// children change — so rebuilding on every Layout made a loop that rebuilt
// every chart on every frame. For 3D charts each rebuild is a new Scene3D with
// a new GPU texture, which ran a phone's browser out of GPU memory in seconds
// (#396). Content changes go through Refresh and its dirty flags.
func (r *chartRenderer) Layout(size geo.Size) {
	if r.laidOut && size == r.layoutSize {
		return
	}
	r.laidOut, r.layoutSize = true, size
	r.fullRebuild(size)
}

// seriesRenderContext builds the context every series renderer receives, so the
// wiring common to all three render sites — the object pool, the animation
// state and the asynchronous-repaint seam — is declared once.
func (r *chartRenderer) seriesRenderContext(s series.Series, coordSys coord.System, plotRect coord.Rect,
	c color.Color, idx int, animActive bool, animProgress float32,
) series.RenderContext {
	return series.RenderContext{
		Series:       s,
		Coord:        coordSys,
		Viewport:     plotRect,
		Color:        c,
		SeriesIndex:  idx,
		Animate:      animActive,
		AnimProgress: animProgress,
		Pool:         r.pool,
		TextColor:    r.textColor,
		Invalidate:   r.invalidate,
	}
}

// invalidate requests a full re-render from off the render goroutine. It is
// handed to series renderers as RenderContext.Invalidate so an asynchronous
// loader (the choropleth's GeoJSON fetch) can draw its data as soon as it
// arrives, rather than waiting for an unrelated refresh.
func (r *chartRenderer) invalidate() {
	c := r.chart
	bellui.Do(func() { c.refreshDirty(dirtyFull) })
}

func (r *chartRenderer) MinSize() geo.Size {
	return geo.NewSize(200, 150)
}

func (r *chartRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *chartRenderer) Refresh() {
	size := r.chart.Size()

	// Consume pending dirty flags.
	r.chart.mu.Lock()
	dirty := r.chart.pendingDirty
	r.chart.pendingDirty = 0
	animActive := r.chart.animCtrl.Active
	r.chart.mu.Unlock()

	// Nothing to do — skip redundant rebuilds (e.g., bellui.Do callbacks
	// that arrive after the pending flags were already consumed).
	if dirty == 0 {
		return
	}

	// Partial refresh: only rebuild tooltip + brush when data/layout haven't changed.
	canPartial := dirty&dirtyFull == 0 && !animActive && r.hasCachedBase && size == r.cachedSize

	if canPartial {
		r.refreshInteractive()
	} else {
		r.fullRebuild(size)
	}
}

// refreshInteractive rebuilds only the interactive layer (tooltip + brush)
// using cached base objects from the last full rebuild. This avoids the cost
// of recomputing axes, series geometry, and the hit tester on every mouse move.
func (r *chartRenderer) refreshInteractive() {
	if r.rebuilding {
		return
	}
	r.rebuilding = true
	defer func() { r.rebuilding = false }()

	// Read fresh interaction state.
	r.chart.mu.RLock()
	tooltipState := r.chart.tooltipState
	brushState := r.chart.brushState
	drawingGesture := r.chart.drawingGesture
	capturing := r.chart.capturing
	r.chart.mu.RUnlock()

	// Create theme resolver (cheap — reads theme colors).
	th := r.chart.Theme()
	variant := bellui.CurrentApp().Settings().ThemeVariant()
	resolver := style.NewResolver(th, variant)
	if len(r.cachedPalette) > 0 {
		resolver.SetPalette(r.cachedPalette)
	}

	// Cached coordinate systems from last full rebuild.
	var coordSystems []coord.System
	var plotRect coord.Rect
	snap := r.chart.renderSnap
	if snap != nil {
		coordSystems = snap.coordSystems
		plotRect = snap.plotRect
	}

	// Rebuild only interactive layer objects.
	objects := make([]core.CanvasObject, 0, len(r.cachedBaseObjs)+32)
	objects = append(objects, r.cachedBaseObjs...)

	// Drawing preview + measure overlay.
	if drawingGesture.Active && len(coordSystems) > 0 {
		objects = append(objects, component.RenderDrawingPreview(
			&drawingGesture, tooltipState.MousePos, coordSystems, plotRect, resolver,
		)...)
	}
	if drawingGesture.MeasureActive && len(coordSystems) > 0 {
		objects = append(objects, component.RenderMeasureOverlay(
			drawingGesture.MeasureStart, drawingGesture.MeasureEnd, coordSystems,
		)...)
	}

	objects = append(objects, renderBrushOverlay(&brushState)...)
	if !capturing { // as in fullRebuild
		tooltipObjs := component.RenderTooltip(
			&tooltipState, r.cachedTooltipCfg, resolver,
			r.cachedSeriesColors, r.cachedSize,
			pointerPlotArea(r.cachedPlotArea, r.cachedYAxisSeries, &tooltipState),
		)
		objects = append(objects, tooltipObjs...)
	}

	r.objects = objects
	canvas.Refresh(r.chart)
}

func (r *chartRenderer) Destroy() {}

func (r *chartRenderer) fullRebuild(size geo.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	if r.rebuilding {
		return
	}
	r.rebuilding = true
	defer func() { r.rebuilding = false }()
	r.pool.Reset()

	r.chart.mu.RLock()
	opt := r.chart.option
	tooltipState := r.chart.tooltipState
	brushState := r.chart.brushState
	capturing := r.chart.capturing
	animProgress := r.chart.animCtrl.Progress
	animActive := r.chart.animCtrl.Active
	var zoomWindow interact.DataWindow
	if r.chart.zoomState != nil {
		zoomWindow = r.chart.zoomState.Window
	} else {
		zoomWindow = interact.FullWindow()
	}
	r.chart.mu.RUnlock()

	// Trigger entrance animation on first render
	if opt.Animation && !r.chart.animStarted {
		r.chart.animStarted = true
		r.chart.animCtrl.Enter(animate.DefaultEntranceDuration)
		animProgress = 0
		animActive = true
	}

	th := r.chart.Theme()
	variant := bellui.CurrentApp().Settings().ThemeVariant()
	resolver := style.NewResolver(th, variant)
	if len(opt.Color) > 0 {
		resolver.SetPalette(opt.Color)
	}
	r.textColor = resolver.TitleColor() // series labels on the plot (#343)

	var objects []core.CanvasObject

	// Background
	bg := canvas.NewRectangle(resolver.BackgroundColor())
	bg.Resize(size)
	objects = append(objects, bg)

	// Classify series: cartesian (line, bar, scatter, candlestick) vs self-layout (pie, radar, gauge) vs 3D
	var cartesianSeries []series.Series
	var selfLayoutSeries []series.Series
	var threeDSeries []series.Series

	for _, s := range opt.Series {
		switch s.SeriesType() {
		case series.TypeBar3D, series.TypeScatter3D, series.TypeSurface, series.TypeLine3D:
			threeDSeries = append(threeDSeries, s)
		case series.TypePie, series.TypeRadar, series.TypeGauge, series.TypeHeatmap,
			series.TypeFunnel, series.TypeTreemap, series.TypeSankey,
			series.TypeSunburst, series.TypeGraph, series.TypeTree,
			series.TypeParallel, series.TypeCustom,
			series.TypeMap, series.TypeGeoScatter, series.TypeChoropleth, series.TypeGeoLines:
			selfLayoutSeries = append(selfLayoutSeries, s)
		default:
			cartesianSeries = append(cartesianSeries, s)
		}
	}

	// Compute layout (title, breadcrumb, legend, plotRect)
	allNonCartesian := append(selfLayoutSeries, threeDSeries...)
	layout := r.computeLayout(size, opt, resolver, cartesianSeries, allNonCartesian)
	objects = append(objects, layout.titleObjs...)
	objects = append(objects, layout.breadcrumbObjs...)

	plotRect := layout.plotRect
	if plotRect.W <= 0 || plotRect.H <= 0 {
		r.objects = objects
		return
	}

	// Reset hit tester
	hitTester := interact.NewHitTester()
	seriesColors := make(map[int]color.Color)

	// Render cartesian series
	var cartesianCoords []*coord.Cartesian2D
	if len(cartesianSeries) > 0 {
		objs, cs := r.renderCartesian(opt, cartesianSeries, plotRect, layout.grid.Left, resolver, hitTester, seriesColors, zoomWindow, animActive, animProgress)
		objects = append(objects, objs...)
		cartesianCoords = cs
	}

	// Render self-layout series (pie, radar, gauge, etc.)
	if len(selfLayoutSeries) > 0 {
		objs := r.renderSelfLayout(selfLayoutSeries, plotRect, resolver, hitTester, seriesColors, animActive, animProgress)
		objects = append(objects, objs...)
	}

	// Render 3D series (bar3d, scatter3d, surface)
	if len(threeDSeries) > 0 {
		objs := r.render3DSeries(threeDSeries, plotRect, resolver, seriesColors, animActive, animProgress)
		objects = append(objects, objs...)
	}

	// Render overlays, legend, zoom buttons, toolbox (static controls)
	controlObjs, zoomButtonsLayout, toolboxLayout := r.renderOverlaysAndControls(
		opt, plotRect, size, resolver, cartesianCoords,
		layout, hitTester, seriesColors, capturing,
	)
	objects = append(objects, controlObjs...)

	// Build coord.System slice for renderState and drawing rendering.
	coordIfaces := make([]coord.System, len(cartesianCoords))
	for i, cs := range cartesianCoords {
		coordIfaces[i] = cs
	}

	// Render committed drawings (part of the base layer).
	r.chart.mu.RLock()
	var drawings []interact.Drawing
	var selectedID interact.DrawingID
	if r.chart.drawingStore != nil {
		drawings = r.chart.drawingStore.All()
	}
	selectedID = r.chart.selectedID
	drawingGesture := r.chart.drawingGesture
	r.chart.mu.RUnlock()

	if len(drawings) > 0 && len(coordIfaces) > 0 {
		drawingObjs := component.RenderDrawings(drawings, selectedID, coordIfaces, plotRect, resolver)
		objects = append(objects, drawingObjs...)
	}

	// Cache base objects (everything above) for partial re-rendering.
	baseObjs := make([]core.CanvasObject, len(objects))
	copy(baseObjs, objects)

	plotArea := component.PlotArea{X: plotRect.X, Y: plotRect.Y, W: plotRect.W, H: plotRect.H}
	var ySeries map[int]pointerYAxis
	if len(cartesianSeries) > 0 {
		plotArea.XAxisTop, plotArea.XAxisOffset, ySeries = r.pointerAxisSides(opt, cartesianSeries)
	}

	// Interactive layer: drawing preview + measure + brush + tooltip.
	if drawingGesture.Active && len(coordIfaces) > 0 {
		objects = append(objects, component.RenderDrawingPreview(
			&drawingGesture, tooltipState.MousePos, coordIfaces, plotRect, resolver,
		)...)
	}
	if drawingGesture.MeasureActive && len(coordIfaces) > 0 {
		objects = append(objects, component.RenderMeasureOverlay(
			drawingGesture.MeasureStart, drawingGesture.MeasureEnd, coordIfaces,
		)...)
	}
	objects = append(objects, renderBrushOverlay(&brushState)...)
	if !capturing { // a saved image leaves the tooltip and axis pointer out
		tooltipObjs := component.RenderTooltip(&tooltipState, opt.Tooltip, resolver, seriesColors, size,
			pointerPlotArea(plotArea, ySeries, &tooltipState))
		objects = append(objects, tooltipObjs...)
	}

	// Store interaction state back on the chart atomically.
	snap := &renderState{
		hitTester:        hitTester,
		legendLayout:     layout.legendLayout,
		seriesColors:     seriesColors,
		zoomButtons:      zoomButtonsLayout,
		toolboxLayout:    toolboxLayout,
		breadcrumbLayout: layout.breadcrumbLayout,
		coordSystems:     coordIfaces,
		plotRect:         plotRect,
	}
	r.chart.mu.Lock()
	r.chart.renderSnap = snap
	r.chart.mu.Unlock()

	// Persist cached state for future partial refreshes.
	r.cachedBaseObjs = baseObjs
	r.cachedSeriesColors = seriesColors
	r.cachedTooltipCfg = opt.Tooltip
	r.cachedPlotArea = plotArea
	r.cachedYAxisSeries = ySeries
	r.cachedPalette = opt.Color
	r.cachedSize = size
	r.hasCachedBase = true

	r.objects = objects
	canvas.Refresh(r.chart)
}

// computeLayout computes the grid, title, breadcrumb, and legend sizing.
// It returns a layoutResult with the plotRect and component objects.
func (r *chartRenderer) computeLayout(
	size geo.Size,
	opt Option,
	resolver *style.Resolver,
	cartesianSeries []series.Series,
	selfLayoutSeries []series.Series,
) layoutResult {
	// Compute grid
	grid := chartGrid(opt)
	if len(cartesianSeries) == 0 && len(selfLayoutSeries) > 0 && opt.Grid == nil {
		grid = Grid{Left: 20, Right: 20, Top: 10, Bottom: 10}
	}

	// Title
	titleCfg := (*component.TitleConfig)(nil)
	if opt.Title != nil {
		titleCfg = &component.TitleConfig{Text: opt.Title.Text, SubText: opt.Title.SubText}
	}
	titleObjs, titleHeight := component.RenderTitle(titleCfg, resolver, grid.Left, grid.Top, size.Width-grid.Left-grid.Right)

	// Breadcrumb trail (shown when drilldown stack is non-empty)
	r.chart.mu.RLock()
	drilldownPath := make([]string, len(r.chart.drilldownStack))
	for i, e := range r.chart.drilldownStack {
		drilldownPath[i] = e.Label
	}
	r.chart.mu.RUnlock()

	var breadcrumbObjs []core.CanvasObject
	var breadcrumbLayout *component.BreadcrumbLayout
	breadcrumbHeight := float32(0)
	if len(drilldownPath) > 0 {
		bcY := grid.Top + titleHeight
		breadcrumbObjs, breadcrumbLayout, breadcrumbHeight = component.RenderBreadcrumb(
			drilldownPath, resolver, grid.Left, bcY, size.Width-grid.Left-grid.Right,
		)
	}

	// Legend (rendered at the bottom, reserve space)
	legendItems := r.buildLegendItems(opt, resolver)
	legendHeight := float32(0)
	var legendObjs []core.CanvasObject
	var legendLayout *component.LegendLayout

	showLegend := opt.Legend != nil || len(opt.Series) > 1
	if showLegend && len(legendItems) > 0 {
		legendY := size.Height - grid.Bottom
		legendObjs, legendLayout, legendHeight = component.RenderLegend(
			legendItems, opt.Legend, grid.Left, legendY, size.Width-grid.Left-grid.Right, resolver,
		)
		// Adjust legend Y to be above the bottom margin
		legendY = size.Height - legendHeight - 5
		legendObjs, legendLayout, legendHeight = component.RenderLegend(
			legendItems, opt.Legend, grid.Left, legendY, size.Width-grid.Left-grid.Right, resolver,
		)
	}

	// The plot starts below the title, or below the zoom and toolbox buttons
	// when they reach lower (#663): they sit at the top right, over the plot's
	// first rows otherwise.
	header := titleHeight + breadcrumbHeight
	if bottom := controlsBottom(opt); bottom > 0 && bottom+controlsGap > header {
		header = bottom + controlsGap
	}
	var room axisSpace
	if len(cartesianSeries) > 0 {
		room = axisRoom(opt, grid, header, legendHeight > 0, resolver.LabelTextSize())
	}
	roomTop, roomBottom, roomLeft, roomRight := room.top, room.bottom, room.left, room.right
	plotRect := coord.Rect{
		X: grid.Left + roomLeft,
		Y: grid.Top + header + roomTop,
		W: size.Width - grid.Left - grid.Right - roomLeft - roomRight,
		H: size.Height - grid.Top - grid.Bottom - header - legendHeight - roomTop - roomBottom,
	}

	return layoutResult{
		plotRect:         plotRect,
		grid:             grid,
		titleObjs:        titleObjs,
		breadcrumbObjs:   breadcrumbObjs,
		legendObjs:       legendObjs,
		legendLayout:     legendLayout,
		breadcrumbLayout: breadcrumbLayout,
	}
}

// axisSpace is extra room on each side of the plot, beyond grid.
type axisSpace struct{ top, bottom, left, right float32 }

// axisRoom is how much further in than grid, the header and the legend
// already put it each of the plot's edges must move, so the axis text drawn
// beyond them stays clear of the title, breadcrumb, controls and legend and
// inside the chart. Text above the plot sits between the header and the plot,
// or within grid.Top when there is no header. An axis's Offset moves it, and
// its text, that much further out, so the room grows by the largest Offset on
// each side.
func axisRoom(opt Option, grid Grid, header float32, legend bool, labelSize float32) axisSpace {
	var top, bottom, left, right float32
	x := axis.DefaultConfig(axis.TypeCategory)
	if len(opt.XAxis) > 0 {
		x = opt.XAxis[0]
	}
	ys := opt.YAxis
	if len(ys) == 0 {
		ys = []axis.Config{axis.DefaultConfig(axis.TypeValue)}
	}

	if e := axisTopExtent(x, ys, labelSize); header > 0 {
		top = e
	} else {
		top = max(0, e-grid.Top)
	}

	// A bottom X axis's tick labels have always lived in grid.Bottom; only
	// its name, a second row, can outgrow it. The legend sits 5 px below the
	// plot's margin.
	// An Offset moves the whole axis, so it needs that much more room on top
	// of what the axis needed before.
	if !xAxisOnTop(x) {
		bottom = axisOffset(x)
		if x.Name != "" {
			avail := grid.Bottom
			if legend {
				avail -= 5
			}
			bottom += max(0, xAxisExtent(x, labelSize)-avail)
		}
	}

	// A left Y axis's labels fill the grid.Left band beside it, so an offset
	// left axis needs its Offset more. A right Y axis's labels need their
	// own width beyond grid.Right, plus the Offset.
	rightReach := float32(-1)
	for i, y := range ys {
		if yAxisOnRight(y, i) {
			rightReach = max(rightReach, axisOffset(y)+axisLabelGap+yRightLabelWidth)
		} else {
			left = max(left, axisOffset(y))
		}
	}
	if rightReach >= 0 {
		right = max(0, rightReach-grid.Right)
	}
	return axisSpace{top: top, bottom: bottom, left: left, right: right}
}

// axisOffset is how far an axis is drawn outward from its plot edge: its
// Offset, or 0 for a negative one.
func axisOffset(cfg axis.Config) float32 { return max(0, cfg.Offset) }

// chartGrid is the grid opt asks for: opt.Grid, or DefaultGrid. A chart with
// no cartesian series may lay its plot out in a tighter one (computeLayout).
func chartGrid(opt Option) Grid {
	if opt.Grid != nil {
		return *opt.Grid
	}
	return DefaultGrid()
}

// The zoom buttons and the toolbox sit at the top right, below grid.Top.
const (
	controlsTop     float32 = 2  // the first row's offset below grid.Top
	controlsRowStep float32 = 26 // the toolbox row's offset below the zoom row
	controlsGap     float32 = 4  // space between the lowest row and the plot
)

// controlsBottom is how far below grid.Top the zoom and toolbox buttons
// reach, or 0 when the chart shows neither.
func controlsBottom(opt Option) float32 {
	zoom := opt.DataZoom != nil && opt.DataZoom.Enabled
	toolbox := opt.Toolbox.HasButtons()
	switch {
	case zoom && toolbox:
		return controlsTop + controlsRowStep + component.ToolboxButtonHeight
	case zoom:
		return controlsTop + component.ZoomButtonHeight
	case toolbox:
		return controlsTop + component.ToolboxButtonHeight
	}
	return 0
}

// renderOverlaysAndControls renders overlays, legend, zoom buttons, and toolbox.
// Brush and tooltip are rendered separately as the "interactive layer".
func (r *chartRenderer) renderOverlaysAndControls(
	opt Option,
	plotRect coord.Rect,
	size geo.Size,
	resolver *style.Resolver,
	cartesianCoords []*coord.Cartesian2D,
	layout layoutResult,
	hitTester *interact.HitTester,
	seriesColors map[int]color.Color,
	hideControls bool, // lay the zoom buttons and toolbox out, but do not draw them (a capture)
) ([]core.CanvasObject, *component.ZoomButtonsLayout, *component.ToolboxLayout) {
	var objects []core.CanvasObject

	// Overlays (reference lines, bands, annotations) — after series, before legend
	if len(opt.Overlays) > 0 && len(cartesianCoords) > 0 {
		var oLines []component.OverlayReferenceLine
		var oBands []component.OverlayReferenceBand
		var oAnns []component.OverlayAnnotation
		for _, ov := range opt.Overlays {
			switch o := ov.(type) {
			case *ReferenceLine:
				oLines = append(oLines, component.OverlayReferenceLine{
					Axis: int(o.Axis), Value: o.Value, YAxisIndex: o.YAxisIndex,
					Color: o.Color, Width: o.Width, Dashed: o.Dashed,
					DashPattern: o.DashPattern, Label: o.Label,
				})
			case *ReferenceBand:
				oBands = append(oBands, component.OverlayReferenceBand{
					Axis: int(o.Axis), From: o.From, To: o.To, YAxisIndex: o.YAxisIndex,
					Color: o.Color, Label: o.Label,
				})
			case *Annotation:
				oAnns = append(oAnns, component.OverlayAnnotation{
					X: o.X, Y: o.Y, YAxisIndex: o.YAxisIndex,
					Text: o.Text, Color: o.Color, TextSize: o.TextSize, Arrow: o.Arrow,
				})
			}
		}
		coordIfaces := make([]coord.System, len(cartesianCoords))
		for i, cs := range cartesianCoords {
			coordIfaces[i] = cs
		}
		overlayObjs := component.RenderOverlays(oLines, oBands, oAnns, coordIfaces, plotRect, resolver)
		objects = append(objects, overlayObjs...)
	}

	// Legend objects (on top of chart)
	objects = append(objects, layout.legendObjs...)

	// Compute grid for button positioning
	grid := chartGrid(opt)

	// Zoom buttons at top-right, horizontal (if DataZoom enabled)
	var zoomButtonsLayout *component.ZoomButtonsLayout
	if opt.DataZoom != nil && opt.DataZoom.Enabled {
		btnRightX := size.Width - grid.Right
		btnY := grid.Top + controlsTop
		btnObjs, btnLayout := component.RenderZoomButtons(btnRightX, btnY, resolver)
		if !hideControls {
			objects = append(objects, btnObjs...)
		}
		zoomButtonsLayout = btnLayout
	}

	// Toolbox (below zoom buttons if present)
	var toolboxLayout *component.ToolboxLayout
	if opt.Toolbox != nil {
		tbX := size.Width - grid.Right - 100
		tbY := grid.Top + controlsTop
		if zoomButtonsLayout != nil {
			tbY += controlsRowStep // below zoom buttons
		}
		tbObjs, tbLayout := component.RenderToolbox(opt.Toolbox, tbX, tbY, resolver)
		if !hideControls {
			objects = append(objects, tbObjs...)
		}
		toolboxLayout = tbLayout
	}

	return objects, zoomButtonsLayout, toolboxLayout
}

func (r *chartRenderer) buildLegendItems(opt Option, resolver *style.Resolver) []component.LegendItem {
	items := make([]component.LegendItem, 0, len(opt.Series))
	for i, s := range opt.Series {
		name := s.Name()
		if name == "" {
			continue
		}
		items = append(items, component.LegendItem{
			Name:    name,
			Color:   resolver.SeriesColor(i),
			Visible: r.chart.IsSeriesVisible(name),
		})
	}
	return items
}

// seriesYAxisIndex returns the YAxisIndex for a series.
func seriesYAxisIndex(s series.Series) int {
	if yi, ok := s.(series.YAxisIndexer); ok {
		return yi.GetYAxisIndex()
	}
	return 0
}

func (r *chartRenderer) renderCartesian(
	opt Option,
	seriesList []series.Series,
	plotRect coord.Rect,
	leftBand float32, // the grid's left margin, where a left Y axis puts its labels
	resolver *style.Resolver,
	hitTester *interact.HitTester,
	seriesColors map[int]color.Color,
	zoomWindow interact.DataWindow,
	animActive bool, animProgress float32,
) ([]core.CanvasObject, []*coord.Cartesian2D) {
	var objects []core.CanvasObject

	xConfigs := opt.XAxis
	if len(xConfigs) == 0 {
		xConfigs = []axis.Config{axis.DefaultConfig(axis.TypeCategory)}
	}
	yConfigs := opt.YAxis
	if len(yConfigs) == 0 {
		yConfigs = []axis.Config{axis.DefaultConfig(axis.TypeValue)}
	}

	// Build one X scale (shared)
	xScale := axis.NewScale(xConfigs[0])

	// Build one Y scale per Y axis config
	yScales := make([]axis.Scale, len(yConfigs))
	for i, cfg := range yConfigs {
		yScales[i] = axis.NewScale(cfg)
	}

	// Use cached data ranges if available; recompute only when dirty.
	cached := r.getOrBuildRangeCache(seriesList, len(yScales))
	xMin, xMax := cached.xMin, cached.xMax
	yRanges := cached.yRanges
	_ = cached.stackTotals // stack totals are already reflected in yRanges

	// Apply zoom window to X range
	fullXSpan := xMax - xMin
	zoomedXMin := xMin + zoomWindow.Start*fullXSpan
	zoomedXMax := xMin + zoomWindow.End*fullXSpan
	xScale.SetRange(zoomedXMin, zoomedXMax)

	for i, yr := range yRanges {
		if math.IsInf(yr.min, 1) {
			yr.min, yr.max = 0, 1
		}
		yScales[i].SetRange(yr.min, yr.max)
	}

	// Build one coordinate system per Y axis, all sharing the same X scale and
	// bounds. An Inverse axis reverses the mapping itself, so series, hit
	// testing and pixel-to-data conversion all follow it.
	coordSystems := make([]*coord.Cartesian2D, len(yScales))
	for i, ys := range yScales {
		cs := coord.NewCartesian2D(xScale, ys)
		cs.SetBounds(plotRect)
		cs.SetInvert(xConfigs[0].Inverse, yConfigs[i].Inverse)
		coordSystems[i] = cs
	}

	// Render axes. By default the first Y axis draws the grid lines; it sits
	// on the left and every other Y axis on the right (see yAxisOnRight).
	objects = append(objects, r.renderXAxis(xConfigs[0], xScale, plotRect, resolver)...)
	for i, ys := range yScales {
		objects = append(objects, r.renderYAxis(yConfigs[i], ys, i, xConfigs[0], plotRect, leftBand, resolver)...)
	}

	// Shared stack offsets for stacked bar/line series
	stackOffsets := make(map[string][]float64)

	// Render series
	for _, s := range seriesList {
		idx := r.seriesIndex(s)
		c := resolver.SeriesColor(idx)
		seriesColors[idx] = c

		if !r.chart.IsSeriesVisible(s.Name()) {
			continue
		}

		yIdx := seriesYAxisIndex(s)
		if yIdx >= len(coordSystems) {
			yIdx = 0
		}
		coordSys := coordSystems[yIdx]

		renderer := series.NewRenderer(s.SeriesType())
		if renderer == nil {
			continue
		}
		ctx := r.seriesRenderContext(s, coordSys, plotRect, c, idx, animActive, animProgress)
		ctx.StackOffsets = stackOffsets
		objects = append(objects, renderer.Render(ctx)...)

		// Update stack offsets after rendering (for stacked bar/line)
		if st, ok := s.(series.Stackable); ok {
			group := st.StackGroup()
			data := st.StackData()
			if group != "" && len(data) > 0 {
				if _, exists := stackOffsets[group]; !exists {
					stackOffsets[group] = make([]float64, len(data))
				}
				for i, v := range data {
					if i < len(stackOffsets[group]) {
						stackOffsets[group][i] += v
					}
				}
			}
		}

		// Register hit areas via HitRegistrar interface
		if hr, ok := s.(series.HitRegistrar); ok {
			hr.RegisterHitAreas(hitTester, idx, coordSys, plotRect)
		}
	}

	return objects, coordSystems
}

func (r *chartRenderer) renderSelfLayout(
	selfLayoutSeries []series.Series,
	plotRect coord.Rect,
	resolver *style.Resolver,
	hitTester *interact.HitTester,
	seriesColors map[int]color.Color,
	animActive bool, animProgress float32,
) []core.CanvasObject {
	var objects []core.CanvasObject

	noneCoord := coord.NewNone()
	noneCoord.SetBounds(plotRect)

	// Build a shared GeoCoord if any MapSeries exists, so all geo series align
	var sharedGeo *coord.GeoCoord
	for _, s := range selfLayoutSeries {
		if ms, ok := s.(*series.MapSeries); ok {
			sharedGeo = coord.NewGeoCoord(ms.Center[0], ms.Center[1], ms.Zoom)
			sharedGeo.SetBounds(plotRect)
			break
		}
	}

	for _, s := range selfLayoutSeries {
		idx := r.seriesIndex(s)
		c := resolver.SeriesColor(idx)
		seriesColors[idx] = c

		renderer := series.NewRenderer(s.SeriesType())
		if renderer == nil {
			continue
		}
		// Use shared GeoCoord for geo series so they align with the map tiles
		coordSys := coord.System(noneCoord)
		if sharedGeo != nil {
			switch s.SeriesType() {
			case series.TypeMap, series.TypeGeoScatter, series.TypeGeoLines, series.TypeChoropleth:
				coordSys = sharedGeo
			}
		}

		ctx := r.seriesRenderContext(s, coordSys, plotRect, c, idx, animActive, animProgress)
		rendered := renderer.Render(ctx)
		objects = append(objects, rendered...)

		// Register pie hit testing
		if ps, ok := s.(*series.PieSeries); ok {
			r.registerPieHitTest(hitTester, idx, ps, plotRect)
		}
		if hs, ok := s.(*series.HeatmapSeries); ok {
			hitTester.AddHeatmapSeries(idx, hs.SeriesName, plotRect, hs.Rows, hs.Cols, hs.Data, hs.XLabels, hs.YLabels)
		}

		// Register choropleth regions for hit testing (tooltips)
		if cs, ok := s.(*series.ChoroplethSeries); ok && sharedGeo != nil && cs.RegionData != nil {
			var pixelData []float64
			var labels []string
			for name, val := range cs.RegionData {
				// Approximate region center: use a simple lookup for major countries
				lon, lat := approxCountryCenter(name)
				if lon != 0 || lat != 0 {
					pos := sharedGeo.DataToPixel(lon, lat)
					pixelData = append(pixelData, float64(pos.X), float64(pos.Y))
					labels = append(labels, fmt.Sprintf("%s: %.1f", name, val))
				}
			}
			hitTester.AddGeoPointsWithLabels(idx, cs.SeriesName, pixelData, labels)
		}

		// Register geo scatter points for hit testing (tooltips)
		if gs, ok := s.(*series.GeoScatterSeries); ok && sharedGeo != nil {
			// Project all points and register as scatter for proximity hit testing
			scatterData := make([]float64, gs.PointCount()*2)
			for pi := 0; pi < gs.PointCount(); pi++ {
				lon, lat := gs.Point(pi)
				pos := sharedGeo.DataToPixel(lon, lat)
				scatterData[pi*2] = float64(pos.X)
				scatterData[pi*2+1] = float64(pos.Y)
			}
			// Use a custom hit entry with pre-computed pixel positions
			hitTester.AddGeoPoints(idx, gs.SeriesName, scatterData)
		}

		// Capture shared GeoCoord for pan/zoom interaction
		if sharedGeo != nil {
			r.chart.mu.Lock()
			r.chart.geoCoord = sharedGeo
			r.chart.isGeo = true
			r.chart.mu.Unlock()
		}
	}

	return objects
}

// render3DSeries renders 3D series (Bar3D, Scatter3D, Surface) and captures
// the 3D scene for orbit camera controls. 3D series don't participate in
// hit testing, so no hitTester is needed.
func (r *chartRenderer) render3DSeries(
	threeDSeries []series.Series,
	plotRect coord.Rect,
	resolver *style.Resolver,
	seriesColors map[int]color.Color,
	animActive bool, animProgress float32,
) []core.CanvasObject {
	var objects []core.CanvasObject

	noneCoord := coord.NewNone()
	noneCoord.SetBounds(plotRect)

	var shared *canvas.Scene3D // the chart's single 3D scene, from the first series that renders one
	for _, s := range threeDSeries {
		idx := r.seriesIndex(s)
		c := resolver.SeriesColor(idx)
		seriesColors[idx] = c

		renderer := series.NewRenderer(s.SeriesType())
		if renderer == nil {
			continue
		}
		ctx := r.seriesRenderContext(s, noneCoord, plotRect, c, idx, animActive, animProgress)
		rendered := renderer.Render(ctx)

		// All 3D series share one scene: a later series adds its meshes to
		// the first series' scene instead of drawing a scene of its own. With
		// a scene per series, orbiting moved only the last one and the series
		// could not occlude each other in depth (#399).
		kept := rendered[:0:0]
		for _, obj := range rendered {
			if scene, ok := obj.(*canvas.Scene3D); ok && shared != nil {
				shared.Meshes = append(shared.Meshes, scene.Meshes...)
				continue
			}
			kept = append(kept, obj)
		}
		objects = append(objects, kept...)

		// Capture 3D scene for orbit controls
		for _, obj := range kept {
			if scene, ok := obj.(*canvas.Scene3D); ok {
				shared = scene
				r.chart.mu.Lock()
				if r.chart.orbitCam != nil {
					// Preserve existing orbit state across rebuilds
					scene.Camera = r.chart.orbitCam.Camera(scene.Camera.FOV, scene.Camera.Near, scene.Camera.Far)
				} else {
					r.chart.orbitCam = canvas.NewOrbitCamera(scene.Camera)
				}
				r.chart.scene3D = scene
				r.chart.is3D = true
				r.chart.mu.Unlock()
			}
		}
	}

	return objects
}

func (r *chartRenderer) registerPieHitTest(hitTester *interact.HitTester, idx int, ps *series.PieSeries, vp coord.Rect) {
	layout := ps.ComputePieLayout(vp)
	if layout == nil {
		return
	}

	startAngle := layout.StartAngle
	var sectors []interact.PieSectorData
	for i, d := range ps.Data {
		fraction := d.Value / layout.Total
		sweep := float32(fraction * 360)
		sectors = append(sectors, interact.MakePieSector(i, startAngle, startAngle+sweep, d.Name, d.Value))
		startAngle += sweep
	}

	hitTester.AddPieSeries(idx, ps.SeriesName, geo.NewPos(layout.CenterX, layout.CenterY), layout.OuterR, sectors)
}

func (r *chartRenderer) seriesIndex(s series.Series) int {
	r.chart.mu.RLock()
	defer r.chart.mu.RUnlock()
	for i, os := range r.chart.option.Series {
		if os == s {
			return i
		}
	}
	return 0
}

// pointerYAxis is where a series' Y axis is drawn, for its pointer tag: on
// which side of the plot, and how far out.
type pointerYAxis struct {
	right  bool
	offset float32
}

// pointerAxisSides reports where the axes the pointer tags sit beside are
// drawn: whether the X axis is on top and its Offset, and for each cartesian
// series (by series index) plotted against a Y axis on the right or offset
// from its edge, that axis's side and Offset. It resolves a series' Y axis as
// renderCartesian does, so the tag follows the axis the series is drawn
// against.
func (r *chartRenderer) pointerAxisSides(opt Option, cartesian []series.Series) (xTop bool, xOffset float32, ySeries map[int]pointerYAxis) {
	if len(opt.XAxis) > 0 {
		xTop = xAxisOnTop(opt.XAxis[0])
		xOffset = axisOffset(opt.XAxis[0])
	}
	ys := opt.YAxis
	for _, s := range cartesian {
		yIdx := seriesYAxisIndex(s)
		if yIdx < 0 || yIdx >= len(ys) {
			yIdx = 0 // as renderCartesian falls back
		}
		if yIdx >= len(ys) {
			continue
		}
		at := pointerYAxis{right: yAxisOnRight(ys[yIdx], yIdx), offset: axisOffset(ys[yIdx])}
		if at == (pointerYAxis{}) {
			continue
		}
		if ySeries == nil {
			ySeries = make(map[int]pointerYAxis)
		}
		ySeries[r.seriesIndex(s)] = at
	}
	return xTop, xOffset, ySeries
}

// pointerPlotArea is the plot area the tooltip draws the pointer tags
// against: base, with YAxisRight and YAxisOffset set for the Y axis of the
// first hit's series.
func pointerPlotArea(base component.PlotArea, ySeries map[int]pointerYAxis, state *component.TooltipState) component.PlotArea {
	if state != nil && len(state.Hits) > 0 {
		at := ySeries[state.Hits[0].SeriesIndex]
		base.YAxisRight, base.YAxisOffset = at.right, at.offset
	}
	return base
}

// xAxisOnTop reports whether an X axis is drawn along the top of the plot:
// for PositionTop. Any other position draws it along the bottom.
func xAxisOnTop(cfg axis.Config) bool {
	return cfg.Position == axis.PositionTop
}

// yAxisOnRight reports whether the Y axis at index is drawn on the right of
// the plot. PositionLeft and PositionRight choose a side; anything else —
// including the zero value, PositionBottom, which means nothing for a Y axis
// — keeps the default: the first Y axis on the left, every other one on the
// right.
func yAxisOnRight(cfg axis.Config, index int) bool {
	switch cfg.Position {
	case axis.PositionLeft:
		return false
	case axis.PositionRight:
		return true
	}
	return index > 0
}

// Axis text sits in rows of one label's height, the first a gap beyond the
// axis line.
const axisLabelGap float32 = 6

func axisRow(labelSize float32) float32 { return labelSize + 4 }

// yRightLabelWidth is the width of a right-hand Y axis's tick label boxes.
const yRightLabelWidth float32 = 50

// xAxisExtent is how far an X axis's tick labels, and its name when it has
// one, reach beyond the plot edge it is drawn along.
func xAxisExtent(cfg axis.Config, labelSize float32) float32 {
	e := axisLabelGap + axisRow(labelSize)
	if cfg.Name != "" {
		e += axisRow(labelSize)
	}
	return e
}

// yNameRise is how far a Y axis name's row sits above the top of the plot.
func yNameRise(labelSize float32) float32 { return labelSize + 8 }

// axisTopExtent is how far the axis text drawn above the plot reaches: an X
// axis at the top, then the Y axis names above that.
func axisTopExtent(x axis.Config, ys []axis.Config, labelSize float32) float32 {
	var e float32
	if xAxisOnTop(x) {
		e = axisOffset(x) + xAxisExtent(x, labelSize)
	}
	for _, y := range ys {
		if y.Name != "" {
			return e + yNameRise(labelSize)
		}
	}
	return e
}

// axisColors resolves an axis's line and label colours: the config's when
// set, the theme's otherwise.
func axisColors(cfg axis.Config, resolver *style.Resolver) (line, label color.Color) {
	line, label = cfg.LineColor, cfg.LabelColor
	if line == nil {
		line = resolver.AxisLineColor()
	}
	if label == nil {
		label = resolver.AxisLabelColor()
	}
	return line, label
}

// tickNorm is a tick's position along its axis in [0, 1], reversed for an
// Inverse axis.
func tickNorm(t axis.Tick, inverse bool) float32 {
	if inverse {
		return float32(1 - t.Norm)
	}
	return float32(t.Norm)
}

// textWidth estimates the drawn width of s at size, as the tick labels do.
func textWidth(s string, size float32) float32 {
	return float32(len(s)) * size * 0.6
}

// renderXAxis draws an X axis along the bottom or top of the plot, moved
// out by its Offset: the axis line, a tick mark and label per tick, and the
// axis name centred beyond the labels, less the parts cfg.Hide hides. With
// SplitLineShow it also draws vertical grid lines across the plot where
// splitTicks puts them: at its ticks, or between the bands of a category axis.
func (r *chartRenderer) renderXAxis(cfg axis.Config, xScale axis.Scale, plotRect coord.Rect, resolver *style.Resolver) []core.CanvasObject {
	var objects []core.CanvasObject
	labelSize := resolver.LabelTextSize()
	lineColor, labelColor := axisColors(cfg, resolver)
	top := xAxisOnTop(cfg)

	y := plotRect.Y + plotRect.H + axisOffset(cfg)
	dir := float32(1) // away from the plot: down for the bottom axis
	if top {
		y = plotRect.Y - axisOffset(cfg)
		dir = -1
	}

	// labelY places a box of height h at distance gap beyond the axis line.
	labelY := func(gap, h float32) float32 {
		if top {
			return y - gap - h
		}
		return y + gap
	}

	// One pass over the ticks: grid lines go under the axis line, tick marks
	// and labels over it.
	var splits, marks []core.CanvasObject
	splitLines := cfg.SplitLine == axis.SplitLineShow
	gridColor := resolver.GridLineColor()
	if splitLines {
		for _, tick := range splitTicks(cfg, xScale) {
			px := plotRect.X + tickNorm(tick, cfg.Inverse)*plotRect.W
			splits = append(splits, &canvas.Line{
				Position1:   geo.NewPos(px, plotRect.Y),
				Position2:   geo.NewPos(px, plotRect.Y+plotRect.H),
				StrokeColor: gridColor,
				StrokeWidth: 0.5,
			})
		}
	}
	for _, tick := range xScale.Ticks() {
		px := plotRect.X + tickNorm(tick, cfg.Inverse)*plotRect.W

		if !cfg.Hide.Ticks {
			marks = append(marks, &canvas.Line{
				Position1:   geo.NewPos(px, y),
				Position2:   geo.NewPos(px, y+4*dir),
				StrokeColor: lineColor,
				StrokeWidth: 1,
			})
		}

		if !cfg.Hide.Labels {
			label := canvas.NewText(tick.Label, labelColor)
			label.TextSize = labelSize
			label.Alignment = bellui.TextAlignCenter
			estWidth := textWidth(tick.Label, labelSize)
			label.Move(geo.NewPos(px-estWidth/2, labelY(axisLabelGap, axisRow(labelSize))))
			label.Resize(geo.NewSize(estWidth, axisRow(labelSize)))
			marks = append(marks, label)
		}
	}

	objects = append(objects, splits...)
	if !cfg.Hide.Line {
		objects = append(objects, &canvas.Line{
			Position1:   geo.NewPos(plotRect.X, y),
			Position2:   geo.NewPos(plotRect.X+plotRect.W, y),
			StrokeColor: lineColor,
			StrokeWidth: 1,
		})
	}
	objects = append(objects, marks...)

	if cfg.Name != "" {
		name := canvas.NewText(cfg.Name, labelColor)
		name.TextSize = labelSize
		name.Alignment = bellui.TextAlignCenter
		name.Move(geo.NewPos(plotRect.X, labelY(axisLabelGap+axisRow(labelSize), axisRow(labelSize))))
		name.Resize(geo.NewSize(plotRect.W, axisRow(labelSize)))
		objects = append(objects, name)
	}

	return objects
}

// splitTicks returns where an axis's grid lines go. A value, log or time axis
// draws them at its ticks. A category axis puts its ticks at the band centres,
// where its bars and points sit, so it draws its grid lines between the bands
// instead: n-1 lines for n visible categories, none at the plot edges (#757).
func splitTicks(cfg axis.Config, scale axis.Scale) []axis.Tick {
	ticks := scale.Ticks()
	if cfg.Type != axis.TypeCategory {
		return ticks
	}
	var between []axis.Tick
	for i := 1; i < len(ticks); i++ {
		between = append(between, axis.Tick{
			Value: (ticks[i-1].Value + ticks[i].Value) / 2,
			Norm:  (ticks[i-1].Norm + ticks[i].Norm) / 2,
		})
	}
	return between
}

// ySplitLines reports whether the Y axis at index draws grid lines: by
// default only the first one does.
func ySplitLines(cfg axis.Config, index int) bool {
	switch cfg.SplitLine {
	case axis.SplitLineShow:
		return true
	case axis.SplitLineHide:
		return false
	}
	return index == 0
}

// renderYAxis draws the Y axis at index on the side yAxisOnRight picks,
// moved out from the plot edge by its Offset: the axis line, a label per tick
// and the axis name above the axis, clear of an X axis drawn at the top
// (topX), less the parts cfg.Hide hides. A right-hand axis also draws tick
// marks. A left-hand axis's labels and name fill a band leftBand wide (the
// grid's left margin) ending at the axis. The first Y axis, or any with
// SplitLineShow, also draws horizontal grid lines where splitTicks puts them.
func (r *chartRenderer) renderYAxis(cfg axis.Config, yScale axis.Scale, index int, topX axis.Config, plotRect coord.Rect, leftBand float32, resolver *style.Resolver) []core.CanvasObject {
	var objects []core.CanvasObject
	labelSize := resolver.LabelTextSize()
	lineColor, labelColor := axisColors(cfg, resolver)
	gridColor := resolver.GridLineColor()
	right := yAxisOnRight(cfg, index)
	splitLines := ySplitLines(cfg, index)

	x := plotRect.X - axisOffset(cfg)
	if right {
		x = plotRect.X + plotRect.W + axisOffset(cfg)
	}

	if !cfg.Hide.Line {
		objects = append(objects, &canvas.Line{
			Position1:   geo.NewPos(x, plotRect.Y),
			Position2:   geo.NewPos(x, plotRect.Y+plotRect.H),
			StrokeColor: lineColor,
			StrokeWidth: 1,
		})
	}

	gridLine := func(tick axis.Tick) {
		// Screen Y grows downwards, so the top of the axis is norm 1.
		py := plotRect.Y + (1-tickNorm(tick, cfg.Inverse))*plotRect.H
		objects = append(objects, &canvas.Line{
			Position1:   geo.NewPos(plotRect.X, py),
			Position2:   geo.NewPos(plotRect.X+plotRect.W, py),
			StrokeColor: gridColor,
			StrokeWidth: 0.5,
		})
	}
	// A category axis's grid lines sit between its ticks, so they are drawn
	// first. Any other axis draws each one with its tick, in the order it
	// always did (TestAxisLiteralConfigs_RenderAsBefore).
	atTicks := splitLines && cfg.Type != axis.TypeCategory
	if splitLines && !atTicks {
		for _, tick := range splitTicks(cfg, yScale) {
			gridLine(tick)
		}
	}

	for _, tick := range yScale.Ticks() {
		// Screen Y grows downwards, so the top of the axis is norm 1.
		py := plotRect.Y + (1-tickNorm(tick, cfg.Inverse))*plotRect.H

		if atTicks {
			gridLine(tick)
		}

		if right && !cfg.Hide.Ticks {
			objects = append(objects, &canvas.Line{
				Position1:   geo.NewPos(x, py),
				Position2:   geo.NewPos(x+4, py),
				StrokeColor: lineColor,
				StrokeWidth: 1,
			})
		}
		if cfg.Hide.Labels {
			continue
		}
		label := canvas.NewText(tick.Label, labelColor)
		label.TextSize = labelSize
		if right {
			label.Alignment = bellui.TextAlignLeading
			label.Move(geo.NewPos(x+axisLabelGap, py-labelSize/2))
			label.Resize(geo.NewSize(yRightLabelWidth, axisRow(labelSize)))
		} else {
			label.Alignment = bellui.TextAlignTrailing
			label.Move(geo.NewPos(x-leftBand, py-labelSize/2))
			label.Resize(geo.NewSize(leftBand-resolver.Padding(), labelSize+4))
		}
		objects = append(objects, label)
	}

	if cfg.Name != "" {
		name := canvas.NewText(cfg.Name, labelColor)
		name.TextSize = labelSize
		ny := plotRect.Y - yNameRise(labelSize)
		if xAxisOnTop(topX) {
			ny -= axisOffset(topX) + xAxisExtent(topX, labelSize)
		}
		if right {
			name.Alignment = bellui.TextAlignLeading
			name.Move(geo.NewPos(x, ny))
			name.Resize(geo.NewSize(textWidth(cfg.Name, labelSize), labelSize+4))
		} else {
			name.Alignment = bellui.TextAlignTrailing
			name.Move(geo.NewPos(x-leftBand, ny))
			name.Resize(geo.NewSize(leftBand, labelSize+4))
		}
		objects = append(objects, name)
	}

	return objects
}

// approxCountryCenter returns approximate (lon, lat) for major countries.
func approxCountryCenter(name string) (float64, float64) {
	centers := map[string][2]float64{
		"United States of America": {-98, 39}, "China": {104, 35}, "Japan": {138, 36},
		"Germany": {10, 51}, "India": {79, 22}, "United Kingdom": {-2, 54},
		"France": {2, 47}, "Canada": {-106, 56}, "Italy": {12, 43}, "Brazil": {-52, -10},
		"Russia": {105, 62}, "Australia": {134, -25}, "South Korea": {128, 36},
		"Mexico": {-102, 24}, "Spain": {-4, 40}, "Indonesia": {118, -2},
		"Turkey": {35, 39}, "Saudi Arabia": {45, 24}, "Argentina": {-64, -34},
		"South Africa": {25, -29}, "Nigeria": {8, 10}, "Egypt": {30, 27},
		"Poland": {20, 52}, "Netherlands": {5, 52}, "Sweden": {16, 62},
	}
	if c, ok := centers[name]; ok {
		return c[0], c[1]
	}
	return 0, 0
}

func updateRange(min, max *float64, v float64) {
	if v < *min {
		*min = v
	}
	if v > *max {
		*max = v
	}
}

// getOrBuildRangeCache returns cached data ranges, rebuilding if dirty or missing.
func (r *chartRenderer) getOrBuildRangeCache(seriesList []series.Series, numYAxes int) *cachedRanges {
	r.chart.mu.RLock()
	cache := r.chart.rangeCache
	dirty := r.chart.rangeDirty
	r.chart.mu.RUnlock()

	if cache != nil && !dirty && len(cache.yRanges) == numYAxes {
		return cache
	}

	// Build fresh cache
	xMin, xMax := math.Inf(1), math.Inf(-1)
	yrs := make([]yRange, numYAxes)
	for i := range yrs {
		yrs[i] = yRange{math.Inf(1), math.Inf(-1)}
	}
	stackTotals := make(map[string][]float64)

	for _, s := range seriesList {
		if !r.chart.IsSeriesVisible(s.Name()) {
			continue
		}
		// Stack totals
		if st, ok := s.(series.Stackable); ok {
			group := st.StackGroup()
			data := st.StackData()
			if group != "" && len(data) > 0 {
				if _, exists := stackTotals[group]; !exists {
					stackTotals[group] = make([]float64, len(data))
				}
				for i, v := range data {
					if i < len(stackTotals[group]) {
						stackTotals[group][i] += v
					}
				}
			}
		}
		// Data ranges
		yIdx := seriesYAxisIndex(s)
		if yIdx >= numYAxes {
			yIdx = 0
		}
		if dr, ok := s.(series.DataRanger); ok {
			sxMin, sxMax, syMin, syMax := dr.DataRange()
			updateRange(&xMin, &xMax, sxMin)
			updateRange(&xMin, &xMax, sxMax)
			updateRange(&yrs[yIdx].min, &yrs[yIdx].max, syMin)
			updateRange(&yrs[yIdx].min, &yrs[yIdx].max, syMax)
		}
	}

	// Include stack totals in Y range
	for _, totals := range stackTotals {
		for _, t := range totals {
			updateRange(&yrs[0].min, &yrs[0].max, t)
		}
	}

	if math.IsInf(xMin, 1) {
		xMin, xMax = 0, 1
	}

	cache = &cachedRanges{
		xMin:        xMin,
		xMax:        xMax,
		yRanges:     yrs,
		stackTotals: stackTotals,
	}

	r.chart.mu.Lock()
	r.chart.rangeCache = cache
	r.chart.rangeDirty = false
	r.chart.mu.Unlock()

	return cache
}
