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

func TestPolygon_FillColorSides(t *testing.T) {
	c := color.White
	polygon := canvas.NewPolygon(3, c)

	assert.Equal(t, c, polygon.FillColor)
	assert.Equal(t, uint(3), polygon.Sides)
}

func TestPolygon_SidesRotation(t *testing.T) {
	polygon := &canvas.Polygon{
		FillColor:    color.NRGBA{R: 255, G: 200, B: 0, A: 180},
		StrokeColor:  color.Black,
		StrokeWidth:  2,
		CornerRadius: 5,
		Angle:        30,
		Sides:        3,
	}

	polygon.Resize(geo.NewSize(50, 50))
	test.AssertObjectRendersToMarkup(t, "polygon.xml", polygon)

	c := software.NewCanvas()
	c.SetContent(polygon)
	c.Resize(geo.NewSize(150, 150))
	polygon.Angle = 0
	polygon.CornerRadius = 0
	test.AssertRendersToImage(t, "polygon_stroke.png", c)

	polygon.StrokeWidth = 0
	test.AssertRendersToImage(t, "polygon.png", c)

	polygon.CornerRadius = 5
	polygon.Sides = 4
	polygon.StrokeWidth = 3
	polygon.Angle = 45
	test.AssertRendersToImage(t, "polygon_4_stroke.png", c)

	polygon.CornerRadius = 10
	polygon.Sides = 5
	polygon.Angle = -135
	test.AssertRendersToImage(t, "polygon_5_stroke.png", c)

	polygon.StrokeWidth = 0
	polygon.Sides = 6
	polygon.Angle = -50
	test.AssertRendersToImage(t, "polygon_6.png", c)
}

func TestPolygon_RadiusMaximum(t *testing.T) {
	polygon := &canvas.Polygon{
		FillColor:    color.NRGBA{R: 255, G: 200, B: 0, A: 180},
		StrokeColor:  color.Black,
		CornerRadius: canvas.RadiusMaximum,
		Angle:        30,
		Sides:        3,
	}

	polygon.Resize(geo.NewSize(50, 50))
	test.AssertObjectRendersToMarkup(t, "maximum_rounded_polygon.xml", polygon)

	c := software.NewCanvas()
	c.SetContent(polygon)
	c.Resize(geo.NewSize(150, 150))

	test.AssertRendersToImage(t, "maximum_rounded_polygon.png", c)

	polygon.Sides = 9
	polygon.Angle = -90
	test.AssertRendersToImage(t, "maximum_rounded_polygon.png", c)

	polygon.Sides = 4
	polygon.StrokeWidth = 3
	polygon.Angle = 55
	test.AssertRendersToImage(t, "maximum_rounded_polygon_stroke.png", c)

	polygon.Sides = 7
	polygon.Angle = -115
	test.AssertRendersToImage(t, "maximum_rounded_polygon_stroke.png", c)
}
