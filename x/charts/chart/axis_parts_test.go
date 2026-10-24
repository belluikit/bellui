package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/series"
	"github.com/belluikit/bellui/x/charts/style"
)

// renderOpt renders opt at 400x300 through the chart's own renderer and
// returns it with the objects of that full rebuild.
func renderOpt(t *testing.T, opt Option) (*Chart, *chartRenderer) {
	t.Helper()
	test.NewTempApp(t)
	c := New(opt)
	c.Resize(geo.NewSize(400, 300))
	r := c.CreateRenderer().(*chartRenderer)
	r.fullRebuild(geo.NewSize(400, 300))
	return c, r
}

// axisLines counts the lines drawn in colour c: the axis line and tick marks
// of an axis given that LineColor. horizontal and vertical split them by
// direction.
func axisLines(objs []core.CanvasObject, c color.Color) (horizontal, vertical int) {
	for _, o := range objs {
		l, ok := o.(*canvas.Line)
		if !ok || !sameColor(l.StrokeColor, c) {
			continue
		}
		if l.Position1.Y == l.Position2.Y {
			horizontal++
		} else if l.Position1.X == l.Position2.X {
			vertical++
		}
	}
	return horizontal, vertical
}

// gridLines counts the grid (split) lines: the theme's grid colour at 0.5 px.
func gridLines(r *chartRenderer, objs []core.CanvasObject) (horizontal, vertical int) {
	gc := style.NewResolver(r.chart.Theme(), bellui.CurrentApp().Settings().ThemeVariant()).GridLineColor()
	for _, o := range objs {
		l, ok := o.(*canvas.Line)
		if !ok || l.StrokeWidth != 0.5 || !sameColor(l.StrokeColor, gc) {
			continue
		}
		if l.Position1.Y == l.Position2.Y {
			horizontal++
		} else if l.Position1.X == l.Position2.X {
			vertical++
		}
	}
	return horizontal, vertical
}

// textsIn returns the texts drawn in colour c.
func textsIn(objs []core.CanvasObject, c color.Color) []*canvas.Text {
	var out []*canvas.Text
	for _, o := range objs {
		if txt, ok := o.(*canvas.Text); ok && sameColor(txt.Color, c) {
			out = append(out, txt)
		}
	}
	return out
}

var (
	xLine  = color.NRGBA{R: 200, A: 255}
	yLine  = color.NRGBA{G: 200, A: 255}
	y2Line = color.NRGBA{B: 200, A: 255}
	xText  = color.NRGBA{R: 201, G: 1, A: 255}
	yText  = color.NRGBA{G: 201, B: 1, A: 255}
	y2Text = color.NRGBA{R: 1, B: 201, A: 255}
)

// partsOption is a chart with a category X axis (three ticks), a left value
// Y axis (0..100, six ticks) and a right value Y axis (0..10, six ticks),
// each with its own line and label colour so its objects can be told apart.
func partsOption(edit func(x, y, y2 *axis.Config)) Option {
	x := categoryX()
	x.LineColor, x.LabelColor = xLine, xText
	y := valueY()
	y.LineColor, y.LabelColor = yLine, yText
	lo, hi := 0.0, 10.0
	y2 := axis.Config{Type: axis.TypeValue, Min: &lo, Max: &hi, LineColor: y2Line, LabelColor: y2Text}
	if edit != nil {
		edit(&x, &y, &y2)
	}
	return Option{
		XAxis: []axis.Config{x},
		YAxis: []axis.Config{y, y2},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "a", Data: []float64{0, 50, 100}},
			&series.LineSeries{SeriesName: "b", Data: []float64{1, 2, 10}, YAxisIndex: 1},
		},
	}
}

