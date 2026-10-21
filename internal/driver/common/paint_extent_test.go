package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
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
