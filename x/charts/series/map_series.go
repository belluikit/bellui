package series

import (
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

	// Get or create tile cache from context
	// For now, create a static cache (the chart widget should hold this)
	tileCache := getOrCreateTileCache(ms.TileURL)

	// Prefetch visible tiles
	visibleTiles := gc.VisibleTiles()
	tileCache.Prefetch(visibleTiles)

	// Render tiles as a composite raster
	raster := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))

		// Fill background (ocean)
		bg := color.NRGBA{R: 170, G: 211, B: 223, A: 255}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetNRGBA(x, y, bg)
			}
		}

		// Scale from logical viewport pixels to raster pixels
		scaleX := float64(w) / float64(vp.W)
		scaleY := float64(h) / float64(vp.H)

		// Each tile covers 256 pixels at its integer zoom level.
		// At fractional zoom, tiles must be scaled by 2^(zoom - floor(zoom))
		// to match the continuous Mercator projection.
		fractionalScale := math.Pow(2, float64(gc.Zoom)-math.Floor(float64(gc.Zoom)))
		tileLogicalSize := 256.0 * fractionalScale

		for _, tc := range visibleTiles {
			tileImg := tileCache.Get(tc)
			if tileImg == nil {
				continue
			}

			pos := gc.TilePixelOffset(tc)
			// Tile position in raster pixel space
			dx := int(float64(pos.X-vp.X) * scaleX)
			dy := int(float64(pos.Y-vp.Y) * scaleY)
			// Tile size in raster pixel space
			tw := int(tileLogicalSize * scaleX)
			th := int(tileLogicalSize * scaleY)

			tileBounds := tileImg.Bounds()
			srcW := tileBounds.Dx()
			srcH := tileBounds.Dy()

			for ry := 0; ry < th; ry++ {
				for rx := 0; rx < tw; rx++ {
					px := dx + rx
					py := dy + ry
					if px >= 0 && px < w && py >= 0 && py < h {
						// Sample from source tile
						sx := rx * srcW / tw
						sy := ry * srcH / th
						img.Set(px, py, tileImg.At(sx+tileBounds.Min.X, sy+tileBounds.Min.Y))
					}
				}
			}
		}

		return img
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
