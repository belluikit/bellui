// Package chart provides the main Chart widget for charts.
package chart

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	fynecanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/animate"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
	"github.com/belluikit/bellui/x/charts/style"
)

// dirtyFlags indicates what state changed and needs re-rendering.
type dirtyFlags uint32

const (
	dirtyFull    dirtyFlags = 1 << iota // option, zoom, legend, resize, animation
	dirtyTooltip                        // hover/tooltip state change only
	dirtyBrush                          // brush selection state change only
	dirtyDrawing                        // drawing gesture/selection change (interactive layer)
)

// Chart is the main fycharts widget. It renders chart data using Fyne canvas primitives.
type Chart struct {
	widget.BaseWidget

	mu     sync.RWMutex
	option Option

	// Interaction state
	tooltipState  component.TooltipState
	legendToggles map[string]bool // series name -> hidden
	renderSnap    *renderState    // atomically published by renderer
	zoomState     *interact.ZoomState
	brushState    interact.BrushState
	animCtrl      *animate.Controller
	animStarted   bool       // tracks whether entrance animation has fired
	pendingDirty  dirtyFlags // what changed since last render; protected by mu

	// Renderer reference for direct rebuilds (bypassing widget.Refresh).
	renderer *chartRenderer

	// Drawing tools state
	drawingStore   *interact.DrawingStore
	drawingTool    interact.DrawingToolMode
	selectedID     interact.DrawingID
	drawingGesture interact.DrawingGestureState

	// Cached data ranges — invalidated by SetOption, legend toggles, drilldown.
	rangeCache *cachedRanges
	rangeDirty bool // true when cache needs rebuild

	// 3D orbit state
	orbitCam *fynecanvas.OrbitCamera
	scene3D  *fynecanvas.Scene3D // current 3D scene for camera updates
	is3D     bool                // true if the chart contains 3D series

	// Geo map state
	geoCoord *coord.GeoCoord // shared geo coord for map pan/zoom
	isGeo    bool

	// Drilldown state
	drilldownStack []drilldownEntry

	// Event bus
	Events *interact.EventBus

	// Convenience callbacks — shorthand alternatives to Events.On(...).
	// Both the callback and Events bus fire for the same event.
	// Deprecated: prefer Events.On(EventClick, ...) for new code.
	OnClick       func(interact.HitResult)
	OnZoomChange  func(interact.DataWindow)
	OnBrushSelect func(interact.BrushSelectEvent)
}

type drilldownEntry struct {
	Label  string
	Option Option
}

var (
	_ core.Widget      = (*Chart)(nil)
	_ event.Tappable   = (*Chart)(nil)
	_ event.Scrollable = (*Chart)(nil)
	_ event.Draggable  = (*Chart)(nil)
	_ event.Focusable  = (*Chart)(nil)
	_ event.Hoverable  = (*Chart)(nil)
	_ desktop.Keyable  = (*Chart)(nil)
)

// New creates a new Chart widget with the given option.
func New(opt Option) *Chart {
	c := &Chart{
		option:        opt,
		legendToggles: make(map[string]bool),
		renderSnap: &renderState{
			hitTester:    interact.NewHitTester(),
			seriesColors: make(map[int]color.Color),
		},
		Events:       interact.NewEventBus(),
		drawingStore: interact.NewDrawingStore(),
	}
	c.animCtrl = animate.NewController(func() { c.refreshDirty(dirtyFull) })
	if opt.DataZoom != nil {
		c.zoomState = interact.NewZoomState(*opt.DataZoom)
	}
	c.ExtendBaseWidget(c)
	return c
}

// refreshDirty marks specific state as changed and triggers a re-render.
// The renderer uses these flags to decide between a full rebuild or a
// lightweight interactive-layer-only refresh.
// Must be called from the Fyne main thread (event handlers).
func (c *Chart) refreshDirty(flags dirtyFlags) {
	c.mu.Lock()
	c.pendingDirty |= flags
	c.mu.Unlock()
	c.Refresh()
}

// SetOption replaces the chart configuration and triggers a re-render.
// If Animation is enabled, a transition animation morphs from the old state.
func (c *Chart) SetOption(opt Option) {
	c.mu.Lock()
	doTransition := c.option.Animation && opt.Animation && c.animStarted
	c.option = opt
	c.rangeDirty = true
	if opt.DataZoom != nil && c.zoomState == nil {
		c.zoomState = interact.NewZoomState(*opt.DataZoom)
	}
	c.mu.Unlock()

	if doTransition {
		c.animCtrl.Transition(animate.DefaultTransitionDuration)
	} else {
		c.refreshDirty(dirtyFull)
	}
}

