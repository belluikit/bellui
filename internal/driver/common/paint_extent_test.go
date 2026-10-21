package common

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

func TestPaintExtent(t *testing.T) {
	test.NewTempApp(t)

	rect := canvas.NewRectangle(color.White)
	rect.Resize(geo.NewSize(30, 20))
	off, size := PaintExtent(rect)
	assert.Equal(t, geo.Position{}, off, "a plain shape draws in its bounds")
	assert.Equal(t, geo.NewSize(30, 20), size)

	// A leading or trailing text wider than its box overflows on the side its
	// direction puts it, which the dirty-region walk does not know, so both
	// sides are covered: one never resized draws at its MinSize from its
	// position, or ending there in RTL.
	text := canvas.NewText("File Transfer", color.White)
	min := text.MinSize()
	off, size = PaintExtent(text)
	assert.Equal(t, geo.NewPos(-min.Width, 0), off)
	assert.Equal(t, geo.NewSize(2*min.Width, min.Height), size, "a text never resized draws at its MinSize")

	// The painters place an overflowing text by its alignment (alignedTextPos):
	// a centred one starts half the overflow left of its box.
	narrow := canvas.NewText("File Transfer", color.White)
	narrow.Alignment = bellui.TextAlignCenter
	narrow.Resize(geo.NewSize(10, min.Height))
	off, size = PaintExtent(narrow)
	assert.Equal(t, geo.NewPos(-(min.Width-10)/2, 0), off, "a centred text overflows both sides evenly")
	assert.Equal(t, min, size)

	narrow.Alignment = bellui.TextAlignTrailing
	off, size = PaintExtent(narrow)
	assert.Equal(t, geo.NewPos(-(min.Width-10), 0), off, "a trailing text overflows its leading side")
	assert.Equal(t, geo.NewSize(10+2*(min.Width-10), min.Height), size)

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

// TestPaintExtent_ContainsSoftwareInk checks PaintExtent against a painter
// rather than against a reading of one: it renders texts in boxes narrower
// than their glyphs, in every alignment and both directions, and asserts every
// inked pixel lies inside the extent. Hand-written expectations shared the
// code's assumption that text starts at its position, so a centred, trailing
// or RTL-leading text drawn left of its box went unnoticed (#602).
func TestPaintExtent_ContainsSoftwareInk(t *testing.T) {
	test.NewTempApp(t)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	for _, dir := range []bellui.Direction{bellui.DirectionLTR, bellui.DirectionRTL} {
		for _, align := range []bellui.TextAlign{bellui.TextAlignLeading, bellui.TextAlignCenter, bellui.TextAlignTrailing} {
			for _, box := range []geo.Size{{}, geo.NewSize(10, 0)} {
				bellui.SetLayoutDirection(dir)
				text := canvas.NewText("File Transfer", color.Black)
				text.Alignment = align
				pos := geo.NewPos(100, 20)
				text.Move(pos)
				text.Resize(box)

				c := test.NewTransparentCanvasWithPainter(software.NewPainter())
				c.SetPadded(false)
				c.SetContent(container.NewWithoutLayout(text))
				c.Resize(geo.NewSize(300, 60))
				img := c.Capture()

				ink := image.Rectangle{}
				b := img.Bounds()
				for y := b.Min.Y; y < b.Max.Y; y++ {
					for x := b.Min.X; x < b.Max.X; x++ {
						if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
							ink = ink.Union(image.Rect(x, y, x+1, y+1))
						}
					}
				}
				name := fmt.Sprintf("dir=%d align=%d box=%v", dir, align, box)
				require.False(t, ink.Empty(), "%s: the text drew nothing, so the check would prove nothing", name)

				// The painters snap the glyph texture to whole pixels, so the
				// extent is compared rounded out to them; the dirty region's
				// partialRepaintPad covers that rounding when painting.
				off, size := PaintExtent(text)
				ext := image.Rect(
					int(math.Floor(float64(pos.X+off.X))), int(math.Floor(float64(pos.Y+off.Y))),
					int(math.Ceil(float64(pos.X+off.X+size.Width))), int(math.Ceil(float64(pos.Y+off.Y+size.Height))),
				)
				assert.True(t, ink.In(ext), "%s: ink %v outside the extent %v", name, ink, ext)
			}
		}
	}
}
