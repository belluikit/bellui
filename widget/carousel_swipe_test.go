package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// A swipe pages the carousel and snaps to the neighbouring item; a short drag
// snaps back. #339 found Dragged/DragEnd were empty, so the documented
// snapping carousel could only be paged with its buttons and dots.
func TestCarousel_SwipeSnaps(t *testing.T) {
	test.NewTempApp(t)
	var changed []int
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"), widget.NewLabel("3"))
	c.OnChanged = func(i int) { changed = append(changed, i) }
	w := test.NewTempWindow(t, c)
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 200))
	mid := geo.NewPos(150, 80)

	test.DragSteps(w.Canvas(), mid, -150, 0, 5, nil) // swipe left: next
	assert.Equal(t, 1, c.CurrentItem)

	test.Drag(w.Canvas(), mid, -10, 0) // too short: snaps back
	assert.Equal(t, 1, c.CurrentItem)

	test.Drag(w.Canvas(), mid, 0, -150) // vertical: not a page swipe
	assert.Equal(t, 1, c.CurrentItem)

	test.Drag(w.Canvas(), mid, -150, 0)
	assert.Equal(t, 2, c.CurrentItem)
	test.Drag(w.Canvas(), mid, -150, 0) // past the last item: stays
	assert.Equal(t, 2, c.CurrentItem)

	test.Drag(w.Canvas(), mid, 150, 0) // swipe right: previous
	assert.Equal(t, 1, c.CurrentItem)
	assert.Equal(t, []int{1, 2, 1}, changed)
}

// Under a right-to-left layout the order is mirrored, so a swipe to the right
// advances.
func TestCarousel_SwipeSnapsRTL(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "ar")
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"))
	w := test.NewTempWindow(t, c)
	w.SetPadded(false)
	w.Resize(geo.NewSize(300, 200))

	test.Drag(w.Canvas(), geo.NewPos(150, 80), 150, 0)
	assert.Equal(t, 1, c.CurrentItem)
}
