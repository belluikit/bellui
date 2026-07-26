package dagwidget

import (
	"math"

	"github.com/belluikit/bellui/geo"
)

// ComputeControlPoints generates cubic bezier control points for an edge
// from a source (output port, right side) to a target (input port, left side).
// The curve departs horizontally from both endpoints.
func ComputeControlPoints(src, tgt geo.Position) (cp1, cp2 geo.Position) {
	dx := tgt.X - src.X
	dy := tgt.Y - src.Y

	if dx >= 0 {
		// Standard case: target is to the right
		offset := max32(abs32(dx)*0.5, 50)
		cp1 = geo.NewPos(src.X+offset, src.Y)
		cp2 = geo.NewPos(tgt.X-offset, tgt.Y)
	} else {
		// Backward case: target is to the left — route around
		offset := max32(abs32(dx)*0.5, 80)
		vertOffset := max32(abs32(dy)*0.5, 60)
		if dy <= 0 {
			vertOffset = -vertOffset
		}
		cp1 = geo.NewPos(src.X+offset, src.Y+vertOffset)
		cp2 = geo.NewPos(tgt.X-offset, tgt.Y-vertOffset)
	}
	return
}

// CubicBezierPoint evaluates the cubic bezier at parameter t ∈ [0, 1].
func CubicBezierPoint(p0, p1, p2, p3 geo.Position, t float32) geo.Position {
	u := 1 - t
	return geo.NewPos(
		u*u*u*p0.X+3*u*u*t*p1.X+3*u*t*t*p2.X+t*t*t*p3.X,
		u*u*u*p0.Y+3*u*u*t*p1.Y+3*u*t*t*p2.Y+t*t*t*p3.Y,
	)
}

// FlattenCubicBezier returns n+1 points approximating the cubic bezier.
func FlattenCubicBezier(p0, p1, p2, p3 geo.Position, n int) []geo.Position {
	if n < 1 {
		n = 1
	}
	points := make([]geo.Position, n+1)
	for i := 0; i <= n; i++ {
		t := float32(i) / float32(n)
		points[i] = CubicBezierPoint(p0, p1, p2, p3, t)
	}
	return points
}

// AdaptiveSegmentCount returns an appropriate number of segments for a bezier
// curve given the distance between endpoints and the current zoom level.
func AdaptiveSegmentCount(src, tgt geo.Position, zoom float32) int {
	dx := float64(tgt.X - src.X)
	dy := float64(tgt.Y - src.Y)
	length := float32(math.Sqrt(dx*dx + dy*dy))

	n := int(length * zoom / 4)
	if n < 16 {
		return 16
	}
	if n > 64 {
		return 64
	}
	return n
}

// ArrowheadPoints returns the two wing endpoints of an arrowhead with its tip
// at tip, oriented along the direction from `from` to `tip`. The wings sweep
// back from the tip at ±30° with the given length. Use the last flattened
// segment of a curve as (from, tip) to align the head with the curve tangent.
func ArrowheadPoints(from, tip geo.Position, length float32) (left, right geo.Position) {
	angle := math.Atan2(float64(tip.Y-from.Y), float64(tip.X-from.X))
	const spread = math.Pi / 6
	l := float64(length)
	left = geo.NewPos(
		tip.X-float32(l*math.Cos(angle-spread)),
		tip.Y-float32(l*math.Sin(angle-spread)),
	)
	right = geo.NewPos(
		tip.X-float32(l*math.Cos(angle+spread)),
		tip.Y-float32(l*math.Sin(angle+spread)),
	)
	return
}

// pullBack returns p moved toward anchor by dist, or p unchanged when the
// two points coincide. Used to keep an arrowhead tip clear of the port circle.
func pullBack(p, anchor geo.Position, dist float32) geo.Position {
	dx := anchor.X - p.X
	dy := anchor.Y - p.Y
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length == 0 || dist >= length {
		return p
	}
	s := dist / length
	return geo.NewPos(p.X+dx*s, p.Y+dy*s)
}

