package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestChip_New(t *testing.T) {
	test.NewTempApp(t)
	c := widget.NewChip("Tag", nil)
	assert.NotNil(t, c)
	assert.Equal(t, "Tag", c.Text)
	assert.False(t, c.Selected)
}

func TestChip_MinSize(t *testing.T) {
	test.NewTempApp(t)
	c := widget.NewChip("Test", nil)
	min := c.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestFilterChip_Toggle(t *testing.T) {
	test.NewTempApp(t)
	var lastVal bool
	c := widget.NewFilterChip("Filter", func(sel bool) { lastVal = sel })

	assert.False(t, c.Selected)

	test.Tap(c)
	assert.True(t, c.Selected)
	assert.True(t, lastVal)

	test.Tap(c)
	assert.False(t, c.Selected)
	assert.False(t, lastVal)
}

func TestChip_Tapped(t *testing.T) {
	test.NewTempApp(t)
	tapped := false
	c := widget.NewChip("Action", func() { tapped = true })

	test.Tap(c)
	assert.True(t, tapped)
}

func newInputChip(text string, tapped, deleted *bool) *widget.Chip {
	c := widget.NewChip(text, func() { *tapped = true })
	c.Style = widget.ChipStyleInput
	c.OnDeleted = func() { *deleted = true }
	c.Resize(c.MinSize())
	return c
}

func TestInputChip_DeleteButtonTap(t *testing.T) {
	test.NewTempApp(t)
	var tapped, deleted bool
	c := newInputChip("Tag", &tapped, &deleted)

	// A tap on the trailing delete (X) region fires OnDeleted, not OnTapped.
	test.TapAt(c, geo.NewPos(c.Size().Width-3, c.Size().Height/2))
	assert.True(t, deleted, "tapping the X fires OnDeleted")
	assert.False(t, tapped, "tapping the X must not fire OnTapped")
}

func TestInputChip_BodyTap(t *testing.T) {
	test.NewTempApp(t)
	var tapped, deleted bool
	c := newInputChip("Tag", &tapped, &deleted)

	// A tap on the chip body (away from the X) fires the normal OnTapped.
	test.TapAt(c, geo.NewPos(3, c.Size().Height/2))
	assert.True(t, tapped)
	assert.False(t, deleted)
}