// AppendData appends float64 values to the series at the given index.
// This is more efficient than SetOption for incremental data updates because
// it modifies the series data in-place, preserves legend toggles, and avoids
// copying the entire Option. The series must implement series.DataAppender
// (LineSeries, BarSeries, ScatterSeries all do).
// Safe to call from any goroutine.
func (c *Chart) AppendData(seriesIndex int, values ...float64) {
	c.mu.Lock()
	if seriesIndex < 0 || seriesIndex >= len(c.option.Series) {
		c.mu.Unlock()
		return
	}
	s := c.option.Series[seriesIndex]
	appender, ok := s.(series.DataAppender)
	if !ok {
		c.mu.Unlock()
		return
	}

	appender.AppendValues(values...)

	// When the user has zoomed away from the live edge (FollowLatest off),
	// preserve the current axis range so the view stays exactly where the
	// user put it. New data beyond the old range is clipped until the user
	// zooms/pans. This matches TradingView behavior.
	if c.zoomState == nil || !c.zoomState.IsZoomed() || c.zoomState.FollowLatest {
		c.rangeDirty = true
	}
	if c.zoomState != nil && c.zoomState.FollowLatest {
		c.zoomState.SnapToLatest()
	}
	c.pendingDirty |= dirtyFull
	c.mu.Unlock()
	// Schedule refresh on the Fyne main thread to avoid thread safety violations.
	bellui.Do(func() {
		c.Refresh()
	})
}

// AppendCandlestick appends candlestick data to the CandlestickSeries at the given index.
// Safe to call from any goroutine.
func (c *Chart) AppendCandlestick(seriesIndex int, candles ...series.CandlestickData) {
	c.mu.Lock()
	if seriesIndex < 0 || seriesIndex >= len(c.option.Series) {
		c.mu.Unlock()
		return
	}
	if cs, ok := c.option.Series[seriesIndex].(*series.CandlestickSeries); ok {
		cs.AppendCandles(candles...)
	}
	if c.zoomState == nil || !c.zoomState.IsZoomed() || c.zoomState.FollowLatest {
		c.rangeDirty = true
	}
	if c.zoomState != nil && c.zoomState.FollowLatest {
		c.zoomState.SnapToLatest()
	}
	c.pendingDirty |= dirtyFull
	c.mu.Unlock()
	bellui.Do(func() {
		c.Refresh()
	})
}

// Option returns a copy of the current chart option.
func (c *Chart) Option() Option {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.option
}

// CreateRenderer returns the widget renderer for this chart.
func (c *Chart) CreateRenderer() core.WidgetRenderer {
	r := newChartRenderer(c)
	c.renderer = r
	return r
}

// MinSize returns the minimum size for the chart widget.
func (c *Chart) MinSize() geo.Size {
	return geo.NewSize(200, 150)
}

// IsSeriesVisible returns whether a series is currently visible.
func (c *Chart) IsSeriesVisible(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	hidden, exists := c.legendToggles[name]
	return !exists || !hidden
}

// ZoomWindow returns the current data window, or a full window if zoom is disabled.
func (c *Chart) ZoomWindow() interact.DataWindow {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.zoomState == nil {
		return interact.FullWindow()
	}
	return c.zoomState.Window
}

// SetFollowLatest enables or disables auto-scroll mode. When enabled,
// AppendData and AppendCandlestick automatically snap the viewport to
// show the most recent data.
func (c *Chart) SetFollowLatest(follow bool) {
	c.mu.Lock()
	if c.zoomState != nil {
		c.zoomState.FollowLatest = follow
	}
	c.mu.Unlock()
}

// SetZoomWindow sets the data window programmatically.
func (c *Chart) SetZoomWindow(w interact.DataWindow) {
	c.mu.Lock()
	if c.zoomState != nil {
		c.zoomState.SetWindow(w.Start, w.End)
	}
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
}

// --- Drilldown navigation ---

// DrillInto pushes the current state and replaces the chart option with a child.
func (c *Chart) DrillInto(label string, childOpt Option) {
	c.mu.Lock()
	c.drilldownStack = append(c.drilldownStack, drilldownEntry{
		Label:  label,
		Option: c.option,
	})
	// Preserve drilldown config on child so multi-level works
	if childOpt.Drilldown == nil && c.option.Drilldown != nil {
		childOpt.Drilldown = c.option.Drilldown
	}
	c.option = childOpt
	c.legendToggles = make(map[string]bool)
	c.rangeDirty = true
	c.animStarted = false
	if childOpt.DataZoom != nil {
		c.zoomState = interact.NewZoomState(*childOpt.DataZoom)
	} else {
		c.zoomState = nil
	}
	c.mu.Unlock()

	c.Events.Emit(interact.DrilldownEvent{
		Event: interact.Event{Type: interact.EventDrilldown},
		Path:  c.DrillPath(),
	})

	if childOpt.Animation {
		c.animCtrl.Enter(animate.DefaultEntranceDuration)
	} else {
		c.refreshDirty(dirtyFull)
	}
}

