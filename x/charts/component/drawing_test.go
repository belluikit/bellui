package component

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/style"
)

func makeDrawingCoord() coord.System {
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 10)
	yScale.SetRange(0, 100)
	c := coord.NewCartesian2D(xScale, yScale)
	c.SetBounds(coord.Rect{X: 50, Y: 10, W: 500, H: 300})
	return c
}

var testPlotRect = coord.Rect{X: 50, Y: 10, W: 500, H: 300}

func base(id string) interact.DrawingBase {
	return interact.DrawingBase{ID_: interact.DrawingID(id), Color_: color.NRGBA{R: 255, G: 100, B: 100, A: 220}}
}

func TestRenderDrawings_HLine(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.HLineDrawing{DrawingBase: base("h1"), Y: 50},
	}
	objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, nil)
	if len(objs) == 0 {
		t.Error("expected objects for HLine")
	}
}

func TestRenderDrawings_VLine(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.VLineDrawing{DrawingBase: base("v1"), X: 5},
	}
	objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, nil)
	if len(objs) == 0 {
		t.Error("expected objects for VLine")
	}
}

func TestRenderDrawings_TrendLine(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.TrendLineDrawing{DrawingBase: base("t1"), Start: interact.DataPoint{X: 1, Y: 20}, End: interact.DataPoint{X: 8, Y: 80}},
	}
	objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, nil)
	if len(objs) == 0 {
		t.Error("expected objects for TrendLine")
	}
}

func TestRenderDrawings_Rectangle(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.RectangleDrawing{DrawingBase: base("r1"), Corner1: interact.DataPoint{X: 2, Y: 30}, Corner2: interact.DataPoint{X: 7, Y: 70}},
	}
	objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, nil)
	if len(objs) < 2 {
		t.Errorf("expected at least 2 objects (fill+border) for Rectangle, got %d", len(objs))
	}
}

func TestRenderDrawings_FibRetracement(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.FibRetracementDrawing{DrawingBase: base("f1"), HighY: 80, LowY: 20, X: 5},
	}
	objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, nil)
	// 7 fib levels × (dashed line segments + label) = many objects
	if len(objs) < 7 {
		t.Errorf("expected at least 7 objects for Fib, got %d", len(objs))
	}
}

func TestRenderDrawings_TextLabel(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.TextLabelDrawing{DrawingBase: base("tl1"), Pos: interact.DataPoint{X: 5, Y: 50}, Text: "Hello"},
	}
	objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, nil)
	if len(objs) < 2 {
		t.Errorf("expected at least 2 objects (bg+text) for TextLabel, got %d", len(objs))
	}
}

func TestRenderDrawings_Selected(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.HLineDrawing{DrawingBase: base("h1"), Y: 50},
	}
	objs := RenderDrawings(drawings, "h1", []coord.System{cs}, testPlotRect, nil)
	// Selected: line + handle
	if len(objs) < 2 {
		t.Errorf("expected line + handle for selected HLine, got %d", len(objs))
	}
}

func TestRenderDrawings_EmptyCoord(t *testing.T) {
	objs := RenderDrawings(nil, "", nil, testPlotRect, nil)
	if objs != nil {
		t.Error("expected nil for empty coordSystems")
	}
}

func TestRenderMeasureOverlay(t *testing.T) {
	cs := makeDrawingCoord()
	objs := RenderMeasureOverlay(
		geo.NewPos(100, 100), geo.NewPos(300, 200),
		[]coord.System{cs},
	)
	if len(objs) == 0 {
		t.Error("expected objects for measure overlay")
	}
}

func TestRenderDrawingPreview_TrendLine(t *testing.T) {
	cs := makeDrawingCoord()
	gesture := &interact.DrawingGestureState{
		Active: true,
		Tool:   interact.DrawingToolTrendLine,
		Step:   1,
		Points: [3]interact.DataPoint{{X: 2, Y: 30}},
		Color:  color.NRGBA{R: 100, G: 200, B: 100, A: 180},
	}
	objs := RenderDrawingPreview(gesture, geo.NewPos(400, 200), []coord.System{cs}, testPlotRect, nil)
	if len(objs) == 0 {
		t.Error("expected preview line for in-progress trend line")
	}
}

// A text drawing's chip is the theme's annotation background, so it reads on a
// light theme too; it was a fixed dark grey (#343).
func TestRenderDrawings_TextLabelChipFollowsTheTheme(t *testing.T) {
	cs := makeDrawingCoord()
	drawings := []interact.Drawing{
		&interact.TextLabelDrawing{DrawingBase: base("tl1"), Pos: interact.DataPoint{X: 5, Y: 50}, Text: "Hello"},
	}
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		resolver := style.NewResolver(theme.DefaultTheme(), v)
		objs := RenderDrawings(drawings, "", []coord.System{cs}, testPlotRect, resolver)
		bg, ok := objs[0].(*canvas.Rectangle)
		if !ok {
			t.Fatalf("the first object is a %T, want the chip", objs[0])
		}
		if want := theme.DefaultTheme().Color(theme.ColorNameOverlayBackground, v); bg.FillColor != want {
			t.Errorf("variant %d: the chip is %v, want the theme's overlay background %v", v, bg.FillColor, want)
		}
	}
}
