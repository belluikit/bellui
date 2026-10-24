// Package chart provides the main Chart widget for charts.
package chart

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	fynecanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/animate"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// dirtyFlags indicates what state changed and needs re-rendering.
type dirtyFlags uint32

const (
	dirtyFull    dirtyFlags = 1 << iota // option, zoom, legend, resize, animation
	dirtyTooltip                        // hover/tooltip state change only
	dirtyBrush                          // brush selection state change only
	dirtyDrawing                        // drawing gesture/selection change (interactive layer)
)

// Chart is the x/charts widget. It renders chart data using Bellui canvas primitives.
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
	capturing     bool       // a capture is under way: draw the chart without its controls (see captureImage); protected by mu

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
	_ event.Pinchable  = (*Chart)(nil)
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

// SaveAsImage captures the chart and offers to save it as a PNG through a
// file-save dialog. The image holds the chart only — title, series, axes,
// legend, overlays and drawings — without the toolbox, the zoom buttons, the
// tooltip or the axis pointer, as ECharts' saveAsImage leaves its toolbox out
// (see captureImage). Call it on the main goroutine: the capture reads the
// window's canvas. A chart that is not on a canvas, or whose window could not
// be drawn, has nothing to capture; that is logged and no dialog opens.
func (c *Chart) SaveAsImage() {
	img := c.captureImage()
	if img == nil {
		log.Println("x/charts: SaveAsImage: the chart is not on a drawn canvas, nothing to capture")
		return
	}

	w := c.window()
	if w == nil {
		log.Println("x/charts: SaveAsImage: no window to show the save dialog in")
		return
	}

	dialog.ShowFileSave(func(writer bellui.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		if err := png.Encode(writer, img); err != nil {
			log.Printf("x/charts: SaveAsImage png.Encode failed: %v", err)
		}
	}, w)
}

// window returns the window whose canvas holds the chart, falling back to the
// app's first window, or nil when the app has none.
func (c *Chart) window() bellui.Window {
	d := bellui.CurrentApp().Driver()
	all := d.AllWindows()
	if cv := d.CanvasForObject(c); cv != nil {
		for _, w := range all {
			if w.Canvas() == cv {
				return w
			}
		}
	}
	if len(all) > 0 {
		return all[0]
	}
	return nil
}

// captureImage returns the chart's pixels as they are drawn on its canvas,
// cut out of a capture of the whole canvas, with the controls left out: the
// toolbox, the zoom buttons, and the tooltip and axis pointer (crosshair and
// value tags) when they are showing. Title, series, axes, legend, overlays and
// committed drawings are included. For the capture the chart is redrawn
// without the controls, then redrawn as it was, whether or not the capture
// succeeded; the tooltip's state is not touched, so a tooltip that was showing
// shows again afterwards.
//
// Only the part of the chart its clipping ancestors leave in view is captured:
// a chart scrolled partly out of sight yields the visible part, never the
// widgets drawn where the rest would be. It returns nil when the chart is not
// on a canvas, is wholly out of view, or the canvas could not be drawn. The
// image is in device pixels, so on a HiDPI screen it is larger than the
// chart's size in canvas units.
func (c *Chart) captureImage() image.Image {
	d := bellui.CurrentApp().Driver()
	cv := d.CanvasForObject(c)
	if cv == nil {
		return nil
	}
	c.setCapturing(true)
	defer c.setCapturing(false)
	full := cv.Capture()
	if full == nil {
		return nil
	}
	cvSize := cv.Size()
	fb := full.Bounds()
	if cvSize.Width <= 0 || cvSize.Height <= 0 || fb.Empty() {
		return nil
	}
	// Derive the scale from the capture itself rather than Canvas.Scale: the
	// capture is in framebuffer pixels, which on some drivers also carry the
	// texture scale on top of the canvas scale.
	sx := float32(fb.Dx()) / cvSize.Width
	sy := float32(fb.Dy()) / cvSize.Height

	pos := d.AbsolutePositionForObject(c)
	size := c.Size()
	// A scroll or clipping container above the chart hides part of it, and
	// the capture holds whatever else is drawn there.
	if clip, clipped := driver.ClipBoundsForObject(cv, c); clipped {
		x0, y0 := max(pos.X, clip.Pos.X), max(pos.Y, clip.Pos.Y)
		x1 := min(pos.X+size.Width, clip.Pos.X+clip.Size.Width)
		y1 := min(pos.Y+size.Height, clip.Pos.Y+clip.Size.Height)
		if x1 <= x0 || y1 <= y0 {
			return nil
		}
		pos, size = geo.NewPos(x0, y0), geo.NewSize(x1-x0, y1-y0)
	}
	r := image.Rect(
		fb.Min.X+int(pos.X*sx), fb.Min.Y+int(pos.Y*sy),
		fb.Min.X+int((pos.X+size.Width)*sx), fb.Min.Y+int((pos.Y+size.Height)*sy),
	).Intersect(fb)
	if r.Empty() {
		return nil
	}

	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), full, r.Min, draw.Src)
	return out
}

