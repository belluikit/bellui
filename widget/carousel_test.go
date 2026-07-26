package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestCarousel_New(t *testing.T) {
	test.NewTempApp(t)
	l1 := widget.NewLabel("Page 1")
	l2 := widget.NewLabel("Page 2")
	c := widget.NewCarousel(l1, l2)
	assert.NotNil(t, c)
	assert.Len(t, c.Items, 2)
	assert.Equal(t, 0, c.CurrentItem)
}

func TestCarousel_Next(t *testing.T) {
	test.NewTempApp(t)
	var changed int
	c := widget.NewCarousel(widget.NewLabel("1"), widget.NewLabel("2"), widget.NewLabel("3"))
	c.OnChanged = func(idx int) { changed = idx }

	c.Next()
	assert.Equal(t, 1, c.CurrentItem)
	assert.Equal(t, 1, changed)

	c.Next()
	assert.Equal(t, 2, c.CurrentItem)

	// Should not go beyond last item.
	c.Next()
	assert.Equal(t, 2, c.CurrentItem)
}

func TestCarousel_Previous(t *testing.T) {
	test.NewTempApp(t)
	c := widget.NewCarousel(widget.NewLabel("A"), widget.NewLabel("B"))
	c.Next()
	assert.Equal(t, 1, c.CurrentItem)

	c.Previous()
	assert.Equal(t, 0, c.CurrentItem)

	// Should not go below 0.
	c.Previous()
	assert.Equal(t, 0, c.CurrentItem)
}

func TestCarousel_MinSize(t *testing.T) {
	test.NewTempApp(t)
	c := widget.NewCarousel(widget.NewLabel("Content"))
	min := c.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}
