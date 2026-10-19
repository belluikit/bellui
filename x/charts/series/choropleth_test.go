package series

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/charts/coord"
)

// twoBoxGeoJSON is a minimal two-feature collection, enough to exercise the
// feature/ring/point walk without a network.
const twoBoxGeoJSON = `{
  "type": "FeatureCollection",
  "features": [
    {"type": "Feature", "properties": {"name": "Left"},
     "geometry": {"type": "Polygon", "coordinates": [[[-20,-10],[-5,-10],[-5,10],[-20,10],[-20,-10]]]}},
    {"type": "Feature", "properties": {"name": "Right"},
     "geometry": {"type": "Polygon", "coordinates": [[[5,-10],[20,-10],[20,10],[5,10],[5,-10]]]}}
  ]
}`

func testViewport() coord.Rect { return coord.Rect{X: 0, Y: 0, W: 200, H: 150} }

func renderChoropleth(t *testing.T, cs *ChoroplethSeries, invalidate func()) []core.CanvasObject {
	t.Helper()
	gc := coord.NewGeoCoord(cs.MapCenter[0], cs.MapCenter[1], cs.MapZoom)
	gc.SetBounds(testViewport())
	return NewRenderer(TypeChoropleth).Render(RenderContext{
		Series:     cs,
		Coord:      gc,
		Viewport:   testViewport(),
		Color:      color.White,
		Invalidate: invalidate,
	})
}

// generatorOf returns the raster produced by a choropleth render.
func rasterOf(t *testing.T, objs []core.CanvasObject) *canvas.Raster {
	t.Helper()
	if len(objs) != 1 {
		t.Fatalf("expected 1 object from the choropleth renderer, got %d", len(objs))
	}
	rast, ok := objs[0].(*canvas.Raster)
	if !ok {
		t.Fatalf("expected a *canvas.Raster, got %T", objs[0])
	}
	return rast
}

// A repaint of an unchanged view must reuse the rasterised image instead of
// walking every feature again — the 226 ms/repaint half of #35.
func TestChoropleth_RasterCachedAcrossRepaints(t *testing.T) {
	cs := &ChoroplethSeries{
		SeriesName: "boxes",
		GeoJSON:    []byte(twoBoxGeoJSON),
		RegionData: map[string]float64{"Left": 1, "Right": 9},
		MapZoom:    2,
	}

	rast := rasterOf(t, renderChoropleth(t, cs, nil))
	first := rast.Generator(200, 150)
	if first == nil {
		t.Fatal("no image produced")
	}
	for i := 0; i < 5; i++ {
		if got := rast.Generator(200, 150); got != first {
			t.Fatalf("repaint %d re-rasterised instead of serving the cache", i)
		}
	}

	// A different pixel size is a genuinely different image.
	if got := rast.Generator(100, 75); got == first {
		t.Error("a size change must re-rasterise")
	}
}

// The cache must survive the renderer being recreated per render (NewRenderer
// returns a fresh renderer each time), because it lives on the series.
func TestChoropleth_RasterCachedAcrossRenders(t *testing.T) {
	cs := &ChoroplethSeries{
		GeoJSON:    []byte(twoBoxGeoJSON),
		RegionData: map[string]float64{"Left": 1},
		MapZoom:    2,
	}

	first := rasterOf(t, renderChoropleth(t, cs, nil)).Generator(200, 150)
	second := rasterOf(t, renderChoropleth(t, cs, nil)).Generator(200, 150)
	if first != second {
		t.Error("a re-render of an unchanged series must reuse the cached image")
	}
}

