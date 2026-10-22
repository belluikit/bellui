package chart

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// layoutOf renders a line chart with the given title, zoom and toolbox at
// 400×360 and returns what the renderer published.
func layoutOf(t *testing.T, title *Title, zoom bool, toolbox *component.ToolboxConfig) *renderState {
	t.Helper()
	test.NewTempApp(t)
	opt := Option{
		Title:   title,
		Toolbox: toolbox,
		Grid:    &Grid{Left: 60, Right: 50, Top: 50, Bottom: 40},
		XAxis:   []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis:   []axis.Config{{Type: axis.TypeValue}},
		Series:  []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{1, 3, 2}}},
	}
	if zoom {
		opt.DataZoom = &interact.ZoomConfig{Enabled: true, MinSpan: 0.1, MaxSpan: 1}
	}
	c := New(opt)
	c.Resize(geo.NewSize(400, 360))
	test.WidgetRenderer(c).Refresh()
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.renderSnap == nil {
		t.Fatal("no render snapshot")
	}
	return c.renderSnap
}

var subtitled = &Title{Text: "Line + Overlays", SubText: "Ref Line · Band · Toolbox"}

// The toolbox row under the zoom buttons ends above the plot (#663).
func TestToolboxRowAbovePlot(t *testing.T) {
	snap := layoutOf(t, subtitled, true, &component.ToolboxConfig{SaveAsImage: true, MagicType: []string{"line", "bar"}})
	if snap.toolboxLayout == nil || len(snap.toolboxLayout.Buttons) == 0 {
		t.Fatal("no toolbox buttons laid out")
	}
	for _, b := range snap.toolboxLayout.Buttons {
		if bottom := b.Rect.Pos.Y + b.Rect.Size.Height; bottom > snap.plotRect.Y {
			t.Errorf("toolbox button %q ends at %v, below the plot's top %v", b.Action, bottom, snap.plotRect.Y)
		}
	}
	if snap.zoomButtons == nil {
		t.Fatal("no zoom buttons laid out")
	}
	if bottom := snap.zoomButtons.ResetRect.Pos.Y + snap.zoomButtons.ResetRect.Size.Height; bottom > snap.plotRect.Y {
		t.Errorf("zoom buttons end at %v, below the plot's top %v", bottom, snap.plotRect.Y)
	}
}

// A chart whose title already ends below its buttons keeps its plot where
// it was: zoom buttons alone are shorter than a title with a subtitle.
func TestZoomOnlyKeepsPlotUnderTitle(t *testing.T) {
	plain := layoutOf(t, subtitled, false, nil)
	zoomed := layoutOf(t, subtitled, true, nil)
	if zoomed.plotRect.Y != plain.plotRect.Y {
		t.Errorf("zoom buttons moved the plot from %v to %v", plain.plotRect.Y, zoomed.plotRect.Y)
	}
}
