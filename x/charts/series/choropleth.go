package series

import (
	"encoding/binary"
	"hash/fnv"
	"image"
	"image/color"
	"log"
	"math"
	"sync"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/geojson"
)

// ChoroplethSeries renders colored geographic regions from GeoJSON data.
type ChoroplethSeries struct {
	SeriesName string
	GeoJSONURL string             // URL to download GeoJSON (e.g., Natural Earth)
	GeoJSON    []byte             // inline GeoJSON (if URL is empty)
	RegionData map[string]float64 // region name/id → value
	ColorStops []HeatmapColorStop // reuse heatmap's color stops
	Min, Max   float64            // data range; 0,0 = auto

	MapCenter [2]float64
	MapZoom   float32

	// raster caches the last rasterised image so a repaint blits instead of
	// walking every feature again. Lazily created on the render goroutine.
	raster *choroplethRaster
	// inline caches the parse of GeoJSON, which would otherwise re-run on
	// every render (~155 ms for the 14 MB country data set).
	inline *inlineGeoJSON
	// centres caches each region's centre for hit testing (choropleth_hit.go).
	centres *regionCentres
}

func (s *ChoroplethSeries) SeriesType() Type { return TypeChoropleth }
func (s *ChoroplethSeries) Name() string     { return s.SeriesName }
func (s *ChoroplethSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeChoropleth, func() Renderer { return &choroplethRenderer{} })
}

type choroplethRenderer struct{}

func (r *choroplethRenderer) Render(ctx RenderContext) []core.CanvasObject {
	cs, ok := ctx.Series.(*ChoroplethSeries)
	if !ok {
		return nil
	}

	vp := ctx.Viewport
	if vp.W <= 0 || vp.H <= 0 {
		return nil
	}

	// Get GeoJSON data. A URL-backed collection loads in the background and is
	// nil until it arrives, at which point ctx.Invalidate repaints the chart.
	fc := r.getGeoJSON(cs, ctx.Invalidate)
	if fc == nil || len(fc.Features) == 0 {
		return nil
	}

	gc, ok2 := ctx.Coord.(*coord.GeoCoord)
	if !ok2 {
		gc = coord.NewGeoCoord(cs.MapCenter[0], cs.MapCenter[1], cs.MapZoom)
	}
	gc.SetBounds(vp)

	// Compute data range
	dMin, dMax := cs.Min, cs.Max
	if dMin == 0 && dMax == 0 {
		dMin = math.Inf(1)
		dMax = math.Inf(-1)
		for _, v := range cs.RegionData {
			if v < dMin {
				dMin = v
			}
			if v > dMax {
				dMax = v
			}
		}
	}
	if dMax <= dMin {
		dMax = dMin + 1
	}

	stops := cs.ColorStops
	if len(stops) == 0 {
		stops = defaultChoroplethStops()
	}

	if cs.raster == nil {
		cs.raster = &choroplethRaster{}
	}
	key := choroplethKey{
		vp:       vp,
		lon:      gc.CenterLon,
		lat:      gc.CenterLat,
		zoom:     gc.Zoom,
		dMin:     dMin,
		dMax:     dMax,
		dataHash: hashRegionData(cs.RegionData),
		stopHash: hashColorStops(stops),
	}
	// Snapshot the projection once. Every point of every ring shares these
	// constants, and the snapshot does not read the shared GeoCoord that
	// SetBounds mutates on the next render.
	proj := gc.Projection()

	rasterize := func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))

		scaleX := float64(w) / float64(vp.W)
		scaleY := float64(h) / float64(vp.H)
		pixels := make([][2]int, 0, 256) // reused across rings
		var rast geojson.Rasterizer      // ditto for the scanline buffers

		for i := range fc.Features {
			feature := &fc.Features[i]
			name := feature.Name()
			id := feature.ID()

			// Look up data value
			val, hasData := cs.RegionData[name]
			if !hasData {
				val, hasData = cs.RegionData[id]
			}

			var fillColor color.NRGBA
			if hasData {
				norm := (val - dMin) / (dMax - dMin)
				if norm < 0 {
					norm = 0
				}
				if norm > 1 {
					norm = 1
				}
				c := interpolateColor(stops, norm)
				cr, cg, cb, ca := c.RGBA()
				fillColor = color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: uint8(ca >> 8)}
			} else {
				fillColor = color.NRGBA{R: 60, G: 60, B: 60, A: 100} // no data
			}

			polygons, err := feature.Geometry.Polygons()
			if err != nil {
				log.Printf("x/charts: choropleth feature %q polygons error: %v", name, err)
				continue
			}

			for _, ring := range polygons {
				pixels = pixels[:0]
				for _, pt := range ring {
					pos := proj.DataToPixel(pt[0], pt[1])
					pixels = append(pixels, [2]int{
						int(float64(pos.X-vp.X) * scaleX),
						int(float64(pos.Y-vp.Y) * scaleY),
					})
				}
				rast.Polygon(img, pixels, fillColor)
			}
		}

		return img
	}

	// The generator walks every feature, ring and point, which measured 226 ms
	// per repaint on the country data set — and nothing about it changes
	// between repaints of the same view (#35). Serve a cached image whenever the
	// projection and the data are unchanged.
	cache := cs.raster
	raster := canvas.NewRaster(func(w, h int) image.Image {
		return cache.image(key, fc, w, h, rasterize)
	})

	raster.Move(geo.NewPos(vp.X, vp.Y))
	raster.Resize(geo.NewSize(vp.W, vp.H))
	raster.ScaleMode = canvas.ImageScaleSmooth

	return []core.CanvasObject{raster}
}