// HitTestBezier checks if point p is within the given threshold distance
// of the cubic bezier curve defined by p0..p3.
func HitTestBezier(p0, p1, p2, p3, p geo.Position, threshold float32) bool {
	points := FlattenCubicBezier(p0, p1, p2, p3, 32)
	for i := 0; i < len(points)-1; i++ {
		if pointToSegmentDist(p, points[i], points[i+1]) < threshold {
			return true
		}
	}
	return false
}

// BezierBoundingBox returns the bounding box of a cubic bezier's control polygon.
// The actual curve is always contained within the convex hull of the control points.
func BezierBoundingBox(p0, p1, p2, p3 geo.Position) (min, max geo.Position) {
	minX := min32(min32(p0.X, p1.X), min32(p2.X, p3.X))
	minY := min32(min32(p0.Y, p1.Y), min32(p2.Y, p3.Y))
	maxX := max32(max32(p0.X, p1.X), max32(p2.X, p3.X))
	maxY := max32(max32(p0.Y, p1.Y), max32(p2.Y, p3.Y))
	return geo.NewPos(minX, minY), geo.NewPos(maxX, maxY)
}

// BezierTightBounds returns the bounding box of the curve itself, by sampling.
// Unlike BezierBoundingBox (control polygon, often far larger than the drawn
// curve — especially for backward-routed edges) this tracks the real extent
// closely; with 64 samples the error is a small fraction of a pixel.
func BezierTightBounds(p0, p1, p2, p3 geo.Position) (min, max geo.Position) {
	points := FlattenCubicBezier(p0, p1, p2, p3, 64)
	min, max = points[0], points[0]
	for _, p := range points[1:] {
		min.X = min32(min.X, p.X)
		min.Y = min32(min.Y, p.Y)
		max.X = max32(max.X, p.X)
		max.Y = max32(max.Y, p.Y)
	}
	return min, max
}

// pointToSegmentDist returns the shortest distance from point p to the line segment a-b.
func pointToSegmentDist(p, a, b geo.Position) float32 {
	abx := b.X - a.X
	aby := b.Y - a.Y
	apx := p.X - a.X
	apy := p.Y - a.Y

	dot := apx*abx + apy*aby
	lenSq := abx*abx + aby*aby

	if lenSq == 0 {
		return float32(math.Sqrt(float64(apx*apx + apy*apy)))
	}

	t := dot / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	projX := a.X + t*abx
	projY := a.Y + t*aby
	dx := p.X - projX
	dy := p.Y - projY
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// rectCenter returns the center of a rectangle.
func rectCenter(r geo.Rectangle) geo.Position {
	return geo.NewPos(r.Pos.X+r.Size.Width/2, r.Pos.Y+r.Size.Height/2)
}

// rectBorderPoint returns the intersection of the rectangle's border with
// the ray from its center toward the given point. When toward coincides
// with the center, the center itself is returned.
func rectBorderPoint(r geo.Rectangle, toward geo.Position) geo.Position {
	return rectBorderPointFrom(r, rectCenter(r), toward)
}

// rectBorderPointFrom generalizes rectBorderPoint to an off-center ray
// origin: it returns the intersection of the rectangle's border with the ray
// from an interior origin toward the given point (used to keep parallel link
// edge lanes attached to the border). A degenerate ray, or an origin outside
// the rectangle, returns the origin unchanged.
func rectBorderPointFrom(r geo.Rectangle, from, toward geo.Position) geo.Position {
	dx := toward.X - from.X
	dy := toward.Y - from.Y
	t := float32(math.MaxFloat32)
	if dx > 0 {
		t = min32(t, (r.Pos.X+r.Size.Width-from.X)/dx)
	} else if dx < 0 {
		t = min32(t, (r.Pos.X-from.X)/dx)
	}
	if dy > 0 {
		t = min32(t, (r.Pos.Y+r.Size.Height-from.Y)/dy)
	} else if dy < 0 {
		t = min32(t, (r.Pos.Y-from.Y)/dy)
	}
	if t < 0 || t == float32(math.MaxFloat32) {
		return from
	}
	return geo.NewPos(from.X+dx*t, from.Y+dy*t)
}

// hitTestSegment checks if point p is within threshold distance of the
// segment a-b.
func hitTestSegment(a, b, p geo.Position, threshold float32) bool {
	return pointToSegmentDist(p, a, b) < threshold
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