// DrillUp pops one level from the drilldown stack.
func (c *Chart) DrillUp() {
	c.mu.Lock()
	n := len(c.drilldownStack)
	if n == 0 {
		c.mu.Unlock()
		return
	}
	entry := c.drilldownStack[n-1]
	c.drilldownStack = c.drilldownStack[:n-1]
	c.option = entry.Option
	c.legendToggles = make(map[string]bool)
	c.rangeDirty = true
	c.animStarted = false
	if c.option.DataZoom != nil {
		c.zoomState = interact.NewZoomState(*c.option.DataZoom)
	} else {
		c.zoomState = nil
	}
	c.mu.Unlock()

	c.Events.Emit(interact.DrilldownEvent{
		Event: interact.Event{Type: interact.EventDrillUp},
		Path:  c.DrillPath(),
	})
	c.refreshDirty(dirtyFull)
}

// drillToDepth pops the stack down to a specific depth.
func (c *Chart) drillToDepth(depth int) {
	c.mu.Lock()
	for len(c.drilldownStack) > depth+1 {
		c.drilldownStack = c.drilldownStack[:len(c.drilldownStack)-1]
	}
	if len(c.drilldownStack) > 0 {
		entry := c.drilldownStack[len(c.drilldownStack)-1]
		c.drilldownStack = c.drilldownStack[:len(c.drilldownStack)-1]
		c.option = entry.Option
		c.legendToggles = make(map[string]bool)
		c.rangeDirty = true
		c.animStarted = false
	}
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
}

// DrillPath returns the current breadcrumb path labels.
func (c *Chart) DrillPath() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	path := make([]string, len(c.drilldownStack))
	for i, e := range c.drilldownStack {
		path[i] = e.Label
	}
	return path
}

// --- Toolbox actions ---

func (c *Chart) handleToolboxAction(action string) {
	switch {
	case action == "save":
		c.SaveAsImage()
	case len(action) > 10 && action[:10] == "magicType:":
		targetType := action[10:]
		c.MagicTypeSwitch(targetType)
	}
}

// SaveAsImage captures the chart as a PNG file via a save dialog.
func (c *Chart) SaveAsImage() {
	// Render chart to image using Fyne's snapshot
	img := c.captureImage()
	if img == nil {
		return
	}

	w := bellui.CurrentApp().Driver().AllWindows()
	if len(w) == 0 {
		return
	}

	dialog.ShowFileSave(func(writer bellui.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		if err := png.Encode(writer, img); err != nil {
			log.Printf("fycharts: SaveAsImage png.Encode failed: %v", err)
		}
	}, w[0])
}

// captureImage renders the chart's current state to an image.
func (c *Chart) captureImage() image.Image {
	size := c.Size()
	w := int(size.Width)
	h := int(size.Height)
	if w <= 0 || h <= 0 {
		return nil
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	// Draw background
	c.mu.RLock()
	opt := c.option
	c.mu.RUnlock()

	th := c.Theme()
	variant := bellui.CurrentApp().Settings().ThemeVariant()
	resolver := style.NewResolver(th, variant)

	bgColor := resolver.BackgroundColor()
	if bgColor != nil {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.Set(x, y, bgColor)
			}
		}
	}
	_ = opt
	return img
}

// MagicTypeSwitch swaps series between bar and line types.
func (c *Chart) MagicTypeSwitch(targetType string) {
	c.mu.Lock()
	changed := false
	newSeries := make([]series.Series, len(c.option.Series))
	for i, s := range c.option.Series {
		switch ts := s.(type) {
		case *series.BarSeries:
			if targetType == "line" {
				newSeries[i] = &series.LineSeries{
					SeriesName: ts.SeriesName,
					Data:       ts.Data,
					YAxisIndex: ts.YAxisIndex,
					Symbol:     series.SymbolCircle,
					Stack:      ts.Stack,
				}
				changed = true
				continue
			}
		case *series.LineSeries:
			if targetType == "bar" {
				newSeries[i] = &series.BarSeries{
					SeriesName:   ts.SeriesName,
					Data:         ts.Data,
					YAxisIndex:   ts.YAxisIndex,
					Stack:        ts.Stack,
					CornerRadius: 3,
				}
				changed = true
				continue
			}
		}
		newSeries[i] = s
	}
	if changed {
		c.option.Series = newSeries
	}
	c.mu.Unlock()
	if changed {
		c.refreshDirty(dirtyFull)
	}
}

