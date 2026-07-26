package canvas_test

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

func TestBackdropBlur_New(t *testing.T) {
	b := canvas.NewBackdropBlur(10)

	assert.NotNil(t, b)
	assert.Equal(t, float32(10), b.BlurRadius)
}

func TestBackdropBlur_MinSize(t *testing.T) {
	b := canvas.NewBackdropBlur(5)
	min := b.MinSize()

	assert.Equal(t, geo.NewSize(0, 0), min, "BackdropBlur should have zero MinSize")
}

func TestBackdropBlur_Refresh(t *testing.T) {
	b := canvas.NewBackdropBlur(20)
	assert.NotPanics(t, func() { b.Refresh() })
}

func TestBackdropBlur_Move(t *testing.T) {
	b := canvas.NewBackdropBlur(5)
	assert.Equal(t, geo.Position{}, b.Position())

	b.Move(geo.NewPos(30, 40))
	assert.Equal(t, geo.NewPos(30, 40), b.Position())
}

func TestBackdropBlur_Resize(t *testing.T) {
	b := canvas.NewBackdropBlur(5)
	assert.Equal(t, geo.Size{}, b.Size())

	b.Resize(geo.NewSize(150, 75))
	assert.Equal(t, geo.NewSize(150, 75), b.Size())
}

func TestBackdropBlur_HideShow(t *testing.T) {
	b := canvas.NewBackdropBlur(5)
	assert.True(t, b.Visible())

	b.Hide()
	assert.False(t, b.Visible())

	b.Show()
	assert.True(t, b.Visible())
}

func TestBackdropBlur_BlurRadius(t *testing.T) {
	b := canvas.NewBackdropBlur(15)
	assert.Equal(t, float32(15), b.BlurRadius)

	b.BlurRadius = 30
	assert.Equal(t, float32(30), b.BlurRadius)
}

func TestBackdropBlur_MinSize_AlwaysZero(t *testing.T) {
	b := canvas.NewBackdropBlur(10)
	b.SetMinSize(geo.NewSize(100, 100))
	// MinSize is overridden to always return zero regardless of SetMinSize.
	assert.Equal(t, geo.NewSize(0, 0), b.MinSize())
}
