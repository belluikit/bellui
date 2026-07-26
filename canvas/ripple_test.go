package canvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

func TestRipple_New(t *testing.T) {
	c := color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0x80}
	r := canvas.NewRipple(c)

	assert.NotNil(t, r)
	assert.Equal(t, c, r.Color)
	assert.Equal(t, float32(0), r.Radius)
	assert.Equal(t, float32(0), r.Opacity)
}

func TestRipple_Properties(t *testing.T) {
	r := canvas.NewRipple(color.White)
	r.Center = geo.NewPos(10, 20)
	r.Radius = 50
	r.Opacity = 0.5
	r.CornerRadius = 8

	assert.Equal(t, geo.NewPos(10, 20), r.Center)
	assert.Equal(t, float32(50), r.Radius)
	assert.InDelta(t, 0.5, float64(r.Opacity), 0.001)
	assert.Equal(t, float32(8), r.CornerRadius)
}

func TestRipple_Refresh(t *testing.T) {
	r := canvas.NewRipple(color.Black)
	// Refresh should not panic even without being added to a canvas.
	assert.NotPanics(t, func() { r.Refresh() })
}

func TestRipple_Move(t *testing.T) {
	r := canvas.NewRipple(color.White)
	assert.Equal(t, geo.Position{}, r.Position())

	r.Move(geo.NewPos(15, 25))
	assert.Equal(t, geo.NewPos(15, 25), r.Position())
}

func TestRipple_Resize(t *testing.T) {
	r := canvas.NewRipple(color.White)
	assert.Equal(t, geo.Size{}, r.Size())

	r.Resize(geo.NewSize(100, 50))
	assert.Equal(t, geo.NewSize(100, 50), r.Size())
}

func TestRipple_HideShow(t *testing.T) {
	r := canvas.NewRipple(color.White)
	assert.True(t, r.Visible())

	r.Hide()
	assert.False(t, r.Visible())

	r.Show()
	assert.True(t, r.Visible())
}

func TestRipple_MinSize(t *testing.T) {
	r := canvas.NewRipple(color.White)
	// baseObject default MinSize is {1, 1} when no explicit min is set.
	assert.Equal(t, geo.NewSize(1, 1), r.MinSize())
}

func TestRipple_Defaults(t *testing.T) {
	r := canvas.NewRipple(color.NRGBA{R: 0x33, G: 0x66, B: 0x99, A: 0xff})

	assert.Equal(t, float32(0), r.Radius)
	assert.Equal(t, float32(0), r.Opacity)
	assert.Equal(t, float32(0), r.CornerRadius)
	assert.Equal(t, geo.Position{}, r.Center)
}