// Each axis.Config.Hide field hides its part and only that part (#684).
func TestAxisHide_Parts(t *testing.T) {
	_, base := renderOpt(t, partsOption(nil))
	bxh, bxv := axisLines(base.objects, xLine)
	byh, byv := axisLines(base.objects, yLine)
	by2h, by2v := axisLines(base.objects, y2Line)
	// The fixture draws what the cases below take away.
	if bxh != 1 || bxv != 3 || byv != 1 || byh != 0 || by2v != 1 || by2h != 6 {
		t.Fatalf("baseline lines: X %d horizontal/%d vertical, Y %d/%d, Y2 %d/%d; want X 1/3, Y 0/1, Y2 6/1",
			bxh, bxv, byh, byv, by2h, by2v)
	}
	if n := len(textsIn(base.objects, xText)); n != 3 {
		t.Fatalf("baseline: %d X labels, want 3", n)
	}
	if n, n2 := len(textsIn(base.objects, yText)), len(textsIn(base.objects, y2Text)); n != 6 || n2 != 6 {
		t.Fatalf("baseline: %d Y and %d Y2 labels, want 6 each", n, n2)
	}

	t.Run("Line", func(t *testing.T) {
		_, r := renderOpt(t, partsOption(func(x, y, y2 *axis.Config) {
			x.Hide.Line, y.Hide.Line, y2.Hide.Line = true, true, true
		}))
		if h, v := axisLines(r.objects, xLine); h != 0 || v != 3 {
			t.Errorf("X axis: %d horizontal, %d vertical lines; want the axis line gone (0) and the 3 ticks kept", h, v)
		}
		if h, v := axisLines(r.objects, yLine); h != 0 || v != 0 {
			t.Errorf("Y axis: %d horizontal, %d vertical lines; want none", h, v)
		}
		if h, v := axisLines(r.objects, y2Line); h != 6 || v != 0 {
			t.Errorf("right Y axis: %d horizontal, %d vertical lines; want the axis line gone (0) and the 6 ticks kept", h, v)
		}
		if n := len(textsIn(r.objects, xText)); n != 3 {
			t.Errorf("hiding the line also changed the labels: %d X labels, want 3", n)
		}
	})

	t.Run("Ticks", func(t *testing.T) {
		_, r := renderOpt(t, partsOption(func(x, _, y2 *axis.Config) {
			x.Hide.Ticks, y2.Hide.Ticks = true, true
		}))
		if h, v := axisLines(r.objects, xLine); h != 1 || v != 0 {
			t.Errorf("X axis: %d horizontal, %d vertical lines; want the line kept (1) and the ticks gone (0)", h, v)
		}
		if h, v := axisLines(r.objects, y2Line); h != 0 || v != 1 {
			t.Errorf("right Y axis: %d horizontal, %d vertical lines; want the ticks gone (0) and the line kept (1)", h, v)
		}
	})

	t.Run("Labels", func(t *testing.T) {
		_, r := renderOpt(t, partsOption(func(x, y, y2 *axis.Config) {
			x.Hide.Labels, y.Hide.Labels, y2.Hide.Labels = true, true, true
			y.Name = "Sales"
		}))
		if n := len(textsIn(r.objects, xText)); n != 0 {
			t.Errorf("%d X labels drawn, want none", n)
		}
		if got := textsIn(r.objects, yText); len(got) != 1 || got[0].Text != "Sales" {
			t.Errorf("Y texts %v, want only the axis name Sales", got)
		}
		if n := len(textsIn(r.objects, y2Text)); n != 0 {
			t.Errorf("%d right Y labels drawn, want none", n)
		}
		if h, v := axisLines(r.objects, xLine); h != 1 || v != 3 {
			t.Errorf("hiding labels changed the X lines: %d/%d, want 1/3", h, v)
		}
	})

	t.Run("SplitLine", func(t *testing.T) {
		_, r := renderOpt(t, partsOption(nil))
		if h, v := gridLines(r, r.objects); h != 6 || v != 0 {
			t.Fatalf("default: %d horizontal, %d vertical grid lines; want the first Y axis's 6 only", h, v)
		}
		_, r = renderOpt(t, partsOption(func(_, y, _ *axis.Config) { y.SplitLine = axis.SplitLineHide }))
		if h, v := gridLines(r, r.objects); h != 0 || v != 0 {
			t.Errorf("SplitLineHide on the first Y axis: %d/%d grid lines, want none", h, v)
		}
		_, r = renderOpt(t, partsOption(func(x, _, _ *axis.Config) { x.SplitLine = axis.SplitLineShow }))
		if h, v := gridLines(r, r.objects); h != 6 || v != 3 {
			t.Errorf("SplitLineShow on the X axis: %d horizontal, %d vertical grid lines; want 6 and 3", h, v)
		}
		_, r = renderOpt(t, partsOption(func(_, y, y2 *axis.Config) {
			y.SplitLine, y2.SplitLine = axis.SplitLineHide, axis.SplitLineShow
		}))
		if h, v := gridLines(r, r.objects); h != 6 || v != 0 {
			t.Errorf("SplitLineShow on the second Y axis only: %d/%d grid lines, want its 6 horizontal", h, v)
		}
	})
}

