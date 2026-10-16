package render

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

// fakeFilterAccess is a minimal FloatingFilterAccess with a fixed region layout:
// one pinned-left column, two main, one pinned-right (all width 80, padding 4).
// Absolute display columns 0..3.
type fakeFilterAccess struct {
	offsetX  float32
	lastCol  int
	lastText string
}

func (f *fakeFilterAccess) HeaderColumnCount() int        { return 4 }
func (f *fakeFilterAccess) HeaderColumnWidths() []float32 { return []float32{80, 80, 80, 80} }
func (f *fakeFilterAccess) CellPadding() float32          { return 4 }
func (f *fakeFilterAccess) PinnedLeftCount() int          { return 1 }
func (f *fakeFilterAccess) PinnedRightCount() int         { return 1 }
func (f *fakeFilterAccess) ScrollOffsetX() float32        { return f.offsetX }

// mainX = 80+4 = 84; mainColsW = 2*(80+4) = 168; pinnedRightX = 84+168 = 252.
func (f *fakeFilterAccess) HeaderRegions() (mainX, pinnedRightX, pinnedRightW float32) {
	return 84, 252, 80
}

func (f *fakeFilterAccess) OnFloatingFilterChanged(col int, text string) {
	f.lastCol, f.lastText = col, text
}

func layoutFilterRow(fake *fakeFilterAccess) (*FloatingFilterRow, *floatingFilterRenderer) {
	row := NewFloatingFilterRow(fake, &Viewport{})
	r := row.CreateRenderer().(*floatingFilterRenderer)
	r.Layout(geo.NewSize(400, 30))
	return row, r
}

// The floating-filter row renders one entry per VISIBLE column (pinned + main +
// right), not just the main columns (pre-rebrand tracker #63).
func TestFloatingFilter_EntryPerVisibleColumn(t *testing.T) {
	row, _ := layoutFilterRow(&fakeFilterAccess{})
	if len(row.entries) != 4 {
		t.Fatalf("entry count = %d, want 4 (all visible columns)", len(row.entries))
	}
}

// The main-band entries live inside the clip container (so the driver clips
// their painting AND their pointer events to the main region), while the pinned
// entries are positioned directly on the row — this is what removes the
// interactive sliver, not a visual-only occluder (pre-rebrand tracker #65).
func TestFloatingFilter_MainEntriesAreClipped(t *testing.T) {
	_, r := layoutFilterRow(&fakeFilterAccess{})

	// Main columns 1 and 2 must be children of the clip's content band.
	band := map[any]bool{}
	for _, o := range r.bands.mainBand.Objects {
		band[o] = true
	}
	for _, i := range []int{1, 2} {
		if !band[r.row.entries[i]] {
			t.Errorf("main entry %d is not inside the clip band", i)
		}
	}
	// Pinned columns 0 and 3 must NOT be in the clip band (they render directly).
	for _, i := range []int{0, 3} {
		if band[r.row.entries[i]] {
			t.Errorf("pinned entry %d wrongly placed inside the clip band", i)
		}
	}
	// The clip is present in the row's top-level objects.
	found := false
	for _, o := range r.Objects() {
		if o == r.bands.mainClip {
			found = true
		}
	}
	if !found {
		t.Error("main clip is not among the row's rendered objects")
	}
}

// The row shares the body's region layout: the main-band entries translate by
// -offset.X under horizontal scroll while the pinned-left/right entries stay
// fixed (pre-rebrand tracker #63).
func TestFloatingFilter_MainScrollsPinnedFixed(t *testing.T) {
	fake := &fakeFilterAccess{offsetX: 0}
	row, r := layoutFilterRow(fake)

	// Main entries live inside the clip, so their absolute X folds in the clip's
	// own position; pinned entries are positioned directly.
	x0 := func() (pl, mainB, pr float32) {
		return row.entries[0].Position().X, // pinned-left a (direct)
			r.bands.mainClip.Position().X + row.entries[1].Position().X, // main b (clipped)
			row.entries[3].Position().X // pinned-right d (direct)
	}
	plBefore, mainBefore, prBefore := x0()

	// The main band is clipped to [mainX, pinnedRightX) = [84, 252).
	if r.bands.mainClip.Position().X != 84 {
		t.Errorf("main clip X = %v, want 84 (mainX)", r.bands.mainClip.Position().X)
	}
	if got := r.bands.mainClip.Size().Width; got != 252-84 {
		t.Errorf("main clip width = %v, want %v (pinnedRightX-mainX)", got, float32(252-84))
	}

	// Unscrolled sanity: main "b" sits at mainX (84), pinned bands at 0 / 252.
	if plBefore != 0 {
		t.Errorf("pinned-left entry X = %v, want 0", plBefore)
	}
	if mainBefore != 84 {
		t.Errorf("main entry absolute X = %v, want 84 (mainX)", mainBefore)
	}
	if prBefore != 252 {
		t.Errorf("pinned-right entry X = %v, want 252 (pinnedRightX)", prBefore)
	}

	// Scroll the main band by 50 and re-layout.
	fake.offsetX = 50
	r.Layout(geo.NewSize(400, 30))
	plAfter, mainAfter, prAfter := x0()

	if plAfter != plBefore {
		t.Errorf("pinned-left entry moved: %v -> %v (want fixed)", plBefore, plAfter)
	}
	if prAfter != prBefore {
		t.Errorf("pinned-right entry moved: %v -> %v (want fixed)", prBefore, prAfter)
	}
	if got := mainBefore - mainAfter; got != 50 {
		t.Errorf("main entry shifted by %v, want 50 (the scroll offset)", got)
	}
}

// Each entry reports its ABSOLUTE display-column index to OnFloatingFilterChanged,
// so the filter is applied to the column drawn above it even when pinned columns
// reorder the display (pre-rebrand tracker #63). Previously the row passed a main-only index.
func TestFloatingFilter_ReportsAbsoluteColumn(t *testing.T) {
	fake := &fakeFilterAccess{}
	row, _ := layoutFilterRow(fake)

	// entries[3] is the pinned-right column (absolute index 3).
	row.entries[3].SetText("q")
	if fake.lastCol != 3 {
		t.Errorf("OnFloatingFilterChanged col = %d, want 3 (absolute index)", fake.lastCol)
	}
	if fake.lastText != "q" {
		t.Errorf("OnFloatingFilterChanged text = %q, want \"q\"", fake.lastText)
	}
}
