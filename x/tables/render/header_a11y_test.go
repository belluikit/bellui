package render

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// fakeHeaderAccess is a minimal HeaderAccess with the same fixed region layout
// as fakeFilterAccess: one pinned-left column, two main, one pinned-right (all
// width 80, padding 4). Absolute display columns 0..3.
type fakeHeaderAccess struct {
	offsetX     float32
	sort        map[int]SortIndicator
	notSortable map[int]bool // columns whose header tap would not sort
	tapped      []int        // columns passed to OnHeaderTapped
}

func (f *fakeHeaderAccess) HeaderColumnCount() int        { return 4 }
func (f *fakeHeaderAccess) HeaderColumnWidths() []float32 { return []float32{80, 80, 80, 80} }
func (f *fakeHeaderAccess) CellPadding() float32          { return 4 }
func (f *fakeHeaderAccess) PinnedLeftCount() int          { return 1 }
func (f *fakeHeaderAccess) PinnedRightCount() int         { return 1 }
func (f *fakeHeaderAccess) ScrollOffsetX() float32        { return f.offsetX }

// mainX = 80+4 = 84; mainColsW = 2*(80+4) = 168; pinnedRightX = 84+168 = 252.
func (f *fakeHeaderAccess) HeaderRegions() (mainX, pinnedRightX, pinnedRightW float32) {
	return 84, 252, 80
}

func (f *fakeHeaderAccess) ColumnHeader(col int) string { return fmt.Sprintf("H%d", col) }
func (f *fakeHeaderAccess) HeaderHeight() float32       { return 32 }
func (f *fakeHeaderAccess) SortState(col int) SortIndicator {
	return f.sort[col]
}
func (f *fakeHeaderAccess) ColumnSortable(col int) bool { return !f.notSortable[col] }
func (f *fakeHeaderAccess) IsFilterActive(col int) bool { return false }
func (f *fakeHeaderAccess) OnHeaderTapped(col int, modifier event.KeyModifier) {
	f.tapped = append(f.tapped, col)
}
func (f *fakeHeaderAccess) OnResizeDragStart(col int, startX float32) {}
func (f *fakeHeaderAccess) OnResizeDrag(posX float32)                 {}
func (f *fakeHeaderAccess) OnResizeDragEnd()                          {}
func (f *fakeHeaderAccess) HeaderBorderCol(x float32) int             { return -1 }
func (f *fakeHeaderAccess) ColumnAtX(x float32) int                   { return -1 }

// The header-cell node reads as a heading carrying the column title, its
// description tracks the sort state, and the "press" (sort) action is declared
// only for sortable columns and routes through OnHeaderTapped.
func TestHeaderCellA11y_Semantics(t *testing.T) {
	fake := &fakeHeaderAccess{}
	n := newHeaderCellA11y(fake)

	n.set(2, "Region", SortIndicatorNone, true)
	if n.AccessibleRole() != core.RoleHeading {
		t.Errorf("role = %v, want RoleHeading", n.AccessibleRole())
	}
	if n.AccessibleLabel() != "Region" {
		t.Errorf("label = %q, want %q", n.AccessibleLabel(), "Region")
	}
	if got := n.AccessibleDescription(); got != "Column header" {
		t.Errorf("unsorted description = %q, want %q", got, "Column header")
	}

	n.set(2, "Region", SortIndicatorAsc, true)
	if got := n.AccessibleDescription(); got != "Column header, sorted ascending" {
		t.Errorf("asc description = %q", got)
	}
	n.set(2, "Region", SortIndicatorDesc, true)
	if got := n.AccessibleDescription(); got != "Column header, sorted descending" {
		t.Errorf("desc description = %q", got)
	}

	// Sortable: one press action, routed to OnHeaderTapped with the column.
	acts := n.AccessibleActions()
	if len(acts) != 1 || acts[0].Name != core.ActionPress {
		t.Fatalf("actions = %v, want single press", acts)
	}
	if acts[0].Description != "Sort by Region" {
		t.Errorf("action description = %q, want %q", acts[0].Description, "Sort by Region")
	}
	n.PerformAccessibleAction(core.ActionPress)
	if len(fake.tapped) != 1 || fake.tapped[0] != 2 {
		t.Fatalf("press routed to %v, want [2]", fake.tapped)
	}

	// Not sortable: no actions, and press is a no-op.
	n.set(2, "Region", SortIndicatorNone, false)
	if acts := n.AccessibleActions(); len(acts) != 0 {
		t.Fatalf("non-sortable actions = %v, want none", acts)
	}
	n.PerformAccessibleAction(core.ActionPress)
	if len(fake.tapped) != 1 {
		t.Errorf("press on non-sortable column must not tap: %v", fake.tapped)
	}
}

// The header renderer realizes one accessible node per VISIBLE column (pinned +
// main + right) covering its header cell: main-band nodes live inside the clip
// (so they translate and clip with their column), pinned nodes sit directly on
// the row (#109). Before this the header row published no semantic nodes at all.
func TestHeaderRenderer_A11yNodePerVisibleColumn(t *testing.T) {
	fake := &fakeHeaderAccess{sort: map[int]SortIndicator{1: SortIndicatorAsc}}
	h := NewHeaderWidget(fake, &Viewport{})
	r := h.CreateRenderer().(*headerRenderer)
	r.Layout(geo.NewSize(400, 32))

	if len(r.a11y) < 4 {
		t.Fatalf("realized %d header nodes, want 4 (all visible columns)", len(r.a11y))
	}
	for i := 0; i < 4; i++ {
		n := r.a11y[i]
		if want := fmt.Sprintf("H%d", i); n.AccessibleLabel() != want {
			t.Errorf("node %d label = %q, want %q", i, n.AccessibleLabel(), want)
		}
		if n.Size().Width != 80 || n.Size().Height != 32 {
			t.Errorf("node %d size = %v, want 80x32", i, n.Size())
		}
	}
	// The sorted column's node announces its direction.
	if got := r.a11y[1].AccessibleDescription(); got != "Column header, sorted ascending" {
		t.Errorf("sorted node description = %q", got)
	}

	// Main columns 1,2 inside the clip band; pinned 0,3 directly on the row.
	band := map[any]bool{}
	for _, o := range r.bands.mainBand.Objects {
		band[o] = true
	}
	for _, i := range []int{1, 2} {
		if !band[r.a11y[i]] {
			t.Errorf("main node %d is not inside the clip band", i)
		}
	}
	for _, i := range []int{0, 3} {
		if band[r.a11y[i]] {
			t.Errorf("pinned node %d wrongly placed inside the clip band", i)
		}
	}
}
