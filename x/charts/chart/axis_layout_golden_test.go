package chart

import (
	"flag"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/series"
)

var updateAxisGolden = flag.Bool("update-axis-golden", false, "rewrite testdata/axis_literal_layout.golden")

// describeObjects writes every drawn object's type, geometry, colour and text
// in order, so two renders compare exactly. Coordinates are rounded to 0.01 px:
// Go may fuse x*y+z into one rounding on arm64 but not on amd64, so the last
// float32 digit of a position differs between them.
func describeObjects(objs []core.CanvasObject) string {
	var b strings.Builder
	num := func(v float32) string {
		return strconv.FormatFloat(math.Round(float64(v)*100)/100, 'f', -1, 64)
	}
	pos := func(p geo.Position) string { return "{" + num(p.X) + " " + num(p.Y) + "}" }
	size := func(s geo.Size) string { return "{" + num(s.Width) + " " + num(s.Height) + "}" }
	col := func(c color.Color) string {
		if c == nil {
			return "nil"
		}
		r, g, bl, a := c.RGBA()
		return fmt.Sprintf("%04x%04x%04x%04x", r, g, bl, a)
	}
	for _, o := range objs {
		switch v := o.(type) {
		case *canvas.Line:
			fmt.Fprintf(&b, "line %s %s %s %s\n", pos(v.Position1), pos(v.Position2), col(v.StrokeColor), num(v.StrokeWidth))
		case *canvas.Text:
			fmt.Fprintf(&b, "text %q %s %s %s %s %d\n", v.Text, pos(v.Position()), size(v.Size()), col(v.Color), num(v.TextSize), v.Alignment)
		case *canvas.Rectangle:
			fmt.Fprintf(&b, "rect %s %s %s %s %s\n", pos(v.Position()), size(v.Size()), col(v.FillColor), col(v.StrokeColor), num(v.StrokeWidth))
		default:
			fmt.Fprintf(&b, "%T %s %s %v\n", o, pos(o.Position()), size(o.Size()), o.Visible())
		}
	}
	return b.String()
}

// Configs written as struct literals — the README's quick start, the builder,
// the demos — leave the axis.Config switches at their zero values. Each case
// here renders such a chart, and the whole set must match the layout recorded
// before #684 added axis.Config.Hide, SplitLine and Offset: an existing chart
// draws exactly the same objects in the same places and colours.
//
// The golden file was written by this test on the code before those fields
// existed (go test -run TestAxisLiteralConfigs_RenderAsBefore -update-axis-golden),
// and must match on both arm64 and amd64.
func TestAxisLiteralConfigs_RenderAsBefore(t *testing.T) {
	lo, hi := 0.0, 100.0
	cases := []struct {
		name string
		opt  Option
	}{
		{"readme quick start", Option{
			XAxis:  []axis.Config{{Type: axis.TypeCategory, Data: []string{"Mon", "Tue", "Wed"}}},
			YAxis:  []axis.Config{{Type: axis.TypeValue}},
			Series: []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{10, 50, 90}}},
		}},
		{"default configs", Option{
			XAxis:  []axis.Config{axis.DefaultConfig(axis.TypeCategory)},
			YAxis:  []axis.Config{axis.DefaultConfig(axis.TypeValue)},
			Series: []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{10, 50, 90}}},
		}},
		{"value x axis", Option{
			XAxis:  []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
			YAxis:  []axis.Config{{Type: axis.TypeValue, SplitCount: 5}},
			Series: []series.Series{&series.ScatterSeries{SeriesName: "s", Data: []float64{1, 2, 3, 4, 5, 1}}},
		}},
		{"three y axes, names, top x", Option{
			XAxis: []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}, Position: axis.PositionTop, Name: "Day"}},
			YAxis: []axis.Config{
				{Type: axis.TypeValue, Name: "Sales", Min: &lo, Max: &hi},
				{Type: axis.TypeValue, Name: "Share"},
				{Type: axis.TypeValue, Position: axis.PositionRight},
			},
			Series: []series.Series{
				&series.LineSeries{SeriesName: "a", Data: []float64{0, 50, 100}},
				&series.LineSeries{SeriesName: "b", Data: []float64{1, 2, 3}, YAxisIndex: 1},
				&series.BarSeries{SeriesName: "c", Data: []float64{7, 8, 9}, YAxisIndex: 2},
			},
		}},
		{"right y axis, bottom name, tight grid", Option{
			Grid:   &Grid{Left: 60, Right: 20, Top: 10, Bottom: 10},
			XAxis:  []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}, Name: "Weekday", Inverse: true}},
			YAxis:  []axis.Config{{Type: axis.TypeValue, Position: axis.PositionRight, Name: "Units"}},
			Series: []series.Series{&series.BarSeries{SeriesName: "s", Data: []float64{3, 1, 2}}},
		}},
	}

	var b strings.Builder
	for _, tc := range cases {
		test.NewTempApp(t)
		c := New(tc.opt)
		c.Resize(geo.NewSize(400, 300))
		r := c.CreateRenderer().(*chartRenderer)
		r.fullRebuild(geo.NewSize(400, 300))
		fmt.Fprintf(&b, "== %s\n%s", tc.name, describeObjects(r.objects))
	}
	got := b.String()

	path := filepath.Join("testdata", "axis_literal_layout.golden")
	if *updateAxisGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := 0; i < len(gl) || i < len(wl); i++ {
			var g, w string
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Fatalf("render differs from the recorded layout at line %d:\n got: %s\nwant: %s", i+1, g, w)
			}
		}
	}
}
