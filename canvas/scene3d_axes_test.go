package canvas

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScene3D_AddAxes_DefaultColors(t *testing.T) {
	s := NewScene3D()
	// nil axis colors fall back to red/green/blue for X/Y/Z respectively.
	s.AddAxes(
		AxisConfig3D{Min: -1, Max: 1},
		AxisConfig3D{Min: -1, Max: 1},
		AxisConfig3D{Min: -1, Max: 1},
	)

	assert.GreaterOrEqual(t, len(s.Meshes), 3)
	// The first three meshes are the X, Y, Z axis lines, in that order.
	assert.Equal(t, color.NRGBA{R: 200, G: 80, B: 80, A: 255}, s.Meshes[0].FillColor)
	assert.Equal(t, color.NRGBA{R: 80, G: 200, B: 80, A: 255}, s.Meshes[1].FillColor)
	assert.Equal(t, color.NRGBA{R: 80, G: 80, B: 200, A: 255}, s.Meshes[2].FillColor)
}

func TestScene3D_AddAxes_MeshCount(t *testing.T) {
	s := NewScene3D()
	// Ticks left at 0 default to 5 per axis.
	s.AddAxes(
		AxisConfig3D{Min: -1, Max: 1},
		AxisConfig3D{Min: -1, Max: 1},
		AxisConfig3D{Min: -1, Max: 1},
	)

	// 3 axis lines + 3*(5+1) ticks + (5+1)*2 grid lines = 3 + 18 + 12 = 33.
	assert.Len(t, s.Meshes, 33)
}

func TestScene3D_AddAxes_CustomColorRespected(t *testing.T) {
	s := NewScene3D()
	custom := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	s.AddAxes(
		AxisConfig3D{Min: -1, Max: 1, Color: custom},
		AxisConfig3D{Min: -1, Max: 1},
		AxisConfig3D{Min: -1, Max: 1},
	)
	// A provided color is not overwritten by the default.
	assert.Equal(t, custom, s.Meshes[0].FillColor)
}

func TestMakeLine3D_Normal(t *testing.T) {
	c := color.NRGBA{R: 255, A: 255}
	m := makeLine3D(Vec3{0, 0, 0}, Vec3{1, 0, 0}, 0.1, c)

	// A non-degenerate line builds a thin box: 8 vertices, 12 triangle faces.
	assert.Len(t, m.Vertices, 8)
	assert.Len(t, m.Faces, 12)
	assert.Equal(t, c, m.FillColor)
}

func TestMakeLine3D_Degenerate(t *testing.T) {
	// Zero-length line (start == end) must be guarded: no geometry produced.
	m := makeLine3D(Vec3{1, 2, 3}, Vec3{1, 2, 3}, 0.1, color.White)
	assert.Empty(t, m.Vertices)
	assert.Empty(t, m.Faces)
}
