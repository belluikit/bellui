package chart

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/series"
)

// A chart of candles only shows its axis tooltip and crosshair when hovered.
// It found no hits, so a trading pane needed an invisible line series to get
// a crosshair (#343).
func TestChart_AxisTooltipOnCandlesOnly(t *testing.T) {
	test.NewTempApp(t)
	c := New(Option{
		XAxis:   []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis:   []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Tooltip: &component.TooltipConfig{Trigger: component.TooltipTriggerAxis, Crosshair: component.CrosshairBoth},
		Series: []series.Series{&series.CandlestickSeries{SeriesName: "k", Data: []series.CandlestickData{
			{Open: 10, Close: 20, Low: 5, High: 25},
			{Open: 20, Close: 30, Low: 15, High: 35},
			{Open: 30, Close: 25, Low: 20, High: 40},
		}}},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	test.TempWidgetRenderer(t, c).Refresh()

	pos := c.renderSnap.coordSystems[0].DataToPixel(1, 25)
	if !c.resolveHover(geo.NewPos(pos.X+3, pos.Y)) {
		t.Fatal("hovering a candle showed no axis tooltip")
	}
	if hits := c.tooltipState.Hits; len(hits) != 1 || hits[0].DataIndex != 1 {
		t.Errorf("hits = %+v, want candle 1", hits)
	}
}