// updateGeoSeriesCoords synchronizes all geo series' center/zoom with the shared GeoCoord.
// Must be called while holding c.mu.Lock.
func (c *Chart) updateGeoSeriesCoords() {
	if c.geoCoord == nil {
		return
	}
	center := [2]float64{c.geoCoord.CenterLon, c.geoCoord.CenterLat}
	zoom := c.geoCoord.Zoom
	for _, s := range c.option.Series {
		switch ts := s.(type) {
		case *series.MapSeries:
			ts.Center = center
			ts.Zoom = zoom
		case *series.GeoScatterSeries:
			ts.MapCenter = center
			ts.MapZoom = zoom
		case *series.GeoLinesSeries:
			ts.MapCenter = center
			ts.MapZoom = zoom
		case *series.ChoroplethSeries:
			ts.MapCenter = center
			ts.MapZoom = zoom
		}
	}
}

// --- Hoverable ---

// MouseIn is called when the cursor enters the chart widget.
func (c *Chart) HoverIn(e *event.HoverEvent) {
	c.handleHover(e.Position)
}

// MouseMoved is called when the cursor moves within the chart widget.
func (c *Chart) HoverMoved(e *event.HoverEvent) {
	c.handleHover(e.Position)
}

// MouseOut is called when the cursor leaves the chart widget.
func (c *Chart) HoverOut() {
	c.mu.Lock()
	if c.tooltipState.Pinned {
		c.mu.Unlock()
		return
	}
	c.tooltipState.Visible = false
	c.tooltipState.Hits = nil
	c.pendingDirty |= dirtyTooltip
	c.mu.Unlock()
	c.Refresh()
	c.Events.Emit(interact.Event{Type: interact.EventMouseOut})
}

func (c *Chart) handleHover(pos geo.Position) {
	changed := c.resolveHover(pos)
	if changed {
		c.refreshDirty(dirtyTooltip)
		// Emit hover event for ChartGroup crosshair sync.
		c.mu.RLock()
		if len(c.tooltipState.Hits) > 0 {
			hit := c.tooltipState.Hits[0]
			hit.PixelPos = pos
			c.mu.RUnlock()
			c.Events.Emit(interact.DataPointEvent{
				Event:       interact.Event{Type: interact.EventHover},
				SeriesIndex: hit.SeriesIndex,
				SeriesName:  hit.SeriesName,
				DataIndex:   hit.DataIndex,
				Value:       hit.Value,
				PixelPos:    pos,
			})
		} else {
			c.mu.RUnlock()
		}
	}
}

// resolveHover updates tooltip state under lock. Returns true if state changed.
func (c *Chart) resolveHover(pos geo.Position) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := c.renderSnap
	if snap == nil || snap.hitTester == nil {
		return false
	}

	tooltipCfg := c.option.Tooltip
	trigger := component.TooltipTriggerItem
	if tooltipCfg != nil {
		trigger = tooltipCfg.Trigger
	}

	if trigger == component.TooltipTriggerNone {
		c.tooltipState.Visible = false
		return false
	}

	threshold := float32(30)

	var hits []interact.HitResult
	if trigger == component.TooltipTriggerAxis {
		hits = snap.hitTester.FindAllAtX(pos, threshold)
	} else {
		if hit := snap.hitTester.FindNearest(pos, threshold); hit != nil {
			hits = []interact.HitResult{*hit}
		}
	}

	if len(hits) > 0 {
		c.tooltipState.Visible = true
		c.tooltipState.Hits = hits
		c.tooltipState.MousePos = pos
	} else {
		c.tooltipState.Visible = false
		c.tooltipState.Hits = nil
	}

	return true
}

// doZoom performs a zoom operation and fires events. Must be called without lock held.
// delta is the zoom direction: positive = zoom in, negative = zoom out.
// Button clicks use a larger multiplier than scroll events for a meaningful per-click change.
func (c *Chart) doZoom(delta float64) {
	w, onZoom, changed := c.applyZoom(func(zs *interact.ZoomState) bool {
		return zs.ZoomAt(delta*3, 0.5)
	})
	if !changed {
		return
	}
	c.emitZoomChange(w, onZoom)
}

// resetZoom resets zoom to show all data.
func (c *Chart) resetZoom() {
	c.mu.Lock()
	c.rangeDirty = true // catch up with any data appended while zoomed away
	c.mu.Unlock()
	w, onZoom, changed := c.applyZoom(func(zs *interact.ZoomState) bool {
		zs.SetWindow(0, 1)
		return true
	})
	if !changed {
		return
	}
	c.emitZoomChange(w, onZoom)
}

