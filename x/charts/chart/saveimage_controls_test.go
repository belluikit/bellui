package chart

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
	"github.com/belluikit/bellui/x/charts/style"
)

// controlsChart is a bar chart with every control the capture leaves out: the
// toolbox, the zoom buttons, and a tooltip with a crosshair and axis tags.
func controlsChart() *Chart {
	return New(Option{
		XAxis:    []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}}},
		YAxis:    []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series:   []series.Series{&series.BarSeries{SeriesName: "s", Data: []float64{3, 5, 4}}},
		Toolbox:  &component.ToolboxConfig{SaveAsImage: true, MagicType: []string{"line", "bar"}},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Tooltip: &component.TooltipConfig{
			Trigger: component.TooltipTriggerItem, Crosshair: component.CrosshairBoth, ShowAxisTags: true,
		},
	})
}

// buttonRects lists every toolbox and zoom button the last render laid out.
func buttonRects(t *testing.T, c *Chart) []image.Rectangle {
	t.Helper()
	snap := c.renderSnap
	if snap == nil || snap.toolboxLayout == nil || snap.zoomButtons == nil {
		t.Fatal("the chart laid out no toolbox or zoom buttons")
	}
	rect := func(pos geo.Position, size geo.Size) image.Rectangle {
		return image.Rect(int(pos.X), int(pos.Y), int(pos.X+size.Width), int(pos.Y+size.Height))
	}
	var rs []image.Rectangle
	for _, b := range snap.toolboxLayout.Buttons {
		rs = append(rs, rect(b.Rect.Pos, b.Rect.Size))
	}
	z := snap.zoomButtons
	for _, b := range []struct {
		Pos  geo.Position
		Size geo.Size
	}{{z.ZoomInRect.Pos, z.ZoomInRect.Size}, {z.ZoomOutRect.Pos, z.ZoomOutRect.Size}, {z.ResetRect.Pos, z.ResetRect.Size}} {
		rs = append(rs, rect(b.Pos, b.Size))
	}
	return rs
}

// offBackground counts the pixels of img inside rs that are not the chart's
// background colour: every one of them is part of a button drawn there.
func offBackground(img image.Image, rs []image.Rectangle, bg color.Color) int {
	want := color.NRGBAModel.Convert(bg).(color.NRGBA)
	n := 0
	for _, r := range rs {
		r = r.Intersect(img.Bounds())
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) != want {
					n++
				}
			}
		}
	}
	return n
}

func chartBackground(c *Chart) color.Color {
	return style.NewResolver(c.Theme(), bellui.CurrentApp().Settings().ThemeVariant()).BackgroundColor()
}

// hasText reports whether the chart's cached renderer — the one the canvas
// paints — draws a text s.
func hasText(t *testing.T, c *Chart, s string) bool {
	t.Helper()
	return findText(test.TempWidgetRenderer(t, c).Objects(), s) != nil
}

// "Save as image" saves the chart only (#717): the toolbox and zoom buttons
// are not in the capture, while the same canvas captured directly shows them.
// Afterwards the chart draws them again.
func TestCaptureImage_LeavesOutToolboxAndZoomButtons(t *testing.T) {
	test.NewTempApp(t)
	c := controlsChart()
	w := test.NewTempWindow(t, c)
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))

	rects := buttonRects(t, c)
	bg := chartBackground(c)
	screen := w.Canvas().Capture()
	if n := offBackground(screen, rects, bg); n == 0 {
		t.Fatal("the canvas shows no button pixels where the toolbox and zoom buttons are laid out; the test measures nothing")
	} else {
		t.Logf("on screen: %d button pixels in %d button rects", n, len(rects))
	}

	img := c.captureImage()
	if img == nil {
		t.Fatal("captureImage returned nil for a chart on a canvas")
	}
	if b, s := img.Bounds(), screen.Bounds(); b.Dx() < s.Dx()-4 || b.Dy() < s.Dy()-4 {
		t.Fatalf("captured %v, want about the whole %v canvas the chart fills", b, s)
	}
	if n := offBackground(img, rects, bg); n != 0 {
		t.Errorf("captured image has %d toolbox/zoom button pixels, want 0", n)
	}

	for _, label := range []string{"Save", "line", "bar", "Reset"} {
		if !hasText(t, c, label) {
			t.Errorf("after the capture the chart does not draw its %q button", label)
		}
	}
	if n := offBackground(w.Canvas().Capture(), rects, bg); n == 0 {
		t.Error("after the capture the canvas shows no toolbox or zoom buttons")
	}
}