// Changing what is drawn must invalidate the cache, or the chart would show
// stale data forever.
func TestChoropleth_CacheInvalidatedByChanges(t *testing.T) {
	newSeries := func() *ChoroplethSeries {
		return &ChoroplethSeries{
			GeoJSON:    []byte(twoBoxGeoJSON),
			RegionData: map[string]float64{"Left": 1, "Right": 9},
			MapZoom:    2,
		}
	}

	for _, tc := range []struct {
		name   string
		mutate func(*ChoroplethSeries)
	}{
		{"region value", func(cs *ChoroplethSeries) { cs.RegionData["Left"] = 7 }},
		{"region added", func(cs *ChoroplethSeries) { cs.RegionData["Middle"] = 3 }},
		{"region removed", func(cs *ChoroplethSeries) { delete(cs.RegionData, "Left") }},
		{"explicit range", func(cs *ChoroplethSeries) { cs.Min, cs.Max = 0, 100 }},
		{"zoom", func(cs *ChoroplethSeries) { cs.MapZoom = 3 }},
		{"centre", func(cs *ChoroplethSeries) { cs.MapCenter = [2]float64{40, 10} }},
		{"colour stops", func(cs *ChoroplethSeries) {
			cs.ColorStops = []HeatmapColorStop{
				{Position: 0, Color: color.NRGBA{R: 255, A: 255}},
				{Position: 1, Color: color.NRGBA{B: 255, A: 255}},
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs := newSeries()
			before := rasterOf(t, renderChoropleth(t, cs, nil)).Generator(200, 150)

			tc.mutate(cs)
			after := rasterOf(t, renderChoropleth(t, cs, nil)).Generator(200, 150)
			if before == after {
				t.Error("cache served a stale image after the series changed")
			}
		})
	}
}

// hashRegionData must not depend on Go's randomised map iteration order, or the
// cache would miss on every repaint.
func TestHashRegionData_OrderIndependent(t *testing.T) {
	data := map[string]float64{}
	for i := 0; i < 64; i++ {
		data[fmt.Sprintf("region-%d", i)] = float64(i) * 1.5
	}
	want := hashRegionData(data)
	for i := 0; i < 20; i++ {
		if got := hashRegionData(data); got != want {
			t.Fatalf("hash changed between calls: %d != %d", got, want)
		}
	}

	data["region-0"] = -1
	if hashRegionData(data) == want {
		t.Error("a changed value must change the hash")
	}
}

// --- loader -----------------------------------------------------------------

func newTestLoader(fetch func(string) ([]byte, error)) *geoJSONLoader {
	return &geoJSONLoader{fetch: fetch}
}

// Get must never block the render goroutine: the first call returns nil and
// downloads in the background, and the caller is told when data lands.
func TestGeoJSONLoader_AsyncAndCached(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	l := newTestLoader(func(string) ([]byte, error) {
		calls.Add(1)
		<-release
		return []byte(twoBoxGeoJSON), nil
	})

	loaded := make(chan struct{})
	if fc := l.Get("http://example.invalid/a.json", func() { close(loaded) }); fc != nil {
		t.Fatal("the first Get must return nil rather than block on the download")
	}
	// Further calls while in flight also return immediately, without re-fetching.
	for i := 0; i < 5; i++ {
		if fc := l.Get("http://example.invalid/a.json", nil); fc != nil {
			t.Fatal("Get returned data before the download finished")
		}
	}

	close(release)
	select {
	case <-loaded:
	case <-time.After(5 * time.Second):
		t.Fatal("onLoaded never fired")
	}

	fc := l.Get("http://example.invalid/a.json", nil)
	if fc == nil || len(fc.Features) != 2 {
		t.Fatalf("expected the parsed collection, got %v", fc)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("expected exactly one download for one URL, got %d", n)
	}
}

// A failure must be remembered, so an offline app stops re-dialling on every
// frame; once the backoff expires it may try again.
func TestGeoJSONLoader_NegativeCachingWithBackoff(t *testing.T) {
	var calls atomic.Int32
	l := newTestLoader(func(string) ([]byte, error) {
		calls.Add(1)
		return nil, errors.New("offline")
	})
	now := time.Now()
	var mu sync.Mutex
	l.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }

	const url = "http://example.invalid/b.json"
	settled := make(chan struct{}, 1)
	l.Get(url, func() { settled <- struct{}{} })
	<-settled

	for i := 0; i < 100; i++ { // a hundred frames while still backing off
		l.Get(url, nil)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("a failure must be cached: expected 1 attempt, got %d", n)
	}

	mu.Lock()
	now = now.Add(geoJSONRetryBase + time.Second)
	mu.Unlock()

	l.Get(url, func() { settled <- struct{}{} })
	<-settled
	if n := calls.Load(); n != 2 {
		t.Errorf("expected a retry once the backoff elapsed, got %d attempts", n)
	}
}

func TestGeoJSONBackoff(t *testing.T) {
	if got := geoJSONBackoff(1); got != geoJSONRetryBase {
		t.Errorf("first failure: got %v, want %v", got, geoJSONRetryBase)
	}
	if got := geoJSONBackoff(2); got != 2*geoJSONRetryBase {
		t.Errorf("second failure: got %v, want %v", got, 2*geoJSONRetryBase)
	}
	if got := geoJSONBackoff(99); got != geoJSONRetryMax {
		t.Errorf("many failures must cap at %v, got %v", geoJSONRetryMax, got)
	}
}

