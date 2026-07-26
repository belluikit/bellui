package canvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

func TestBoxShadow_New(t *testing.T) {
	c := color.NRGBA{R: 0, G: 0, B: 0, A: 0x80}
	s := canvas.NewBoxShadow(c, 8)

	assert.NotNil(t, s)
	assert.Equal(t, c, s.Color)
	assert.Equal(t, float32(8), s.BlurRadius)
}

func TestBoxShadow_Fields(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 4)
	s.OffsetX = 2
	s.OffsetY = 4
	s.CornerRadius = 12

	assert.Equal(t, float32(2), s.OffsetX)
	assert.Equal(t, float32(4), s.OffsetY)
	assert.Equal(t, float32(12), s.CornerRadius)
}

func TestBoxShadow_Refresh(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 6)
	assert.NotPanics(t, func() { s.Refresh() })
}

func TestBoxShadow_Move(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 4)
	assert.Equal(t, geo.Position{}, s.Position())

	s.Move(geo.NewPos(5, 10))
	assert.Equal(t, geo.NewPos(5, 10), s.Position())
}

func TestBoxShadow_Resize(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 4)
	assert.Equal(t, geo.Size{}, s.Size())

	s.Resize(geo.NewSize(200, 100))
	assert.Equal(t, geo.NewSize(200, 100), s.Size())
}

func TestBoxShadow_HideShow(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 4)
	assert.True(t, s.Visible())

	s.Hide()
	assert.False(t, s.Visible())

	s.Show()
	assert.True(t, s.Visible())
}

func TestBoxShadow_Defaults(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 10)

	assert.Equal(t, float32(0), s.OffsetX)
	assert.Equal(t, float32(0), s.OffsetY)
	assert.Equal(t, float32(0), s.CornerRadius)
}

func TestBoxShadow_MinSize(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 4)
	assert.Equal(t, geo.NewSize(1, 1), s.MinSize())
}
