package chart

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/series"
)

// Two regions: "Atlantis", a square around (10, 10) with a small island far
// off, and "Lemuria", a square around (-20, -15). Neither is in the old
// built-in list of country centres.
const twoRegions = `{"type":"FeatureCollection","features":[
 {"type":"Feature","properties":{"name":"Atlantis"},"geometry":{"type":"MultiPolygon","coordinates":[
  [[[5,5],[15,5],[15,15],[5,15],[5,5]]],
  [[[40,40],[41,40],[41,41],[40,41],[40,40]]]]}},
 {"type":"Feature","properties":{"name":"Lemuria"},"geometry":{"type":"Polygon","coordinates":[
  [[-25,-20],[-15,-20],[-15,-10],[-25,-10],[-25,-20]]]}}]}`

// A choropleth's tooltip finds every region with data, at the centre of the
// region's largest polygon. Only about 25 countries from a built-in list had
// a tooltip, and only when a map series shared the projection (#343).
func TestChoropleth_TooltipFindsEveryRegion(t *testing.T) {
	test.NewTempApp(t)
	cs := &series.ChoroplethSeries{
		SeriesName: "c",
		GeoJSON:    []byte(twoRegions),
		RegionData: map[string]float64{"Atlantis": 3, "Lemuria": 7, "Nowhere": 1},
		MapZoom:    1,
	}
	c := New(Option{Series: []series.Series{cs}})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(600, 400))
	test.TempWidgetRenderer(t, c).Refresh()

	gc := coord.NewGeoCoord(cs.MapCenter[0], cs.MapCenter[1], cs.MapZoom)
	gc.SetBounds(c.renderSnap.plotRect)
	for name, centre := range map[string][2]float64{"Atlantis: 3.0": {10, 10}, "Lemuria: 7.0": {-20, -15}} {
		hit := c.renderSnap.hitTester.FindNearest(gc.DataToPixel(centre[0], centre[1]), 2)
		if hit == nil || hit.Value.Label != name {
			t.Errorf("hovering the centre of %q hit %+v", name, hit)
		}
	}
}
