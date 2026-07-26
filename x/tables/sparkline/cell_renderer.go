package sparkline

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/tables/cell"
)

// CellRenderer is a cell.RendererFactory that displays sparkline charts in grid cells.
type CellRenderer struct {
	ChartType Type
	Opts      Options
}

// NewCellRenderer creates a sparkline cell renderer.
func NewCellRenderer(chartType Type, opts ...func(*Options)) *CellRenderer {
	r := &CellRenderer{
		ChartType: chartType,
		Opts: Options{
			StrokeWidth: 1.5,
		},
	}
	for _, opt := range opts {
		opt(&r.Opts)
	}
	return r
}

// CreateCell creates a new Sparkline widget for use as a cell template.
func (r *CellRenderer) CreateCell() core.CanvasObject {
	return New(r.ChartType, nil, func(o *Options) {
		*o = r.Opts
	})
}

// UpdateCell updates the sparkline with the cell value (expected to be []float64).
func (r *CellRenderer) UpdateCell(obj core.CanvasObject, params cell.Params) {
	s, ok := obj.(*Sparkline)
	if !ok {
		return
	}
	data, ok := params.Value.([]float64)
	if !ok {
		return
	}
	s.SetData(data)
}