// applyZoom runs fn under lock with the zoom state. Returns the resulting window,
// callback, and whether anything changed. If zoomState is nil, returns changed=false.
func (c *Chart) applyZoom(fn func(*interact.ZoomState) bool) (interact.DataWindow, func(interact.DataWindow), bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.zoomState == nil {
		return interact.DataWindow{}, nil, false
	}
	if !fn(c.zoomState) {
		return interact.DataWindow{}, nil, false
	}
	// User-initiated zoom/pan disables auto-follow so the viewport
	// stays where the user put it instead of snapping back on the next append.
	// Clear pending dirty flags and rangeDirty so any in-flight bellui.Do
	// callback from AppendData becomes a no-op and doesn't override the zoom.
	c.zoomState.FollowLatest = false
	c.pendingDirty = 0
	c.rangeDirty = false
	return c.zoomState.Window, c.OnZoomChange, true
}

// emitZoomChange fires zoom events and forces a synchronous rebuild.
// Must be called without lock held, from the Fyne main thread.
func (c *Chart) emitZoomChange(w interact.DataWindow, onZoom func(interact.DataWindow)) {
	c.Events.Emit(interact.ZoomChangeEvent{
		Event: interact.Event{Type: interact.EventZoomChange},
		Start: w.Start, End: w.End, FromUser: true,
	})
	if onZoom != nil {
		onZoom(w)
	}
	// Force a synchronous full rebuild. We bypass refreshDirty/pendingDirty
	// because the flag-based system can lose zoom updates when racing with
	// AppendData's bellui.Do callbacks on streaming charts.
	// Note: rangeDirty is NOT set here — the range stays frozen so the axis
	// doesn't shift on every scroll tick during streaming. The range is
	// rebuilt on zoom reset (resetZoom) instead.
	if c.renderer != nil {
		c.renderer.fullRebuild(c.Size())
	}
}

// --- Tappable ---

// tapResult describes the result of tap hit-testing, resolved under lock.
type tapResult struct {
	kind          string // "zoom", "toolbox", "legend", "breadcrumb", "datapoint", ""
	zoomAction    string
	toolboxAction string
	legendIndex   int
	legendName    string
	legendVisible bool
	bcDepth       int
	hit           *interact.HitResult
	onClick       func(interact.HitResult)
	drillCfg      *DrilldownConfig
}

// Tapped handles tap/click events on the chart for zoom buttons, legend toggles, and data points.
func (c *Chart) Tapped(e *event.PointEvent) {
	// Request keyboard focus so TypedKey/TypedRune events are delivered.
	if cv := bellui.CurrentApp().Driver().CanvasForObject(c); cv != nil {
		cv.Focus(c)
	}

	// Pinned tooltip: click to pin visible tooltip, click again to dismiss.
	// Skip pin logic when a drawing tool is active — drawing takes priority.
	c.mu.Lock()
	hasDrawingTool := c.drawingTool != interact.DrawingToolNone
	if c.tooltipState.Pinned && !hasDrawingTool {
		// Dismiss pinned tooltip.
		c.tooltipState.Pinned = false
		c.tooltipState.Visible = false
		c.tooltipState.Hits = nil
		c.pendingDirty |= dirtyTooltip
		c.mu.Unlock()
		c.Refresh()
		return
	}
	if c.tooltipState.Pinned && hasDrawingTool {
		// Clear pin silently so drawing can proceed.
		c.tooltipState.Pinned = false
	}
	if !hasDrawingTool && c.tooltipState.Visible && len(c.tooltipState.Hits) > 0 {
		// Pin the current tooltip.
		c.tooltipState.Pinned = true
		c.tooltipState.PinTime = time.Now()
		c.mu.Unlock()
		// Schedule auto-dismiss.
		dur := 15 * time.Second
		if c.option.Tooltip != nil && c.option.Tooltip.PinDuration > 0 {
			dur = c.option.Tooltip.PinDuration
		}
		go func() {
			time.Sleep(dur)
			c.mu.Lock()
			if c.tooltipState.Pinned && c.tooltipState.IsPinExpired(dur) {
				c.tooltipState.Pinned = false
				c.tooltipState.Visible = false
				c.tooltipState.Hits = nil
				c.pendingDirty |= dirtyTooltip
			}
			c.mu.Unlock()
			bellui.Do(func() { c.Refresh() })
		}()
		return
	}
	c.mu.Unlock()

	// Check UI controls first — zoom buttons, toolbox, legend, breadcrumb
	// must always be clickable regardless of drawing tool mode.
	result := c.resolveTap(e.Position)

	switch result.kind {
	case "zoom":
		switch result.zoomAction {
		case "in":
			c.doZoom(1)
		case "out":
			c.doZoom(-1)
		case "reset":
			c.resetZoom()
		}
	case "toolbox":
		c.handleToolboxAction(result.toolboxAction)
	case "legend":
		c.Events.Emit(interact.LegendToggleEvent{
			Event:       interact.Event{Type: interact.EventLegendToggle},
			SeriesIndex: result.legendIndex,
			SeriesName:  result.legendName,
			Visible:     result.legendVisible,
		})
		c.refreshDirty(dirtyFull)
	case "breadcrumb":
		if result.bcDepth < 0 {
			c.DrillUp()
		} else {
			c.drillToDepth(result.bcDepth)
		}
	case "datapoint":
		// Drawing tool and selection take priority over data point clicks.
		if c.handleDrawingTap(e.Position) {
			c.Refresh()
			return
		}
		if c.handleDrawingSelect(e.Position) {
			c.Refresh()
			return
		}

		hit := result.hit
		c.Events.Emit(interact.DataPointEvent{
			Event:       interact.Event{Type: interact.EventClick},
			SeriesIndex: hit.SeriesIndex,
			SeriesName:  hit.SeriesName,
			DataIndex:   hit.DataIndex,
			Value:       hit.Value,
		})
		if result.onClick != nil {
			result.onClick(*hit)
		}
		if result.drillCfg != nil && result.drillCfg.DataProvider != nil {
			childOpt := result.drillCfg.DataProvider(*hit, c.DrillPath())
			if childOpt != nil {
				label := hit.SeriesName
				if hit.Value.Label != "" {
					label = hit.Value.Label
				}
				c.DrillInto(label, *childOpt)
			}
		}
	default:
		// Click on empty area — try drawing tool, then try selection/deselection.
		if c.handleDrawingTap(e.Position) {
			c.Refresh()
			return
		}
		c.handleDrawingSelect(e.Position)
		c.Refresh()
	}
}

