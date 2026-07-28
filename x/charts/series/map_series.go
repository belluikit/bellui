package series

import (
	"encoding/binary"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/tiles"
)

// MapSeries configures a tile-based geographic map.
type MapSeries struct {
	SeriesName string
	Center     [2]float64 // [longitude, latitude]
	Zoom       float32    // 1-18
	TileURL    string     // tile URL template; empty = OpenStreetMap

	// composite caches the last tile composite so a repaint blits instead of
	// recompositing every visible tile. Lazily created on the render goroutine.
	composite *mapRaster
}

func (s *MapSeries) SeriesType() Type { return TypeMap }
func (s *MapSeries) Name() string     { return s.SeriesName }
func (s *MapSeries) sealed()          {}

// mapState holds runtime state for a map series (tile cache, geo coord).
// Stored on the chart widget and reused across renders.
type MapState struct {
	GeoCoord  *coord.GeoCoord
	TileCache *tiles.Cache
}

func init() {
	RegisterRenderer(TypeMap, func() Renderer { return &mapRenderer{} })
}

type mapRenderer struct{}

func (r *mapRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ms, ok := ctx.Series.(*MapSeries)
	if !ok {
		return nil
	}

	vp := ctx.Viewport
	if vp.W <= 0 || vp.H <= 0 {
		return nil
	}

	// Use the shared GeoCoord from ctx.Coord if available
	gc, ok2 := ctx.Coord.(*coord.GeoCoord)
	if !ok2 {
		gc = coord.NewGeoCoord(ms.Center[0], ms.Center[1], ms.Zoom)
		gc.SetBounds(vp)
	}

	// Get or create the tile cache, which is shared by URL across every map.
	tileCache := getOrCreateTileCache(ms.TileURL)

	// Prefetch visible tiles
	visibleTiles := gc.VisibleTiles()
	tileCache.Prefetch(visibleTiles)

	// Collect the tiles that are actually available; the set is part of the
	// cache key, so an arriving tile invalidates the composite and nothing else
	// does (#39).
	type placedTile struct {
		tc  coord.TileCoord
		img *image.NRGBA
	}
	loaded := make([]placedTile, 0, len(visibleTiles))
	loadedCoords := make([]coord.TileCoord, 0, len(visibleTiles))
	for _, tc := range visibleTiles {
		if img := tileCache.Get(tc); img != nil {
			loaded = append(loaded, placedTile{tc: tc, img: img})
			loadedCoords = append(loadedCoords, tc)
		}
	}

	// Repaint when a tile lands: without this the map stays blank until some
	// unrelated refresh happens to rebuild the chart (#39). The registration is
	// one-shot and only while tiles are outstanding — the repaint re-renders,
	// which registers again if anything is still missing — so a fully-loaded map
	// leaves nothing behind holding a reference to the chart.
	if len(loaded) < len(visibleTiles) && ctx.Invalidate != nil {
		invalidate := ctx.Invalidate
		tileCache.SetListener(ms, func() {
			tileCache.SetListener(ms, nil)
			invalidate()
		})
	} else {
		tileCache.SetListener(ms, nil)
	}

	// Each tile covers 256 pixels at its integer zoom level. At fractional zoom,
	// tiles must be scaled by 2^(zoom - floor(zoom)) to match the continuous
	// Mercator projection.
	fractionalScale := math.Pow(2, float64(gc.Zoom)-math.Floor(float64(gc.Zoom)))
	tileLogicalSize := 256.0 * fractionalScale

	composite := func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		fillNRGBA(img, color.NRGBA{R: 170, G: 211, B: 223, A: 255}) // ocean

		// Scale from logical viewport pixels to raster pixels
		scaleX := float64(w) / float64(vp.W)
		scaleY := float64(h) / float64(vp.H)

		for _, t := range loaded {
			pos := gc.TilePixelOffset(t.tc)
			blitTile(img,
				int(float64(pos.X-vp.X)*scaleX), int(float64(pos.Y-vp.Y)*scaleY),
				int(tileLogicalSize*scaleX), int(tileLogicalSize*scaleY),
				t.img)
		}
		return img
	}

	if ms.composite == nil {
		ms.composite = &mapRaster{}
	}
	key := mapKey{
		vp:       vp,
		lon:      gc.CenterLon,
		lat:      gc.CenterLat,
		zoom:     gc.Zoom,
		tileHash: hashTiles(loadedCoords),
	}
	cache := ms.composite
	raster := canvas.NewRaster(func(w, h int) image.Image {
		return cache.image(key, w, h, composite)
	})

	raster.Move(geo.NewPos(vp.X, vp.Y))
	raster.Resize(geo.NewSize(vp.W, vp.H))
	raster.ScaleMode = canvas.ImageScaleSmooth

	// Attribution text
	attr := canvas.NewText("\u00a9 OpenStreetMap contributors", color.NRGBA{R: 200, G: 200, B: 200, A: 180})
	attr.TextSize = 9
	attr.Move(geo.NewPos(vp.X+vp.W-170, vp.Y+vp.H-14))
	attr.Resize(geo.NewSize(165, 12))

	return []core.CanvasObject{raster, attr}
}

