package container_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func TestPathClip_MinSize(t *testing.T) {
	rect := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
	rect.SetMinSize(geo.NewSize(50, 50))

	clipPath := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(50, 0)
		p.LineTo(25, 50)
		p.Close()
	})

	pc := container.NewPathClip(clipPath, rect)
	minSize := pc.MinSize()
	assert.Equal(t, float32(50), minSize.Width)
	assert.Equal(t, float32(50), minSize.Height)
}

func TestPathClip_NilContent(t *testing.T) {
	clipPath := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(10, 0)
		p.LineTo(5, 10)
		p.Close()
	})

	pc := container.NewPathClip(clipPath, nil)
	// Should not panic.
	assert.NotNil(t, pc)
}

func TestPathClip_Render(t *testing.T) {
	rect := canvas.NewRectangle(color.NRGBA{G: 200, A: 255})
	rect.SetMinSize(geo.NewSize(80, 80))

	// Triangle clip path
	clipPath := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 80)
		p.LineTo(40, 0)
		p.LineTo(80, 80)
		p.Close()
	})

	pc := container.NewPathClip(clipPath, rect)
	pc.Resize(geo.NewSize(80, 80))

	// Verify the widget creates a renderer without error.
	r := test.TempWidgetRenderer(t, pc)
	assert.NotNil(t, r)
	assert.Equal(t, 1, len(r.Objects()))
}
