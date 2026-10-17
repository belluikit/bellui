package chart

import (
	"testing"

	fynecanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

func TestBuildCartesianScales(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		XAxis: []axis.Config{axis.DefaultConfig(axis.TypeCategory)},
		YAxis: []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "line1",
				Data:       []float64{10, 50, 90, 30, 70},
			},
		},
	}

	c := New(opt)
	c.Resize(test.NewApp().NewWindow("").Canvas().Size())
	c.Resize(fyne800x600())

	// Verify the chart was created and has the right option
	gotOpt := c.Option()
	if len(gotOpt.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(gotOpt.Series))
	}
	if gotOpt.Series[0].Name() != "line1" {
		t.Errorf("expected series name 'line1', got %q", gotOpt.Series[0].Name())
	}
}

func TestBuildCartesianScales_MultiYAxis(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		XAxis: []axis.Config{axis.DefaultConfig(axis.TypeCategory)},
		YAxis: []axis.Config{
			axis.DefaultConfig(axis.TypeValue),
			axis.DefaultConfig(axis.TypeValue),
		},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "temp",
				Data:       []float64{20, 25, 30, 28},
				YAxisIndex: 0,
			},
			&series.BarSeries{
				SeriesName: "rain",
				Data:       []float64{100, 200, 150, 300},
				YAxisIndex: 1,
			},
		},
	}

	c := New(opt)
	c.Resize(fyne800x600())

	gotOpt := c.Option()
	if len(gotOpt.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(gotOpt.Series))
	}
	if len(gotOpt.YAxis) != 2 {
		t.Fatalf("expected 2 Y axes, got %d", len(gotOpt.YAxis))
	}
}

func TestSeriesClassification_3D(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.Bar3DSeries{
				SeriesName: "3d",
				Data: []series.Bar3DData{
					{X: 0, Z: 0, Value: 5},
				},
			},
		},
	}

	c := New(opt)
	c.Resize(fyne800x600())

	// The chart should render without panic. Verify it has a series.
	gotOpt := c.Option()
	if len(gotOpt.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(gotOpt.Series))
	}
	if gotOpt.Series[0].SeriesType() != series.TypeBar3D {
		t.Errorf("expected TypeBar3D, got %v", gotOpt.Series[0].SeriesType())
	}
}

func TestSeriesClassification_Mixed(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		XAxis: []axis.Config{axis.DefaultConfig(axis.TypeCategory)},
		YAxis: []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series: []series.Series{
			&series.LineSeries{
				SeriesName: "line",
				Data:       []float64{10, 20, 30},
			},
			&series.PieSeries{
				SeriesName: "pie",
				Data: []series.PieDataItem{
					{Name: "A", Value: 40},
					{Name: "B", Value: 60},
				},
			},
		},
	}

	c := New(opt)
	c.Resize(fyne800x600())

	gotOpt := c.Option()
	if len(gotOpt.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(gotOpt.Series))
	}
}

func TestSetOption_UpdatesSeries(t *testing.T) {
	_ = test.NewApp()

	opt1 := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "v1", Data: []float64{1, 2, 3}},
		},
	}
	c := New(opt1)
	c.Resize(fyne800x600())

	opt2 := Option{
		Series: []series.Series{
			&series.BarSeries{SeriesName: "v2", Data: []float64{10, 20, 30}},
		},
	}
	c.SetOption(opt2)

	gotOpt := c.Option()
	if len(gotOpt.Series) != 1 {
		t.Fatalf("expected 1 series after SetOption, got %d", len(gotOpt.Series))
	}
	if gotOpt.Series[0].Name() != "v2" {
		t.Errorf("expected series name 'v2', got %q", gotOpt.Series[0].Name())
	}
}

func TestSeriesVisibility(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{1, 2, 3}},
			&series.LineSeries{SeriesName: "B", Data: []float64{4, 5, 6}},
		},
	}
	c := New(opt)

	if !c.IsSeriesVisible("A") {
		t.Error("series A should be visible by default")
	}
	if !c.IsSeriesVisible("B") {
		t.Error("series B should be visible by default")
	}
	// Non-existent series should be "visible" (not hidden)
	if !c.IsSeriesVisible("C") {
		t.Error("non-existent series should report as visible")
	}
}