// mapKey identifies a tile composite: the projection it was drawn for plus a
// digest of which tiles had arrived at the time.
type mapKey struct {
	vp       coord.Rect
	lon, lat float64
	zoom     float32
	tileHash uint64
}

// mapRaster memoises one composited tile image per series.
type mapRaster struct {
	mu   sync.Mutex
	key  mapKey
	w, h int
	img  image.Image
}

// image returns the cached composite when the key and pixel size match the last
// one, and otherwise rebuilds and stores it.
func (m *mapRaster) image(key mapKey, w, h int, build func(w, h int) image.Image) image.Image {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.img != nil && m.w == w && m.h == h && m.key == key {
		return m.img
	}

	img := build(w, h)
	m.key, m.w, m.h, m.img = key, w, h, img
	return img
}

// hashTiles digests the set of loaded tiles. VisibleTiles returns them in a
// stable order, so a plain sequential hash suffices.
func hashTiles(tcs []coord.TileCoord) uint64 {
	h := fnv.New64a()
	for _, tc := range tcs {
		binary.Write(h, binary.LittleEndian, [3]int64{int64(tc.Z), int64(tc.X), int64(tc.Y)})
	}
	return h.Sum64()
}

// fillNRGBA paints every pixel one colour: fill the first row, then copy it
// down, which beats a per-pixel SetNRGBA loop.
func fillNRGBA(dst *image.NRGBA, c color.NRGBA) {
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	if w == 0 || h == 0 {
		return
	}
	row := dst.Pix[:w*4]
	for x := 0; x < w; x++ {
		copy(row[x*4:], []byte{c.R, c.G, c.B, c.A})
	}
	for y := 1; y < h; y++ {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+w*4], row)
	}
}

// blitTile draws src into dst at (dx, dy), nearest-neighbour scaled to tw x th
// and clipped to dst. It indexes both pixel buffers directly rather than going
// through the image.Image interface per pixel.
func blitTile(dst *image.NRGBA, dx, dy, tw, th int, src *image.NRGBA) {
	if tw <= 0 || th <= 0 {
		return
	}
	srcW, srcH := src.Rect.Dx(), src.Rect.Dy()
	if srcW == 0 || srcH == 0 {
		return
	}
	dstW, dstH := dst.Rect.Dx(), dst.Rect.Dy()

	// Clip the destination rectangle, so the inner loops carry no bounds test.
	y0, y1 := max(0, -dy), min(th, dstH-dy)
	x0, x1 := max(0, -dx), min(tw, dstW-dx)

	for ry := y0; ry < y1; ry++ {
		sy := ry * srcH / th
		srcRow := src.Pix[sy*src.Stride:]
		dstRow := dst.Pix[(dy+ry)*dst.Stride:]
		for rx := x0; rx < x1; rx++ {
			sx := rx * srcW / tw
			copy(dstRow[(dx+rx)*4:(dx+rx)*4+4], srcRow[sx*4:sx*4+4])
		}
	}
}

// Global tile cache (shared across map instances with same URL)
var (
	globalCaches  = make(map[string]*tiles.Cache)
	globalCacheMu sync.Mutex
)

func getOrCreateTileCache(tileURL string) *tiles.Cache {
	if tileURL == "" {
		tileURL = tiles.DefaultTileURL
	}
	globalCacheMu.Lock()
	defer globalCacheMu.Unlock()

	if c, ok := globalCaches[tileURL]; ok {
		return c
	}
	c := tiles.NewCache(tileURL, nil) // no refresh callback in static mode
	globalCaches[tileURL] = c
	return c
}
