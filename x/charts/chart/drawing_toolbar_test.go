package chart

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

func drawingChart(t *testing.T) *Chart {
	t.Helper()
	test.NewTempApp(t)
	return New(Option{
		XAxis:  []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis:  []axis.Config{{Type: axis.TypeValue}},
		Series: []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{1, 3, 2}}},
	})
}

// toolChanges records the chart's EventDrawingToolChanged events.
func toolChanges(c *Chart) *[]interact.DrawingToolEvent {
	var got []interact.DrawingToolEvent
	c.Events.On(interact.EventDrawingToolChanged, func(evt any) {
		got = append(got, evt.(interact.DrawingToolEvent))
	})
	return &got
}

// A change of tool is announced once, with the tool it replaced; setting the
// tool that is already active announces nothing.
func TestSetDrawingToolEmitsChange(t *testing.T) {
	c := drawingChart(t)
	got := toolChanges(c)

	c.SetDrawingTool(interact.DrawingToolHLine)
	c.SetDrawingTool(interact.DrawingToolHLine)
	c.SetDrawingTool(interact.DrawingToolNone)

	want := []interact.DrawingToolEvent{
		{Event: interact.Event{Type: interact.EventDrawingToolChanged}, Tool: interact.DrawingToolHLine, Previous: interact.DrawingToolNone},
		{Event: interact.Event{Type: interact.EventDrawingToolChanged}, Tool: interact.DrawingToolNone, Previous: interact.DrawingToolHLine},
	}
	if len(*got) != len(want) {
		t.Fatalf("got %d events %+v, want %d", len(*got), *got, len(want))
	}
	for i := range want {
		if (*got)[i] != want[i] {
			t.Errorf("event %d = %+v, want %+v", i, (*got)[i], want[i])
		}
	}
}

// activeButtons returns the labels of the toolbar's highlighted buttons.
func activeButtons(tb *DrawingToolbar) []string {
	var labels []string
	for _, b := range tb.buttons {
		if b.Importance == widget.HighImportance {
			labels = append(labels, b.Text)
		}
	}
	return labels
}

// The toolbar sets the chart's tool, highlights the active one whoever set
// it, and its Clear All removes the drawings and goes back to Select.
func TestDrawingToolbarFollowsChart(t *testing.T) {
	c := drawingChart(t)
	tb := NewDrawingToolbar(c)
	test.WidgetRenderer(tb) // the toolbar renders

	if got := activeButtons(tb); len(got) != 1 || got[0] != "Select" {
		t.Fatalf("new toolbar highlights %v, want [Select]", got)
	}
	if n := len(tb.buttons); n != len(interact.DefaultDrawingTools())+1 {
		t.Fatalf("toolbar has %d buttons, want every default tool and Clear All", n)
	}

	text := tb.buttons[len(tb.buttons)-3] // ..., Text, Measure, Clear All
	if text.Text != "Text" {
		t.Fatalf("button order: got %q where Text should be", text.Text)
	}
	test.Tap(text)
	if c.DrawingTool() != interact.DrawingToolTextLabel {
		t.Errorf("tapping Text set the tool to %v", c.DrawingTool())
	}
	if got := activeButtons(tb); len(got) != 1 || got[0] != "Text" {
		t.Errorf("after tapping Text the toolbar highlights %v", got)
	}

	// A change the app makes is shown too.
	c.SetDrawingTool(interact.DrawingToolRay)
	if got := activeButtons(tb); len(got) != 1 || got[0] != "Ray" {
		t.Errorf("after SetDrawingTool(Ray) the toolbar highlights %v", got)
	}

	c.AddDrawing(&interact.HLineDrawing{DrawingBase: interact.DrawingBase{ID_: "h1"}})
	test.Tap(tb.buttons[len(tb.buttons)-1])
	if n := len(c.Drawings()); n != 0 {
		t.Errorf("Clear All left %d drawings", n)
	}
	if c.DrawingTool() != interact.DrawingToolNone {
		t.Errorf("Clear All left the tool at %v", c.DrawingTool())
	}
	if got := activeButtons(tb); len(got) != 1 || got[0] != "Select" {
		t.Errorf("after Clear All the toolbar highlights %v", got)
	}
}

// An app chooses the tools and their order; Clear All always comes last.
func TestDrawingToolbarCustomTools(t *testing.T) {
	c := drawingChart(t)
	tb := NewDrawingToolbar(c, interact.DrawingToolTextLabel, interact.DrawingToolHLine)
	var labels []string
	for _, b := range tb.buttons {
		labels = append(labels, b.Text)
	}
	want := []string{"Text", "H-Line", "Clear All"}
	if len(labels) != len(want) {
		t.Fatalf("buttons %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("buttons %v, want %v", labels, want)
			break
		}
	}
	if got := activeButtons(tb); len(got) != 0 {
		t.Errorf("with Select not offered, the toolbar highlights %v", got)
	}
}