// setCapturing turns the capture drawing on or off and redraws the chart in
// it at once, so the frame a capture paints next has the controls left out
// (on) or back (off).
func (c *Chart) setCapturing(on bool) {
	c.mu.Lock()
	c.capturing = on
	c.mu.Unlock()
	c.refreshDirty(dirtyFull)
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
//
// It is idempotent: the chart repaints and emits [interact.EventMouseOut] only
// when the call actually cleared a visible tooltip. An "out" for a chart that
// was not showing anything carries no information, and emitting it anyway is
// what let a [ChartGroup]'s hover sync echo between panes forever — A's out
// calls B.HoverOut, which emits, which calls A.HoverOut, … (#36).
func (c *Chart) HoverOut() {
	c.mu.Lock()
	if c.tooltipState.Pinned {
		c.mu.Unlock()
		return
	}
	if !c.tooltipState.Visible && c.tooltipState.Hits == nil {
		c.mu.Unlock()
		return // nothing was hovered: no state change, nothing to announce
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

// isMobile reports whether the app runs on a phone or tablet. A variable so
// tests can take the mobile path without a mobile build.
var isMobile = func() bool { return bellui.CurrentDevice().IsMobile() }

// Tapped handles tap/click events on the chart for zoom buttons, legend toggles, and data points.
func (c *Chart) Tapped(e *event.PointEvent) {
	if isMobile() {
		// The mobile driver opens the on-screen keyboard for whatever has the
		// focus, so a tap focuses the chart only when it leaves the Text tool
		// waiting for text (#415).
		defer c.syncMobileFocus()
	} else if cv := bellui.CurrentApp().Driver().CanvasForObject(c); cv != nil {
		// Request keyboard focus so TypedKey/TypedRune events are delivered.
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

// Pinched zooms like the scroll wheel does, by the pinch's scale: a DataZoom
// window around the point between the fingers, or the 3D camera or map around
// their centre, as the wheel zooms them.
func (c *Chart) Pinched(e *event.PinchEvent) {
	c.applyScrollResult(c.resolvePinch(e))
}

// PinchEnded is part of event.Pinchable; each step is already applied.
func (c *Chart) PinchEnded() {}

// resolvePinch is resolveScroll for a pinch: the same three zoom modes,
// driven by a scale factor instead of a wheel delta. A step that only moved
// the fingers (scale 1) changes nothing.
func (c *Chart) resolvePinch(e *event.PinchEvent) scrollResult {
	scale := float64(e.Scale)
	if scale <= 0 || scale == 1 {
		return scrollResult{}
	}
	return c.resolveZoom(e.Position.X, zoomStep{
		camera: float32(1 / scale),        // spreading the fingers moves the camera closer
		levels: float32(math.Log2(scale)), // map zoom levels double per step
		window: func(z *interact.ZoomState, centerNorm float64) bool { return z.ZoomBy(scale, centerNorm) },
	})
}

// TryScrolled implements driver.ScrollConsumer. Returns true if the chart
// consumed the scroll event (for zoom/pan). Returns false if the chart has
// no use for the event, allowing Fyne's driver to propagate it to the
// parent Scroll container.
func (c *Chart) TryScrolled(e *event.ScrollEvent) bool {
	// A touch scroll is a two-finger pan (#398): it scrolls the page around
	// the chart. On touch, a chart zooms by pinch.
	if e.Source == event.ScrollSourceTouch {
		return false
	}
	return c.applyScrollResult(c.resolveScroll(e))
}

// applyScrollResult refreshes or notifies for a resolved zoom, reporting
// whether it consumed the input.
func (c *Chart) applyScrollResult(result scrollResult) bool {
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
	delta := e.Scrolled.DY
	camera := float32(1.0)
	if delta > 0 {
		camera = 0.9
	} else if delta < 0 {
		camera = 1.1
	}
	levels := float32(0) // geo map zoom — smooth fractional steps
	if delta > 0 {
		levels = 0.15
	} else if delta < 0 {
		levels = -0.15
	}
	// Normalize scroll delta: clamp magnitude to [0, 5] to avoid extreme
	// jumps while preserving proportional feel from trackpad and mouse wheel.
	windowDelta := float64(delta)
	if windowDelta > 5 {
		windowDelta = 5
	} else if windowDelta < -5 {
		windowDelta = -5
	}
	return c.resolveZoom(e.Position.X, zoomStep{
		camera: camera,
		levels: levels,
		window: func(z *interact.ZoomState, centerNorm float64) bool { return z.ZoomAt(windowDelta, centerNorm) },
	})
}

// zoomStep is one zoom input — a wheel notch or a pinch step — expressed for
// each of the chart's zoom modes.
type zoomStep struct {
	camera float32                                              // 3D orbit camera distance factor; below 1 moves closer
	levels float32                                              // map zoom levels to add
	window func(z *interact.ZoomState, centerNorm float64) bool // DataZoom window change
}

// resolveZoom applies a zoom step to whichever zoom mode the chart has — 3D
// orbit camera, geo map, or DataZoom window around canvas x — and says what
// to refresh or notify.
func (c *Chart) resolveZoom(x float32, s zoomStep) scrollResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 3D orbit zoom — update camera on existing Scene3D, no full rebuild
	if c.is3D && c.orbitCam != nil && c.scene3D != nil {
		c.orbitCam.Zoom(s.camera)
		c.scene3D.Camera = c.orbitCam.Camera(c.scene3D.Camera.FOV, c.scene3D.Camera.Near, c.scene3D.Camera.Far)
		return scrollResult{kind: "3d", scene: c.scene3D}
	}

	if c.isGeo && c.geoCoord != nil {
		c.geoCoord.Zoom += s.levels
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
	centerNorm := float64(x) / float64(c.Size().Width)
	if c.xInverseLocked() {
		centerNorm = 1 - centerNorm
	}
	if s.window(c.zoomState, centerNorm) {
		c.zoomState.FollowLatest = false
		return scrollResult{kind: "zoom", window: c.zoomState.Window, onZoom: c.OnZoomChange}
	}
	return scrollResult{}
}

// xInverseLocked reports whether the X axis runs right to left
// (axis.Config.Inverse), so screen-space zoom anchors and pan deltas must be
// mirrored into the data window. The caller holds c.mu.
func (c *Chart) xInverseLocked() bool {
	return len(c.option.XAxis) > 0 && c.option.XAxis[0].Inverse
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
		if c.xInverseLocked() {
			deltaNorm = -deltaNorm // data runs right to left
		}
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

// syncMobileFocus gives the chart the focus, and with it the on-screen
// keyboard, while its Text tool is waiting for text, and gives it up
// otherwise. Mobile only: on desktop the chart takes the focus on every tap,
// for its keyboard shortcuts. Hardware-keyboard shortcuts on mobile are #660.
func (c *Chart) syncMobileFocus() {
	c.mu.RLock()
	wantText := c.drawingGesture.Active && c.drawingGesture.Tool == interact.DrawingToolTextLabel
	c.mu.RUnlock()
	cv := bellui.CurrentApp().Driver().CanvasForObject(c)
	if cv == nil {
		return
	}
	switch focused := cv.Focused() == c; {
	case wantText && !focused:
		cv.Focus(c)
	case !wantText && focused:
		cv.Unfocus()
	}
}

// TypedKey is called when a non-printable key is pressed while focused.
func (c *Chart) TypedKey(e *event.KeyEvent) {
	if isMobile() {
		defer c.syncMobileFocus() // Return or Escape may end the text input
	}
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
