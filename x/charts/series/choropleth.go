package series

import (
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"net/http"
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
}

func (s *ChoroplethSeries) SeriesType() Type { return TypeChoropleth }
func (s *ChoroplethSeries) Name() string     { return s.SeriesName }
func (s *ChoroplethSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeChoropleth, func() Renderer { return &choroplethRenderer{} })
}

// Cached GeoJSON downloads
var (
	geojsonCache   = make(map[string]*geojson.FeatureCollection)
	geojsonCacheMu sync.Mutex
)

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

	// Get GeoJSON data
	fc := r.getGeoJSON(cs)
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

	// Render as a single raster
	raster := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))

		scaleX := float64(w) / float64(vp.W)
		scaleY := float64(h) / float64(vp.H)

		for _, feature := range fc.Features {
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
				log.Printf("fycharts: choropleth feature %q polygons error: %v", name, err)
				continue
			}

			for _, ring := range polygons {
				pixels := make([][2]int, len(ring))
				for i, pt := range ring {
					pos := gc.DataToPixel(pt[0], pt[1])
					pixels[i] = [2]int{
						int(float64(pos.X-vp.X) * scaleX),
						int(float64(pos.Y-vp.Y) * scaleY),
					}
				}
				geojson.RasterizePolygon(img, pixels, fillColor)
			}
		}

		return img
	})

	raster.Move(geo.NewPos(vp.X, vp.Y))
	raster.Resize(geo.NewSize(vp.W, vp.H))
	raster.ScaleMode = canvas.ImageScaleSmooth

	return []core.CanvasObject{raster}
}

func (r *choroplethRenderer) getGeoJSON(cs *ChoroplethSeries) *geojson.FeatureCollection {
	// Inline GeoJSON
	if len(cs.GeoJSON) > 0 {
		fc, err := geojson.Parse(cs.GeoJSON)
		if err != nil {
			log.Printf("fycharts: choropleth inline GeoJSON parse error: %v", err)
			return nil
		}
		return fc
	}

	// URL-based with caching
	if cs.GeoJSONURL == "" {
		return nil
	}

	geojsonCacheMu.Lock()
	if fc, ok := geojsonCache[cs.GeoJSONURL]; ok {
		geojsonCacheMu.Unlock()
		return fc
	}
	geojsonCacheMu.Unlock()

	// Synchronous download (first render only; cached after)
	resp, err := http.Get(cs.GeoJSONURL)
	if err != nil {
		log.Printf("fycharts: choropleth GeoJSON HTTP GET failed for %s: %v", cs.GeoJSONURL, err)
		return nil
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("fycharts: choropleth GeoJSON ReadAll failed for %s: %v", cs.GeoJSONURL, err)
		return nil
	}

	fc, err := geojson.Parse(data)
	if err != nil {
		log.Printf("fycharts: choropleth GeoJSON parse error for %s: %v", cs.GeoJSONURL, err)
		return nil
	}

	geojsonCacheMu.Lock()
	geojsonCache[cs.GeoJSONURL] = fc
	geojsonCacheMu.Unlock()

	return fc
}

func defaultChoroplethStops() []HeatmapColorStop {
	return []HeatmapColorStop{
		{Position: 0.0, Color: color.NRGBA{R: 240, G: 249, B: 232, A: 255}},
		{Position: 0.5, Color: color.NRGBA{R: 116, G: 196, B: 118, A: 255}},
		{Position: 1.0, Color: color.NRGBA{R: 0, G: 109, B: 44, A: 255}},
	}
}
