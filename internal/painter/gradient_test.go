package painter

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/stretchr/testify/assert"
)

func TestGradientShaderParams(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}
	halfBlue := color.NRGBA{B: 0xff, A: 0x80}

	t.Run("not a gradient", func(t *testing.T) {
		_, _, ok := GradientShaderParams(canvas.NewRectangle(red), 10, 10)
		assert.False(t, ok)
	})
	t.Run("empty image", func(t *testing.T) {
		_, draw, ok := GradientShaderParams(canvas.NewHorizontalGradient(red, red), 0, 10)
		assert.True(t, ok)
		assert.False(t, draw)
	})
	t.Run("no colours", func(t *testing.T) {
		_, draw, ok := GradientShaderParams(&canvas.RadialGradient{}, 10, 10)
		assert.True(t, ok)
		assert.False(t, draw)
	})
	t.Run("start and end", func(t *testing.T) {
		g := canvas.NewLinearGradient(red, nil, 90)
		p, draw, ok := GradientShaderParams(g, 20, 10)
		assert.True(t, ok)
		assert.True(t, draw)
		assert.Equal(t, float32(0), p.Kind)
		assert.Equal(t, [4]float32{-1, 0, 20, 20}, p.Linear)
		assert.Equal(t, 2, p.StopCount)
		assert.Equal(t, float32(1), p.StopPos[1])
		assert.Equal(t, [4]float32{1, 0, 0, 1}, p.StopCol[0])
		assert.Equal(t, [4]float32{}, p.StopCol[1], "a missing end is transparent")
	})
	t.Run("stops sorted, clamped and premultiplied", func(t *testing.T) {
		g := canvas.NewRadialGradient(nil, nil)
		g.CenterOffsetX = -0.25
		g.Stops = []canvas.GradientStop{{Position: 1.5, Color: halfBlue}, {Position: -1, Color: red}, {Position: 0.5, Color: red}}
		p, draw, ok := GradientShaderParams(g, 40, 20)
		assert.True(t, ok)
		assert.True(t, draw)
		assert.Equal(t, float32(1), p.Kind)
		assert.Equal(t, [4]float32{10, 10, 30, 10}, p.Radial)
		assert.Equal(t, 3, p.StopCount)
		assert.Equal(t, [8]float32{0, 0.5, 1}, p.StopPos)
		assert.InDelta(t, 0x80*0x101/float32(0xffff), p.StopCol[2][2], 1e-6, "blue is premultiplied by its alpha")
		assert.Equal(t, p.StopCol[2][2], p.StopCol[2][3])
	})
	t.Run("one stop is padded to two", func(t *testing.T) {
		g := canvas.NewRadialGradient(nil, nil)
		g.Stops = []canvas.GradientStop{{Position: 0.3, Color: red}}
		p, _, ok := GradientShaderParams(g, 10, 10)
		assert.True(t, ok)
		assert.Equal(t, 2, p.StopCount)
		assert.Equal(t, p.StopPos[0], p.StopPos[1])
		assert.Equal(t, p.StopCol[0], p.StopCol[1])
	})
	t.Run("too many stops", func(t *testing.T) {
		g := canvas.NewRadialGradient(nil, nil)
		g.Stops = make([]canvas.GradientStop, MaxGradientStops+1)
		_, _, ok := GradientShaderParams(g, 10, 10)
		assert.False(t, ok)
	})
}