func TestLineSeries_DataRange(t *testing.T) {
	ls := &series.LineSeries{
		SeriesName: "test",
		Data:       []float64{10, -5, 30, 20},
	}
	dr, ok := (interface{})(ls).(series.DataRanger)
	if !ok {
		t.Fatal("LineSeries should implement DataRanger")
	}
	xMin, xMax, yMin, yMax := dr.DataRange()
	if xMin != 0 || xMax != 3 {
		t.Errorf("xRange: got [%f, %f], want [0, 3]", xMin, xMax)
	}
	if yMin != -5 || yMax != 30 {
		t.Errorf("yRange: got [%f, %f], want [-5, 30]", yMin, yMax)
	}
}

func TestBarSeries_DataRange(t *testing.T) {
	bs := &series.BarSeries{
		SeriesName: "test",
		Data:       []float64{10, 50, -10, 30},
	}
	xMin, xMax, yMin, yMax := bs.DataRange()
	if xMin != 0 || xMax != 3 {
		t.Errorf("xRange: got [%f, %f], want [0, 3]", xMin, xMax)
	}
	// Bar always includes 0 in Y
	if yMin != -10 || yMax != 50 {
		t.Errorf("yRange: got [%f, %f], want [-10, 50]", yMin, yMax)
	}
}

func TestAppendData_LineSeries(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	// Append new data points.
	c.AppendData(0, 40, 50)

	// Verify the series data was extended in-place.
	gotOpt := c.Option()
	ls := gotOpt.Series[0].(*series.LineSeries)
	if len(ls.Data) != 5 {
		t.Fatalf("expected 5 data points, got %d", len(ls.Data))
	}
	if ls.Data[3] != 40 || ls.Data[4] != 50 {
		t.Errorf("expected appended values [40, 50], got [%v, %v]", ls.Data[3], ls.Data[4])
	}
}

func TestAppendData_PreservesLegendToggles(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 20}},
			&series.LineSeries{SeriesName: "B", Data: []float64{30, 40}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	// Toggle series B to hidden.
	c.mu.Lock()
	c.legendToggles["B"] = true
	c.mu.Unlock()

	// Append data — should NOT reset legend toggles.
	c.AppendData(0, 50)

	if c.IsSeriesVisible("B") {
		t.Error("expected series B to remain hidden after AppendData")
	}
}

func TestAppendData_OutOfRange(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10}},
		},
	}
	c := New(opt)

	// Should not panic for out-of-range index.
	c.AppendData(-1, 20)
	c.AppendData(5, 20)
}

func TestAppendCandlestick(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.CandlestickSeries{SeriesName: "C", Data: []series.CandlestickData{
				{Open: 10, High: 15, Low: 8, Close: 12},
			}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	c.AppendCandlestick(0, series.CandlestickData{Open: 12, High: 18, Low: 11, Close: 16})

	gotOpt := c.Option()
	cs := gotOpt.Series[0].(*series.CandlestickSeries)
	if len(cs.Data) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(cs.Data))
	}
	if cs.Data[1].Close != 16 {
		t.Errorf("expected appended candle close 16, got %v", cs.Data[1].Close)
	}
}

func TestFollowLatest_SnapsToEnd(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30, 40, 50}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	// Zoom in to show only the last 20%.
	c.SetZoomWindow(interact.DataWindow{Start: 0.8, End: 1.0})

	// Enable follow latest.
	c.SetFollowLatest(true)

	// Append data — viewport should snap End to 1.0.
	c.AppendData(0, 60, 70, 80)

	w := c.ZoomWindow()
	if w.End != 1.0 {
		t.Errorf("expected zoom End=1.0 after AppendData with FollowLatest, got %v", w.End)
	}
	if w.Span() < 0.19 || w.Span() > 0.21 {
		t.Errorf("expected zoom span ~0.2 preserved, got %v", w.Span())
	}
}