// labelSpan is the x range a set of labels' boxes covers.
func labelSpan(texts []*canvas.Text) (lo, hi float32) {
	lo, hi = 1e9, -1e9
	for _, txt := range texts {
		lo = min(lo, txt.Position().X)
		hi = max(hi, txt.Position().X+txt.Size().Width)
	}
	return lo, hi
}

// Three Y axes, two of them on the right, one of those with an Offset, are
// drawn side by side: each axis's labels cover their own x range, and every
// range is inside the chart (#684, item from #697).
func TestAxisOffset_ThreeYAxes(t *testing.T) {
	y3Text := color.NRGBA{R: 120, G: 120, B: 1, A: 255}
	opt := partsOption(nil)
	lo, hi := 0.0, 1000.0
	opt.YAxis = append(opt.YAxis, axis.Config{
		Type: axis.TypeValue, Min: &lo, Max: &hi,
		Position: axis.PositionRight, Offset: 60, LabelColor: y3Text, Name: "Third",
	})
	opt.Series = append(opt.Series, &series.LineSeries{SeriesName: "c", Data: []float64{10, 500, 900}, YAxisIndex: 2})
	c, r := renderOpt(t, opt)
	width := c.Size().Width
	plot := c.renderSnap.plotRect

	type span struct {
		name   string
		lo, hi float32
	}
	var spans []span
	for _, a := range []struct {
		name string
		c    color.Color
	}{{"left", yText}, {"right", y2Text}, {"right offset 60", y3Text}} {
		texts := textsIn(r.objects, a.c)
		var ticks []*canvas.Text
		for _, txt := range texts {
			if txt.Text != "Third" {
				ticks = append(ticks, txt)
			}
		}
		if len(ticks) != 6 {
			t.Fatalf("%s axis: %d tick labels, want 6", a.name, len(ticks))
		}
		l, h := labelSpan(ticks)
		if l < 0 || h > width {
			t.Errorf("%s axis labels span x %v..%v, want inside the chart's 0..%v", a.name, l, h, width)
		}
		spans = append(spans, span{a.name, l, h})
	}
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			a, b := spans[i], spans[j]
			if a.lo < b.hi && b.lo < a.hi {
				t.Errorf("%s labels (x %v..%v) overlap %s labels (x %v..%v)", a.name, a.lo, a.hi, b.name, b.lo, b.hi)
			}
		}
	}

	// The offset axis's line sits 60 px right of the plot, and its name
	// starts there and stays inside the chart.
	var found bool
	for _, o := range r.objects {
		if l, ok := o.(*canvas.Line); ok && l.Position1.X == plot.X+plot.W+60 && l.Position2.X == l.Position1.X &&
			l.Position1.Y == plot.Y && l.Position2.Y == plot.Y+plot.H {
			found = true
		}
	}
	if !found {
		t.Errorf("no axis line at x=%v, 60 px right of the plot", plot.X+plot.W+60)
	}
	if name := findText(r.objects, "Third"); name == nil || name.Position().X != plot.X+plot.W+60 || name.Position().X+name.Size().Width > width {
		t.Errorf("offset axis name %v, want it at x=%v and inside the chart's %v", name, plot.X+plot.W+60, width)
	}
}

