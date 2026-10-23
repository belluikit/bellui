package chart

import (
	"image"
	"image/color"
	"testing"

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
