package widget_test

import (
	"sort"
	"strconv"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

type sortPerson struct {
	name string
	age  int
}

// display returns the rows in current display order of the proxy, as name/age
// pairs, so tests can assert the effective sort.
func displayNames(m model.Model[sortPerson]) []string {
	out := make([]string, m.Len())
	for i := 0; i < m.Len(); i++ {
		out[i] = m.Item(i).name
	}
	return out
}

func displayAges(m model.Model[sortPerson]) []int {
	out := make([]int, m.Len())
	for i := 0; i < m.Len(); i++ {
		out[i] = m.Item(i).age
	}
	return out
}

func nameCol() widget.SortableColumn[sortPerson] {
	return widget.SortableColumn[sortPerson]{Title: "Name", Less: func(a, b sortPerson) bool { return a.name < b.name }}
}

func ageCol() widget.SortableColumn[sortPerson] {
	return widget.SortableColumn[sortPerson]{Title: "Age", Less: func(a, b sortPerson) bool { return a.age < b.age }}
}

func newSortFixture() (*model.SortProxy[sortPerson], *widget.Table, *widget.TableSorter[sortPerson]) {
	src := model.NewSliceModel(
		sortPerson{"Bo", 30},
		sortPerson{"Ada", 30},
		sortPerson{"Cy", 20},
	)
	proxy := model.NewSortProxy[sortPerson](src)
	table := widget.NewTable(nil, nil, nil)
	widget.BindTableModel[sortPerson](
		table, proxy, 2,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(p sortPerson, col int, cell core.CanvasObject) {
			if col == 0 {
				cell.(*widget.Label).SetText(p.name)
			} else {
				cell.(*widget.Label).SetText(strconv.Itoa(p.age))
			}
		},
	)
	sorter := widget.BindTableSort[sortPerson](table, proxy, []widget.SortableColumn[sortPerson]{nameCol(), ageCol()})
	return proxy, table, sorter
}

func TestTableSorter_SingleColumnToggleCycle(t *testing.T) {
	test.NewTempApp(t)
	proxy, _, sorter := newSortFixture()

	// Initial: source order, no active sort.
	assert.Equal(t, []string{"Bo", "Ada", "Cy"}, displayNames(proxy))
	assert.Empty(t, sorter.Sorts())

	tests := []struct {
		name    string
		wantDir widget.SortDirection
		want    []string
	}{
		{"ascending", widget.SortAscending, []string{"Ada", "Bo", "Cy"}},
		{"descending", widget.SortDescending, []string{"Cy", "Bo", "Ada"}},
		{"none restores source order", widget.SortNone, []string{"Bo", "Ada", "Cy"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sorter.SortByColumn(0)
			assert.Equal(t, tc.wantDir, sorter.DirectionOf(0))
			assert.Equal(t, tc.want, displayNames(proxy))
			if tc.wantDir == widget.SortNone {
				assert.Empty(t, sorter.Sorts())
			}
		})
	}
}

func TestTableSorter_MultiColumnStableBThenA(t *testing.T) {
	test.NewTempApp(t)
	proxy, _, sorter := newSortFixture()

	// Click column A (name) then column B (age): rows sort by B then A.
	sorter.SortByColumn(0) // name asc: Ada(30), Bo(30), Cy(20)
	sorter.SortByColumn(1) // age asc primary, name asc tie-break

	// Primary must be the most recently clicked column (age).
	got := sorter.Sorts()
	assert.Equal(t, []widget.ColumnSort{
		{Column: 1, Direction: widget.SortAscending},
		{Column: 0, Direction: widget.SortAscending},
	}, got)

	// Sorted by age asc; the two age-30 rows keep name-asc order (Ada before Bo).
	assert.Equal(t, []int{20, 30, 30}, displayAges(proxy))
	assert.Equal(t, []string{"Cy", "Ada", "Bo"}, displayNames(proxy))
}

func TestTableSorter_ReclickingColumnRepromotesAndTogglesDirection(t *testing.T) {
	test.NewTempApp(t)
	proxy, _, sorter := newSortFixture()

	sorter.SortByColumn(1) // age asc (primary)
	sorter.SortByColumn(0) // name asc (primary), age secondary
	assert.Equal(t, []widget.ColumnSort{
		{Column: 0, Direction: widget.SortAscending},
		{Column: 1, Direction: widget.SortAscending},
	}, sorter.Sorts())

	// Re-click age: it toggles to descending AND becomes primary again.
	sorter.SortByColumn(1)
	assert.Equal(t, []widget.ColumnSort{
		{Column: 1, Direction: widget.SortDescending},
		{Column: 0, Direction: widget.SortAscending},
	}, sorter.Sorts())
	// age desc primary; the age-30 pair (Ada, Bo) come first, name-asc within.
	assert.Equal(t, []int{30, 30, 20}, displayAges(proxy))
	assert.Equal(t, []string{"Ada", "Bo", "Cy"}, displayNames(proxy))
}

func TestTableSorter_ComparatorRecomputedOnEachClick(t *testing.T) {
	test.NewTempApp(t)
	proxy, _, sorter := newSortFixture()

	sorter.SortByColumn(0)
	assert.Equal(t, []string{"Ada", "Bo", "Cy"}, displayNames(proxy)) // name asc
	sorter.SortByColumn(0)
	assert.Equal(t, []string{"Cy", "Bo", "Ada"}, displayNames(proxy)) // name desc

	// Clear removes all keys and restores source order.
	sorter.Clear()
	assert.Empty(t, sorter.Sorts())
	assert.Equal(t, []string{"Bo", "Ada", "Cy"}, displayNames(proxy))
}

func TestTableSorter_NilLessColumnIsInert(t *testing.T) {
	test.NewTempApp(t)
	src := model.NewSliceModel(sortPerson{"Bo", 30}, sortPerson{"Ada", 20})
	proxy := model.NewSortProxy[sortPerson](src)
	table := widget.NewTable(nil, nil, nil)
	sorter := widget.BindTableSort[sortPerson](table, proxy, []widget.SortableColumn[sortPerson]{
		{Title: "Static"}, // nil Less
		ageCol(),
	})

	sorter.SortByColumn(0) // inert
	assert.Empty(t, sorter.Sorts())
	assert.Equal(t, []string{"Bo", "Ada"}, displayNames(proxy))

	sorter.SortByColumn(1) // works
	assert.Equal(t, widget.SortAscending, sorter.DirectionOf(1))
	assert.Equal(t, []int{20, 30}, displayAges(proxy))

	sorter.SortByColumn(9) // out of range, ignored
	assert.Len(t, sorter.Sorts(), 1)
}

// TestTableSorter_HeaderClickSortsViaWidget exercises the CreateHeader/
// UpdateHeader hooks the Table uses: a real header widget, bound to a column,
// sorts when tapped as event.Tappable.
func TestTableSorter_HeaderClickSortsViaWidget(t *testing.T) {
	test.NewTempApp(t)
	proxy, table, sorter := newSortFixture()

	// Build a header cell and bind it to column 0 (name) via the Table's hooks.
	hdr := table.CreateHeader()
	table.UpdateHeader(widget.TableCellID{Row: -1, Col: 0}, hdr)

	tappable, ok := hdr.(event.Tappable)
	assert.True(t, ok, "sort header must be Tappable so it receives header clicks")

	test.Tap(tappable)
	assert.Equal(t, widget.SortAscending, sorter.DirectionOf(0))
	assert.Equal(t, []string{"Ada", "Bo", "Cy"}, displayNames(proxy))

	test.Tap(tappable)
	assert.Equal(t, widget.SortDescending, sorter.DirectionOf(0))
	assert.Equal(t, []string{"Cy", "Bo", "Ada"}, displayNames(proxy))

	// A header not bound to a sortable column (row-number header) is inert.
	rowHdr := table.CreateHeader()
	table.UpdateHeader(widget.TableCellID{Row: 3, Col: -1}, rowHdr)
	test.Tap(rowHdr.(event.Tappable))
	assert.Equal(t, widget.SortDescending, sorter.DirectionOf(0), "unrelated header tap must not change sort")
}

func TestTableSorter_StringLessLocaleAware(t *testing.T) {
	test.NewTempApp(t)
	src := model.NewSliceModel(
		sortPerson{"cherry", 0}, sortPerson{"apple", 0}, sortPerson{"Banana", 0},
	)
	proxy := model.NewSortProxy[sortPerson](src)
	table := widget.NewTable(nil, nil, nil)
	sorter := widget.BindTableSort[sortPerson](table, proxy, []widget.SortableColumn[sortPerson]{
		{Title: "Name", Less: func(a, b sortPerson) bool { return model.StringLess(a.name, b.name) }},
	})

	sorter.SortByColumn(0)
	got := displayNames(proxy)

	// Collation is case-insensitive at the primary level, so Banana sorts near
	// b, not after all lower-case (as a raw code-point sort would order it).
	want := append([]string(nil), got...)
	sort.Slice(want, func(i, j int) bool { return model.StringLess(want[i], want[j]) })
	assert.Equal(t, want, got)
	assert.Equal(t, "apple", got[0])
}