// Two left Y axes, the second offset by the grid's left margin, keep their
// labels apart and inside the chart; the plot moves right to make room.
func TestAxisOffset_TwoLeftYAxes(t *testing.T) {
	grid := DefaultGrid()
	without, _ := renderOpt(t, partsOption(nil))
	opt := partsOption(func(_, _, y2 *axis.Config) {
		y2.Position, y2.Offset = axis.PositionLeft, grid.Left
	})
	c, r := renderOpt(t, opt)
	plot := c.renderSnap.plotRect
	if want := without.renderSnap.plotRect.X + grid.Left; plot.X != want {
		t.Errorf("plot starts at x=%v, want %v: moved right by the offset", plot.X, want)
	}
	l1, h1 := labelSpan(textsIn(r.objects, yText))
	l2, h2 := labelSpan(textsIn(r.objects, y2Text))
	if l1 < 0 || l2 < 0 || h1 > plot.X || h2 > plot.X {
		t.Errorf("left labels span %v..%v and %v..%v, want both inside 0..%v", l1, h1, l2, h2, plot.X)
	}
	if l1 < h2 && l2 < h1 {
		t.Errorf("left axis labels (x %v..%v) overlap the offset left axis's (x %v..%v)", l1, h1, l2, h2)
	}
}

// An offset X axis moves out with its labels, which stay inside the chart,
// at the bottom and at the top.
func TestAxisOffset_XAxis(t *testing.T) {
	for _, top := range []bool{false, true} {
		c, r := renderOpt(t, partsOption(func(x, _, _ *axis.Config) {
			x.Offset, x.Name = 20, "Weekday"
			if top {
				x.Position = axis.PositionTop
			}
		}))
		plot := c.renderSnap.plotRect
		wantY := plot.Y + plot.H + 20
		if top {
			wantY = plot.Y - 20
		}
		var line bool
		for _, o := range r.objects {
			if l, ok := o.(*canvas.Line); ok && sameColor(l.StrokeColor, xLine) && l.Position1.Y == wantY && l.Position2.Y == wantY {
				line = true
			}
		}
		if !line {
			t.Errorf("top=%v: no X axis line at y=%v, 20 px out from the plot", top, wantY)
		}
		for _, txt := range textsIn(r.objects, xText) {
			if b := txt.Position().Y + txt.Size().Height; txt.Position().Y < 0 || b > c.Size().Height {
				t.Errorf("top=%v: %q at y %v..%v, want inside the chart's 0..%v", top, txt.Text, txt.Position().Y, b, c.Size().Height)
			}
		}
	}
}

// The pointer tags sit beside an offset axis, not at the plot edge: a series
// on the right Y axis offset by 60 gets its Y tag beyond that axis, and an X
// axis offset by 20 its X tag beyond it.
func TestPointerTags_FollowAxisOffset(t *testing.T) {
	opt := partsOption(func(x, _, y2 *axis.Config) {
		x.Offset = 20
		y2.Offset = 60
	})
	opt.Tooltip = pointerTags
	c, r := renderOpt(t, opt)
	plot := c.renderSnap.plotRect

	full, partial := hoverPoint(t, c, r, 1, 1, 1, 2)
	for name, objs := range map[string][]core.CanvasObject{"full rebuild": full, "partial refresh": partial} {
		yTag, xTag := findText(objs, "2.00"), findText(objs, "1.0")
		if yTag == nil || xTag == nil {
			t.Fatalf("%s: Y tag %v, X tag %v, want both", name, yTag, xTag)
		}
		if l := yTag.Position().X; l < plot.X+plot.W+60 {
			t.Errorf("%s: Y tag at x=%v, want it beyond the offset axis at %v", name, l, plot.X+plot.W+60)
		}
		if y := xTag.Position().Y; y < plot.Y+plot.H+20 {
			t.Errorf("%s: X tag at y=%v, want it below the offset X axis at %v", name, y, plot.Y+plot.H+20)
		}
	}
}
