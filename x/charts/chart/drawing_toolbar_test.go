package chart

import (
	"testing"

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
