package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func TestPaintExtent(t *testing.T) {
	test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)
	rect.Resize(geo.NewSize(30, 20))
	off, size := PaintExtent(rect)
	assert.Equal(t, geo.Position{}, off, "a plain shape draws in its bounds")
	assert.Equal(t, geo.NewSize(30, 20), size)

	text := canvas.NewText("File Transfer", color.White)
	min := text.MinSize()
	off, size = PaintExtent(text)
	assert.Equal(t, geo.Position{}, off)
	assert.Equal(t, min, size, "a text never resized draws at its MinSize")

	text.Resize(geo.NewSize(min.Width+50, min.Height+10))
	_, size = PaintExtent(text)
	assert.Equal(t, geo.NewSize(min.Width+50, min.Height+10), size, "a larger box contains the glyphs")

	circle := canvas.NewCircle(color.White)
	circle.StrokeColor = color.Black
	circle.StrokeWidth = 4
	circle.Resize(geo.NewSize(20, 20))
	off, size = PaintExtent(circle)
	assert.Equal(t, geo.NewPos(-6, -6), off, "a stroke overflows by the painters' VectorPad")
	assert.Equal(t, geo.NewSize(32, 32), size)

	shadow := canvas.NewBoxShadow(color.Black, 5)
	shadow.OffsetY = 4
	shadow.Resize(geo.NewSize(20, 20))
	off, size = PaintExtent(shadow)
	assert.Equal(t, geo.NewPos(-10, -6), off, "a shadow's blur spreads around its offset box")
	assert.Equal(t, geo.NewSize(40, 40), size)
}

// TestPartialDirtyRegion_TextOutsideItsBox pins #602: a text positioned but
// never resized, as the navigation drawer's captions were, still draws its
// glyphs, so refreshing it must clear and redraw where they are.
func TestPartialDirtyRegion_TextOutsideItsBox(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("File Transfer", color.White)
	text.Move(geo.NewPos(200, 200))
	content := container.NewWithoutLayout(text)
	content.Resize(geo.NewSize(400, 400))
	c := newPartialCanvas(content, geo.NewSize(400, 400))
	c.PartialDirtyRegion()

	text.Text = "Files"
	text.Refresh()
	min := canvas.NewText("File Transfer", color.White).MinSize()
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(200))
	assert.LessOrEqual(t, pos.Y, float32(200))
	assert.GreaterOrEqual(t, pos.X+sz.Width, 200+min.Width, "the old glyphs' width is cleared")
	assert.GreaterOrEqual(t, pos.Y+sz.Height, 200+min.Height, "and their height")
}
