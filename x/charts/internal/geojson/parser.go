// Package geojson provides a minimal GeoJSON parser for choropleth map rendering.
package geojson

import (
	"encoding/json"
	"fmt"
)

// FeatureCollection is the top-level GeoJSON container.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// Feature represents a single geographic feature.
type Feature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   Geometry               `json:"geometry"`
}

// Name returns the feature name from common GeoJSON property fields.
func (f *Feature) Name() string {
	for _, key := range []string{"name", "NAME", "admin", "ADMIN", "name_en", "NAME_EN", "iso_a3", "ISO_A3"} {
		if v, ok := f.Properties[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// ID returns a feature identifier from common fields.
func (f *Feature) ID() string {
	for _, key := range []string{"iso_a3", "ISO_A3", "iso_a2", "ISO_A2", "id", "ID", "adm0_a3"} {
		if v, ok := f.Properties[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return f.Name()
}

// Geometry holds the shape data.
//
// Coordinates are decoded once, when the document is parsed, rather than on
// every call to [Geometry.Polygons]. They used to be kept as a json.RawMessage
// and re-unmarshalled per call, which meant the choropleth re-parsed the whole
// coordinate payload on every rasterisation -- about half the cost of the first
// paint of a new view on a country-sized data set (belluikit/bellui#40). It also
// retained a second copy of every coordinate as raw JSON for the life of the
// collection, which is now dropped.
type Geometry struct {
	Type string `json:"type"`

	rings [][][2]float64
	err   error
}

// UnmarshalJSON decodes the geometry, resolving its coordinates into rings up
// front. A geometry whose coordinates are malformed does not fail the whole
// document: the error is recorded and reported by [Geometry.Polygons], so one
// bad feature is skipped rather than costing the caller every other feature.
func (g *Geometry) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	g.Type, g.rings, g.err = raw.Type, nil, nil
	switch raw.Type {
	case "Polygon":
		var rings [][][2]float64
		if err := json.Unmarshal(raw.Coordinates, &rings); err != nil {
			g.err = fmt.Errorf("parse Polygon: %w", err)
			return nil
		}
		if len(rings) > 0 {
			g.rings = [][][2]float64{rings[0]} // outer ring only
		}

	case "MultiPolygon":
		var multiRings [][][][2]float64
		if err := json.Unmarshal(raw.Coordinates, &multiRings); err != nil {
			g.err = fmt.Errorf("parse MultiPolygon: %w", err)
			return nil
		}
		for _, poly := range multiRings {
			if len(poly) > 0 {
				g.rings = append(g.rings, poly[0]) // outer ring only
			}
		}
	}
	// Non-polygon geometries are skipped, with no rings and no error.
	return nil
}

// Polygons returns the outer ring of each polygon in this geometry: one for a
// Polygon, one per member for a MultiPolygon, none for any other type.
//
// The returned slice is the geometry's own storage and is shared by every
// caller -- it must not be modified. Callers may read it concurrently.
func (g *Geometry) Polygons() ([][][2]float64, error) {
	return g.rings, g.err
}

// Parse reads GeoJSON bytes into a FeatureCollection.
func Parse(data []byte) (*FeatureCollection, error) {
	var fc FeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse GeoJSON: %w", err)
	}
	return &fc, nil
}
