package render

import (
	"math"
	"sort"

	"github.com/belluikit/bellui/geo"
)

// Range represents an inclusive min/max range of indices.
type Range struct {
	Min, Max int
}

// Viewport calculates the visible cell range based on scroll position and viewport size.
// For uniform row heights, this is O(1). For variable row heights, O(log n).
type Viewport struct {
	// Column geometry (pre-computed from resolved columns).
	ColPositions []float32 // cumulative X positions (left edges)
	ColWidths    []float32 // width of each column

	// Row geometry.
	DefaultRowHeight float32
	RowHeights       map[int]float32 // per-row height overrides (display-row keyed); empty ⇒ uniform
	rowTops          []float32       // cumulative row top edges (len TotalRows+1); nil ⇒ uniform. Built from RowHeights.

	Padding float32 // separator thickness

	TotalRows int
	TotalCols int

	// Computed visible ranges.
	VisibleRows Range
	VisibleCols Range
	// Pixel offset of the first visible row/col within the viewport.
	RowPixelOffset float32
	ColPixelOffset float32
}

// Update recalculates visible ranges from the scroll offset and viewport size.
func (v *Viewport) Update(offset geo.Position, viewSize geo.Size) {
	v.updateVisibleRows(offset.Y, viewSize.Height)
	v.updateVisibleCols(offset.X, viewSize.Width)
}

func (v *Viewport) updateVisibleRows(offsetY, height float32) {
	if v.TotalRows == 0 {
		v.VisibleRows = Range{0, -1}
		return
	}

	if len(v.RowHeights) == 0 {
		// Uniform row heights — O(1)
		rowH := v.DefaultRowHeight + v.Padding
		if rowH <= 0 {
			rowH = 1
		}
		minRow := int(math.Floor(float64(offsetY / rowH)))
		if minRow < 0 {
			minRow = 0
		}
		maxRow := int(math.Ceil(float64((offsetY + height) / rowH)))
		if maxRow >= v.TotalRows {
			maxRow = v.TotalRows - 1
		}
		v.VisibleRows = Range{minRow, maxRow}
		v.RowPixelOffset = float32(minRow) * rowH
	} else {
		// Variable row heights — O(log n) via prefix-sum binary search
		v.visibleRowsVariable(offsetY, height)
	}
}

// visibleRowsVariable resolves the visible range with per-row heights in
// O(log n) via binary search over the cumulative rowTops prefix sum.
func (v *Viewport) visibleRowsVariable(offsetY, height float32) {
	v.ensureRowTops()

	// First row whose bottom edge is past the viewport top.
	minRow := sort.Search(v.TotalRows, func(i int) bool {
		return v.rowTops[i+1] > offsetY
	})
	if minRow >= v.TotalRows {
		minRow = v.TotalRows - 1
	}
	v.RowPixelOffset = v.rowTops[minRow]

	// Last row whose top edge is before the viewport bottom.
	maxRow := sort.Search(v.TotalRows, func(i int) bool {
		return v.rowTops[i] >= offsetY+height
	}) - 1
	if maxRow < minRow {
		maxRow = minRow
	}
	if maxRow >= v.TotalRows {
		maxRow = v.TotalRows - 1
	}
	v.VisibleRows = Range{minRow, maxRow}
}

func (v *Viewport) updateVisibleCols(offsetX, width float32) {
	if v.TotalCols == 0 {
		v.VisibleCols = Range{0, -1}
		return
	}

	// Binary search for first visible column.
	minCol := sort.Search(len(v.ColPositions), func(i int) bool {
		return v.ColPositions[i]+v.ColWidths[i] > offsetX
	})
	if minCol >= v.TotalCols {
		minCol = v.TotalCols - 1
	}
	v.ColPixelOffset = v.ColPositions[minCol]

	// Find last visible column.
	maxCol := minCol
	for maxCol < v.TotalCols-1 && v.ColPositions[maxCol+1] < offsetX+width {
		maxCol++
	}

	v.VisibleCols = Range{minCol, maxCol}
}