// resolveTap determines what was tapped under a single lock/unlock cycle.
func (c *Chart) resolveTap(pos geo.Position) tapResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := c.renderSnap
	if snap == nil {
		return tapResult{}
	}

	if snap.zoomButtons != nil {
		if action := snap.zoomButtons.HitTest(pos); action != "" {
			return tapResult{kind: "zoom", zoomAction: action}
		}
	}

	if snap.toolboxLayout != nil {
		if action := snap.toolboxLayout.HitTest(pos); action != "" {
			return tapResult{kind: "toolbox", toolboxAction: action}
		}
	}

	if snap.legendLayout != nil {
		idx := snap.legendLayout.HitTest(pos)
		if idx >= 0 && idx < len(c.option.Series) {
			name := c.option.Series[idx].Name()
			c.legendToggles[name] = !c.legendToggles[name]
			c.rangeDirty = true
			visible := !c.legendToggles[name]
			return tapResult{kind: "legend", legendIndex: idx, legendName: name, legendVisible: visible}
		}
	}

	if snap.breadcrumbLayout != nil {
		action, depth := snap.breadcrumbLayout.HitTest(pos)
		if action == "back" {
			return tapResult{kind: "breadcrumb", bcDepth: -1}
		}
		if depth >= 0 {
			return tapResult{kind: "breadcrumb", bcDepth: depth}
		}
	}

	if snap.hitTester != nil {
		if hit := snap.hitTester.FindNearest(pos, 20); hit != nil {
			return tapResult{
				kind:     "datapoint",
				hit:      hit,
				onClick:  c.OnClick,
				drillCfg: c.option.Drilldown,
			}
		}
	}

	return tapResult{}
}

// --- Scrollable (DataZoom) ---

// scrollResult describes the outcome of resolving a scroll event under lock.
type scrollResult struct {
	kind   string // "3d", "geo", "zoom", ""
	scene  *fynecanvas.Scene3D
	window interact.DataWindow
	onZoom func(interact.DataWindow)
}

// Scrolled handles scroll events for zooming (DataZoom or 3D camera zoom).
func (c *Chart) Scrolled(e *event.ScrollEvent) {
	c.TryScrolled(e)
}

// TryScrolled implements driver.ScrollConsumer. Returns true if the chart
// consumed the scroll event (for zoom/pan). Returns false if the chart has
// no use for the event, allowing Fyne's driver to propagate it to the
// parent Scroll container.
func (c *Chart) TryScrolled(e *event.ScrollEvent) bool {
	result := c.resolveScroll(e)

	switch result.kind {
	case "3d":
		result.scene.Refresh()
		return true
	case "geo":
		c.refreshDirty(dirtyFull)
		return true
	case "zoom":
		c.emitZoomChange(result.window, result.onZoom)
		return true
	default:
		// Not consumed — Fyne will try the parent scrollable.
		return false
	}
}

