// Package dataset provides column-oriented data storage, encoding, and transforms for charts.
package dataset

import (
	"fmt"
	"sort"
	"time"
)

// DimensionType defines the data type of a dimension.
type DimensionType int

const (
	DimFloat  DimensionType = iota // float64
	DimString                      // string (categorical)
	DimTime                        // time.Time
)

// Dimension describes one column of a dataset.
type Dimension struct {
	Name string
	Type DimensionType
}

// Dataset holds tabular data in column-oriented storage for efficient access.
type Dataset struct {
	Dimensions []Dimension
	FloatCols  [][]float64
	StringCols [][]string
	TimeCols   [][]time.Time
	rowCount   int
}

// New creates an empty Dataset with the given dimensions.
func New(dims ...Dimension) *Dataset {
	ds := &Dataset{Dimensions: dims}
	ds.FloatCols = make([][]float64, len(dims))
	ds.StringCols = make([][]string, len(dims))
	ds.TimeCols = make([][]time.Time, len(dims))
	return ds
}

// RowCount returns the number of rows.
func (d *Dataset) RowCount() int { return d.rowCount }

// AppendRow adds one row. Values must match dimension types in order.
func (d *Dataset) AppendRow(values ...interface{}) error {
	if len(values) != len(d.Dimensions) {
		return fmt.Errorf("expected %d values, got %d", len(d.Dimensions), len(values))
	}
	for i, dim := range d.Dimensions {
		switch dim.Type {
		case DimFloat:
			v, ok := toFloat64(values[i])
			if !ok {
				return fmt.Errorf("dimension %d (%s): expected numeric, got %T", i, dim.Name, values[i])
			}
			d.FloatCols[i] = append(d.FloatCols[i], v)
		case DimString:
			s, _ := values[i].(string)
			d.StringCols[i] = append(d.StringCols[i], s)
		case DimTime:
			t, ok := values[i].(time.Time)
			if !ok {
				return fmt.Errorf("dimension %d (%s): expected time.Time, got %T", i, dim.Name, values[i])
			}
			d.TimeCols[i] = append(d.TimeCols[i], t)
		}
	}
	d.rowCount++
	return nil
}

// FloatColumn returns the float64 slice for a dimension by index.
func (d *Dataset) FloatColumn(dim int) []float64 {
	if dim >= 0 && dim < len(d.FloatCols) {
		return d.FloatCols[dim]
	}
	return nil
}

// StringColumn returns the string slice for a dimension by index.
func (d *Dataset) StringColumn(dim int) []string {
	if dim >= 0 && dim < len(d.StringCols) {
		return d.StringCols[dim]
	}
	return nil
}

// TimeColumn returns the time.Time slice for a dimension by index.
func (d *Dataset) TimeColumn(dim int) []time.Time {
	if dim >= 0 && dim < len(d.TimeCols) {
		return d.TimeCols[dim]
	}
	return nil
}

// FromFloat64 creates a Dataset from a float64 matrix (row-major).
// Dimensions are named "dim0", "dim1", etc.
func FromFloat64(data [][]float64) *Dataset {
	if len(data) == 0 {
		return New()
	}
	cols := len(data[0])
	dims := make([]Dimension, cols)
	for i := range dims {
		dims[i] = Dimension{Name: fmt.Sprintf("dim%d", i), Type: DimFloat}
	}
	ds := New(dims...)
	for _, row := range data {
		vals := make([]interface{}, len(row))
		for i, v := range row {
			vals[i] = v
		}
		_ = ds.AppendRow(vals...)
	}
	return ds
}

// Encode maps dataset dimensions to visual roles.
type Encode struct {
	X     int // dimension index for X axis
	Y     int // dimension index for Y axis
	Size  int // dimension index for point size (-1 if unused)
	Color int // dimension index for color mapping (-1 if unused)
	Label int // dimension index for data labels (-1 if unused)
}

// DefaultEncode returns an Encode mapping dim0→X, dim1→Y.
func DefaultEncode() Encode {
	return Encode{X: 0, Y: 1, Size: -1, Color: -1, Label: -1}
}

// ExtractXY extracts X and Y float64 slices from a Dataset using an Encode mapping.
func ExtractXY(ds *Dataset, enc Encode) (xs, ys []float64) {
	xs = ds.FloatColumn(enc.X)
	ys = ds.FloatColumn(enc.Y)
	return
}

// Transform is a data transformation step.
type Transform interface {
	Apply(ds *Dataset) (*Dataset, error)
}

// FilterTransform keeps rows where a float dimension satisfies a predicate.
type FilterTransform struct {
	Dimension int
	Predicate func(float64) bool
}

// Apply filters rows by the predicate.
func (f *FilterTransform) Apply(ds *Dataset) (*Dataset, error) {
	col := ds.FloatColumn(f.Dimension)
	if col == nil {
		return nil, fmt.Errorf("dimension %d is not a float column", f.Dimension)
	}
	result := New(ds.Dimensions...)
	for row := 0; row < ds.rowCount; row++ {
		if !f.Predicate(col[row]) {
			continue
		}
		vals := make([]interface{}, len(ds.Dimensions))
		for d, dim := range ds.Dimensions {
			switch dim.Type {
			case DimFloat:
				vals[d] = ds.FloatCols[d][row]
			case DimString:
				vals[d] = ds.StringCols[d][row]
			case DimTime:
				vals[d] = ds.TimeCols[d][row]
			}
		}
		_ = result.AppendRow(vals...)
	}
	return result, nil
}

// SortTransform sorts rows by a float dimension.
type SortTransform struct {
	Dimension  int
	Descending bool
}

// Apply sorts the dataset by the given dimension.
func (s *SortTransform) Apply(ds *Dataset) (*Dataset, error) {
	col := ds.FloatColumn(s.Dimension)
	if col == nil {
		return nil, fmt.Errorf("dimension %d is not a float column", s.Dimension)
	}

	// Build index and sort
	indices := make([]int, ds.rowCount)
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		if s.Descending {
			return col[indices[i]] > col[indices[j]]
		}
		return col[indices[i]] < col[indices[j]]
	})

	// Rebuild dataset in sorted order
	result := New(ds.Dimensions...)
	for _, idx := range indices {
		vals := make([]interface{}, len(ds.Dimensions))
		for d, dim := range ds.Dimensions {
			switch dim.Type {
			case DimFloat:
				vals[d] = ds.FloatCols[d][idx]
			case DimString:
				vals[d] = ds.StringCols[d][idx]
			case DimTime:
				vals[d] = ds.TimeCols[d][idx]
			}
		}
		_ = result.AppendRow(vals...)
	}
	return result, nil
}

// Pipeline chains multiple transforms.
type Pipeline struct {
	Steps []Transform
}

// Apply runs all transforms in sequence.
func (p *Pipeline) Apply(ds *Dataset) (*Dataset, error) {
	current := ds
	for _, step := range p.Steps {
		var err error
		current, err = step.Apply(current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}