// ContentSize returns the total size of all rows and columns.
func (v *Viewport) ContentSize() geo.Size {
	var totalH float32
	if len(v.RowHeights) == 0 {
		totalH = float32(v.TotalRows) * (v.DefaultRowHeight + v.Padding)
	} else {
		v.ensureRowTops()
		totalH = v.rowTops[v.TotalRows]
	}

	var totalW float32
	if v.TotalCols > 0 {
		lastCol := v.TotalCols - 1
		totalW = v.ColPositions[lastCol] + v.ColWidths[lastCol]
	}

	return geo.NewSize(totalW, totalH)
}

// RowY returns the Y position and height of the given row.
func (v *Viewport) RowY(row int) (y, height float32) {
	if len(v.RowHeights) == 0 {
		return float32(row) * (v.DefaultRowHeight + v.Padding), v.DefaultRowHeight
	}
	v.ensureRowTops()
	if row < 0 {
		return 0, v.DefaultRowHeight
	}
	if row >= v.TotalRows {
		return v.rowTops[v.TotalRows], v.DefaultRowHeight
	}
	return v.rowTops[row], v.rowTops[row+1] - v.rowTops[row] - v.Padding
}

// ColX returns the X position and width of the given column.
func (v *Viewport) ColX(col int) (x, width float32) {
	if col < 0 || col >= v.TotalCols {
		return 0, 0
	}
	return v.ColPositions[col], v.ColWidths[col]
}

// RecomputeColPositions rebuilds column position data from widths.
func (v *Viewport) RecomputeColPositions() {
	v.ColPositions = make([]float32, len(v.ColWidths))
	x := float32(0)
	for i, w := range v.ColWidths {
		v.ColPositions[i] = x
		x += w + v.Padding
	}
}

// RecomputeRowPositions rebuilds the cumulative row-top prefix sum from
// RowHeights. Call it after RowHeights, TotalRows, DefaultRowHeight, or Padding
// change. It is a no-op (and frees the array) when heights are uniform, so the
// uniform path stays allocation-free. O(TotalRows).
func (v *Viewport) RecomputeRowPositions() {
	if len(v.RowHeights) == 0 || v.TotalRows == 0 {
		v.rowTops = nil
		return
	}
	v.rowTops = make([]float32, v.TotalRows+1)
	y := float32(0)
	for i := 0; i < v.TotalRows; i++ {
		v.rowTops[i] = y
		h := v.DefaultRowHeight
		if hh, ok := v.RowHeights[i]; ok {
			h = hh
		}
		y += h + v.Padding
	}
	v.rowTops[v.TotalRows] = y
}

// ensureRowTops lazily (re)builds the prefix sum if it is missing or stale
// (TotalRows changed). It is the safety net for callers that set RowHeights
// directly without calling RecomputeRowPositions; the main grid rebuilds
// explicitly on geometry change.
func (v *Viewport) ensureRowTops() {
	if len(v.rowTops) != v.TotalRows+1 {
		v.RecomputeRowPositions()
	}
}

// RowAtY returns the row index at content-space Y position y, or -1 if y is
// outside the rows. O(1) for uniform heights, O(log n) for variable.
func (v *Viewport) RowAtY(y float32) int {
	if y < 0 || v.TotalRows == 0 {
		return -1
	}
	if len(v.RowHeights) == 0 {
		rowH := v.DefaultRowHeight + v.Padding
		if rowH <= 0 {
			return -1
		}
		row := int(y / rowH)
		if row >= v.TotalRows {
			return -1
		}
		return row
	}
	v.ensureRowTops()
	if y >= v.rowTops[v.TotalRows] {
		return -1
	}
	row := sort.Search(v.TotalRows, func(i int) bool { return v.rowTops[i+1] > y })
	if row >= v.TotalRows {
		return -1
	}
	return row
}
