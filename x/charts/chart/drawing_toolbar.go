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
// interact.DefaultDrawingTools when none are. It follows c's active tool for
// as long as c lives.
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
	t.buttons = append(t.buttons, widget.NewButton("Clear All", func() {
		c.ClearDrawings()
		c.SetDrawingTool(interact.DrawingToolNone)
	}))
	t.ExtendBaseWidget(t)
	t.showActive(c.DrawingTool())
	c.Events.On(interact.EventDrawingToolChanged, func(evt any) {
		t.showActive(evt.(interact.DrawingToolEvent).Tool)
	})
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
	return widget.NewSimpleRenderer(container.New(layout.NewRowWrapLayout(), objs...))
}
