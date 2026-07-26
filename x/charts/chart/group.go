package chart

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/interact"
)

// Pane configures a single chart within a ChartGroup.
type Pane struct {
	Chart  *Chart
	Height float32 // relative height weight (e.g., 3 for main chart, 1 for volume)
}

// ChartGroup is a container widget that stacks multiple Chart instances
// vertically with synchronized zoom, pan, and crosshair propagation.
// This enables TradingView-style multi-pane layouts (e.g., price chart
// above, volume pane below, MACD pane at the bottom).
type ChartGroup struct {
	widget.BaseWidget

	panes       []Pane
	syncEnabled bool // whether to sync zoom/crosshair across panes
}

var _ core.Widget = (*ChartGroup)(nil)

// NewChartGroup creates a synchronized group of chart panes.
// Heights are relative weights: {3, 1, 1} means the first pane takes 60%.
func NewChartGroup(panes ...Pane) *ChartGroup {
	g := &ChartGroup{
		panes:       panes,
		syncEnabled: true,
	}
	g.ExtendBaseWidget(g)

	// Wire up zoom and crosshair synchronization between all panes.
	for i := range panes {
		g.wireZoomSync(i)
		g.wireHoverSync(i)
	}

	return g
}

// wireZoomSync registers a zoom change listener on pane[i] that propagates
// the zoom window to all other panes.
func (g *ChartGroup) wireZoomSync(sourceIdx int) {
	source := g.panes[sourceIdx].Chart
	source.Events.On(interact.EventZoomChange, func(evt interface{}) {
		if !g.syncEnabled {
			return
		}
		e := evt.(interact.ZoomChangeEvent)
		if !e.FromUser {
			return // avoid infinite recursion from programmatic zoom
		}
		w := interact.DataWindow{Start: e.Start, End: e.End}
		for j, p := range g.panes {
			if j == sourceIdx {
				continue
			}
			// Set the window without triggering another FromUser event.
			p.Chart.SetZoomWindow(w)
		}
	})
}

// wireHoverSync registers hover listeners that propagate crosshair position
// across panes automatically. When the user hovers over one pane, all other
// panes show a vertical crosshair at the same X position.
func (g *ChartGroup) wireHoverSync(sourceIdx int) {
	source := g.panes[sourceIdx].Chart
	source.Events.On(interact.EventHover, func(evt interface{}) {
		if !g.syncEnabled {
			return
		}
		e := evt.(interact.DataPointEvent)
		for j, p := range g.panes {
			if j == sourceIdx {
				continue
			}
			targetSize := p.Chart.Size()
			syntheticPos := geo.NewPos(e.PixelPos.X, targetSize.Height/2)
			p.Chart.HoverMoved(&event.HoverEvent{
				PointEvent: event.PointEvent{Position: syntheticPos},
			})
		}
	})
	source.Events.On(interact.EventMouseOut, func(_ interface{}) {
		if !g.syncEnabled {
			return
		}
		for j, p := range g.panes {
			if j == sourceIdx {
				continue
			}
			p.Chart.HoverOut()
		}
	})
}

// SetSyncEnabled toggles zoom/crosshair synchronization.
func (g *ChartGroup) SetSyncEnabled(enabled bool) {
	g.syncEnabled = enabled
}

// Panes returns the list of panes for external configuration.
func (g *ChartGroup) Panes() []Pane {
	return g.panes
}

// CreateRenderer returns the widget renderer for ChartGroup.
func (g *ChartGroup) CreateRenderer() core.WidgetRenderer {
	return newGroupRenderer(g)
}

// MinSize returns the minimum size for the group.
func (g *ChartGroup) MinSize() geo.Size {
	minW := float32(200)
	minH := float32(0)
	for _, p := range g.panes {
		ps := p.Chart.MinSize()
		if ps.Width > minW {
			minW = ps.Width
		}
		minH += ps.Height
	}
	return geo.NewSize(minW, minH)
}

// --- Crosshair propagation ---

// PropagateCrosshair sends a hover position from one pane to all others
// at the same X coordinate. Call this from a custom hover handler.
func (g *ChartGroup) PropagateCrosshair(sourceIdx int, pos geo.Position) {
	if !g.syncEnabled {
		return
	}
	for i, p := range g.panes {
		if i == sourceIdx {
			continue
		}
		// Simulate a hover at the same X but vertically centered in the target pane.
		targetSize := p.Chart.Size()
		syntheticPos := geo.NewPos(pos.X, targetSize.Height/2)
		p.Chart.HoverMoved(&event.HoverEvent{
			PointEvent: event.PointEvent{Position: syntheticPos},
		})
	}
}

// ClearCrosshair clears the crosshair from all panes except the source.
func (g *ChartGroup) ClearCrosshair(sourceIdx int) {
	if !g.syncEnabled {
		return
	}
	for i, p := range g.panes {
		if i == sourceIdx {
			continue
		}
		p.Chart.HoverOut()
	}
}

// --- Group renderer ---

type groupRenderer struct {
	group     *ChartGroup
	container *bellui.Container
	objects   []core.CanvasObject
}

func newGroupRenderer(g *ChartGroup) *groupRenderer {
	objects := make([]core.CanvasObject, len(g.panes))
	for i, p := range g.panes {
		objects[i] = p.Chart
	}
	c := container.NewWithoutLayout(objects...)
	return &groupRenderer{
		group:     g,
		container: c,
		objects:   []core.CanvasObject{c},
	}
}

func (r *groupRenderer) Layout(size geo.Size) {
	// Compute total weight and allocate height proportionally.
	totalWeight := float32(0)
	for _, p := range r.group.panes {
		w := p.Height
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	y := float32(0)
	for _, p := range r.group.panes {
		w := p.Height
		if w <= 0 {
			w = 1
		}
		h := (w / totalWeight) * size.Height
		p.Chart.Move(geo.NewPos(0, y))
		p.Chart.Resize(geo.NewSize(size.Width, h))
		y += h
	}
	r.container.Resize(size)
}

func (r *groupRenderer) MinSize() geo.Size {
	return r.group.MinSize()
}

func (r *groupRenderer) Refresh() {
	for _, p := range r.group.panes {
		p.Chart.Refresh()
	}
}

func (r *groupRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *groupRenderer) Destroy() {}
