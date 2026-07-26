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
