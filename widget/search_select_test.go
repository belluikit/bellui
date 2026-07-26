package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/test"
)

func TestSearchSelect_Creation(t *testing.T) {
	s := NewSearchSelect([]string{"A", "B", "C"}, nil)
	assert.NotNil(t, s)
	assert.Equal(t, 3, len(s.Options))
	assert.Equal(t, "", s.Selected)
}

func TestSearchSelect_SetSelected(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana", "Cherry"}, nil)
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetSelected("Banana")
	assert.Equal(t, "Banana", s.Selected)
}

func TestSearchSelect_SetSelectedNotInOptions(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana"}, nil)
	s.SetSelected("Mango")
	assert.Equal(t, "Mango", s.Selected, "SetSelected should accept values not yet in options (e.g. async loading)")
}

func TestSearchSelect_SetSelectedPreferred(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana"}, nil)
	s.Preferred = []string{"Cherry"}
	s.SetSelected("Cherry")
	assert.Equal(t, "Cherry", s.Selected)
}

func TestSearchSelect_OnChanged(t *testing.T) {
	var received string
	s := NewSearchSelect([]string{"A", "B", "C"}, func(val string) {
		received = val
	})
	w := test.NewTempWindow(t, s)
	_ = w

	s.SetSelected("B")
	assert.Equal(t, "B", received)
}

func TestSearchSelect_SetOptions(t *testing.T) {
	s := NewSearchSelect([]string{"A"}, nil)
	s.SetOptions([]string{"X", "Y", "Z"})
	assert.Equal(t, 3, len(s.Options))
	assert.Equal(t, "X", s.Options[0])
}

func TestSearchSelect_FilteredItems_NoFilter(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana", "Cherry"}, nil)
	items := s.filteredItems()
	assert.Equal(t, 3, len(items))
}

func TestSearchSelect_FilteredItems_WithFilter(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana", "Cherry"}, nil)
	s.filter = "an"
	items := s.filteredItems()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "Banana", items[0])
}

func TestSearchSelect_FilteredItems_CaseInsensitive(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "BANANA", "cherry"}, nil)
	s.filter = "AN"
	items := s.filteredItems()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "BANANA", items[0])
}

func TestSearchSelect_FilteredItems_WithPreferred(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana", "Cherry"}, nil)
	s.Preferred = []string{"Cherry", "Apple"}

	items := s.filteredItems()
	// Preferred first, then divider, then remaining options.
	assert.Equal(t, "Cherry", items[0])
	assert.Equal(t, "Apple", items[1])
	assert.Equal(t, "", items[2]) // divider
	assert.Equal(t, "Banana", items[3])
	assert.Equal(t, 4, len(items))
}

func TestSearchSelect_FilteredItems_PreferredFiltered(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana", "Cherry"}, nil)
	s.Preferred = []string{"Cherry", "Apple"}
	s.filter = "ch"

	items := s.filteredItems()
	// Only Cherry matches (from preferred), no divider since no others match.
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "Cherry", items[0])
}

func TestSearchSelect_FilteredItems_PreferredAndOthersFiltered(t *testing.T) {
	s := NewSearchSelect([]string{"Apple", "Banana", "Cherry", "Blueberry"}, nil)
	s.Preferred = []string{"Banana"}
	s.filter = "b"

	items := s.filteredItems()
	// Banana (preferred) + divider + Blueberry (other)
	assert.Equal(t, 3, len(items))
	assert.Equal(t, "Banana", items[0])
	assert.Equal(t, "", items[1]) // divider
	assert.Equal(t, "Blueberry", items[2])
}

func TestSearchSelect_ClearSelected(t *testing.T) {
	s := NewSearchSelect([]string{"A", "B"}, nil)
	s.SetSelected("A")
	assert.Equal(t, "A", s.Selected)
	s.ClearSelected()
	assert.Equal(t, "", s.Selected)
}

func TestSearchSelect_Disable(t *testing.T) {
	s := NewSearchSelect([]string{"A"}, nil)
	w := test.NewTempWindow(t, s)
	_ = w

	s.Disable()
	assert.True(t, s.Disabled())
	s.Enable()
	assert.False(t, s.Disabled())
}

func TestSearchSelect_Popup(t *testing.T) {
	s := NewSearchSelect([]string{"A", "B", "C"}, nil)
	w := test.NewTempWindow(t, s)

	test.Tap(s)
	assert.NotNil(t, w.Canvas().Overlays().Top())

	// Dismiss.
	s.popUp.Hide()
	s.popUp = nil
}
