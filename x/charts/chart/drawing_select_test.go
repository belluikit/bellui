package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// With a tooltip enabled, a click on a drawing selects it. It used to pin the
// tooltip the hover had opened, so the drawing could never be selected (#343).
func TestChart_ClickSelectsDrawingWhileTooltipShows(t *testing.T) {
	test.NewTempApp(t)
	c := New(Option{
		Tooltip: &component.TooltipConfig{Trigger: component.TooltipTriggerItem},
		Series:  []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{1, 2, 3, 4}}},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 600))
	test.TempWidgetRenderer(t, c).Refresh()
	c.AddDrawing(&interact.HLineDrawing{DrawingBase: interact.DrawingBase{ID_: "level", Color_: color.White}, Y: 2})

	pos := c.renderSnap.coordSystems[0].DataToPixel(1, 2) // the point at 2 lies on the line
	c.handleHover(pos)
	if !c.tooltipState.Visible || len(c.tooltipState.Hits) == 0 {
		t.Fatal("hovering the point showed no tooltip")
	}

	c.Tapped(&event.PointEvent{Position: pos})
	if got := c.SelectedDrawing(); got != "level" {
		t.Errorf("SelectedDrawing = %q after clicking the line, want %q", got, "level")
	}
	if c.tooltipState.Pinned {
		t.Error("the click pinned the tooltip instead of selecting the drawing")
	}

	// Away from any drawing, a click still pins the tooltip.
	other := c.renderSnap.coordSystems[0].DataToPixel(3, 4)
	c.handleHover(other)
	c.Tapped(&event.PointEvent{Position: other})
	if !c.tooltipState.Pinned {
		t.Error("a click on a point away from any drawing no longer pins the tooltip")
	}
}
