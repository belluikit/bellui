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
