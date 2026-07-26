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

func TestPath_NewPath(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
		p.LineTo(100, 100)
		p.Close()
	})

	elements := p.Elements()
	assert.Equal(t, 4, len(elements))
	assert.Equal(t, canvas.PathMoveTo, elements[0].Cmd())
	assert.Equal(t, canvas.PathLineTo, elements[1].Cmd())
	assert.Equal(t, canvas.PathLineTo, elements[2].Cmd())
	assert.Equal(t, canvas.PathClose, elements[3].Cmd())
}

func TestPath_Bounds(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(10, 20)
		p.LineTo(50, 5)
		p.LineTo(30, 60)
		p.Close()
	})

	minX, minY, maxX, maxY := p.Bounds()
	assert.Equal(t, float32(10), minX)
	assert.Equal(t, float32(5), minY)
	assert.Equal(t, float32(50), maxX)
	assert.Equal(t, float32(60), maxY)
}

func TestPath_BoundsEmpty(t *testing.T) {
	p := canvas.NewPath(nil)
	minX, minY, maxX, maxY := p.Bounds()
	assert.Equal(t, float32(0), minX)
	assert.Equal(t, float32(0), minY)
	assert.Equal(t, float32(0), maxX)
	assert.Equal(t, float32(0), maxY)
}

func TestPath_Reset(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(10, 10)
	})
	assert.Equal(t, 2, len(p.Elements()))

	p.Reset()
	assert.Equal(t, 0, len(p.Elements()))

	minX, minY, maxX, maxY := p.Bounds()
	assert.Equal(t, float32(0), minX)
	assert.Equal(t, float32(0), minY)
	assert.Equal(t, float32(0), maxX)
	assert.Equal(t, float32(0), maxY)
}

func TestPath_QuadTo(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.QuadTo(50, 0, 50, 50)
	})

	elements := p.Elements()
	assert.Equal(t, 2, len(elements))
	assert.Equal(t, canvas.PathQuadTo, elements[1].Cmd())
	assert.Equal(t, geo.NewPos(50, 0), elements[1].Point(0))
	assert.Equal(t, geo.NewPos(50, 50), elements[1].Point(1))
}

func TestPath_CubicTo(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.CubicTo(10, 0, 20, 50, 30, 50)
	})

	elements := p.Elements()
	assert.Equal(t, 2, len(elements))
	assert.Equal(t, canvas.PathCubicTo, elements[1].Cmd())
	assert.Equal(t, geo.NewPos(10, 0), elements[1].Point(0))
	assert.Equal(t, geo.NewPos(20, 50), elements[1].Point(1))
	assert.Equal(t, geo.NewPos(30, 50), elements[1].Point(2))
}

func TestPath_RenderFill(t *testing.T) {
	// Triangle path.
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 50)
		p.LineTo(25, 0)
		p.LineTo(50, 50)
		p.Close()
	})
	p.FillColor = color.NRGBA{R: 255, A: 255}
	p.Resize(geo.NewSize(50, 50))

	c := software.NewCanvas()
	c.SetContent(p)
	c.Resize(geo.NewSize(70, 70))

	test.AssertRendersToImage(t, "path_fill_triangle.png", c)
}

func TestPath_RenderStroke(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 50)
		p.LineTo(25, 0)
		p.LineTo(50, 50)
		p.Close()
	})
	p.StrokeColor = color.NRGBA{B: 255, A: 255}
	p.StrokeWidth = 2
	p.Resize(geo.NewSize(50, 50))

	c := software.NewCanvas()
	c.SetContent(p)
	c.Resize(geo.NewSize(70, 70))

	test.AssertRendersToImage(t, "path_stroke_triangle.png", c)
}

func TestPath_RenderCurve(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 50)
		p.CubicTo(0, 0, 50, 0, 50, 50)
		p.Close()
	})
	p.FillColor = color.NRGBA{G: 200, A: 255}
	p.StrokeColor = color.Black
	p.StrokeWidth = 2
	p.Resize(geo.NewSize(50, 50))

	c := software.NewCanvas()
	c.SetContent(p)
	c.Resize(geo.NewSize(70, 70))

	test.AssertRendersToImage(t, "path_curve.png", c)
}

func TestPath_RenderDash(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(5, 25)
		p.LineTo(95, 25)
	})
	p.StrokeColor = color.NRGBA{R: 255, A: 255}
	p.StrokeWidth = 3
	p.DashPattern = []float32{8, 4}
	p.Resize(geo.NewSize(100, 50))

	c := software.NewCanvas()
	c.SetContent(p)
	c.Resize(geo.NewSize(120, 70))

	test.AssertRendersToImage(t, "path_dash.png", c)
}

func TestPath_FillRuleWinding(t *testing.T) {
	// Concentric rectangles wound in opposite directions.
	// Winding rule: inner rectangle should be hollow (winding numbers cancel).
	p := canvas.NewPath(func(p *canvas.Path) {
		// Outer rectangle (CCW)
		p.MoveTo(0, 0)
		p.LineTo(50, 0)
		p.LineTo(50, 50)
		p.LineTo(0, 50)
		p.Close()
		// Inner rectangle (CW — opposite winding)
		p.MoveTo(15, 15)
		p.LineTo(15, 35)
		p.LineTo(35, 35)
		p.LineTo(35, 15)
		p.Close()
	})
	p.FillColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
	p.FillRule = canvas.FillRuleWinding
	p.Resize(geo.NewSize(50, 50))

	c := software.NewCanvas()
	c.SetContent(p)
	c.Resize(geo.NewSize(70, 70))

	test.AssertRendersToImage(t, "path_fill_winding.png", c)
}

func TestPath_FillRuleEvenOdd(t *testing.T) {
	// Concentric rectangles wound in the same direction.
	// Even-odd rule: inner rectangle should be hollow regardless of winding.
	p := canvas.NewPath(func(p *canvas.Path) {
		// Outer rectangle (CCW)
		p.MoveTo(0, 0)
		p.LineTo(50, 0)
		p.LineTo(50, 50)
		p.LineTo(0, 50)
		p.Close()
		// Inner rectangle (also CCW — same winding)
		p.MoveTo(15, 15)
		p.LineTo(35, 15)
		p.LineTo(35, 35)
		p.LineTo(15, 35)
		p.Close()
	})
	p.FillColor = color.NRGBA{R: 255, G: 100, B: 0, A: 255}
	p.FillRule = canvas.FillRuleEvenOdd
	p.Resize(geo.NewSize(50, 50))

	c := software.NewCanvas()
	c.SetContent(p)
	c.Resize(geo.NewSize(70, 70))

	test.AssertRendersToImage(t, "path_fill_evenodd.png", c)
}