// getGeoJSON resolves the series' boundary data. Inline GeoJSON is parsed on
// the spot; a URL is fetched in the background (see geojson_loader.go) and
// returns nil until it lands, whereupon onLoaded repaints the chart.
func (r *choroplethRenderer) getGeoJSON(cs *ChoroplethSeries, onLoaded func()) *geojson.FeatureCollection {
	if len(cs.GeoJSON) > 0 {
		if cs.inline != nil && sameBytes(cs.inline.src, cs.GeoJSON) {
			return cs.inline.fc
		}
		fc, err := geojson.Parse(cs.GeoJSON)
		if err != nil {
			log.Printf("x/charts: choropleth inline GeoJSON parse error: %v", err)
			return nil
		}
		cs.inline = &inlineGeoJSON{src: cs.GeoJSON, fc: fc}
		return fc
	}
	return geoJSONCache.Get(cs.GeoJSONURL, onLoaded)
}

// inlineGeoJSON pairs a parsed collection with the buffer it came from.
type inlineGeoJSON struct {
	src []byte
	fc  *geojson.FeatureCollection
}

// sameBytes reports whether two slices share a backing array and length, i.e.
// are the same document. Assigning a new slice to ChoroplethSeries.GeoJSON
// invalidates the parse; mutating one in place does not.
func sameBytes(a, b []byte) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// choroplethKey identifies a rasterised view: the projection it was drawn for
// plus a digest of the data that coloured it. Two renders with equal keys (and
// the same feature collection) produce byte-identical images.
type choroplethKey struct {
	vp         coord.Rect
	lon, lat   float64
	zoom       float32
	dMin, dMax float64
	dataHash   uint64
	stopHash   uint64
}

// choroplethRaster memoises one rasterised image per series.
type choroplethRaster struct {
	mu   sync.Mutex
	key  choroplethKey
	fc   *geojson.FeatureCollection
	w, h int
	img  image.Image
}

// image returns the cached image when key, feature collection and pixel size
// all match the last rasterisation, and otherwise rebuilds and stores it.
func (c *choroplethRaster) image(key choroplethKey, fc *geojson.FeatureCollection, w, h int,
	build func(w, h int) image.Image,
) image.Image {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.img != nil && c.fc == fc && c.w == w && c.h == h && c.key == key {
		return c.img
	}

	img := build(w, h)
	c.key, c.fc, c.w, c.h, c.img = key, fc, w, h, img
	return img
}

// hashRegionData digests a region→value map order-independently, so a repaint
// with unchanged data always produces the same digest despite Go's randomised
// map iteration. Region counts are in the hundreds, so this is far cheaper than
// the rasterisation it guards.
func hashRegionData(data map[string]float64) uint64 {
	var sum uint64
	for name, v := range data {
		h := fnv.New64a()
		h.Write([]byte(name))
		binary.Write(h, binary.LittleEndian, v)
		sum += h.Sum64() // commutative: independent of iteration order
	}
	return sum ^ uint64(len(data))
}

func hashColorStops(stops []HeatmapColorStop) uint64 {
	h := fnv.New64a()
	for _, s := range stops {
		binary.Write(h, binary.LittleEndian, s.Position)
		r, g, b, a := s.Color.RGBA()
		binary.Write(h, binary.LittleEndian, [4]uint32{r, g, b, a})
	}
	return h.Sum64()
}

func defaultChoroplethStops() []HeatmapColorStop {
	return []HeatmapColorStop{
		{Position: 0.0, Color: color.NRGBA{R: 240, G: 249, B: 232, A: 255}},
		{Position: 0.5, Color: color.NRGBA{R: 116, G: 196, B: 118, A: 255}},
		{Position: 1.0, Color: color.NRGBA{R: 0, G: 109, B: 44, A: 255}},
	}
}