// A pinch on a zoomable chart zooms its data window around the focus (#379).
func TestChart_PinchZoomsDataWindow(t *testing.T) {
	_ = test.NewApp()
	c := New(Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 20, 30, 40, 50}},
		},
	})
	c.Resize(fyne800x600())
	var notified int
	c.OnZoomChange = func(interact.DataWindow) { notified++ }

	c.Pinched(&event.PinchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(400, 300)}, Scale: 2})
	if w := c.ZoomWindow(); w.Span() < 0.49 || w.Span() > 0.51 {
		t.Errorf("a 2x pinch should halve the visible span, got %v", w.Span())
	}
	if notified != 1 {
		t.Errorf("OnZoomChange should fire once, got %d", notified)
	}
}

func TestKeyboardEvents(t *testing.T) {
	_ = test.NewApp()

	c := New(Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 20}},
		},
	})

	var receivedRune rune
	var receivedKey event.KeyName
	c.Events.On(interact.EventKeyPress, func(evt interface{}) {
		e := evt.(interact.KeyPressEvent)
		receivedRune = e.Rune
		receivedKey = e.KeyName
	})

	// Simulate typed rune.
	c.TypedRune('z')
	if receivedRune != 'z' {
		t.Errorf("expected rune 'z', got '%c'", receivedRune)
	}

	// Simulate typed key.
	c.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	if receivedKey != event.KeyEscape {
		t.Errorf("expected KeyEscape, got %v", receivedKey)
	}
}

func TestPartialRefresh_TooltipOnly(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		XAxis: []axis.Config{axis.DefaultConfig(axis.TypeCategory)},
		YAxis: []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 50, 90, 30}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	renderer := c.CreateRenderer().(*chartRenderer)

	// Force a full rebuild to populate cache.
	renderer.fullRebuild(fyne800x600())
	if !renderer.hasCachedBase {
		t.Fatal("expected hasCachedBase after full rebuild")
	}
	if len(renderer.cachedBaseObjs) == 0 {
		t.Fatal("expected non-empty cachedBaseObjs")
	}
	baseCount := len(renderer.cachedBaseObjs)

	// Simulate a tooltip-only dirty flag (as if mouse moved).
	c.mu.Lock()
	c.pendingDirty = dirtyTooltip
	c.mu.Unlock()

	// Refresh should take the partial path.
	renderer.Refresh()

	// Objects should include base objects.
	if len(renderer.objects) < baseCount {
		t.Errorf("expected at least %d objects, got %d", baseCount, len(renderer.objects))
	}

	// Verify that the base objects are the exact same slice prefix.
	for i := 0; i < baseCount && i < len(renderer.objects); i++ {
		if renderer.objects[i] != renderer.cachedBaseObjs[i] {
			t.Errorf("base object %d changed during partial refresh", i)
			break
		}
	}
}

func TestPartialRefresh_FallsBackOnDirtyFull(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 50, 90}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	renderer := c.CreateRenderer().(*chartRenderer)
	renderer.fullRebuild(fyne800x600())
	oldBaseCount := len(renderer.cachedBaseObjs)

	// Set dirtyFull flag.
	c.mu.Lock()
	c.pendingDirty = dirtyFull
	c.mu.Unlock()

	// Refresh should take the full path, rebuilding base objects.
	renderer.Refresh()

	// After full rebuild, base objects should be repopulated.
	if !renderer.hasCachedBase {
		t.Fatal("expected hasCachedBase after full refresh")
	}
	if len(renderer.cachedBaseObjs) == 0 {
		t.Fatal("expected non-empty cachedBaseObjs after full refresh")
	}
	_ = oldBaseCount
}

func TestPartialRefresh_SizeChangeForcesFullRebuild(t *testing.T) {
	_ = test.NewApp()

	opt := Option{
		Series: []series.Series{
			&series.LineSeries{SeriesName: "A", Data: []float64{10, 50, 90}},
		},
	}
	c := New(opt)
	c.Resize(fyne800x600())

	renderer := c.CreateRenderer().(*chartRenderer)
	renderer.fullRebuild(fyne800x600())
	if !renderer.hasCachedBase {
		t.Fatal("expected hasCachedBase")
	}

	// Change the widget size.
	c.Resize(geo.NewSize(400, 300))

	// Set only tooltip dirty.
	c.mu.Lock()
	c.pendingDirty = dirtyTooltip
	c.mu.Unlock()

	// Because size changed, Refresh should still do a full rebuild.
	renderer.Refresh()

	// Cached size should now be updated.
	if renderer.cachedSize != geo.NewSize(400, 300) {
		t.Errorf("expected cachedSize 400x300, got %v", renderer.cachedSize)
	}
}

