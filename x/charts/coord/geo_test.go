package coord

import (
	"math"
	"testing"
)

func TestGeoCoord_DataToPixel_Center(t *testing.T) {
	gc := NewGeoCoord(0, 0, 2)
	gc.SetBounds(Rect{X: 0, Y: 0, W: 512, H: 512})

	// Center of the map should be center of viewport
	pos := gc.DataToPixel(0, 0)
	if math.Abs(float64(pos.X-256)) > 2 || math.Abs(float64(pos.Y-256)) > 2 {
		t.Errorf("center (0,0) should map to ~(256,256), got (%v,%v)", pos.X, pos.Y)
	}
}

func TestGeoCoord_RoundTrip(t *testing.T) {
	gc := NewGeoCoord(-74, 40.7, 4) // New York
	gc.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})

	testPts := [][2]float64{{-74, 40.7}, {0, 51.5}, {139.7, 35.7}}
	for _, pt := range testPts {
		pos := gc.DataToPixel(pt[0], pt[1])
		lon, lat := gc.PixelToData(pos)
		if math.Abs(lon-pt[0]) > 0.1 || math.Abs(lat-pt[1]) > 0.1 {
			t.Errorf("round-trip (%v,%v) → pixel (%v,%v) → (%v,%v)", pt[0], pt[1], pos.X, pos.Y, lon, lat)
		}
	}
}

func TestGeoCoord_VisibleTiles(t *testing.T) {
	gc := NewGeoCoord(0, 0, 2)
	gc.SetBounds(Rect{X: 0, Y: 0, W: 512, H: 512})

	tiles := gc.VisibleTiles()
	if len(tiles) == 0 {
		t.Error("expected visible tiles at zoom 2")
	}

	// At zoom 2, there are 4x4=16 tiles total; viewport should see several
	for _, tc := range tiles {
		if tc.Z != 2 {
			t.Errorf("tile zoom = %d, want 2", tc.Z)
		}
		if tc.X < 0 || tc.X >= 4 || tc.Y < 0 || tc.Y >= 4 {
			t.Errorf("tile (%d,%d) out of range for zoom 2", tc.X, tc.Y)
		}
	}
}

// A Projection snapshot must place points exactly where GeoCoord.DataToPixel
// does -- it only hoists the constants they share out of the per-point path
// (belluikit/bellui#40).
func TestGeoCoord_ProjectionMatchesDataToPixel(t *testing.T) {
	for _, g := range []*GeoCoord{
		NewGeoCoord(0, 0, 2),
		NewGeoCoord(-73.9, 40.7, 5),
		NewGeoCoord(139.7, 35.7, 0.5),
	} {
		g.SetBounds(Rect{X: 10, Y: 20, W: 900, H: 600})
		p := g.Projection()

		for _, pt := range [][2]float64{
			{0, 0}, {-180, 0}, {180, 0}, {-73.9, 40.7}, {139.7, -35.7}, {12.5, 78.2},
		} {
			want := g.DataToPixel(pt[0], pt[1])
			got := p.DataToPixel(pt[0], pt[1])
			if got != want {
				t.Errorf("centre (%v,%v) zoom %v, point %v: projection gave %v, DataToPixel gave %v",
					g.CenterLon, g.CenterLat, g.Zoom, pt, got, want)
			}
		}
	}
}

// The snapshot is a value: it must keep projecting for the view it was taken
// for even after the shared GeoCoord is re-pointed, which is what makes it safe
// to project against off the render goroutine.
func TestGeoCoord_ProjectionIsASnapshot(t *testing.T) {
	g := NewGeoCoord(0, 0, 2)
	g.SetBounds(Rect{X: 0, Y: 0, W: 900, H: 600})
	p := g.Projection()
	want := p.DataToPixel(30, 45)

	g.SetBounds(Rect{X: 100, Y: 100, W: 400, H: 400})
	g.CenterLon, g.CenterLat, g.Zoom = 120, -20, 7

	if got := p.DataToPixel(30, 45); got != want {
		t.Errorf("projection followed the mutated GeoCoord: got %v, want %v", got, want)
	}
}
