package series

import (
	"fmt"
	"image/color"
	"os"
	"testing"

	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/canvas"

	"github.com/belluikit/bellui/x/charts/coord"
)

// benchBigSeries approximates the real country data set: a few hundred regions
// whose rings carry thousands of points each, which is where the ~200 ms first
// rasterisation the issue measured comes from. The existing 200x60 fixture is
// an order of magnitude too small to show it.
func benchBigSeries(features, points int) *ChoroplethSeries {
	data := map[string]float64{}
	for i := 0; i < features; i++ {
		data[fmt.Sprintf("r%d", i)] = float64(i)
	}
	return &ChoroplethSeries{
		GeoJSON:    syntheticGeoJSON(features, points),
		RegionData: data,
		MapZoom:    2,
	}
}

// BenchmarkChoroplethFirstRasterizeBig measures the first rasterisation of a new
// view with the parse already done -- the ~205 ms hitch cmd/geoprobe reports,
// which happens once when the GeoJSON lands and again on every pan or zoom step.
func BenchmarkChoroplethFirstRasterizeBig(b *testing.B) {
	test.NewTempApp(b) // a drawn object asks the current app to repaint (#515)
	gc := coord.NewGeoCoord(0, 0, 2)
	vp := coord.Rect{X: 0, Y: 0, W: 900, H: 600}
	gc.SetBounds(vp)
	cs := benchBigSeries(250, 2000)

	render := func() *canvas.Raster {
		objs := NewRenderer(TypeChoropleth).Render(RenderContext{Series: cs, Coord: gc, Viewport: vp, Color: color.White})
		return objs[0].(*canvas.Raster)
	}
	render().Generator(900, 600) // warm the parse and the memo

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.raster = nil // force a genuine re-rasterisation, as a pan would
		render().Generator(900, 600)
	}
}

// BenchmarkChoroplethFirstRasterizeRealData measures the first rasterisation of
// a new view against the actual 14 MB country boundary file cmd/geoprobe uses,
// which is what the ~205 ms figure in belluikit/bellui#40 was measured on. The
// synthetic fixture above approximates it; this is the thing itself.
//
// Point BELLUI_GEOJSON at a copy of
// https://raw.githubusercontent.com/datasets/geo-countries/master/data/countries.geojson
// to run it; it is skipped otherwise so the suite needs no network or fixture.
func BenchmarkChoroplethFirstRasterizeRealData(b *testing.B) {
	test.NewTempApp(b) // a drawn object asks the current app to repaint (#515)
	path := os.Getenv("BELLUI_GEOJSON")
	if path == "" {
		b.Skip("set BELLUI_GEOJSON to a countries.geojson to run this benchmark")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}

	gc := coord.NewGeoCoord(0, 20, 2)
	vp := coord.Rect{X: 0, Y: 0, W: 900, H: 600}
	gc.SetBounds(vp)

	cs := &ChoroplethSeries{GeoJSON: src, MapZoom: 2, RegionData: map[string]float64{}}
	render := func() *canvas.Raster {
		objs := NewRenderer(TypeChoropleth).Render(RenderContext{Series: cs, Coord: gc, Viewport: vp, Color: color.White})
		return objs[0].(*canvas.Raster)
	}
	render().Generator(900, 600) // warm the parse, as the async loader does

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.raster = nil // a genuine new view, as a pan or zoom step produces
		render().Generator(900, 600)
	}
}