// A tooltip and axis pointer that are showing are left out of the capture:
// it is pixel-identical to the capture of the same chart with nothing hovered.
// The tooltip shows again afterwards.
func TestCaptureImage_LeavesOutTooltipAndPointer(t *testing.T) {
	test.NewTempApp(t)
	c := controlsChart()
	w := test.NewTempWindow(t, c)
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 300))

	plain := c.captureImage()
	if plain == nil {
		t.Fatal("captureImage returned nil for a chart on a canvas")
	}

	bar := c.renderSnap.coordSystems[0].DataToPixel(1, 5)
	c.handleHover(geo.NewPos(bar.X, bar.Y+10))
	if !c.tooltipState.Visible || len(c.tooltipState.Hits) == 0 {
		t.Fatal("hovering the bar showed no tooltip")
	}
	const tip = "s: 5.0"
	if !hasText(t, c, tip) {
		t.Fatalf("the chart does not draw the tooltip %q it is showing", tip)
	}

	hovered := c.captureImage()
	if hovered == nil {
		t.Fatal("captureImage returned nil with the tooltip showing")
	}
	if d := diffPixels(plain, hovered); d != 0 {
		t.Errorf("the capture with the tooltip showing differs from the plain one in %d pixels, want 0", d)
	}
	if d := diffPixels(plain, w.Canvas().Capture()); d == 0 {
		t.Error("the canvas with the tooltip showing equals the plain capture; the tooltip was never on screen")
	}
	if !hasText(t, c, tip) || !c.tooltipState.Visible {
		t.Error("after the capture the tooltip is no longer drawn")
	}
}

// A capture that yields nothing — the chart is scrolled wholly out of view —
// still gives the chart its controls back.
func TestCaptureImage_RestoresControlsWhenNothingIsCaptured(t *testing.T) {
	test.NewTempApp(t)
	c := controlsChart()
	scroll := container.NewVScroll(container.NewVBox(
		container.NewGridWrap(geo.NewSize(400, 400)), // spacer above the chart
		container.NewGridWrap(geo.NewSize(400, 300), c),
	))
	w := test.NewTempWindow(t, scroll)
	w.SetPadded(false)
	w.Resize(geo.NewSize(400, 200))
	if bellui.CurrentApp().Driver().CanvasForObject(c) == nil || c.renderSnap == nil {
		t.Fatal("the chart is not on the canvas, so the capture would return before drawing anything")
	}

	if img := c.captureImage(); img != nil {
		t.Fatalf("captured %v for a chart out of view, want nil", img.Bounds())
	}
	if !hasText(t, c, "Save") || !hasText(t, c, "Reset") {
		t.Error("after a capture that returned nil the chart does not draw its toolbox and zoom buttons")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.capturing {
		t.Error("the chart is still in capture drawing after the capture returned")
	}
}

func diffPixels(a, b image.Image) int {
	if a.Bounds().Size() != b.Bounds().Size() {
		return a.Bounds().Dx() * a.Bounds().Dy()
	}
	n := 0
	ab, bb := a.Bounds(), b.Bounds()
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ca := color.NRGBAModel.Convert(a.At(ab.Min.X+x, ab.Min.Y+y))
			cb := color.NRGBAModel.Convert(b.At(bb.Min.X+x, bb.Min.Y+y))
			if ca != cb {
				n++
			}
		}
	}
	return n
}
