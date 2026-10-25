package series

import (
	"fmt"
	"math"
	"sort"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/geojson"
)

// RegisterHitAreas registers one hit point per region that has data, at the
// centre of the region's largest polygon, labelled "name: value". The chart
// used to place them from a built-in list of about 25 countries, so every
// other region had no tooltip (#343). Until URL-backed GeoJSON has loaded
// there is nothing to register; the repaint that follows the load registers
// the regions.
func (s *ChoroplethSeries) RegisterHitAreas(collector HitAreaCollector, seriesIndex int, coordSys coord.System, plotRect coord.Rect) {
	if len(s.RegionData) == 0 {
		return
	}
	// Render has already asked for the data and handed the loader its
	// repaint, so this lookup starts nothing of its own.
	fc := (&choroplethRenderer{}).getGeoJSON(s, nil)
	if fc == nil {
		return
	}
	gc, ok := coordSys.(*coord.GeoCoord)
	if !ok { // the projection Render uses when no map shares one
		gc = coord.NewGeoCoord(s.MapCenter[0], s.MapCenter[1], s.MapZoom)
		gc.SetBounds(plotRect)
	}

	centres := s.regionCentres(fc)
	names := make([]string, 0, len(s.RegionData))
	for name := range s.RegionData {
		names = append(names, name)
	}
	sort.Strings(names)

	points := make([]HitPoint, 0, len(names))
	for _, name := range names {
		c, ok := centres[name]
		if !ok {
			continue // no region of that name or ID in the GeoJSON
		}
		val := s.RegionData[name]
		points = append(points, HitPoint{
			DataIndex: len(points),
			PixelPos:  gc.DataToPixel(c[0], c[1]),
			Value:     HitValue{X: c[0], Y: c[1], Label: fmt.Sprintf("%s: %.1f", name, val)},
		})
	}
	collector.AddHitPoints(seriesIndex, s.SeriesName, TypeChoropleth, points)
}

// regionCentres maps each feature's name and ID to the centre (lon, lat) of
// its largest polygon, caching the result for the collection: the country data
// set has thousands of rings.
func (s *ChoroplethSeries) regionCentres(fc *geojson.FeatureCollection) map[string][2]float64 {
	if s.centres != nil && s.centres.fc == fc {
		return s.centres.byName
	}
	byName := make(map[string][2]float64, 2*len(fc.Features))
	for i := range fc.Features {
		f := &fc.Features[i]
		polygons, err := f.Geometry.Polygons()
		if err != nil {
			continue
		}
		c, ok := largestRingCentre(polygons)
		if !ok {
			continue
		}
		if n := f.Name(); n != "" {
			byName[n] = c
		}
		if id := f.ID(); id != "" {
			if _, taken := byName[id]; !taken {
				byName[id] = c
			}
		}
	}
	s.centres = &regionCentres{fc: fc, byName: byName}
	return byName
}

// regionCentres caches ChoroplethSeries.regionCentres for one collection.
type regionCentres struct {
	fc     *geojson.FeatureCollection
	byName map[string][2]float64
}

// largestRingCentre is the area centroid of the ring with the largest area:
// the mainland rather than an outlying island, so a country's tooltip sits
// over the bulk of it.
func largestRingCentre(rings [][][2]float64) ([2]float64, bool) {
	var best [2]float64
	bestArea := 0.0
	for _, ring := range rings {
		var a, cx, cy float64
		for i := range ring {
			p, q := ring[i], ring[(i+1)%len(ring)]
			cross := p[0]*q[1] - q[0]*p[1]
			a += cross
			cx += (p[0] + q[0]) * cross
			cy += (p[1] + q[1]) * cross
		}
		if area := math.Abs(a / 2); area > bestArea {
			bestArea = area
			best = [2]float64{cx / (3 * a), cy / (3 * a)}
		}
	}
	return best, bestArea > 0
}
