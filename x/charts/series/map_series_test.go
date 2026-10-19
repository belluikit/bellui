package series

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"

	"github.com/belluikit/bellui/x/charts/coord"
)

func mapViewport() coord.Rect { return coord.Rect{X: 0, Y: 0, W: 256, H: 256} }

func renderMap(t *testing.T, ms *MapSeries, invalidate func()) []core.CanvasObject {
	t.Helper()
	gc := coord.NewGeoCoord(ms.Center[0], ms.Center[1], ms.Zoom)
	gc.SetBounds(mapViewport())
	return NewRenderer(TypeMap).Render(RenderContext{
		Series:     ms,
		Coord:      gc,
		Viewport:   mapViewport(),
		Color:      color.White,
		Invalidate: invalidate,
	})
}

func mapRasterOf(t *testing.T, objs []core.CanvasObject) *canvas.Raster {
	t.Helper()
	for _, o := range objs {
		if rast, ok := o.(*canvas.Raster); ok {
			return rast
		}
	}
	t.Fatal("the map renderer produced no raster")
	return nil
}

// solidTilePNG serves as a stand-in map tile.
func solidTilePNG(t *testing.T, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Tiles used to be downloaded and then never drawn: the cache was built with a
// nil onLoaded, so nothing repainted when one arrived (#39).
func TestMapSeries_RepaintsWhenTileArrives(t *testing.T) {
	tile := solidTilePNG(t, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(tile)
	}))
	defer srv.Close()

	ms := &MapSeries{SeriesName: "base", Zoom: 1, TileURL: srv.URL + "/{z}/{x}/{y}.png"}

	repainted := make(chan struct{}, 64)
	objs := renderMap(t, ms, func() { repainted <- struct{}{} })

	// The first render has no tiles yet — just the ocean background.
	before := mapRasterOf(t, objs).Generator(256, 256)
	if before == nil {
		t.Fatal("no composite produced")
	}

	select {
	case <-repainted:
	case <-time.After(10 * time.Second):
		t.Fatal("a downloaded tile never asked the chart to repaint")
	}

	// Re-render now that tiles have landed: the composite must differ.
	deadline := time.Now().Add(10 * time.Second)
	for {
		after := mapRasterOf(t, renderMap(t, ms, nil)).Generator(256, 256)
		if !imagesEqual(before, after) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("tiles arrived but the composite never changed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func imagesEqual(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}

// A repaint of an unchanged view must reuse the composite rather than walking
// every visible tile again.
func TestMapSeries_CompositeCachedAcrossRepaints(t *testing.T) {
	ms := &MapSeries{SeriesName: "base", Zoom: 1, TileURL: "http://127.0.0.1:1/{z}/{x}/{y}.png"}

	rast := mapRasterOf(t, renderMap(t, ms, nil))
	first := rast.Generator(256, 256)
	for i := 0; i < 5; i++ {
		if got := rast.Generator(256, 256); got != first {
			t.Fatalf("repaint %d recomposited instead of serving the cache", i)
		}
	}
	// And across renders, since the renderer is recreated each time.
	if second := mapRasterOf(t, renderMap(t, ms, nil)).Generator(256, 256); second != first {
		t.Error("a re-render of an unchanged view must reuse the composite")
	}

	// The memo holds one entry, so a different pixel size replaces it.
	if got := rast.Generator(128, 128); got == first {
		t.Error("a size change must recomposite")
	}
}

func TestMapSeries_CompositeInvalidatedByViewChange(t *testing.T) {
	ms := &MapSeries{SeriesName: "base", Zoom: 1, TileURL: "http://127.0.0.1:1/{z}/{x}/{y}.png"}
	first := mapRasterOf(t, renderMap(t, ms, nil)).Generator(256, 256)

	ms.Center = [2]float64{40, 10}
	if after := mapRasterOf(t, renderMap(t, ms, nil)).Generator(256, 256); after == first {
		t.Error("a centre change must recomposite")
	}
}

func TestHashTiles(t *testing.T) {
	a := []coord.TileCoord{{Z: 1, X: 0, Y: 0}, {Z: 1, X: 1, Y: 0}}
	if hashTiles(a) != hashTiles(a) {
		t.Error("the digest must be stable")
	}
	if hashTiles(a) == hashTiles(a[:1]) {
		t.Error("an arriving tile must change the digest")
	}
	if hashTiles(a) == hashTiles([]coord.TileCoord{{Z: 1, X: 0, Y: 0}, {Z: 1, X: 2, Y: 0}}) {
		t.Error("a different tile must change the digest")
	}
}

func TestFillNRGBA(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	want := color.NRGBA{R: 1, G: 2, B: 3, A: 4}
	fillNRGBA(img, want)

	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			if got := img.NRGBAAt(x, y); got != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}

	fillNRGBA(image.NewNRGBA(image.Rect(0, 0, 0, 0)), want) // must not panic
}

// blitTile replaced a per-pixel At/Set loop; it must place the same pixels,
// including when the tile hangs off the edges of the destination.
func TestBlitTile_MatchesReferenceLoop(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(10 * x), G: uint8(10 * y), B: 7, A: 255})
		}
	}

	for _, tc := range []struct{ dx, dy, tw, th int }{
		{0, 0, 4, 4},   // exact
		{2, 3, 8, 8},   // upscaled, partly off the right/bottom
		{-3, -2, 6, 6}, // partly off the left/top
		{20, 20, 4, 4}, // wholly outside
		{0, 0, 2, 2},   // downscaled
		{1, 1, 0, 0},   // degenerate
		{-9, -9, 5, 5}, // wholly outside, negative
		{5, 5, 10, 10}, // straddles the far corner
	} {
		name := fmt.Sprintf("dx=%d,dy=%d,%dx%d", tc.dx, tc.dy, tc.tw, tc.th)
		t.Run(name, func(t *testing.T) {
			got := image.NewNRGBA(image.Rect(0, 0, 10, 10))
			want := image.NewNRGBA(image.Rect(0, 0, 10, 10))

			blitTile(got, tc.dx, tc.dy, tc.tw, tc.th, src)
			referenceBlit(want, tc.dx, tc.dy, tc.tw, tc.th, src)

			if !imagesEqual(got, want) {
				t.Error("blitTile disagrees with the original per-pixel loop")
			}
		})
	}
}

