package painter

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

func TestFlattenPath_Lines(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(10, 0)
		p.LineTo(10, 10)
		p.Close()
	})

	subpaths := FlattenPath(p.Elements(), 1.0)
	assert.Equal(t, 1, len(subpaths))
	assert.Equal(t, 4, len(subpaths[0])) // 3 points + close
	assert.Equal(t, geo.NewPos(0, 0), subpaths[0][0])
	assert.Equal(t, geo.NewPos(10, 0), subpaths[0][1])
	assert.Equal(t, geo.NewPos(10, 10), subpaths[0][2])
	assert.Equal(t, geo.NewPos(0, 0), subpaths[0][3])
}

func TestFlattenPath_MultipleSubpaths(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(10, 10)
		p.MoveTo(20, 20)
		p.LineTo(30, 30)
	})

	subpaths := FlattenPath(p.Elements(), 1.0)
	assert.Equal(t, 2, len(subpaths))
	assert.Equal(t, 2, len(subpaths[0]))
	assert.Equal(t, 2, len(subpaths[1]))
}

func TestFlattenPath_QuadBezier(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.QuadTo(50, 0, 50, 50)
	})

	subpaths := FlattenPath(p.Elements(), 1.0)
	assert.Equal(t, 1, len(subpaths))
	// Should have more than 2 points (flattened curve).
	assert.Greater(t, len(subpaths[0]), 2)
	// First point should be the moveTo.
	assert.Equal(t, geo.NewPos(0, 0), subpaths[0][0])
	// Last point should be the quadTo endpoint.
	last := subpaths[0][len(subpaths[0])-1]
	assert.Equal(t, geo.NewPos(50, 50), last)
}

func TestFlattenPath_CubicBezier(t *testing.T) {
	p := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.CubicTo(0, 50, 50, 50, 50, 0)
	})

	subpaths := FlattenPath(p.Elements(), 1.0)
	assert.Equal(t, 1, len(subpaths))
	assert.Greater(t, len(subpaths[0]), 2)
	assert.Equal(t, geo.NewPos(0, 0), subpaths[0][0])
	last := subpaths[0][len(subpaths[0])-1]
	assert.Equal(t, geo.NewPos(50, 0), last)
}

func TestTriangulateFill_Triangle(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(5, 10),
			geo.NewPos(0, 0), // closed
		},
	}

	tris := TriangulateFill(subpaths)
	// A triangle should produce exactly 1 triangle = 6 floats.
	assert.Equal(t, 6, len(tris))
}

func TestTriangulateFill_Square(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(10, 10),
			geo.NewPos(0, 10),
			geo.NewPos(0, 0), // closed
		},
	}

	tris := TriangulateFill(subpaths)
	// A square should produce 2 triangles = 12 floats.
	assert.Equal(t, 12, len(tris))
}

func TestTriangulateFill_ConcaveL(t *testing.T) {
	// An L-shaped polygon (concave).
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(10, 5),
			geo.NewPos(5, 5),
			geo.NewPos(5, 10),
			geo.NewPos(0, 10),
			geo.NewPos(0, 0), // closed
		},
	}

	tris := TriangulateFill(subpaths)
	// 6 vertices = 4 triangles = 24 floats.
	assert.Equal(t, 24, len(tris))
}

func TestExpandStroke_SimpleLine(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
		},
	}

	tris := ExpandStroke(subpaths, 2, canvas.LineCapButt, canvas.LineJoinMiter, 4, nil, 0)
	// A line with butt caps: 1 segment quad = 2 triangles = 12 floats.
	assert.Equal(t, 12, len(tris))
}

func TestExpandStroke_RoundCaps(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
		},
	}

	tris := ExpandStroke(subpaths, 2, canvas.LineCapRound, canvas.LineJoinMiter, 4, nil, 0)
	// Should have more geometry than butt caps due to semicircle caps.
	assert.Greater(t, len(tris), 12)
}