// The toolbox's drawing tools sit above the plot, under the zoom and toolbox
// row, and wrap when the chart is too narrow for one row.
func TestToolboxDrawingToolsAbovePlot(t *testing.T) {
	cfg := &component.ToolboxConfig{SaveAsImage: true, DrawingTools: interact.DefaultDrawingTools()}
	snap := layoutOf(t, subtitled, true, cfg)
	if snap.toolboxLayout == nil {
		t.Fatal("no toolbox laid out")
	}
	rowBottom := snap.zoomButtons.ResetRect.Pos.Y + snap.zoomButtons.ResetRect.Size.Height
	var tools, rows int
	lastY := float32(-1)
	for _, b := range snap.toolboxLayout.Buttons {
		if b.Action == "save" {
			if bottom := b.Rect.Pos.Y + b.Rect.Size.Height; bottom > rowBottom {
				rowBottom = bottom
			}
			continue
		}
		tools++
		if b.Rect.Pos.Y != lastY {
			rows, lastY = rows+1, b.Rect.Pos.Y
		}
		if bottom := b.Rect.Pos.Y + b.Rect.Size.Height; bottom > snap.plotRect.Y {
			t.Errorf("drawing tool %q ends at %v, below the plot's top %v", b.Action, bottom, snap.plotRect.Y)
		}
	}
	if tools != len(interact.DefaultDrawingTools())+1 {
		t.Fatalf("%d drawing tool buttons, want every default tool and Clear All", tools)
	}
	if rows < 2 {
		t.Errorf("ten tools on a 290pt-wide chart fit in %d row", rows)
	}
	for _, b := range snap.toolboxLayout.Buttons {
		if b.Action != "save" && b.Rect.Pos.Y < rowBottom {
			t.Errorf("drawing tool %q starts at %v, inside the zoom/toolbox row ending at %v", b.Action, b.Rect.Pos.Y, rowBottom)
		}
	}
}

// Tapping a drawing tool inside the chart sets it; Clear All clears.
func TestToolboxDrawingToolActions(t *testing.T) {
	c := drawingChart(t)
	got := toolChanges(c)
	c.handleToolboxAction(component.ActionDrawingTool + "8") // DrawingToolTextLabel
	if c.DrawingTool() != interact.DrawingToolTextLabel {
		t.Errorf("the Text action set the tool to %v", c.DrawingTool())
	}
	c.AddDrawing(&interact.HLineDrawing{DrawingBase: interact.DrawingBase{ID_: "h1"}})
	c.handleToolboxAction(component.ActionClearDrawings)
	if n := len(c.Drawings()); n != 0 || c.DrawingTool() != interact.DrawingToolNone {
		t.Errorf("Clear All left %d drawings and tool %v", n, c.DrawingTool())
	}
	if len(*got) != 2 {
		t.Errorf("got %d tool changes, want 2", len(*got))
	}
}

// With no zoom or toolbox buttons above them, the drawing tools start below
// the title rather than over it.
func TestToolboxDrawingToolsBelowTitle(t *testing.T) {
	snap := layoutOf(t, subtitled, false, &component.ToolboxConfig{DrawingTools: interact.DefaultDrawingTools()})
	untitled := layoutOf(t, nil, false, &component.ToolboxConfig{DrawingTools: interact.DefaultDrawingTools()})
	top := func(s *renderState) float32 {
		y := s.plotRect.Y
		for _, b := range s.toolboxLayout.Buttons {
			y = min(y, b.Rect.Pos.Y)
		}
		return y
	}
	// layoutOf's grid has Top 50; a title with a subtitle is taller than the
	// 2pt the buttons start at below it with no title.
	if got, bare := top(snap), top(untitled); got <= bare {
		t.Errorf("with a title the drawing tools start at %v, no lower than without one (%v)", got, bare)
	}
}

// Clear All while a label is being typed leaves no drawing: the label in
// progress is dropped, not finished into the chart Clear All just emptied.
func TestClearAllDropsLabelInProgress(t *testing.T) {
	for name, clearAll := range map[string]func(*Chart){
		"toolbar": func(c *Chart) {
			tb := NewDrawingToolbar(c)
			test.Tap(tb.buttons[len(tb.buttons)-1])
		},
		"in-chart": func(c *Chart) { c.handleToolboxAction(component.ActionClearDrawings) },
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := focusTestChart(t, false)
			c.SetDrawingTool(interact.DrawingToolTextLabel)
			tapAt(c, 400, 300)
			c.TypedRune('a')
			clearAll(c)
			if n := len(c.Drawings()); n != 0 {
				t.Errorf("Clear All during a label left %d drawings", n)
			}
			if c.DrawingTool() != interact.DrawingToolNone {
				t.Errorf("Clear All left the tool at %v", c.DrawingTool())
			}
		})
	}
}

