package canvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

func TestLine_MinSize(t *testing.T) {
	line := canvas.NewLine(color.Black)
	min := line.MinSize()

	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestLine_Move(t *testing.T) {
	line := canvas.NewLine(color.Black)

	line.Resize(geo.NewSize(10, 10))
	assert.Equal(t, geo.NewPos(0, 0), line.Position1)
	assert.Equal(t, geo.NewPos(10, 10), line.Position2)

	line.Move(geo.NewPos(5, 5))
	assert.Equal(t, geo.NewPos(5, 5), line.Position1)
	assert.Equal(t, geo.NewPos(15, 15), line.Position2)

	// rotate
	line.Position1 = geo.NewPos(0, 10)
	line.Position2 = geo.NewPos(10, 0)
	line.Move(geo.NewPos(10, 10))
	assert.Equal(t, geo.NewPos(10, 20), line.Position1)
	assert.Equal(t, geo.NewPos(20, 10), line.Position2)
}

func TestLine_Resize(t *testing.T) {
	line := canvas.NewLine(color.Black)

	line.Resize(geo.NewSize(10, 0))
	size := line.Size()

	assert.Equal(t, float32(10), size.Width)
	assert.Equal(t, float32(0), size.Height)

	// rotate
	line.Position1 = geo.NewPos(0, 10)
	line.Position2 = geo.NewPos(10, 0)
	line.Resize(geo.NewSize(20, 20))
	assert.Equal(t, geo.NewPos(0, 20), line.Position1)
	assert.Equal(t, geo.NewPos(20, 0), line.Position2)
}

func TestLine_StrokeColor(t *testing.T) {
	c := color.White
	line := canvas.NewLine(c)

	assert.Equal(t, c, line.StrokeColor)
}
