package chart

import (
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/interact"
)

// DrawingToolbar is the default drawing-tools UI for a chart: one button per
// drawing tool, the active one highlighted, and a Clear All button. It wraps
// onto more rows when it is narrower than its buttons. The app places it
// anywhere; ToolboxConfig.DrawingTools shows the same tools inside the chart.
//
// An app that wants a different UI does not use it: it calls
// Chart.SetDrawingTool from its own controls and follows
// interact.EventDrawingToolChanged to show the active tool.
//
// Experimental: added in #796 and may change until a second app uses it. The
// stable route is Chart.SetDrawingTool with interact.EventDrawingToolChanged.
type DrawingToolbar struct {
	widget.BaseWidget

	chart   *Chart
	tools   []interact.DrawingToolMode
	buttons []*widget.Button // one per tool, then Clear All
}

// NewDrawingToolbar returns the drawing tools for c, in the order given, or
// interact.DefaultDrawingTools when none are. It follows c's active tool while
// it is shown: it subscribes to c's events when its renderer is created and
// lets go when that renderer is destroyed, so a toolbar the app drops does not
// stay subscribed for as long as c lives.
func NewDrawingToolbar(c *Chart, tools ...interact.DrawingToolMode) *DrawingToolbar {
	if len(tools) == 0 {
		tools = interact.DefaultDrawingTools()
	}
	t := &DrawingToolbar{chart: c, tools: tools}
	for _, tool := range tools {
		t.buttons = append(t.buttons, widget.NewButton(tool.String(), func() {
			c.SetDrawingTool(tool)
		}))
	}
	t.buttons = append(t.buttons, widget.NewButton("Clear All", c.clearAllDrawings))
	t.ExtendBaseWidget(t)
	t.showActive(c.DrawingTool())
	return t
}

// showActive highlights the button of the active tool.
func (t *DrawingToolbar) showActive(active interact.DrawingToolMode) {
	for i, tool := range t.tools {
		b := t.buttons[i]
		want := widget.MediumImportance
		if tool == active {
			want = widget.HighImportance
		}
		if b.Importance != want {
			b.Importance = want
			b.Refresh()
		}
	}
}

// CreateRenderer implements core.Widget.
func (t *DrawingToolbar) CreateRenderer() core.WidgetRenderer {
	objs := make([]core.CanvasObject, len(t.buttons))
	for i, b := range t.buttons {
		objs[i] = b
	}
	t.showActive(t.chart.DrawingTool()) // it may have changed while not shown
	id := t.chart.Events.On(interact.EventDrawingToolChanged, func(evt any) {
		t.showActive(evt.(interact.DrawingToolEvent).Tool)
	})
	return &drawingToolbarRenderer{
		WidgetRenderer: widget.NewSimpleRenderer(container.New(layout.NewRowWrapLayout(), objs...)),
		chart:          t.chart,
		handler:        id,
	}
}

// drawingToolbarRenderer holds the toolbar's subscription to its chart's
// events for as long as the toolbar is rendered.
type drawingToolbarRenderer struct {
	core.WidgetRenderer
	chart   *Chart
	handler int // the EventDrawingToolChanged handler, removed on Destroy
}

// Destroy implements core.WidgetRenderer.
func (r *drawingToolbarRenderer) Destroy() {
	r.chart.Events.Off(r.handler)
	r.WidgetRenderer.Destroy()
}