// toolboxButtonRect returns the centre of the toolbox button for action.
func toolboxButtonRect(t *testing.T, c *Chart, action string) geo.Position {
	t.Helper()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, b := range c.renderSnap.toolboxLayout.Buttons {
		if b.Action == action {
			return b.Rect.Pos.Add(geo.NewPos(b.Rect.Size.Width/2, b.Rect.Size.Height/2))
		}
	}
	t.Fatalf("no toolbox button %q", action)
	return geo.Position{}
}

// A tap on an in-chart drawing tool reaches it while a tooltip shows: an axis
// tooltip follows the pointer's x alone, so it is up over the buttons above
// the plot, and the tap used to pin it instead.
func TestToolboxDrawingToolTapWithTooltip(t *testing.T) {
	test.NewTempApp(t)
	data := make([]float64, 40) // a point every few pixels, so a tooltip at any x
	for i := range data {
		data[i] = float64(i % 7)
	}
	c := New(Option{
		Tooltip: &component.TooltipConfig{Trigger: component.TooltipTriggerAxis},
		Toolbox: &component.ToolboxConfig{DrawingTools: interact.DefaultDrawingTools()},
		XAxis:   []axis.Config{{Type: axis.TypeCategory, Data: make([]string, len(data))}},
		YAxis:   []axis.Config{{Type: axis.TypeValue}},
		Series:  []series.Series{&series.LineSeries{SeriesName: "s", Data: data}},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 600))
	test.TempWidgetRenderer(t, c).Refresh()

	// tapUnderTooltip hovers over the button for action, so a tooltip shows,
	// then taps it.
	tapUnderTooltip := func(action string) {
		t.Helper()
		pos := toolboxButtonRect(t, c, action)
		c.resolveHover(pos)
		c.mu.RLock()
		visible := c.tooltipState.Visible
		c.mu.RUnlock()
		if !visible {
			t.Fatalf("hovering %q shows no tooltip, so this does not test a tap under one", action)
		}
		tapAt(c, pos.X, pos.Y)
		c.mu.RLock()
		pinned := c.tooltipState.Pinned
		c.mu.RUnlock()
		if pinned {
			t.Errorf("a tap on %q pinned the tooltip", action)
		}
	}

	tapUnderTooltip(component.ActionDrawingTool + "3") // DrawingToolTrendLine
	if c.DrawingTool() != interact.DrawingToolTrendLine {
		t.Errorf("tapping Trend under a tooltip set the tool to %v", c.DrawingTool())
	}
	c.SetDrawingTool(interact.DrawingToolNone) // a tooltip pins only with no tool active
	c.AddDrawing(&interact.HLineDrawing{DrawingBase: interact.DrawingBase{ID_: "h1"}})
	tapUnderTooltip(component.ActionClearDrawings)
	if n := len(c.Drawings()); n != 0 {
		t.Errorf("tapping Clear All under a tooltip left %d drawings", n)
	}
}

// A chart with no cartesian series and no Grid lays its plot out in a tighter
// grid than DefaultGrid; its toolbox and drawing tools sit above that plot,
// not over it.
func TestToolboxDrawingToolsOnSelfLayoutChart(t *testing.T) {
	test.NewTempApp(t)
	c := New(Option{
		Toolbox: &component.ToolboxConfig{SaveAsImage: true, DrawingTools: interact.DefaultDrawingTools()},
		Series: []series.Series{&series.PieSeries{SeriesName: "p", Data: []series.PieDataItem{
			{Name: "a", Value: 1}, {Name: "b", Value: 2},
		}}},
	})
	c.Resize(geo.NewSize(400, 360))
	test.WidgetRenderer(c).Refresh()
	c.mu.RLock()
	snap := c.renderSnap
	c.mu.RUnlock()
	if snap == nil || snap.toolboxLayout == nil {
		t.Fatal("no toolbox laid out")
	}
	var saveBottom float32
	for _, b := range snap.toolboxLayout.Buttons {
		if b.Action == "save" {
			saveBottom = b.Rect.Pos.Y + b.Rect.Size.Height
		}
	}
	if saveBottom == 0 {
		t.Fatal("no Save button laid out")
	}
	for _, b := range snap.toolboxLayout.Buttons {
		if bottom := b.Rect.Pos.Y + b.Rect.Size.Height; bottom > snap.plotRect.Y {
			t.Errorf("toolbox button %q ends at %v, below the plot's top %v", b.Action, bottom, snap.plotRect.Y)
		}
		if b.Action != "save" && b.Rect.Pos.Y < saveBottom {
			t.Errorf("drawing tool %q starts at %v, over the Save button ending at %v", b.Action, b.Rect.Pos.Y, saveBottom)
		}
	}
}