// fyne800x600 returns a standard test size.
func fyne800x600() geo.Size {
	return geo.NewSize(800, 600)
}

// Layout at an unchanged size must not rebuild the chart. It used to, and a
// rebuild relays the chart out, so every chart rebuilt on every frame — for 3D
// charts a new Scene3D and GPU texture per frame (#396).
func TestChartRenderer_LayoutSameSizeDoesNotRebuild(t *testing.T) {
	_ = test.NewApp()
	c := New(Option{Series: []series.Series{&series.Bar3DSeries{SeriesName: "S", Data: []series.Bar3DData{{X: 0, Z: 0, Value: 1}, {X: 1, Z: 1, Value: 2}}}}})
	r := c.CreateRenderer().(*chartRenderer)
	size := geo.NewSize(400, 300)
	c.Resize(size)
	r.Layout(size)
	scene := c.scene3D
	if scene == nil {
		t.Fatal("expected a 3D scene after the first layout")
	}

	r.Layout(size)
	r.Layout(size)
	if c.scene3D != scene {
		t.Error("a same-size Layout rebuilt the chart (new Scene3D)")
	}

	r.Layout(geo.NewSize(500, 300))
	if c.scene3D == scene {
		t.Error("a resize must still rebuild the chart")
	}
}

// A touch scroll — a two-finger pan — passes through a chart to the page
// around it; a wheel scroll still zooms (#398).
func TestChart_TouchScrollPassesThrough(t *testing.T) {
	_ = test.NewApp()
	c := New(Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series:   []series.Series{&series.LineSeries{SeriesName: "A", Data: []float64{1, 2, 3, 4}}},
	})
	c.Resize(fyne800x600())
	at := event.PointEvent{Position: geo.NewPos(400, 300)}
	if c.TryScrolled(&event.ScrollEvent{PointEvent: at, Scrolled: geo.NewDelta(0, 3), Source: event.ScrollSourceTouch}) {
		t.Error("a touch scroll must not be consumed by the chart")
	}
	if w := c.ZoomWindow(); w.Span() < 0.999 {
		t.Errorf("a touch scroll must not zoom, span %v", w.Span())
	}
	if !c.TryScrolled(&event.ScrollEvent{PointEvent: at, Scrolled: geo.NewDelta(0, 3), Source: event.ScrollSourceMouseWheel}) {
		t.Error("a wheel scroll should still zoom the chart")
	}
}

// A multi-series 3D chart is one scene holding every series' meshes, so
// orbiting moves the whole chart and the series occlude each other (#399).
func TestChart_MultiSeries3DIsOneScene(t *testing.T) {
	_ = test.NewApp()
	z := func(v float64) *float64 { return &v }
	line := func(name string, depth float64) *series.Line3DSeries {
		return &series.Line3DSeries{SeriesName: name, Data: []float64{0, 1, 1, 2, 2, 1}, ZPosition: z(depth)}
	}
	single := New(Option{Series: []series.Series{line("A", 0)}})
	sr := single.CreateRenderer().(*chartRenderer)
	single.Resize(geo.NewSize(400, 300))
	sr.Layout(geo.NewSize(400, 300))
	perSeries := len(single.scene3D.Meshes)

	c := New(Option{Series: []series.Series{line("A", 0), line("B", 1), line("C", 2)}})
	r := c.CreateRenderer().(*chartRenderer)
	c.Resize(geo.NewSize(400, 300))
	r.Layout(geo.NewSize(400, 300))

	scenes := 0
	for _, o := range r.Objects() {
		if _, ok := o.(*fynecanvas.Scene3D); ok {
			scenes++
		}
	}
	if scenes != 1 {
		t.Fatalf("expected one Scene3D for three 3D series, got %d", scenes)
	}
	if got := len(c.scene3D.Meshes); got != 3*perSeries {
		t.Errorf("the shared scene should hold all three series' meshes: got %d, want %d", got, 3*perSeries)
	}
}
