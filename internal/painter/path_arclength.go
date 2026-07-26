package painter

import (
	"math"
	"sort"

	"github.com/belluikit/bellui/geo"
)

// PathSampler provides arc-length parameterized access to a flattened polyline.
// Given a distance along the path, it returns the interpolated position and
// tangent angle at that point.
type PathSampler struct {
	points   []geo.Position
	cumLens  []float32 // cumLens[i] = cumulative length from start to points[i]
	totalLen float32
}

// NewPathSampler creates a PathSampler from a polyline (flattened path points).
// The points slice must contain at least 2 points.
func NewPathSampler(points []geo.Position) *PathSampler {
	n := len(points)
	if n < 2 {
		return &PathSampler{points: points, cumLens: make([]float32, n)}
	}

	cumLens := make([]float32, n)
	cumLens[0] = 0
	for i := 1; i < n; i++ {
		dx := points[i].X - points[i-1].X
		dy := points[i].Y - points[i-1].Y
		segLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		cumLens[i] = cumLens[i-1] + segLen
	}

	return &PathSampler{
		points:   points,
		cumLens:  cumLens,
		totalLen: cumLens[n-1],
	}
}

// TotalLength returns the total arc length of the path.
func (ps *PathSampler) TotalLength() float32 {
	return ps.totalLen
}

// Sample returns the position (x, y) and tangent angle (radians, measured
// from the +X axis) at distance dist along the path.
// Distances before the start clamp to the first point; distances beyond
// the end clamp to the last point. The tangent at clamped endpoints uses
// the first or last segment direction respectively.
func (ps *PathSampler) Sample(dist float32) (x, y, angle float32) {
	n := len(ps.points)
	if n == 0 {
		return 0, 0, 0
	}
	if n == 1 {
		return ps.points[0].X, ps.points[0].Y, 0
	}

	// Clamp to path range.
	if dist <= 0 {
		p := ps.points[0]
		dx := ps.points[1].X - p.X
		dy := ps.points[1].Y - p.Y
		return p.X, p.Y, float32(math.Atan2(float64(dy), float64(dx)))
	}
	if dist >= ps.totalLen {
		p := ps.points[n-1]
		dx := p.X - ps.points[n-2].X
		dy := p.Y - ps.points[n-2].Y
		return p.X, p.Y, float32(math.Atan2(float64(dy), float64(dx)))
	}

	// Binary search for the segment containing dist.
	idx := sort.Search(n, func(i int) bool {
		return ps.cumLens[i] >= dist
	})
	if idx >= n {
		idx = n - 1
	}
	if idx < 1 {
		idx = 1
	}

	// Interpolate within the segment [idx-1, idx].
	segStart := ps.cumLens[idx-1]
	segEnd := ps.cumLens[idx]
	segLen := segEnd - segStart
	if segLen < 1e-9 {
		// Degenerate zero-length segment; use endpoint.
		p := ps.points[idx]
		return p.X, p.Y, 0
	}

	t := (dist - segStart) / segLen
	p0 := ps.points[idx-1]
	p1 := ps.points[idx]
	x = p0.X + t*(p1.X-p0.X)
	y = p0.Y + t*(p1.Y-p0.Y)

	dx := p1.X - p0.X
	dy := p1.Y - p0.Y
	angle = float32(math.Atan2(float64(dy), float64(dx)))
	return x, y, angle
}

// ConcatenateSubpaths joins multiple subpaths (from FlattenPath) into a
// single continuous polyline for text placement. Subpaths are concatenated
// in order; if the end of one subpath differs from the start of the next,
// a connecting segment is implicitly created.
func ConcatenateSubpaths(subpaths [][]geo.Position) []geo.Position {
	total := 0
	for _, sp := range subpaths {
		total += len(sp)
	}
	if total == 0 {
		return nil
	}

	result := make([]geo.Position, 0, total)
	for _, sp := range subpaths {
		if len(sp) == 0 {
			continue
		}
		result = append(result, sp...)
	}
	return result
}