// The toolbar follows the chart while it has a renderer, and lets go of the
// chart's events when that renderer is destroyed, so a toolbar an app throws
// away does not stay subscribed for as long as the chart lives.
func TestDrawingToolbarUnsubscribesOnDestroy(t *testing.T) {
	c := drawingChart(t)
	tb := NewDrawingToolbar(c)
	// The renderers are made and destroyed here, as the driver's cache does,
	// rather than through test.WidgetRenderer, which keeps the one it made.
	r := tb.CreateRenderer()
	c.SetDrawingTool(interact.DrawingToolRay)
	if got := activeButtons(tb); len(got) != 1 || got[0] != "Ray" {
		t.Fatalf("a rendered toolbar highlights %v after SetDrawingTool(Ray)", got)
	}

	r.Destroy()
	c.SetDrawingTool(interact.DrawingToolHLine)
	if got := activeButtons(tb); len(got) != 1 || got[0] != "Ray" {
		t.Errorf("a toolbar whose renderer was destroyed still follows the chart: highlights %v", got)
	}

	// Rendered again, it shows the tool the chart has now and follows again.
	defer tb.CreateRenderer().Destroy()
	if got := activeButtons(tb); len(got) != 1 || got[0] != "H-Line" {
		t.Errorf("a re-rendered toolbar highlights %v, want [H-Line]", got)
	}
	c.SetDrawingTool(interact.DrawingToolFibRetracement)
	if got := activeButtons(tb); len(got) != 1 || got[0] != "Fib" {
		t.Errorf("a re-rendered toolbar highlights %v after SetDrawingTool(Fib)", got)
	}
}

// The zoom buttons and the legend take a tap under a tooltip too, as the
// toolbox does: an axis tooltip is up over them just as much.
func TestControlsTapWithTooltip(t *testing.T) {
	test.NewTempApp(t)
	data := make([]float64, 40) // a point every few pixels, so a tooltip at any x
	for i := range data {
		data[i] = float64(i % 7)
	}
	c := New(Option{
		Tooltip:  &component.TooltipConfig{Trigger: component.TooltipTriggerAxis},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.1, MaxSpan: 1, ZoomSensitivity: 0.1},
		Legend:   &component.LegendConfig{},
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: make([]string, len(data))}},
		YAxis:    []axis.Config{{Type: axis.TypeValue}},
		Series:   []series.Series{&series.LineSeries{SeriesName: "s", Data: data}},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 600))
	test.TempWidgetRenderer(t, c).Refresh()

	tapUnderTooltip := func(name string, pos geo.Position) {
		t.Helper()
		c.resolveHover(pos)
		c.mu.RLock()
		visible := c.tooltipState.Visible
		c.mu.RUnlock()
		if !visible {
			t.Fatalf("hovering the %s shows no tooltip, so this does not test a tap under one", name)
		}
		tapAt(c, pos.X, pos.Y)
		c.mu.RLock()
		pinned := c.tooltipState.Pinned
		c.mu.RUnlock()
		if pinned {
			t.Errorf("a tap on the %s pinned the tooltip", name)
		}
	}

	c.mu.RLock()
	zoomIn := c.renderSnap.zoomButtons.ZoomInRect
	legend := c.renderSnap.legendLayout
	c.mu.RUnlock()
	before := c.ZoomWindow()
	tapUnderTooltip("zoom-in button", zoomIn.Pos.Add(geo.NewPos(zoomIn.Size.Width/2, zoomIn.Size.Height/2)))
	if c.ZoomWindow() == before {
		t.Errorf("tapping zoom-in under a tooltip left the window at %+v", before)
	}

	if legend == nil || len(legend.Items) == 0 {
		t.Fatal("no legend laid out")
	}
	item := legend.Items[0]
	tapUnderTooltip("legend", geo.NewPos(item.Rect.X+item.Size.Width/2, item.Rect.Y+item.Size.Height/2))
	c.mu.RLock()
	hidden := c.legendToggles["s"]
	c.mu.RUnlock()
	if !hidden {
		t.Error("tapping the legend under a tooltip did not hide the series")
	}

	c.DrillInto("child", c.Option())
	test.TempWidgetRenderer(t, c).Refresh()
	c.mu.RLock()
	crumbs := c.renderSnap.breadcrumbLayout
	c.mu.RUnlock()
	if crumbs == nil {
		t.Fatal("no breadcrumb laid out after drilling in")
	}
	back := crumbs.BackButton
	tapUnderTooltip("breadcrumb", back.Pos.Add(geo.NewPos(back.Size.Width/2, back.Size.Height/2)))
	if path := c.DrillPath(); len(path) != 0 {
		t.Errorf("tapping the breadcrumb's back button under a tooltip left the path at %v", path)
	}
}