// resolveScroll processes scroll input under a single lock. Mutates 3D/geo/zoom state.
func (c *Chart) resolveScroll(e *event.ScrollEvent) scrollResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 3D orbit zoom — update camera on existing Scene3D, no full rebuild
	if c.is3D && c.orbitCam != nil && c.scene3D != nil {
		delta := float64(e.Scrolled.DY)
		factor := float32(1.0)
		if delta > 0 {
			factor = 0.9
		} else if delta < 0 {
			factor = 1.1
		}
		c.orbitCam.Zoom(factor)
		c.scene3D.Camera = c.orbitCam.Camera(c.scene3D.Camera.FOV, c.scene3D.Camera.Near, c.scene3D.Camera.Far)
		return scrollResult{kind: "3d", scene: c.scene3D}
	}

	// Geo map zoom — smooth fractional zoom
	if c.isGeo && c.geoCoord != nil {
		delta := float64(e.Scrolled.DY)
		step := float32(0.15)
		if delta > 0 {
			c.geoCoord.Zoom += step
		} else if delta < 0 {
			c.geoCoord.Zoom -= step
		}
		if c.geoCoord.Zoom < 1 {
			c.geoCoord.Zoom = 1
		}
		if c.geoCoord.Zoom > 18 {
			c.geoCoord.Zoom = 18
		}
		c.updateGeoSeriesCoords()
		return scrollResult{kind: "geo"}
	}

	if c.zoomState == nil {
		return scrollResult{}
	}

	size := c.Size()
	centerNorm := float64(e.Position.X) / float64(size.Width)
	// Normalize scroll delta: clamp magnitude to [0, 5] to avoid extreme
	// jumps while preserving proportional feel from trackpad and mouse wheel.
	delta := float64(e.Scrolled.DY)
	if delta > 5 {
		delta = 5
	} else if delta < -5 {
		delta = -5
	}
	if c.zoomState.ZoomAt(delta, centerNorm) {
		c.zoomState.FollowLatest = false
		return scrollResult{kind: "zoom", window: c.zoomState.Window, onZoom: c.OnZoomChange}
	}
	return scrollResult{}
}

// --- Draggable (Pan + Brush) ---

// dragResult describes the outcome of resolving a drag event under lock.
type dragResult struct {
	kind   string // "geo", "3d", "brush", "pan", ""
	scene  *fynecanvas.Scene3D
	window interact.DataWindow
	onZoom func(interact.DataWindow)
}

// Dragged handles drag events for 3D orbit, panning (when zoomed), or brush selection.
func (c *Chart) Dragged(e *event.DragEvent) {
	result := c.resolveDrag(e)

	switch result.kind {
	case "drawing":
		c.refreshDirty(dirtyDrawing)
	case "geo":
		c.refreshDirty(dirtyFull)
	case "brush":
		c.refreshDirty(dirtyBrush)
	case "3d":
		result.scene.Refresh()
	case "pan":
		c.emitZoomChange(result.window, result.onZoom)
	}
}

// resolveDrag processes drag input under a single lock. Mutates geo/3D/brush/zoom state.
func (c *Chart) resolveDrag(e *event.DragEvent) dragResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Drawing tool drag (Measure)
	if c.handleDrawingDrag(e) {
		return dragResult{kind: "drawing"}
	}

	// Geo map pan
	if c.isGeo && c.geoCoord != nil {
		oldLon, oldLat := c.geoCoord.PixelToData(geo.NewPos(0, 0))
		newLon, newLat := c.geoCoord.PixelToData(geo.NewPos(-e.Dragged.DX, -e.Dragged.DY))
		c.geoCoord.CenterLon += newLon - oldLon
		c.geoCoord.CenterLat += newLat - oldLat
		c.updateGeoSeriesCoords()
		return dragResult{kind: "geo"}
	}

	// 3D orbit rotation
	if c.is3D && c.orbitCam != nil && c.scene3D != nil {
		sensitivity := float32(0.01)
		c.orbitCam.Rotate(-e.Dragged.DX*sensitivity, e.Dragged.DY*sensitivity)
		c.scene3D.Camera = c.orbitCam.Camera(c.scene3D.Camera.FOV, c.scene3D.Camera.Near, c.scene3D.Camera.Far)
		return dragResult{kind: "3d", scene: c.scene3D}
	}

	// Brush mode
	if c.option.Brush != nil && c.option.Brush.Enabled {
		if !c.brushState.Active {
			c.brushState.Active = true
			c.brushState.StartPos = e.Position
		}
		c.brushState.CurrentPos = e.Position
		return dragResult{kind: "brush"}
	}

	// Pan mode
	if c.zoomState != nil && c.zoomState.IsZoomed() {
		size := c.Size()
		deltaNorm := -float64(e.Dragged.DX) / float64(size.Width) * c.zoomState.Window.Span()
		if c.zoomState.Pan(deltaNorm) {
			c.zoomState.FollowLatest = false
			return dragResult{kind: "pan", window: c.zoomState.Window, onZoom: c.OnZoomChange}
		}
	}

	return dragResult{}
}