// referenceBlit is the loop blitTile replaced, kept as the oracle.
func referenceBlit(dst *image.NRGBA, dx, dy, tw, th int, src *image.NRGBA) {
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	for ry := 0; ry < th; ry++ {
		for rx := 0; rx < tw; rx++ {
			px, py := dx+rx, dy+ry
			if px >= 0 && px < w && py >= 0 && py < h {
				sx := rx * srcW / tw
				sy := ry * srcH / th
				dst.Set(px, py, src.At(sx+bounds.Min.X, sy+bounds.Min.Y))
			}
		}
	}
}

// The repaint registration must not outlive the need for it: it holds the
// chart's invalidate closure, so a fully-loaded map that kept one registered
// would pin the chart for the life of the process-global tile cache.
func TestMapSeries_ListenerClearedWhenNothingOutstanding(t *testing.T) {
	tile := solidTilePNG(t, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(tile)
	}))
	defer srv.Close()

	ms := &MapSeries{SeriesName: "base", Zoom: 1, TileURL: srv.URL + "/{z}/{x}/{y}.png"}
	cache := getOrCreateTileCache(ms.TileURL)

	repainted := make(chan struct{}, 64)
	renderMap(t, ms, func() { repainted <- struct{}{} })

	// While tiles are outstanding a listener is registered.
	if !cache.HasListener(ms) {
		t.Error("expected a repaint registration while tiles are still loading")
	}

	// Re-render until every visible tile has arrived.
	deadline := time.Now().Add(10 * time.Second)
	for cache.HasListener(ms) {
		if time.Now().After(deadline) {
			t.Fatal("tiles never finished loading")
		}
		time.Sleep(20 * time.Millisecond)
		renderMap(t, ms, func() { repainted <- struct{}{} })
	}
}

// benchMapSeries builds a map whose tiles are all pre-loaded, so the benchmarks
// measure compositing rather than the network.
func benchMapSeries(b *testing.B) (*MapSeries, *coord.GeoCoord, coord.Rect) {
	b.Helper()
	ms := &MapSeries{SeriesName: "base", Zoom: 3, TileURL: "http://bench.invalid/{z}/{x}/{y}.png"}
	vp := coord.Rect{X: 0, Y: 0, W: 900, H: 600}
	gc := coord.NewGeoCoord(0, 0, ms.Zoom)
	gc.SetBounds(vp)

	cache := getOrCreateTileCache(ms.TileURL)
	tile := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for i := 0; i < len(tile.Pix); i += 4 {
		tile.Pix[i], tile.Pix[i+1], tile.Pix[i+2], tile.Pix[i+3] = 90, 140, 90, 255
	}
	for _, tc := range gc.VisibleTiles() {
		cache.Seed(tc, tile)
	}
	return ms, gc, vp
}

// BenchmarkMapRepaint is the steady-state cost of painting an unchanged map.
func BenchmarkMapRepaint(b *testing.B) {
	test.NewTempApp(b) // a drawn object asks the current app to repaint (#515)
	ms, gc, vp := benchMapSeries(b)
	render := func() *canvas.Raster {
		objs := NewRenderer(TypeMap).Render(RenderContext{Series: ms, Coord: gc, Viewport: vp, Color: color.White})
		return objs[0].(*canvas.Raster)
	}
	render().Generator(900, 600) // warm

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		render().Generator(900, 600)
	}
}

// BenchmarkMapComposite is the cost the memo elides: a full recomposite.
func BenchmarkMapComposite(b *testing.B) {
	test.NewTempApp(b) // a drawn object asks the current app to repaint (#515)
	ms, gc, vp := benchMapSeries(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fresh := &MapSeries{SeriesName: ms.SeriesName, Zoom: ms.Zoom, TileURL: ms.TileURL}
		objs := NewRenderer(TypeMap).Render(RenderContext{Series: fresh, Coord: gc, Viewport: vp, Color: color.White})
		objs[0].(*canvas.Raster).Generator(900, 600)
	}
}

// The composite used to sample through the image.Image interface per pixel;
// these two benchmarks are why it now indexes the pixel buffers directly.
func benchBlit(b *testing.B, blit func(dst *image.NRGBA, dx, dy, tw, th int, src *image.NRGBA)) {
	src := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	dst := image.NewNRGBA(image.Rect(0, 0, 900, 600))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blit(dst, 100, 50, 300, 300, src)
	}
}

func BenchmarkBlitTile(b *testing.B)          { benchBlit(b, blitTile) }
func BenchmarkBlitTileReference(b *testing.B) { benchBlit(b, referenceBlit) }
