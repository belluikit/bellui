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

// DataToPixel maps (longitude, latitude) to pixel position within the viewport.
func (g *GeoCoord) DataToPixel(lon, lat float64) geo.Position {
	// World pixel coordinates at this zoom level
	scale := math.Pow(2, float64(g.Zoom)) * 256
	wx := (lon + 180) / 360 * scale
	wy := (1 - math.Log(math.Tan(lat*math.Pi/180)+1/math.Cos(lat*math.Pi/180))/math.Pi) / 2 * scale

	// Center pixel
	cx := (g.CenterLon + 180) / 360 * scale
	cy := (1 - math.Log(math.Tan(g.CenterLat*math.Pi/180)+1/math.Cos(g.CenterLat*math.Pi/180))/math.Pi) / 2 * scale

	// Offset from center, mapped to viewport
	px := g.bounds.X + g.bounds.W/2 + float32(wx-cx)
	py := g.bounds.Y + g.bounds.H/2 + float32(wy-cy)

	return geo.NewPos(px, py)
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
