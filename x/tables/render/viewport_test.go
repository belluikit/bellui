package render

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestViewport_UniformRows(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        100,
		TotalCols:        5,
		ColWidths:        []float32{100, 100, 100, 100, 100},
	}
	vp.RecomputeColPositions()

	// Viewport showing top-left area
	vp.Update(geo.NewPos(0, 0), geo.NewSize(400, 200))

	// With 32px per row (30 + 2 padding), 200px viewport should show ~6 rows
	if vp.VisibleRows.Min != 0 {
		t.Errorf("expected min row 0, got %d", vp.VisibleRows.Min)
	}
	if vp.VisibleRows.Max < 5 || vp.VisibleRows.Max > 7 {
		t.Errorf("expected max row ~6, got %d", vp.VisibleRows.Max)
	}

	// Columns: 4 columns should be visible in 400px (100 + 2 padding each = 102px)
	if vp.VisibleCols.Min != 0 {
		t.Errorf("expected min col 0, got %d", vp.VisibleCols.Min)
	}
	if vp.VisibleCols.Max < 3 {
		t.Errorf("expected at least 4 visible cols, got max %d", vp.VisibleCols.Max)
	}
}

func TestViewport_ScrolledDown(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        1000,
		TotalCols:        3,
		ColWidths:        []float32{100, 100, 100},
	}
	vp.RecomputeColPositions()

	// Scroll to row ~50
	offset := float32(50) * 32 // 50 * (30 + 2)
	vp.Update(geo.NewPos(0, offset), geo.NewSize(300, 200))

	if vp.VisibleRows.Min != 50 {
		t.Errorf("expected min row 50, got %d", vp.VisibleRows.Min)
	}
}

func TestViewport_ContentSize(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        100,
		TotalCols:        3,
		ColWidths:        []float32{100, 200, 150},
	}
	vp.RecomputeColPositions()

	size := vp.ContentSize()

	expectedH := float32(100) * 32 // 100 * (30 + 2)
	if size.Height != expectedH {
		t.Errorf("expected height %.0f, got %.0f", expectedH, size.Height)
	}

	// Width: 100 + 2 + 200 + 2 + 150 = 454
	expectedW := vp.ColPositions[2] + vp.ColWidths[2]
	if size.Width != expectedW {
		t.Errorf("expected width %.0f, got %.0f", expectedW, size.Width)
	}
}

func TestViewport_EmptyData(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        0,
		TotalCols:        0,
	}

	vp.Update(geo.NewPos(0, 0), geo.NewSize(400, 200))

	if vp.VisibleRows.Min != 0 || vp.VisibleRows.Max != -1 {
		t.Errorf("expected empty row range, got %v", vp.VisibleRows)
	}
	if vp.VisibleCols.Min != 0 || vp.VisibleCols.Max != -1 {
		t.Errorf("expected empty col range, got %v", vp.VisibleCols)
	}
}

func TestViewport_VariableRowHeights(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        10,
		TotalCols:        1,
		ColWidths:        []float32{100},
		RowHeights:       map[int]float32{3: 60, 7: 80},
	}
	vp.RecomputeColPositions()

	vp.Update(geo.NewPos(0, 0), geo.NewSize(100, 200))

	if vp.VisibleRows.Min != 0 {
		t.Errorf("expected min row 0, got %d", vp.VisibleRows.Min)
	}
}

func TestViewport_RowY(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        5,
		TotalCols:        1,
		ColWidths:        []float32{100},
	}
	vp.RecomputeColPositions()

	y, h := vp.RowY(0)
	if y != 0 {
		t.Errorf("row 0: expected y=0, got %f", y)
	}
	if h != 30 {
		t.Errorf("row 0: expected h=30, got %f", h)
	}

	y, h = vp.RowY(2)
	expectedY := float32(2) * 32
	if y != expectedY {
		t.Errorf("row 2: expected y=%f, got %f", expectedY, y)
	}
}

func TestViewport_ColX(t *testing.T) {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        1,
		TotalCols:        3,
		ColWidths:        []float32{100, 200, 150},
	}
	vp.RecomputeColPositions()

	x, w := vp.ColX(0)
	if x != 0 || w != 100 {
		t.Errorf("col 0: expected x=0,w=100, got x=%f,w=%f", x, w)
	}

	x, w = vp.ColX(1)
	if x != 102 || w != 200 { // 100 + 2 padding
		t.Errorf("col 1: expected x=102,w=200, got x=%f,w=%f", x, w)
	}
}