// dragEndResult holds the outcome of resolving a drag end under lock.
type dragEndResult struct {
	active  bool
	evt     interact.BrushSelectEvent
	onBrush func(interact.BrushSelectEvent)
}

// DragEnd completes a drag operation, finalizing brush selection if active.
func (c *Chart) DragEnd() {
	result := c.resolveDragEnd()
	if !result.active {
		return
	}
	c.Events.Emit(result.evt)
	if result.onBrush != nil {
		result.onBrush(result.evt)
	}
	c.refreshDirty(dirtyBrush)
}

// resolveDragEnd finalizes brush state under a single lock.
func (c *Chart) resolveDragEnd() dragEndResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Handle drawing tool drag end (measure).
	if c.handleDrawingDragEnd() {
		return dragEndResult{} // not a brush event
	}

	if !c.brushState.Active {
		return dragEndResult{}
	}

	c.brushState.Active = false
	brushPos, brushSize := c.brushState.BrushRect()
	var ht *interact.HitTester
	if c.renderSnap != nil {
		ht = c.renderSnap.hitTester
	}
	selected := interact.FindSelectedPoints(ht, brushPos, brushSize)

	return dragEndResult{
		active: true,
		evt: interact.BrushSelectEvent{
			Event:    interact.Event{Type: interact.EventBrushSelect},
			Selected: selected,
		},
		onBrush: c.OnBrushSelect,
	}
}

// renderBrushOverlay produces canvas objects for the active brush selection.
func renderBrushOverlay(state *interact.BrushState) []core.CanvasObject {
	if !state.Active {
		return nil
	}
	pos, size := state.BrushRect()
	if size.Width < 2 || size.Height < 2 {
		return nil
	}

	fill := fynecanvas.NewRectangle(color.NRGBA{R: 91, G: 143, B: 249, A: 40})
	fill.Move(pos)
	fill.Resize(size)

	border := fynecanvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{R: 91, G: 143, B: 249, A: 180}
	border.StrokeWidth = 1
	border.Move(pos)
	border.Resize(size)

	return []core.CanvasObject{fill, border}
}

// --- Focusable + Keyable (keyboard events) ---

// FocusGained is called when the chart receives keyboard focus.
func (c *Chart) FocusGained() {}

// FocusLost is called when the chart loses keyboard focus.
func (c *Chart) FocusLost() {}

// TypedRune is called when a printable character is pressed while focused.
func (c *Chart) TypedRune(r rune) {
	// Text label input mode: accumulate characters.
	c.mu.Lock()
	if c.drawingGesture.Active && c.drawingGesture.Tool == interact.DrawingToolTextLabel {
		c.drawingGesture.TextBuf += string(r)
		c.pendingDirty |= dirtyDrawing
		c.mu.Unlock()
		c.Refresh()
		return
	}
	c.mu.Unlock()

	c.Events.Emit(interact.KeyPressEvent{
		Event: interact.Event{Type: interact.EventKeyPress},
		Rune:  r,
	})
}

// TypedKey is called when a non-printable key is pressed while focused.
func (c *Chart) TypedKey(e *event.KeyEvent) {
	// Delete selected drawing.
	if e.Name == event.KeyDelete || e.Name == event.KeyBackspace {
		// In text label mode, backspace removes last character.
		c.mu.Lock()
		if c.drawingGesture.Active && c.drawingGesture.Tool == interact.DrawingToolTextLabel && len(c.drawingGesture.TextBuf) > 0 {
			c.drawingGesture.TextBuf = c.drawingGesture.TextBuf[:len(c.drawingGesture.TextBuf)-1]
			c.pendingDirty |= dirtyDrawing
			c.mu.Unlock()
			c.Refresh()
			return
		}
		c.mu.Unlock()

		if c.handleDrawingDelete() {
			return
		}
	}
	// Escape cancels active gesture or deselects.
	if e.Name == event.KeyEscape {
		if c.handleDrawingCancel() {
			return
		}
	}
	// Enter finalizes text label.
	if e.Name == event.KeyReturn {
		c.mu.Lock()
		if c.drawingGesture.Active && c.drawingGesture.Tool == interact.DrawingToolTextLabel {
			c.finalizeTextLabel()
			c.mu.Unlock()
			c.Refresh()
			return
		}
		c.mu.Unlock()
	}

	c.Events.Emit(interact.KeyPressEvent{
		Event:   interact.Event{Type: interact.EventKeyPress},
		KeyName: e.Name,
	})
}

// KeyDown is called when a key is pressed (desktop only).
func (c *Chart) KeyDown(e *event.KeyEvent) {}

// KeyUp is called when a key is released (desktop only).
func (c *Chart) KeyUp(e *event.KeyEvent) {}
