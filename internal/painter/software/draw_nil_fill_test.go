package software_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPainter_paintRectangleWithNilFillColor(t *testing.T) {
	rect := canvas.NewRectangle(nil)
	rect.Resize(geo.NewSize(10, 10))

	c := test.NewTransparentCanvasWithPainter(software.NewPainter())
	c.SetPadded(false)
	c.SetContent(rect)
	c.Resize(geo.NewSize(10, 10))

	var img image.Image
	require.NotPanics(t, func() { img = c.Capture() })
	require.NotNil(t, img)
	assert.Equal(t, color.NRGBA{}, color.NRGBAModel.Convert(img.At(5, 5)))
}
