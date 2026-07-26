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
type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// Polygons returns the polygon rings for this geometry.
// For Polygon: returns one polygon (outer ring + holes).
// For MultiPolygon: returns multiple polygons.
func (g *Geometry) Polygons() ([][][2]float64, error) {
	switch g.Type {
	case "Polygon":
		var rings [][][2]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			return nil, fmt.Errorf("parse Polygon: %w", err)
		}
		// Return outer ring only (index 0)
		if len(rings) > 0 {
			return [][][2]float64{rings[0]}, nil
		}
		return nil, nil

	case "MultiPolygon":
		var multiRings [][][][2]float64
		if err := json.Unmarshal(g.Coordinates, &multiRings); err != nil {
			return nil, fmt.Errorf("parse MultiPolygon: %w", err)
		}
		var result [][][2]float64
		for _, poly := range multiRings {
			if len(poly) > 0 {
				result = append(result, poly[0]) // outer ring only
			}
		}
		return result, nil

	default:
		return nil, nil // Skip non-polygon geometries
	}
}

// Parse reads GeoJSON bytes into a FeatureCollection.
func Parse(data []byte) (*FeatureCollection, error) {
	var fc FeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse GeoJSON: %w", err)
	}
	return &fc, nil
}
