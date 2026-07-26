package painter

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/geo"

	"github.com/stretchr/testify/assert"
)

func TestPathSampler_HorizontalLine(t *testing.T) {
	points := []geo.Position{
		geo.NewPos(0, 0),
		geo.NewPos(100, 0),
	}
	ps := NewPathSampler(points)

	assert.InDelta(t, float32(100), ps.TotalLength(), 0.01)

	// Start
	x, y, angle := ps.Sample(0)
	assert.InDelta(t, float32(0), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)

	// Midpoint
	x, y, angle = ps.Sample(50)
	assert.InDelta(t, float32(50), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)

	// End
	x, y, angle = ps.Sample(100)
	assert.InDelta(t, float32(100), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)
}

func TestPathSampler_DiagonalLine(t *testing.T) {
	points := []geo.Position{
		geo.NewPos(0, 0),
		geo.NewPos(100, 100),
	}
	ps := NewPathSampler(points)

	expectedLen := float32(math.Sqrt(2) * 100)
	assert.InDelta(t, expectedLen, ps.TotalLength(), 0.01)

	// Midpoint
	x, y, angle := ps.Sample(expectedLen / 2)
	assert.InDelta(t, float32(50), x, 0.1)
	assert.InDelta(t, float32(50), y, 0.1)
	assert.InDelta(t, float32(math.Pi/4), angle, 0.01) // 45 degrees
}

func TestPathSampler_LShapedPath(t *testing.T) {
	points := []geo.Position{
		geo.NewPos(0, 0),
		geo.NewPos(100, 0),  // horizontal segment, length 100
		geo.NewPos(100, 50), // vertical segment, length 50
	}
	ps := NewPathSampler(points)

	assert.InDelta(t, float32(150), ps.TotalLength(), 0.01)

	// In the horizontal segment
	x, y, angle := ps.Sample(50)
	assert.InDelta(t, float32(50), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01) // horizontal

	// At the corner
	x, y, _ = ps.Sample(100)
	assert.InDelta(t, float32(100), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)

	// In the vertical segment
	x, y, angle = ps.Sample(125)
	assert.InDelta(t, float32(100), x, 0.01)
	assert.InDelta(t, float32(25), y, 0.01)
	assert.InDelta(t, float32(math.Pi/2), angle, 0.01) // downward
}

func TestPathSampler_ClampNegative(t *testing.T) {
	points := []geo.Position{
		geo.NewPos(10, 20),
		geo.NewPos(110, 20),
	}
	ps := NewPathSampler(points)

	x, y, angle := ps.Sample(-5)
	assert.InDelta(t, float32(10), x, 0.01)
	assert.InDelta(t, float32(20), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)
}

func TestPathSampler_ClampOverflow(t *testing.T) {
	points := []geo.Position{
		geo.NewPos(0, 0),
		geo.NewPos(50, 0),
	}
	ps := NewPathSampler(points)

	x, y, angle := ps.Sample(200)
	assert.InDelta(t, float32(50), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)
}

func TestPathSampler_SinglePoint(t *testing.T) {
	points := []geo.Position{geo.NewPos(5, 10)}
	ps := NewPathSampler(points)

	assert.InDelta(t, float32(0), ps.TotalLength(), 0.01)

	x, y, angle := ps.Sample(0)
	assert.InDelta(t, float32(5), x, 0.01)
	assert.InDelta(t, float32(10), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)
}

func TestPathSampler_Empty(t *testing.T) {
	ps := NewPathSampler(nil)
	assert.InDelta(t, float32(0), ps.TotalLength(), 0.01)

	x, y, angle := ps.Sample(0)
	assert.InDelta(t, float32(0), x, 0.01)
	assert.InDelta(t, float32(0), y, 0.01)
	assert.InDelta(t, float32(0), angle, 0.01)
}

func TestConcatenateSubpaths(t *testing.T) {
	sub1 := []geo.Position{geo.NewPos(0, 0), geo.NewPos(10, 0)}
	sub2 := []geo.Position{geo.NewPos(10, 0), geo.NewPos(10, 10)}

	result := ConcatenateSubpaths([][]geo.Position{sub1, sub2})
	assert.Len(t, result, 4)
	assert.Equal(t, geo.NewPos(0, 0), result[0])
	assert.Equal(t, geo.NewPos(10, 10), result[3])
}

func TestConcatenateSubpaths_Empty(t *testing.T) {
	result := ConcatenateSubpaths(nil)
	assert.Nil(t, result)

	result = ConcatenateSubpaths([][]geo.Position{{}, {}})
	assert.Nil(t, result)
}
