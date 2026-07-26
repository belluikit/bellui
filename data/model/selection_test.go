package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func identity(s string) string { return s }

func TestSelection_SingleMode(t *testing.T) {
	m := NewSliceModel("a", "b", "c")
	sel := NewSelectionModel[string, string](m, identity)

	sel.Select("a")
	assert.Equal(t, []string{"a"}, sel.Selected())

	sel.Select("b") // single mode: replaces
	assert.Equal(t, []string{"b"}, sel.Selected())
	assert.False(t, sel.IsSelected("a"))
	assert.True(t, sel.IsSelected("b"))

	sel.Toggle("b")
	assert.Equal(t, 0, sel.Len())
}

func TestSelection_MultiMode(t *testing.T) {
	m := NewSliceModel("a", "b", "c", "d")
	sel := NewSelectionModel[string, string](m, identity)
	sel.SetMode(SelectMulti)

	sel.Select("a")
	sel.Select("c")
	sel.Toggle("d")
	assert.ElementsMatch(t, []string{"a", "c", "d"}, sel.Selected())

	sel.Unselect("c")
	assert.ElementsMatch(t, []string{"a", "d"}, sel.Selected())

	// Switching back to single keeps only the most recent.
	sel.SetMode(SelectSingle)
	assert.Equal(t, []string{"d"}, sel.Selected())
}

func TestSelection_SelectRange(t *testing.T) {
	m := NewSliceModel("a", "b", "c", "d", "e")
	sel := NewSelectionModel[string, string](m, identity)
	sel.SetMode(SelectMulti)

	sel.Select("b")    // anchor at index 1
	sel.SelectRange(3) // b..d inclusive
	assert.ElementsMatch(t, []string{"b", "c", "d"}, sel.Selected())
}

// TestSelection_SurvivesFilterRoundTrip is the golden remap case: select an item,
// filter it out, then unfilter — the selection must be intact (identity-keyed,
// not index-keyed).
func TestSelection_SurvivesFilterRoundTrip(t *testing.T) {
	src := NewSliceModel("apple", "banana", "cherry", "date")
	filtered := NewFilterProxy[string](src)
	sel := NewSelectionModel[string, string](filtered, identity)
	sel.SetMode(SelectMulti)

	// Select "banana" (display index 1) and "date" (index 3).
	sel.Select("banana")
	sel.Select("date")
	assert.Equal(t, []int{1, 3}, sel.SelectedIndices())

	// Filter to items containing "a": banana and date survive, indices shift.
	filtered.SetPredicate(func(s string) bool { return strings.Contains(s, "a") })
	assert.Equal(t, []string{"apple", "banana", "date"}, collect[string](filtered))
	assert.Equal(t, []int{1, 2}, sel.SelectedIndices(), "selection remaps to new positions")
	assert.True(t, sel.IsSelected("banana"))

	// Narrow so nothing selected is visible — keys stay selected.
	filtered.SetPredicate(func(s string) bool { return s == "apple" })
	assert.Empty(t, sel.SelectedIndices(), "no selected item currently visible")
	assert.True(t, sel.IsSelected("banana"), "still selected by identity")

	// Relax the filter fully — selection reappears intact.
	filtered.SetPredicate(nil)
	assert.ElementsMatch(t, []string{"banana", "date"}, sel.Selected())
	assert.Equal(t, []int{1, 3}, sel.SelectedIndices(), "back to original positions")
}

// TestSelection_SurvivesResort checks selection identity across a SortProxy
// reorder, and that IndexOf tracks the moved row.
func TestSelection_SurvivesResort(t *testing.T) {
	src := NewSliceModel("banana", "apple", "cherry")
	sorted := NewSortProxy[string](src)
	sel := NewSelectionModel[string, string](sorted, identity)

	sel.Select("cherry")
	assert.Equal(t, 2, sel.IndexOf("cherry")) // source order

	sorted.SetLess(func(a, b string) bool { return a > b }) // reverse
	assert.Equal(t, []string{"cherry", "banana", "apple"}, collect[string](sorted))
	assert.True(t, sel.IsSelected("cherry"))
	assert.Equal(t, 0, sel.IndexOf("cherry"), "IndexOf follows the moved row")
}

func TestSelection_NotifiesAndFiresOnAdd(t *testing.T) {
	m := NewSliceModel("a", "b")
	sel := NewSelectionModel[string, string](m, identity)

	l := &countingListener{}
	sel.AddListener(l)
	assert.Equal(t, 1, l.count(), "fires immediately")

	sel.Select("a")
	sel.Select("b")
	sel.Clear()
	assert.Equal(t, 4, l.count())

	sel.Clear() // already empty: no-op, no notification
	assert.Equal(t, 4, l.count())
}

func TestSelection_IndexOfMissing(t *testing.T) {
	m := NewSliceModel("a", "b")
	sel := NewSelectionModel[string, string](m, identity)
	assert.Equal(t, -1, sel.IndexOf("zzz"))
}