// A URL-backed series draws nothing until its data arrives, and asks for a
// repaint when it does — it must not stall the render goroutine meanwhile.
func TestChoropleth_RendersNothingWhileLoading(t *testing.T) {
	prev := geoJSONCache
	release := make(chan struct{})
	geoJSONCache = newTestLoader(func(string) ([]byte, error) {
		<-release
		return []byte(twoBoxGeoJSON), nil
	})
	t.Cleanup(func() { geoJSONCache = prev })

	cs := &ChoroplethSeries{GeoJSONURL: "http://example.invalid/c.json", MapZoom: 2}

	repainted := make(chan struct{})
	var once sync.Once
	done := make(chan []core.CanvasObject, 1)
	go func() { done <- renderChoropleth(t, cs, func() { once.Do(func() { close(repainted) }) }) }()

	select {
	case objs := <-done:
		if objs != nil {
			t.Fatalf("expected no objects while loading, got %d", len(objs))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Render blocked on the download")
	}

	close(release)
	select {
	case <-repainted:
	case <-time.After(5 * time.Second):
		t.Fatal("the chart was never asked to repaint after the data arrived")
	}

	objs := renderChoropleth(t, cs, nil)
	if len(objs) != 1 {
		t.Fatalf("expected the choropleth to draw once loaded, got %d objects", len(objs))
	}
	if img := rasterOf(t, objs).Generator(200, 150); img == nil || img.Bounds() != image.Rect(0, 0, 200, 150) {
		t.Error("expected a rasterised image once the data arrived")
	}
}

// Inline GeoJSON must be parsed once, not on every render.
func TestChoropleth_InlineGeoJSONParsedOnce(t *testing.T) {
	cs := &ChoroplethSeries{GeoJSON: []byte(twoBoxGeoJSON), MapZoom: 2}

	renderChoropleth(t, cs, nil)
	first := cs.inline
	if first == nil || first.fc == nil {
		t.Fatal("inline GeoJSON was not cached")
	}

	renderChoropleth(t, cs, nil)
	if cs.inline.fc != first.fc {
		t.Error("inline GeoJSON was re-parsed on the second render")
	}

	cs.GeoJSON = []byte(twoBoxGeoJSON) // a new buffer is new content
	renderChoropleth(t, cs, nil)
	if cs.inline.fc == first.fc {
		t.Error("replacing the GeoJSON buffer must re-parse it")
	}
}

// syntheticGeoJSON builds a collection of `features` polygons of `points` each,
// roughly the shape of a world country data set.
func syntheticGeoJSON(features, points int) []byte {
	var b strings.Builder
	b.WriteString(`{"type":"FeatureCollection","features":[`)
	for f := 0; f < features; f++ {
		if f > 0 {
			b.WriteByte(',')
		}
		lon0 := -180 + float64(f%36)*10
		lat0 := -80 + float64(f/36)*10
		fmt.Fprintf(&b, `{"type":"Feature","properties":{"name":"r%d"},"geometry":{"type":"Polygon","coordinates":[[`, f)
		for p := 0; p < points; p++ {
			angle := 2 * math.Pi * float64(p) / float64(points)
			fmt.Fprintf(&b, `[%f,%f],`, lon0+4*math.Cos(angle), lat0+4*math.Sin(angle))
		}
		fmt.Fprintf(&b, `[%f,%f]]]}}`, lon0+4, lat0)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func benchSeries() *ChoroplethSeries {
	data := map[string]float64{}
	for i := 0; i < 200; i++ {
		data[fmt.Sprintf("r%d", i)] = float64(i)
	}
	return &ChoroplethSeries{
		GeoJSON:    syntheticGeoJSON(200, 60),
		RegionData: data,
		MapZoom:    2,
	}
}

// BenchmarkChoroplethRepaint is the steady-state cost the issue measured at
// 226 ms: the same view painted again and again.
func BenchmarkChoroplethRepaint(b *testing.B) {
	test.NewTempApp(b) // a drawn object asks the current app to repaint (#515)
	cs := benchSeries()
	gc := coord.NewGeoCoord(0, 0, cs.MapZoom)
	vp := coord.Rect{X: 0, Y: 0, W: 900, H: 600}
	gc.SetBounds(vp)
	render := func() *canvas.Raster {
		objs := NewRenderer(TypeChoropleth).Render(RenderContext{Series: cs, Coord: gc, Viewport: vp, Color: color.White})
		return objs[0].(*canvas.Raster)
	}
	rast := render()
	rast.Generator(900, 600) // warm

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		render().Generator(900, 600)
	}
}

// BenchmarkChoroplethFirstRasterize is the unavoidable cost of the first paint
// (and of any genuine view or data change) — the work the cache elides.
func BenchmarkChoroplethFirstRasterize(b *testing.B) {
	test.NewTempApp(b) // a drawn object asks the current app to repaint (#515)
	gc := coord.NewGeoCoord(0, 0, 2)
	vp := coord.Rect{X: 0, Y: 0, W: 900, H: 600}
	gc.SetBounds(vp)
	src := syntheticGeoJSON(200, 60)
	data := map[string]float64{}
	for i := 0; i < 200; i++ {
		data[fmt.Sprintf("r%d", i)] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs := &ChoroplethSeries{GeoJSON: src, RegionData: data, MapZoom: 2}
		objs := NewRenderer(TypeChoropleth).Render(RenderContext{Series: cs, Coord: gc, Viewport: vp, Color: color.White})
		objs[0].(*canvas.Raster).Generator(900, 600)
	}
}
