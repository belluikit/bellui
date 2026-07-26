package widget

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"

	"github.com/stretchr/testify/assert"
)

func TestNewUnderlineDecoration(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}

	t.Run("solid is one stroked line of the given color", func(t *testing.T) {
		objs := newUnderlineDecoration(10, 50, 20, 14, UnderlineSolid, red)
		if assert.Len(t, objs, 1) {
			ln, ok := objs[0].(*canvas.Line)
			assert.True(t, ok, "expected a *canvas.Line")
			assert.Equal(t, red, ln.StrokeColor)
			assert.Greater(t, ln.StrokeWidth, float32(0))
			assert.Equal(t, float32(10), ln.Position1.X)
			assert.Equal(t, float32(50), ln.Position2.X)
			assert.Equal(t, ln.Position1.Y, ln.Position2.Y, "solid underline is horizontal")
		}
	})

	t.Run("wavy is a polyline of multiple colored segments", func(t *testing.T) {
		objs := newUnderlineDecoration(10, 60, 20, 14, UnderlineWavy, red)
		assert.Greater(t, len(objs), 1, "a squiggle is more than one segment")
		var minY, maxY float32
		for i, o := range objs {
			ln, ok := o.(*canvas.Line)
			if !assert.True(t, ok) {
				continue
			}
			assert.Equal(t, red, ln.StrokeColor)
			if i == 0 {
				minY, maxY = ln.Position1.Y, ln.Position1.Y
			}
			for _, y := range []float32{ln.Position1.Y, ln.Position2.Y} {
				minY = min(minY, y)
				maxY = max(maxY, y)
			}
		}
		assert.Greater(t, maxY, minY, "the wave must oscillate vertically")
	})

	t.Run("dotted is a row of marks", func(t *testing.T) {
		objs := newUnderlineDecoration(10, 40, 20, 14, UnderlineDotted, red)
		assert.Greater(t, len(objs), 1)
		for _, o := range objs {
			rect, ok := o.(*canvas.Rectangle)
			if assert.True(t, ok, "expected *canvas.Rectangle dots") {
				assert.Equal(t, red, rect.FillColor)
			}
		}
	})

	t.Run("empty or inverted span yields nothing", func(t *testing.T) {
		assert.Nil(t, newUnderlineDecoration(50, 50, 20, 14, UnderlineSolid, red))
		assert.Nil(t, newUnderlineDecoration(60, 10, 20, 14, UnderlineWavy, red))
	})
}

func TestNewStrikeDecoration(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}

	t.Run("one horizontal stroked line of the given color at midY", func(t *testing.T) {
		objs := newStrikeDecoration(10, 50, 15, 14, red)
		if assert.Len(t, objs, 1) {
			ln, ok := objs[0].(*canvas.Line)
			assert.True(t, ok, "expected a *canvas.Line")
			assert.Equal(t, red, ln.StrokeColor)
			assert.Greater(t, ln.StrokeWidth, float32(0))
			assert.Equal(t, float32(10), ln.Position1.X)
			assert.Equal(t, float32(50), ln.Position2.X)
			assert.Equal(t, float32(15), ln.Position1.Y)
			assert.Equal(t, ln.Position1.Y, ln.Position2.Y, "strike is horizontal")
		}
	})

	t.Run("empty or inverted span yields nothing", func(t *testing.T) {
		assert.Nil(t, newStrikeDecoration(50, 50, 15, 14, red))
		assert.Nil(t, newStrikeDecoration(60, 10, 15, 14, red))
	})
}
