package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

// visibleSelectionRects counts the non-empty selection rectangles built by the
// selectable renderer of e.
func visibleSelectionRects(t *testing.T, e *Entry) int {
	t.Helper()
	r := cache.Renderer(e.sel).(*selectableRenderer)
	r.buildSelection()
	count := 0
	for _, o := range r.selections {
		if o.Size().Width > 0 {
			count++
		}
	}
	return count
}

// TestSelectable_RTLMultiRect verifies that a logically-contiguous selection
// straddling a direction boundary is drawn as multiple disjoint rectangles,
// while a pure-LTR selection stays a single rectangle.
func TestSelectable_RTLMultiRect(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	test.NewTempApp(t)

	e := NewEntry()
	e.SetText("abc مرحبا def")
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(320, 80))
	e.Resize(e.MinSize().Max(geo.NewSize(260, 0)))
	w.Canvas().Focus(e)

	// Select logical [2,5): 'c', the space, and only the first Arabic letter,
	// which (reading right-to-left) sits at the far right of the Arabic block —
	// so the selection is visually disjoint.
	e.Select(0, 2, 0, 5)
	assert.Greater(t, visibleSelectionRects(t, e), 1,
		"partial selection crossing a direction boundary should produce multiple rectangles")

	// A pure-LTR selection on the same widget is a single rectangle.
	e.SetText("abcdefghij")
	e.Select(0, 1, 0, 6)
	assert.Equal(t, 1, visibleSelectionRects(t, e),
		"pure-LTR selection should be a single rectangle")
}
