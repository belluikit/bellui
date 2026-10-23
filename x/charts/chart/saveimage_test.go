package chart

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/series"
)

// countNear counts the pixels of img within tol of want on every channel.
func countNear(img image.Image, want color.NRGBA, tol int) int {
	near := func(a, b uint8) bool {
		d := int(a) - int(b)
		return d >= -tol && d <= tol
	}
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if near(c.R, want.R) && near(c.G, want.G) && near(c.B, want.B) {
				n++
			}
		}
	}
	return n
}

// TestCaptureImage_DrawsTheChart pins the toolbox's "save as image" (#339): it
// wrote a PNG of the background colour alone, because captureImage filled the
// background and returned without drawing the chart.
func TestCaptureImage_DrawsTheChart(t *testing.T) {
	test.NewTempApp(t)

	red := color.NRGBA{R: 230, G: 20, B: 20, A: 255}
	c := New(Option{
		XAxis: []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis: []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series: []series.Series{
			&series.BarSeries{SeriesName: "s", Data: []float64{3, 5, 4}, Color: red},
		},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	c.Resize(geo.NewSize(400, 300))

	img := c.captureImage()
	if img == nil {
		t.Fatal("captureImage returned nil for a chart on a canvas")
	}
	if b := img.Bounds(); b.Dx() < 300 || b.Dy() < 200 {
		t.Fatalf("captured %v, want roughly the chart's 400x300", b)
	}
	// Three bars of a few hundred pixels each: anything in the thousands means
	// the series was drawn, zero means only the background was.
	n := countNear(img, red, 12)
	t.Logf("captured %v, %d bar-coloured pixels", img.Bounds(), n)
	if n < 1000 {
		t.Errorf("captured image has %d bar-coloured pixels, want the bars drawn (>= 1000)", n)
	}
}

// A chart partly scrolled out of view: the capture holds only the part the
// scroll shows, not the widgets drawn where the rest of the chart would be.
// Every other fixture puts the chart at the canvas origin, unclipped.
func TestCaptureImage_ClippedByScroll(t *testing.T) {
	test.NewTempApp(t)

	c := New(Option{
		XAxis:  []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis:  []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series: []series.Series{&series.BarSeries{SeriesName: "s", Data: []float64{3, 5, 4}}},
	})
	// GridWrap holds the chart at 400x300, taller than the 150 px scroll.
	scroll := container.NewVScroll(container.NewGridWrap(geo.NewSize(400, 300), c))
	scroll.Move(geo.NewPos(0, 50))
	scroll.Resize(geo.NewSize(400, 150))

	green := color.NRGBA{G: 220, A: 255}
	header := canvas.NewRectangle(green)
	header.Resize(geo.NewSize(400, 50))

	w := test.NewTempWindow(t, container.NewWithoutLayout(header, scroll))
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 200))
	scroll.Offset = geo.NewPos(0, 100) // the chart's top 100 px are hidden
	scroll.Refresh()

	if pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(c); pos.Y > 0 {
		t.Fatalf("chart at y=%v, want it scrolled partly above the scroll's top", pos.Y)
	}
	img := c.captureImage()
	if img == nil {
		t.Fatal("captureImage returned nil for a chart partly in view")
	}
	if n := countNear(img, green, 12); n > 0 {
		t.Errorf("captured %v holds %d pixels of the header above the scroll, want none", img.Bounds(), n)
	}
	if b := img.Bounds(); b.Dy() > 150 {
		t.Errorf("captured %v, want at most the 150 px the scroll shows", b)
	}
}
