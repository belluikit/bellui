package geojson

import "testing"

func TestParse_SimplePolygon(t *testing.T) {
	data := []byte(`{
		"type": "FeatureCollection",
		"features": [{
			"type": "Feature",
			"properties": {"name": "TestCountry", "iso_a3": "TST"},
			"geometry": {
				"type": "Polygon",
				"coordinates": [[[0,0],[10,0],[10,10],[0,10],[0,0]]]
			}
		}]
	}`)

	fc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	f := fc.Features[0]
	if f.Name() != "TestCountry" {
		t.Errorf("name = %q, want TestCountry", f.Name())
	}
	if f.ID() != "TST" {
		t.Errorf("id = %q, want TST", f.ID())
	}

	polys, err := f.Geometry.Polygons()
	if err != nil {
		t.Fatal(err)
	}
	if len(polys) != 1 {
		t.Fatalf("expected 1 polygon, got %d", len(polys))
	}
	if len(polys[0]) != 5 { // closed ring: 4 corners + closing point
		t.Errorf("expected 5 points in ring, got %d", len(polys[0]))
	}
}

func TestParse_MultiPolygon(t *testing.T) {
	data := []byte(`{
		"type": "FeatureCollection",
		"features": [{
			"type": "Feature",
			"properties": {"name": "Islands"},
			"geometry": {
				"type": "MultiPolygon",
				"coordinates": [
					[[[0,0],[1,0],[1,1],[0,1],[0,0]]],
					[[[5,5],[6,5],[6,6],[5,6],[5,5]]]
				]
			}
		}]
	}`)

	fc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	polys, err := fc.Features[0].Geometry.Polygons()
	if err != nil {
		t.Fatal(err)
	}
	if len(polys) != 2 {
		t.Errorf("expected 2 polygons for MultiPolygon, got %d", len(polys))
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// Coordinates are decoded once, when the document is parsed. They used to be
// kept as raw JSON and re-unmarshalled on every Polygons call, which meant the
// choropleth re-parsed its whole coordinate payload on every rasterisation
// (belluikit/bellui#40). Repeated calls must hand back the same backing arrays.
func TestGeometry_PolygonsDecodedOnce(t *testing.T) {
	fc, err := Parse([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature",
		"properties":{"name":"A"},"geometry":{"type":"Polygon",
		"coordinates":[[[0,0],[10,0],[10,10],[0,10],[0,0]]]}}]}`))
	if err != nil {
		t.Fatal(err)
	}

	g := &fc.Features[0].Geometry
	first, err := g.Polygons()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := g.Polygons()

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected 1 ring, got %d and %d", len(first), len(second))
	}
	if &first[0][0] != &second[0][0] {
		t.Error("Polygons re-decoded the coordinates instead of returning the parsed rings")
	}
}

// One malformed geometry must not cost the caller every other feature: the
// error is recorded and reported per-geometry, as it was when Polygons decoded
// lazily, rather than failing the whole document.
func TestParse_MalformedGeometryIsPerFeature(t *testing.T) {
	fc, err := Parse([]byte(`{"type":"FeatureCollection","features":[
		{"type":"Feature","properties":{"name":"bad"},"geometry":{"type":"Polygon","coordinates":"nope"}},
		{"type":"Feature","properties":{"name":"good"},"geometry":{"type":"Polygon",
			"coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}}]}`))
	if err != nil {
		t.Fatalf("one bad geometry must not fail the document: %v", err)
	}
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(fc.Features))
	}

	if _, err := fc.Features[0].Geometry.Polygons(); err == nil {
		t.Error("expected the malformed geometry to report its error")
	}
	polys, err := fc.Features[1].Geometry.Polygons()
	if err != nil {
		t.Errorf("the good feature must still parse: %v", err)
	}
	if len(polys) != 1 {
		t.Errorf("expected 1 ring for the good feature, got %d", len(polys))
	}
}

// A null or non-polygon geometry yields no rings and no error.
func TestParse_NonPolygonGeometry(t *testing.T) {
	fc, err := Parse([]byte(`{"type":"FeatureCollection","features":[
		{"type":"Feature","properties":{},"geometry":null},
		{"type":"Feature","properties":{},"geometry":{"type":"Point","coordinates":[1,2]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for i := range fc.Features {
		polys, err := fc.Features[i].Geometry.Polygons()
		if err != nil {
			t.Errorf("feature %d: unexpected error %v", i, err)
		}
		if len(polys) != 0 {
			t.Errorf("feature %d: expected no rings, got %d", i, len(polys))
		}
	}
}
