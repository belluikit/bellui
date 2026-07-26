package canvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func TestTextOnPath_MinSize(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("Hello", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 20)
		p.LineTo(200, 20)
	})

	min := top.MinSize()
	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestTextOnPath_MinSize_Empty(t *testing.T) {
	test.NewTempApp(t)

	// No text
	top := canvas.NewTextOnPath("", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
	})
	assert.Equal(t, geo.Size{}, top.MinSize())

	// No path
	top2 := canvas.NewTextOnPath("Hello", color.Black, nil)
	assert.Equal(t, geo.Size{}, top2.MinSize())
}

func TestTextOnPath_PathBuilding(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(50, 0)
		p.QuadTo(75, 25, 100, 0)
		p.CubicTo(110, -10, 120, 10, 130, 0)
		p.Close()
	})

	elems := top.Elements()
	assert.Len(t, elems, 5)
	assert.Equal(t, canvas.PathMoveTo, elems[0].Cmd())
	assert.Equal(t, canvas.PathLineTo, elems[1].Cmd())
	assert.Equal(t, canvas.PathQuadTo, elems[2].Cmd())
	assert.Equal(t, canvas.PathCubicTo, elems[3].Cmd())
	assert.Equal(t, canvas.PathClose, elems[4].Cmd())
}

func TestTextOnPath_Reset(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
	})
	assert.Len(t, top.Elements(), 2)

	top.Reset()
	assert.Len(t, top.Elements(), 0)
}

func TestTextOnPath_Move(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
	})
	assert.Equal(t, geo.Position{}, top.Position())

	top.Move(geo.NewPos(10, 20))
	assert.Equal(t, geo.NewPos(10, 20), top.Position())
}

func TestTextOnPath_HideShow(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
	})
	assert.True(t, top.Visible())

	top.Hide()
	assert.False(t, top.Visible())

	top.Show()
	assert.True(t, top.Visible())
}

func TestTextOnPath_Render(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("Hello Path", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(10, 30)
		p.LineTo(300, 30)
	})
	top.Resize(geo.NewSize(320, 60))

	w := test.NewWindow(top)
	defer w.Close()
	w.Resize(geo.NewSize(320, 60))

	img := w.Canvas().Capture()
	assert.NotNil(t, img)
}

func TestTextOnPath_RenderCurve(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("Curved", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(10, 50)
		p.QuadTo(100, 0, 200, 50)
	})
	top.Resize(geo.NewSize(220, 80))

	w := test.NewWindow(top)
	defer w.Close()
	w.Resize(geo.NewSize(220, 80))

	img := w.Canvas().Capture()
	assert.NotNil(t, img)
}

func TestTextOnPath_BaselineShift(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("Shifted", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 40)
		p.LineTo(200, 40)
	})
	top.BaselineShift = -10

	w := test.NewWindow(top)
	defer w.Close()
	w.Resize(geo.NewSize(220, 80))

	img := w.Canvas().Capture()
	assert.NotNil(t, img)
}

func TestTextOnPath_Offset(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("Offset", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 20)
		p.LineTo(300, 20)
	})
	top.Offset = 50

	w := test.NewWindow(top)
	defer w.Close()
	w.Resize(geo.NewSize(320, 50))

	img := w.Canvas().Capture()
	assert.NotNil(t, img)
}

func TestTextOnPath_Resize(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
	})
	top.Resize(geo.NewSize(200, 50))
	assert.Equal(t, geo.NewSize(200, 50), top.Size())
}

func TestTextOnPath_SetMinSize_NoOp(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("Hello", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 20)
		p.LineTo(200, 20)
	})
	originalMin := top.MinSize()
	top.SetMinSize(geo.NewSize(999, 999))
	// SetMinSize is a no-op for TextOnPath.
	assert.Equal(t, originalMin, top.MinSize())
}

func TestTextOnPath_RefreshNoPanic(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
	})
	assert.NotPanics(t, func() { top.Refresh() })
}

func TestTextOnPath_NilBuildPath(t *testing.T) {
	top := canvas.NewTextOnPath("test", color.Black, nil)
	assert.Len(t, top.Elements(), 0)
	assert.Equal(t, geo.Size{}, top.MinSize())
}

func TestTextOnPath_BoundsTrackPath(t *testing.T) {
	test.NewTempApp(t)

	top := canvas.NewTextOnPath("X", color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(10, 20)
		p.LineTo(110, 20)
		p.LineTo(110, 80)
	})

	min := top.MinSize()
	// Path spans 100 wide (10→110) and 60 tall (20→80), plus text padding.
	assert.Greater(t, min.Width, float32(100))
	assert.Greater(t, min.Height, float32(60))
}
