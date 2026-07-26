package dagwidget

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestCubicBezierEndpoints(t *testing.T) {
	p0 := geo.NewPos(0, 0)
	p1 := geo.NewPos(100, 0)
	p2 := geo.NewPos(200, 100)
	p3 := geo.NewPos(300, 100)

	start := CubicBezierPoint(p0, p1, p2, p3, 0)
	if dist(start, p0) > 0.001 {
		t.Fatalf("t=0 should equal p0, got %v", start)
	}

	end := CubicBezierPoint(p0, p1, p2, p3, 1)
	if dist(end, p3) > 0.001 {
		t.Fatalf("t=1 should equal p3, got %v", end)
	}
}

func TestFlattenCubicBezier(t *testing.T) {
	p0 := geo.NewPos(0, 0)
	p3 := geo.NewPos(300, 0)
	cp1, cp2 := ComputeControlPoints(p0, p3)

	points := FlattenCubicBezier(p0, cp1, cp2, p3, 32)
	if len(points) != 33 {
		t.Fatalf("expected 33 points, got %d", len(points))
	}

	// First and last should match endpoints
	if dist(points[0], p0) > 0.001 {
		t.Fatalf("first point should be p0")
	}
	if dist(points[32], p3) > 0.001 {
		t.Fatalf("last point should be p3")
	}
}

func TestHitTestBezierOnCurve(t *testing.T) {
	src := geo.NewPos(0, 0)
	tgt := geo.NewPos(300, 0)
	cp1, cp2 := ComputeControlPoints(src, tgt)

	// Mid-point of a horizontal bezier should be near (150, 0)
	mid := CubicBezierPoint(src, cp1, cp2, tgt, 0.5)
	if !HitTestBezier(src, cp1, cp2, tgt, mid, 5) {
		t.Fatal("mid-point should hit the curve")
	}
}

func TestHitTestBezierOffCurve(t *testing.T) {
	src := geo.NewPos(0, 0)
	tgt := geo.NewPos(300, 0)
	cp1, cp2 := ComputeControlPoints(src, tgt)

	far := geo.NewPos(150, 200)
	if HitTestBezier(src, cp1, cp2, tgt, far, 5) {
		t.Fatal("far point should not hit the curve")
	}
}

func TestComputeControlPointsForward(t *testing.T) {
	src := geo.NewPos(0, 0)
	tgt := geo.NewPos(300, 100)
	cp1, cp2 := ComputeControlPoints(src, tgt)

	// cp1 should be to the right of src
	if cp1.X <= src.X {
		t.Fatal("cp1 should be to the right of src")
	}
	// cp2 should be to the left of tgt
	if cp2.X >= tgt.X {
		t.Fatal("cp2 should be to the left of tgt")
	}
}

func TestComputeControlPointsBackward(t *testing.T) {
	src := geo.NewPos(300, 0)
	tgt := geo.NewPos(0, 100)
	cp1, cp2 := ComputeControlPoints(src, tgt)

	// cp1 should still depart to the right of src
	if cp1.X <= src.X {
		t.Fatal("cp1 should be to the right of src even in backward case")
	}
	// cp2 should depart to the left of tgt
	if cp2.X >= tgt.X {
		t.Fatal("cp2 should be to the left of tgt even in backward case")
	}
}

func TestAdaptiveSegmentCount(t *testing.T) {
	near := geo.NewPos(0, 0)
	far := geo.NewPos(1000, 0)

	nNear := AdaptiveSegmentCount(near, geo.NewPos(50, 0), 1.0)
	nFar := AdaptiveSegmentCount(near, far, 1.0)

	if nNear < 16 {
		t.Fatalf("minimum should be 16, got %d", nNear)
	}
	if nFar <= nNear {
		t.Fatalf("far edges should have more segments: near=%d far=%d", nNear, nFar)
	}
}

func TestBezierBoundingBox(t *testing.T) {
	p0 := geo.NewPos(10, 20)
	p1 := geo.NewPos(50, 5)
	p2 := geo.NewPos(80, 90)
	p3 := geo.NewPos(100, 50)

	bMin, bMax := BezierBoundingBox(p0, p1, p2, p3)
	if bMin.X != 10 || bMin.Y != 5 {
		t.Fatalf("unexpected min: %v", bMin)
	}
	if bMax.X != 100 || bMax.Y != 90 {
		t.Fatalf("unexpected max: %v", bMax)
	}
}

func TestArrowheadPoints(t *testing.T) {
	// Tangent pointing right: wings should sit behind the tip, one above and one below.
	from := geo.NewPos(0, 0)
	tip := geo.NewPos(100, 0)
	left, right := ArrowheadPoints(from, tip, 10)

	if left.X >= tip.X || right.X >= tip.X {
		t.Fatalf("wings should be behind the tip: left=%v right=%v", left, right)
	}
	if left.Y*right.Y >= 0 {
		t.Fatalf("wings should straddle the tangent: left=%v right=%v", left, right)
	}
	if dist(left, tip)-10 > 0.001 || dist(right, tip)-10 > 0.001 {
		t.Fatalf("wing length should be 10: left=%v right=%v", dist(left, tip), dist(right, tip))
	}
}

func TestPullBack(t *testing.T) {
	p := geo.NewPos(100, 0)
	anchor := geo.NewPos(0, 0)

	moved := pullBack(p, anchor, 6)
	if dist(moved, geo.NewPos(94, 0)) > 0.001 {
		t.Fatalf("expected (94,0), got %v", moved)
	}

	// Degenerate cases: coincident points and over-long pullback leave p unchanged.
	if got := pullBack(p, p, 6); got != p {
		t.Fatalf("coincident points should return p, got %v", got)
	}
	if got := pullBack(p, anchor, 200); got != p {
		t.Fatalf("pullback past anchor should return p, got %v", got)
	}
}

func dist(a, b geo.Position) float32 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	return float32(math.Sqrt(dx*dx + dy*dy))
}
