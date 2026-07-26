package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
)

// GeoScatterSeries renders data points at geographic (lon, lat) coordinates.
// Designed to overlay on a MapSeries.
type GeoScatterSeries struct {
	SeriesName string
	Data       []float64 // [lon0, lat0, lon1, lat1, ...]
	Symbol     SymbolType
	SymbolSize float32
	Color      color.Color

	// MapCenter and MapZoom must match the MapSeries for correct overlay positioning.
	MapCenter [2]float64
	MapZoom   float32
}

func (s *GeoScatterSeries) SeriesType() Type { return TypeGeoScatter }
func (s *GeoScatterSeries) Name() string     { return s.SeriesName }
func (s *GeoScatterSeries) sealed()          {}

func (s *GeoScatterSeries) PointCount() int { return len(s.Data) / 2 }
func (s *GeoScatterSeries) Point(i int) (float64, float64) {
	idx := i * 2
	if idx+1 >= len(s.Data) {
		return 0, 0
	}
	return s.Data[idx], s.Data[idx+1]
}

func init() {
	RegisterRenderer(TypeGeoScatter, func() Renderer { return &geoScatterRenderer{} })
}

type geoScatterRenderer struct{}

func (r *geoScatterRenderer) Render(ctx RenderContext) []core.CanvasObject {
	gs, ok := ctx.Series.(*GeoScatterSeries)
	if !ok || gs.PointCount() == 0 {
		return nil
	}

	// Use the shared GeoCoord from ctx.Coord if available, otherwise create one
	gc, ok2 := ctx.Coord.(*coord.GeoCoord)
	if !ok2 {
		gc = coord.NewGeoCoord(gs.MapCenter[0], gs.MapCenter[1], gs.MapZoom)
		gc.SetBounds(ctx.Viewport)
	}

	size := gs.SymbolSize
	if size <= 0 {
		size = 6
	}
	half := size / 2

	c := ResolveColor(gs.Color, ctx.Color)

	var objects []core.CanvasObject
	for i := 0; i < gs.PointCount(); i++ {
		lon, lat := gs.Point(i)
		pos := gc.DataToPixel(lon, lat)

		// Strictly clip to viewport
		if pos.X+half < ctx.Viewport.X || pos.X-half > ctx.Viewport.X+ctx.Viewport.W ||
			pos.Y+half < ctx.Viewport.Y || pos.Y-half > ctx.Viewport.Y+ctx.Viewport.H {
			continue
		}

		circle := canvas.NewCircle(c)
		circle.Position1 = geo.NewPos(pos.X-half, pos.Y-half)
		circle.Position2 = geo.NewPos(pos.X+half, pos.Y+half)
		objects = append(objects, circle)
	}

	return objects
}
