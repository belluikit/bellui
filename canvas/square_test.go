package canvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func TestSquare_FillColor(t *testing.T) {
	c := color.White
	rect := canvas.NewSquare(c)

	assert.Equal(t, c, rect.FillColor)
}

func TestSquare_Radius(t *testing.T) {
	rect := &canvas.Rectangle{
		Aspect:       1,
		FillColor:    color.NRGBA{R: 255, G: 200, B: 0, A: 180},
		StrokeColor:  color.NRGBA{R: 255, G: 120, B: 0, A: 255},
		StrokeWidth:  2.0,
		CornerRadius: 12,
	}

	rect.Resize(geo.NewSize(50, 50))
	test.AssertObjectRendersToMarkup(t, "rounded_square.xml", rect)

	c := software.NewCanvas()
	c.SetContent(rect)
	c.Resize(geo.NewSize(60, 60))
	test.AssertRendersToImage(t, "rounded_rect_stroke.png", c)

	rect.StrokeWidth = 0
	test.AssertRendersToImage(t, "rounded_rect.png", c)
}

func TestSquare_RadiusMaximum(t *testing.T) {
	rect := &canvas.Rectangle{
		Aspect:       1,
		FillColor:    color.NRGBA{R: 255, G: 200, B: 0, A: 180},
		StrokeColor:  color.NRGBA{R: 255, G: 120, B: 0, A: 255},
		StrokeWidth:  2.0,
		CornerRadius: canvas.RadiusMaximum,
	}

	c := software.NewCanvas()
	c.SetContent(rect)
	c.Resize(geo.NewSize(90, 60))
	test.AssertRendersToImage(t, "maximum_rounded_square_stroke.png", c)

	rect.StrokeWidth = 0
	test.AssertRendersToImage(t, "maximum_rounded_square.png", c)

	rect.CornerRadius = 0
	rect.TopRightCornerRadius = 0
	rect.TopLeftCornerRadius = 5
	rect.BottomLeftCornerRadius = 0
	rect.BottomRightCornerRadius = canvas.RadiusMaximum
	test.AssertRendersToImage(t, "maximum_rounded_per_corner_square.png", c)
}
