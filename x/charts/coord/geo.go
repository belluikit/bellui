package coord

import (
	"math"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
)

// GeoCoord implements a Web Mercator (EPSG:3857) coordinate system
// for geographic map rendering. DataToPixel maps (longitude, latitude)
// to pixel positions within the viewport bounds.
type GeoCoord struct {
	bounds               Rect
	CenterLon, CenterLat float64
	Zoom                 float32
}

// NewGeoCoord creates a GeoCoord with the given center and zoom level.
func NewGeoCoord(centerLon, centerLat float64, zoom float32) *GeoCoord {
	return &GeoCoord{CenterLon: centerLon, CenterLat: centerLat, Zoom: zoom}
}

func (g *GeoCoord) SetBounds(bounds Rect) { g.bounds = bounds }
func (g *GeoCoord) Bounds() Rect          { return g.bounds }
func (g *GeoCoord) XScale() axis.Scale    { return nil }
func (g *GeoCoord) YScale() axis.Scale    { return nil }

// Projection is an immutable snapshot of a [GeoCoord]'s view — its zoom, centre
// and viewport resolved into the constants every point of a projection shares.
//
// It exists for two reasons. It hoists the zoom scale and the centre pixel
// (a Pow, and a Log/Tan/Cos triple) out of the per-point path, where projecting
// a country-sized data set recomputed them hundreds of thousands of times. And
// because it is a value that copies the state it needs, work projecting against
// it does not read the shared GeoCoord — which [GeoCoord.SetBounds] mutates on
// every render — so a projection may be used from another goroutine.
type Projection struct {
	scale            float64
	cx, cy           float64
	originX, originY float32
}

// Projection captures the current view. Take one snapshot and project many
// points against it rather than calling [GeoCoord.DataToPixel] per point.
func (g *GeoCoord) Projection() Projection {
	scale := math.Pow(2, float64(g.Zoom)) * 256
	return Projection{
		scale:   scale,
		cx:      (g.CenterLon + 180) / 360 * scale,
		cy:      mercatorY(g.CenterLat) * scale,
		originX: g.bounds.X + g.bounds.W/2,
		originY: g.bounds.Y + g.bounds.H/2,
	}
}

// DataToPixel maps (longitude, latitude) to a pixel position within the
// viewport the projection was taken for.
func (p Projection) DataToPixel(lon, lat float64) geo.Position {
	wx := (lon + 180) / 360 * p.scale
	wy := mercatorY(lat) * p.scale
	return geo.NewPos(p.originX+float32(wx-p.cx), p.originY+float32(wy-p.cy))
}

// mercatorY is the Web Mercator northing of a latitude, normalised to [0, 1].
func mercatorY(lat float64) float64 {
	rad := lat * math.Pi / 180
	return (1 - math.Log(math.Tan(rad)+1/math.Cos(rad))/math.Pi) / 2
}

// DataToPixel maps (longitude, latitude) to pixel position within the viewport.
//
// Projecting many points is much cheaper through a single
// [GeoCoord.Projection] snapshot, which shares the per-view constants this
// recomputes on every call.
func (g *GeoCoord) DataToPixel(lon, lat float64) geo.Position {
	return g.Projection().DataToPixel(lon, lat)
}

// PixelToData maps a pixel position back to (longitude, latitude).
func (g *GeoCoord) PixelToData(pos geo.Position) (float64, float64) {
	scale := math.Pow(2, float64(g.Zoom)) * 256

	cx := (g.CenterLon + 180) / 360 * scale
	cy := (1 - math.Log(math.Tan(g.CenterLat*math.Pi/180)+1/math.Cos(g.CenterLat*math.Pi/180))/math.Pi) / 2 * scale

	wx := cx + float64(pos.X-g.bounds.X-g.bounds.W/2)
	wy := cy + float64(pos.Y-g.bounds.Y-g.bounds.H/2)

	lon := wx/scale*360 - 180
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*wy/scale)))
	lat := latRad * 180 / math.Pi

	return lon, lat
}

// TileAt returns the tile coordinates (x, y) at the current zoom for a world pixel position.
func (g *GeoCoord) TileAt(worldX, worldY float64) (int, int) {
	return int(worldX / 256), int(worldY / 256)
}

// VisibleTiles returns all tile (z, x, y) coordinates visible in the current viewport.
func (g *GeoCoord) VisibleTiles() []TileCoord {
	scale := math.Pow(2, float64(g.Zoom)) * 256
	intZoom := int(math.Floor(float64(g.Zoom)))
	maxTile := int(math.Pow(2, float64(intZoom)))

	// Fractional scale: tiles at integer zoom cover tileSize world pixels
	fracScale := math.Pow(2, float64(g.Zoom)-float64(intZoom))
	tileSize := 256.0 * fracScale

	// Center in world pixels (at fractional scale)
	cx := (g.CenterLon + 180) / 360 * scale
	cy := (1 - math.Log(math.Tan(g.CenterLat*math.Pi/180)+1/math.Cos(g.CenterLat*math.Pi/180))/math.Pi) / 2 * scale

	// Viewport edges in world pixels
	left := cx - float64(g.bounds.W/2)
	right := cx + float64(g.bounds.W/2)
	top := cy - float64(g.bounds.H/2)
	bottom := cy + float64(g.bounds.H/2)

	// Tile range (using fractional tile size)
	tileLeft := int(math.Floor(left / tileSize))
	tileRight := int(math.Floor(right / tileSize))
	tileTop := int(math.Floor(top / tileSize))
	tileBottom := int(math.Floor(bottom / tileSize))

	z := intZoom
	var tiles []TileCoord
	for ty := tileTop; ty <= tileBottom; ty++ {
		for tx := tileLeft; tx <= tileRight; tx++ {
			// Wrap X for world wrapping
			wx := tx % maxTile
			if wx < 0 {
				wx += maxTile
			}
			if ty >= 0 && ty < maxTile {
				tiles = append(tiles, TileCoord{Z: z, X: wx, Y: ty})
			}
		}
	}
	return tiles
}

// TilePixelOffset returns the pixel position within the viewport for the top-left corner of a tile.
// Tiles are fetched at integer zoom floor(g.Zoom), but positioned using the fractional scale.
func (g *GeoCoord) TilePixelOffset(tc TileCoord) geo.Position {
	scale := math.Pow(2, float64(g.Zoom)) * 256
	cx := (g.CenterLon + 180) / 360 * scale
	cy := (1 - math.Log(math.Tan(g.CenterLat*math.Pi/180)+1/math.Cos(g.CenterLat*math.Pi/180))/math.Pi) / 2 * scale

	// Tile world position: at integer zoom, each tile covers 256px.
	// At fractional zoom, tiles cover 256 * 2^(frac) pixels.
	fracScale := math.Pow(2, float64(g.Zoom)-math.Floor(float64(g.Zoom)))
	tileSize := 256.0 * fracScale
	tileWorldX := float64(tc.X) * tileSize
	tileWorldY := float64(tc.Y) * tileSize

	px := g.bounds.X + g.bounds.W/2 + float32(tileWorldX-cx)
	py := g.bounds.Y + g.bounds.H/2 + float32(tileWorldY-cy)

	return geo.NewPos(px, py)
}

// TileCoord identifies a map tile.
type TileCoord struct {
	Z, X, Y int
}