func TestExpandStroke_DashPattern(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(100, 0),
		},
	}

	tris := ExpandStroke(subpaths, 2, canvas.LineCapButt, canvas.LineJoinMiter, 4, []float32{10, 5}, 0)
	// Should produce multiple dash segments.
	assert.Greater(t, len(tris), 12)
}

func TestApplyDashPattern(t *testing.T) {
	pts := []geo.Position{
		geo.NewPos(0, 0),
		geo.NewPos(30, 0),
	}

	segments := applyDashPattern(pts, []float32{10, 5}, 0)
	// 30 length, pattern 10+5=15: dash[0..10], gap[10..15], dash[15..25], gap[25..30]
	assert.Equal(t, 2, len(segments))
}

func TestFanTriangulate_Triangle(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(5, 10),
			geo.NewPos(0, 0), // closed
		},
	}

	tris := FanTriangulate(subpaths)
	// Triangle fan from vertex 0: 1 triangle = 6 floats.
	assert.Equal(t, 6, len(tris))
}

func TestFanTriangulate_Square(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(10, 10),
			geo.NewPos(0, 10),
			geo.NewPos(0, 0), // closed
		},
	}

	tris := FanTriangulate(subpaths)
	// Square: fan from vertex 0 produces 2 triangles = 12 floats.
	assert.Equal(t, 12, len(tris))
	// First triangle: (0,0)-(10,0)-(10,10)
	assert.Equal(t, float32(0), tris[0])
	assert.Equal(t, float32(0), tris[1])
	assert.Equal(t, float32(10), tris[2])
	assert.Equal(t, float32(0), tris[3])
	assert.Equal(t, float32(10), tris[4])
	assert.Equal(t, float32(10), tris[5])
}

func TestFanTriangulate_MultipleSubpaths(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(5, 10),
			geo.NewPos(0, 0),
		},
		{
			geo.NewPos(20, 20),
			geo.NewPos(30, 20),
			geo.NewPos(25, 30),
			geo.NewPos(20, 20),
		},
	}

	tris := FanTriangulate(subpaths)
	// Two triangles, 1 each = 12 floats.
	assert.Equal(t, 12, len(tris))
}

func TestFanTriangulate_DegenerateSubpath(t *testing.T) {
	subpaths := [][]geo.Position{
		{geo.NewPos(0, 0), geo.NewPos(10, 0)}, // only 2 points, no triangle
	}

	tris := FanTriangulate(subpaths)
	assert.Equal(t, 0, len(tris))
}

func TestExpandFillFringe_Triangle(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
			geo.NewPos(5, 10),
			geo.NewPos(0, 0), // closed
		},
	}

	fringe := ExpandFillFringe(subpaths, 1.0)
	// Closed triangle has 3 edges, each produces 2 triangles = 6 triangles.
	// Each triangle has 3 vertices × 3 floats = 9 floats.
	// Total: 6 triangles × 9 = 54 floats.
	assert.Equal(t, 54, len(fringe))

	// Verify alpha values: every 3rd float should be 0 or 1.
	for i := 2; i < len(fringe); i += 3 {
		a := fringe[i]
		assert.True(t, a == 0 || a == 1, "alpha at index %d should be 0 or 1, got %f", i, a)
	}
}

func TestExpandStrokeAA_SimpleLine(t *testing.T) {
	subpaths := [][]geo.Position{
		{
			geo.NewPos(0, 0),
			geo.NewPos(10, 0),
		},
	}

	tris := ExpandStrokeAA(subpaths, 2, canvas.LineCapButt, canvas.LineJoinMiter, 4, nil, 0, 1.0)
	// Should have more geometry than non-AA stroke (core + outer fringe).
	assert.Greater(t, len(tris), 0)
	// Stride is 3 (x, y, alpha).
	assert.Equal(t, 0, len(tris)%3)
}
