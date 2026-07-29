package render

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

// Hot-path lookups must stay fast (O(log n)) even with a million variable-height
// rows — this is the whole point of the prefix sum vs. the old linear scan.
func BenchmarkViewport_RowAtY_VariableMillion(b *testing.B) {
	const n = 1_000_000
	overrides := make(map[int]float32, n/10)
	for i := 0; i < n; i += 10 {
		overrides[i] = 60 // every 10th row is taller
	}
	vp := &Viewport{DefaultRowHeight: 30, Padding: 2, TotalRows: n, TotalCols: 1, ColWidths: []float32{100}, RowHeights: overrides}
	vp.RecomputeColPositions()
	vp.RecomputeRowPositions()
	total := vp.ContentSize().Height

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Sweep across the whole content height.
		_ = vp.RowAtY(float32(i%1000) / 1000 * total)
	}
}

// newVariableViewport builds a 6-row viewport with two taller rows.
// Pitch (height+padding): rows 0,1,3,5 = 32; row 2 = 52; row 4 = 72.
// rowTops = [0, 32, 64, 116, 148, 220, 252].
func newVariableViewport() *Viewport {
	vp := &Viewport{
		DefaultRowHeight: 30,
		Padding:          2,
		TotalRows:        6,
		TotalCols:        1,
		ColWidths:        []float32{100},
		RowHeights:       map[int]float32{2: 50, 4: 70},
	}
	vp.RecomputeColPositions()
	vp.RecomputeRowPositions()
	return vp
}

func TestViewport_VariableRowY(t *testing.T) {
	vp := newVariableViewport()
	cases := []struct {
		row   int
		wantY float32
		wantH float32
	}{
		{0, 0, 30},
		{1, 32, 30},
		{2, 64, 50},
		{3, 116, 30},
		{4, 148, 70},
		{5, 220, 30},
	}
	for _, c := range cases {
		y, h := vp.RowY(c.row)
		if y != c.wantY || h != c.wantH {
			t.Errorf("RowY(%d) = (%v, %v), want (%v, %v)", c.row, y, h, c.wantY, c.wantH)
		}
	}
}

func TestViewport_VariableContentSize(t *testing.T) {
	vp := newVariableViewport()
	if got := vp.ContentSize().Height; got != 252 {
		t.Errorf("ContentSize().Height = %v, want 252", got)
	}
}

func TestViewport_VariableRowAtY(t *testing.T) {
	vp := newVariableViewport()
	cases := []struct {
		y    float32
		want int
	}{
		{0, 0}, {31, 0},
		{32, 1}, {63, 1},
		{64, 2}, {115, 2},
		{116, 3},
		{148, 4}, {219, 4},
		{220, 5}, {251, 5},
		{252, -1}, // past the last row
		{-1, -1},  // above the top
	}
	for _, c := range cases {
		if got := vp.RowAtY(c.y); got != c.want {
			t.Errorf("RowAtY(%v) = %d, want %d", c.y, got, c.want)
		}
	}
}

// RowY and RowAtY must be exact inverses at row boundaries.
func TestViewport_VariableRoundTrip(t *testing.T) {
	vp := newVariableViewport()
	for row := 0; row < vp.TotalRows; row++ {
		y, h := vp.RowY(row)
		if got := vp.RowAtY(y); got != row {
			t.Errorf("RowAtY(top of row %d = %v) = %d, want %d", row, y, got, row)
		}
		// A point inside the row (before the separator) resolves to the same row.
		if got := vp.RowAtY(y + h - 1); got != row {
			t.Errorf("RowAtY(inside row %d = %v) = %d, want %d", row, y+h-1, got, row)
		}
	}
}

func TestViewport_VariableVisibleRows(t *testing.T) {
	vp := newVariableViewport()

	vp.Update(geo.NewPos(0, 0), geo.NewSize(100, 100))
	if vp.VisibleRows.Min != 0 || vp.VisibleRows.Max != 2 {
		t.Errorf("top: VisibleRows = %+v, want {0 2}", vp.VisibleRows)
	}
	if vp.RowPixelOffset != 0 {
		t.Errorf("top: RowPixelOffset = %v, want 0", vp.RowPixelOffset)
	}

	vp.Update(geo.NewPos(0, 100), geo.NewSize(100, 100))
	if vp.VisibleRows.Min != 2 || vp.VisibleRows.Max != 4 {
		t.Errorf("scrolled: VisibleRows = %+v, want {2 4}", vp.VisibleRows)
	}
	if vp.RowPixelOffset != 64 {
		t.Errorf("scrolled: RowPixelOffset = %v, want 64 (top of row 2)", vp.RowPixelOffset)
	}
}

// A uniform viewport (no overrides) must keep the O(1) path and allocate no
// prefix sum.
func TestViewport_UniformUnaffected(t *testing.T) {
	vp := &Viewport{DefaultRowHeight: 30, Padding: 2, TotalRows: 100, TotalCols: 1, ColWidths: []float32{100}}
	vp.RecomputeColPositions()
	vp.RecomputeRowPositions()

	if vp.rowTops != nil {
		t.Errorf("uniform viewport built a prefix sum (%d entries), want nil", len(vp.rowTops))
	}
	if y, h := vp.RowY(10); y != 320 || h != 30 {
		t.Errorf("RowY(10) = (%v, %v), want (320, 30)", y, h)
	}
	if got := vp.RowAtY(320); got != 10 {
		t.Errorf("RowAtY(320) = %d, want 10", got)
	}
	if got := vp.ContentSize().Height; got != 3200 {
		t.Errorf("ContentSize().Height = %v, want 3200", got)
	}
}
