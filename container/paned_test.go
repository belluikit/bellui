package container

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

func TestNewHPaned(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(20, 20))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(20, 20))
	c := canvas.NewRectangle(color.Black)
	c.SetMinSize(geo.NewSize(20, 20))

	p := NewHPaned(a, b, c)
	assert.Equal(t, 3, len(p.Panels))
	assert.Equal(t, 3, len(p.Weights))
	assert.True(t, p.Horizontal)
}

func TestNewVPaned(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	b := canvas.NewRectangle(color.Black)

	p := NewVPaned(a, b)
	assert.Equal(t, 2, len(p.Panels))
	assert.False(t, p.Horizontal)
}

func TestPaned_MinSize(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(10, 10))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(10, 10))
	c := canvas.NewRectangle(color.Black)
	c.SetMinSize(geo.NewSize(10, 10))

	t.Run("Horizontal", func(t *testing.T) {
		p := NewHPaned(a, b, c)
		_ = test.WidgetRenderer(p)
		min := p.MinSize()
		divThick := paneDividerThickness(p)
		assert.Equal(t, float32(30)+divThick*2, min.Width) // 3 * 10 + 2 dividers
		assert.Equal(t, float32(10), min.Height)
	})

	t.Run("Vertical", func(t *testing.T) {
		p := NewVPaned(a, b, c)
		_ = test.WidgetRenderer(p)
		min := p.MinSize()
		divThick := paneDividerThickness(p)
		assert.Equal(t, float32(10), min.Width)
		assert.Equal(t, float32(30)+divThick*2, min.Height)
	})
}

func TestPaned_Layout(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(10, 10))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(10, 10))

	p := NewHPaned(a, b)
	r := test.WidgetRenderer(p)

	p.Resize(geo.NewSize(200, 100))
	r.Layout(geo.NewSize(200, 100))

	// Both panels should have non-zero sizes
	assert.Greater(t, a.Size().Width, float32(0))
	assert.Greater(t, b.Size().Width, float32(0))
	assert.Equal(t, float32(100), a.Size().Height)
	assert.Equal(t, float32(100), b.Size().Height)
}

func TestPaned_SetWeight(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(10, 10))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(10, 10))

	p := NewHPaned(a, b)
	assert.Equal(t, 1.0, p.Weights[0])
	assert.Equal(t, 1.0, p.Weights[1])

	p.SetWeight(0, 2.0)
	assert.Equal(t, 2.0, p.Weights[0])
}

func TestPaned_AddPanel(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(10, 10))
	p := NewHPaned(a)

	// Force renderer creation.
	r := test.WidgetRenderer(p)

	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(10, 10))
	p.AddPanel(b)

	assert.Equal(t, 2, len(p.Panels))
	assert.Equal(t, 2, len(p.Weights))

	// Renderer must include both panels AND the new divider.
	objects := r.Objects()
	assert.Equal(t, 3, len(objects), "2 panels + 1 divider")

	// Add a third panel.
	c := canvas.NewRectangle(color.Black)
	c.SetMinSize(geo.NewSize(10, 10))
	p.AddPanel(c)

	objects = r.Objects()
	assert.Equal(t, 5, len(objects), "3 panels + 2 dividers")
}

func TestPaned_RemovePanel(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(10, 10))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(10, 10))
	c := canvas.NewRectangle(color.Black)
	c.SetMinSize(geo.NewSize(10, 10))

	p := NewHPaned(a, b, c)
	r := test.WidgetRenderer(p)

	objects := r.Objects()
	assert.Equal(t, 5, len(objects), "3 panels + 2 dividers before removal")

	p.RemovePanel(1)

	assert.Equal(t, 2, len(p.Panels))
	assert.Equal(t, a, p.Panels[0])
	assert.Equal(t, c, p.Panels[1])

	objects = r.Objects()
	assert.Equal(t, 3, len(objects), "2 panels + 1 divider after removal")
}

func TestPaned_EqualWeights(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(10, 10))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(geo.NewSize(10, 10))

	p := NewHPaned(a, b)
	_ = test.WidgetRenderer(p)
	p.Resize(geo.NewSize(200, 100))

	divThick := paneDividerThickness(p)
	expectedWidth := (200 - divThick) / 2

	// With equal weights, panels should be roughly equal width
	assert.InDelta(t, expectedWidth, a.Size().Width, 1)
	assert.InDelta(t, expectedWidth, b.Size().Width, 1)
}
