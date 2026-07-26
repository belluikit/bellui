package painter

import (
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

const flattenTolerance = 0.25 // max deviation in logical pixels before subdivision

// FlattenPath converts a Path's elements into flattened subpaths (polylines).
// Curves are subdivided into line segments until the deviation from the true
// curve is less than tolerance.
func FlattenPath(elements []canvas.PathElement, scale float32) [][]geo.Position {
	if len(elements) == 0 {
		return nil
	}

	tolerance := float32(flattenTolerance)
	if scale > 1 {
		tolerance /= scale
	}

	var subpaths [][]geo.Position
	var current []geo.Position
	var cursor geo.Position

	for _, el := range elements {
		switch el.Cmd() {
		case canvas.PathMoveTo:
			if len(current) > 0 {
				subpaths = append(subpaths, current)
			}
			p := el.Point(0)
			cursor = p
			current = []geo.Position{p}

		case canvas.PathLineTo:
			p := el.Point(0)
			current = append(current, p)
			cursor = p

		case canvas.PathQuadTo:
			ctrl := el.Point(0)
			end := el.Point(1)
			current = flattenQuadBezier(current, cursor, ctrl, end, tolerance)
			cursor = end

		case canvas.PathCubicTo:
			ctrl1 := el.Point(0)
			ctrl2 := el.Point(1)
			end := el.Point(2)
			current = flattenCubicBezier(current, cursor, ctrl1, ctrl2, end, tolerance)
			cursor = end

		case canvas.PathClose:
			if len(current) > 1 {
				// Close the subpath by appending the start point if not already there.
				start := current[0]
				last := current[len(current)-1]
				if start.X != last.X || start.Y != last.Y {
					current = append(current, start)
				}
			}
		}
	}

	if len(current) > 0 {
		subpaths = append(subpaths, current)
	}
	return subpaths
}

// flattenQuadBezier adaptively subdivides a quadratic Bezier curve into line segments.
func flattenQuadBezier(out []geo.Position, p0, p1, p2 geo.Position, tolerance float32) []geo.Position {
	return subdivideQuad(out, p0, p1, p2, tolerance, 0)
}

func subdivideQuad(out []geo.Position, p0, p1, p2 geo.Position, tolerance float32, depth int) []geo.Position {
	if depth > 10 {
		return append(out, p2)
	}

	// Flatness test: distance from control point to the line (p0, p2).
	dx := p2.X - p0.X
	dy := p2.Y - p0.Y
	d := float32(math.Abs(float64((p1.X-p2.X)*dy - (p1.Y-p2.Y)*dx)))
	lenSq := dx*dx + dy*dy

	if d*d <= tolerance*tolerance*lenSq {
		return append(out, p2)
	}

	// de Casteljau subdivision at t=0.5
	m01 := midpoint(p0, p1)
	m12 := midpoint(p1, p2)
	m := midpoint(m01, m12)

	out = subdivideQuad(out, p0, m01, m, tolerance, depth+1)
	out = subdivideQuad(out, m, m12, p2, tolerance, depth+1)
	return out
}

// flattenCubicBezier adaptively subdivides a cubic Bezier curve into line segments.
func flattenCubicBezier(out []geo.Position, p0, p1, p2, p3 geo.Position, tolerance float32) []geo.Position {
	return subdivideCubic(out, p0, p1, p2, p3, tolerance, 0)
}

func subdivideCubic(out []geo.Position, p0, p1, p2, p3 geo.Position, tolerance float32, depth int) []geo.Position {
	if depth > 10 {
		return append(out, p3)
	}

	// Flatness test: max distance of control points from the line (p0, p3).
	dx := p3.X - p0.X
	dy := p3.Y - p0.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return append(out, p3)
	}

	d1 := float32(math.Abs(float64((p1.X-p3.X)*dy - (p1.Y-p3.Y)*dx)))
	d2 := float32(math.Abs(float64((p2.X-p3.X)*dy - (p2.Y-p3.Y)*dx)))
	d := d1
	if d2 > d {
		d = d2
	}

	if d*d <= tolerance*tolerance*lenSq {
		return append(out, p3)
	}

	// de Casteljau subdivision at t=0.5
	m01 := midpoint(p0, p1)
	m12 := midpoint(p1, p2)
	m23 := midpoint(p2, p3)
	m012 := midpoint(m01, m12)
	m123 := midpoint(m12, m23)
	m := midpoint(m012, m123)

	out = subdivideCubic(out, p0, m01, m012, m, tolerance, depth+1)
	out = subdivideCubic(out, m, m123, m23, p3, tolerance, depth+1)
	return out
}

func midpoint(a, b geo.Position) geo.Position {
	return geo.NewPos((a.X+b.X)*0.5, (a.Y+b.Y)*0.5)
}

// TriangulateFill converts closed subpaths into triangles using ear-clipping.
// Returns a flat slice of (x, y) float32 pairs, every 6 floats forming one triangle.
func TriangulateFill(subpaths [][]geo.Position) []float32 {
	var tris []float32
	for _, sp := range subpaths {
		tris = earClipTriangulate(tris, sp)
	}
	return tris
}

// FanTriangulate generates triangle fan geometry from each subpath's first
// vertex to every edge. The resulting triangles overlap, so they are intended
// for stencil-buffer fill (not direct color rendering). The stencil resolves
// the correct interior via winding or even-odd rules.
// Returns a flat slice of (x, y) float32 pairs, every 6 floats forming one triangle.
func FanTriangulate(subpaths [][]geo.Position) []float32 {
	var tris []float32
	for _, sp := range subpaths {
		n := len(sp)
		// Remove closing duplicate if present.
		if n > 1 && sp[0].X == sp[n-1].X && sp[0].Y == sp[n-1].Y {
			n--
		}
		if n < 3 {
			continue
		}
		pivot := sp[0]
		for i := 1; i < n-1; i++ {
			tris = append(tris, pivot.X, pivot.Y, sp[i].X, sp[i].Y, sp[i+1].X, sp[i+1].Y)
		}
	}
	return tris
}

// earClipTriangulate appends triangles from a closed polygon to out.
// The polygon should have the first point equal to the last point (closed).
func earClipTriangulate(out []float32, poly []geo.Position) []float32 {
	n := len(poly)
	// Remove closing duplicate if present.
	if n > 1 && poly[0].X == poly[n-1].X && poly[0].Y == poly[n-1].Y {
		n--
	}
	if n < 3 {
		return out
	}

	// Build index list.
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	// Determine polygon winding. If clockwise, reverse.
	if signedArea(poly[:n]) < 0 {
		for i, j := 0, len(indices)-1; i < j; i, j = i+1, j-1 {
			indices[i], indices[j] = indices[j], indices[i]
		}
	}

	remaining := len(indices)
	failCount := 0

	for remaining > 3 {
		found := false
		for i := 0; i < remaining; i++ {
			prev := (i + remaining - 1) % remaining
			next := (i + 1) % remaining

			a := poly[indices[prev]]
			b := poly[indices[i]]
			c := poly[indices[next]]

			if !isConvexVertex(a, b, c) {
				continue
			}

			ear := true
			for j := 0; j < remaining; j++ {
				if j == prev || j == i || j == next {
					continue
				}
				if pointInTriangle(poly[indices[j]], a, b, c) {
					ear = false
					break
				}
			}

			if ear {
				out = append(out, a.X, a.Y, b.X, b.Y, c.X, c.Y)
				// Remove vertex i from the list.
				indices = append(indices[:i], indices[i+1:]...)
				remaining--
				failCount = 0
				found = true
				break
			}
		}

		if !found {
			failCount++
			if failCount > remaining {
				break // degenerate polygon, bail out
			}
		}
	}

	if remaining == 3 {
		a := poly[indices[0]]
		b := poly[indices[1]]
		c := poly[indices[2]]
		out = append(out, a.X, a.Y, b.X, b.Y, c.X, c.Y)
	}

	return out
}

// signedArea returns twice the signed area of a polygon.
// Positive = counter-clockwise, negative = clockwise.
func signedArea(poly []geo.Position) float32 {
	var area float32
	n := len(poly)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += poly[i].X * poly[j].Y
		area -= poly[j].X * poly[i].Y
	}
	return area
}

// isConvexVertex returns true if the angle at vertex b (going a→b→c) is convex
// (left turn in CCW winding).
func isConvexVertex(a, b, c geo.Position) bool {
	return cross2D(b.X-a.X, b.Y-a.Y, c.X-b.X, c.Y-b.Y) > 0
}

// pointInTriangle tests if point p is inside triangle (a, b, c).
func pointInTriangle(p, a, b, c geo.Position) bool {
	d1 := cross2D(b.X-a.X, b.Y-a.Y, p.X-a.X, p.Y-a.Y)
	d2 := cross2D(c.X-b.X, c.Y-b.Y, p.X-b.X, p.Y-b.Y)
	d3 := cross2D(a.X-c.X, a.Y-c.Y, p.X-c.X, p.Y-c.Y)

	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func cross2D(ax, ay, bx, by float32) float32 {
	return ax*by - ay*bx
}

// ExpandStroke converts flattened subpaths into filled triangle geometry
// representing the stroke. Returns a flat slice of (x, y) float32 pairs.
func ExpandStroke(subpaths [][]geo.Position, width float32, cap canvas.LineCap, join canvas.LineJoin, miterLimit float32, dashPattern []float32, dashOffset float32) []float32 {
	if width <= 0 {
		return nil
	}
	halfW := width * 0.5

	if miterLimit <= 0 {
		miterLimit = 4
	}

	var out []float32
	for _, sp := range subpaths {
		if len(dashPattern) > 0 {
			dashed := applyDashPattern(sp, dashPattern, dashOffset)
			for _, seg := range dashed {
				out = expandOpenStroke(out, seg, halfW, cap, join, miterLimit)
			}
		} else {
			closed := len(sp) > 2 && sp[0].X == sp[len(sp)-1].X && sp[0].Y == sp[len(sp)-1].Y
			if closed {
				out = expandClosedStroke(out, sp, halfW, join, miterLimit)
			} else {
				out = expandOpenStroke(out, sp, halfW, cap, join, miterLimit)
			}
		}
	}
	return out
}

// expandOpenStroke generates stroke triangles for an open polyline.
func expandOpenStroke(out []float32, pts []geo.Position, halfW float32, cap canvas.LineCap, join canvas.LineJoin, miterLimit float32) []float32 {
	n := len(pts)
	if n < 2 {
		return out
	}

	// Start cap.
	out = addCap(out, pts[1], pts[0], halfW, cap)

	// Segments and joins.
	for i := 0; i < n-1; i++ {
		nx, ny := segmentNormal(pts[i], pts[i+1])
		// Quad for this segment.
		out = appendQuad(
			out,
			pts[i].X+nx*halfW, pts[i].Y+ny*halfW,
			pts[i+1].X+nx*halfW, pts[i+1].Y+ny*halfW,
			pts[i+1].X-nx*halfW, pts[i+1].Y-ny*halfW,
			pts[i].X-nx*halfW, pts[i].Y-ny*halfW,
		)

		// Join to next segment.
		if i < n-2 {
			out = addJoin(out, pts[i], pts[i+1], pts[i+2], halfW, join, miterLimit)
		}
	}

	// End cap.
	out = addCap(out, pts[n-2], pts[n-1], halfW, cap)

	return out
}

// expandClosedStroke generates stroke triangles for a closed polyline.
func expandClosedStroke(out []float32, pts []geo.Position, halfW float32, join canvas.LineJoin, miterLimit float32) []float32 {
	n := len(pts) - 1 // exclude closing duplicate
	if n < 2 {
		return out
	}

	for i := 0; i < n; i++ {
		next := (i + 1) % n
		nx, ny := segmentNormal(pts[i], pts[next])
		out = appendQuad(
			out,
			pts[i].X+nx*halfW, pts[i].Y+ny*halfW,
			pts[next].X+nx*halfW, pts[next].Y+ny*halfW,
			pts[next].X-nx*halfW, pts[next].Y-ny*halfW,
			pts[i].X-nx*halfW, pts[i].Y-ny*halfW,
		)

		nextNext := (next + 1) % n
		out = addJoin(out, pts[i], pts[next], pts[nextNext], halfW, join, miterLimit)
	}
	return out
}

// segmentNormal returns the unit normal perpendicular to the segment (a→b), pointing left.
func segmentNormal(a, b geo.Position) (float32, float32) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length == 0 {
		return 0, -1
	}
	return -dy / length, dx / length
}

// appendQuad appends two triangles forming a quad (a, b, c, d) to out.
func appendQuad(out []float32, ax, ay, bx, by, cx, cy, dx, dy float32) []float32 {
	return append(
		out,
		ax, ay, bx, by, cx, cy, // triangle 1
		ax, ay, cx, cy, dx, dy, // triangle 2
	)
}

// addJoin adds geometry for a line join at point b (segments a→b and b→c).
func addJoin(out []float32, a, b, c geo.Position, halfW float32, join canvas.LineJoin, miterLimit float32) []float32 {
	n1x, n1y := segmentNormal(a, b)
	n2x, n2y := segmentNormal(b, c)

	// Cross product determines turn direction.
	cross := n1x*n2y - n1y*n2x
	if math.Abs(float64(cross)) < 1e-6 {
		return out // collinear, no join needed
	}

	switch join {
	case canvas.LineJoinBevel:
		out = addBevelJoin(out, b, n1x, n1y, n2x, n2y, halfW, cross)
	case canvas.LineJoinRound:
		out = addRoundJoin(out, b, n1x, n1y, n2x, n2y, halfW, cross)
	default: // miter
		out = addMiterJoin(out, b, n1x, n1y, n2x, n2y, halfW, cross, miterLimit)
	}
	return out
}

func addBevelJoin(out []float32, b geo.Position, n1x, n1y, n2x, n2y, halfW, cross float32) []float32 {
	if cross > 0 {
		// Left turn: join on the right (negative normal side).
		return append(
			out,
			b.X, b.Y,
			b.X-n1x*halfW, b.Y-n1y*halfW,
			b.X-n2x*halfW, b.Y-n2y*halfW,
		)
	}
	// Right turn: join on the left (positive normal side).
	return append(
		out,
		b.X, b.Y,
		b.X+n1x*halfW, b.Y+n1y*halfW,
		b.X+n2x*halfW, b.Y+n2y*halfW,
	)
}

func addRoundJoin(out []float32, b geo.Position, n1x, n1y, n2x, n2y, halfW, cross float32) []float32 {
	// Determine which side the join is on.
	var startX, startY, endX, endY float32
	if cross > 0 {
		startX, startY = b.X-n1x*halfW, b.Y-n1y*halfW
		endX, endY = b.X-n2x*halfW, b.Y-n2y*halfW
	} else {
		startX, startY = b.X+n2x*halfW, b.Y+n2y*halfW
		endX, endY = b.X+n1x*halfW, b.Y+n1y*halfW
	}

	// Fan of triangles approximating the arc.
	angle1 := float32(math.Atan2(float64(startY-b.Y), float64(startX-b.X)))
	angle2 := float32(math.Atan2(float64(endY-b.Y), float64(endX-b.X)))
	sweep := angle2 - angle1
	if sweep > math.Pi {
		sweep -= 2 * math.Pi
	} else if sweep < -math.Pi {
		sweep += 2 * math.Pi
	}

	segments := int(math.Ceil(math.Abs(float64(sweep)) / (math.Pi / 8)))
	if segments < 1 {
		segments = 1
	}

	da := sweep / float32(segments)
	prevX, prevY := startX, startY
	for i := 1; i <= segments; i++ {
		a := angle1 + da*float32(i)
		nx := b.X + halfW*float32(math.Cos(float64(a)))
		ny := b.Y + halfW*float32(math.Sin(float64(a)))
		out = append(out, b.X, b.Y, prevX, prevY, nx, ny)
		prevX, prevY = nx, ny
	}
	return out
}

func addMiterJoin(out []float32, b geo.Position, n1x, n1y, n2x, n2y, halfW, cross, miterLimit float32) []float32 {
	// Compute miter point.
	// The miter direction bisects the two normals.
	mx := n1x + n2x
	my := n1y + n2y
	mLen := float32(math.Sqrt(float64(mx*mx + my*my)))
	if mLen < 1e-6 {
		return addBevelJoin(out, b, n1x, n1y, n2x, n2y, halfW, cross)
	}
	mx /= mLen
	my /= mLen

	// Miter length = halfW / cos(half-angle).
	dot := n1x*mx + n1y*my
	if dot == 0 {
		return addBevelJoin(out, b, n1x, n1y, n2x, n2y, halfW, cross)
	}
	miterLen := halfW / dot

	if miterLen/halfW > miterLimit {
		return addBevelJoin(out, b, n1x, n1y, n2x, n2y, halfW, cross)
	}

	if cross > 0 {
		miterX := b.X - mx*miterLen
		miterY := b.Y - my*miterLen
		out = append(
			out,
			b.X, b.Y,
			b.X-n1x*halfW, b.Y-n1y*halfW,
			miterX, miterY,
		)
		out = append(
			out,
			b.X, b.Y,
			miterX, miterY,
			b.X-n2x*halfW, b.Y-n2y*halfW,
		)
	} else {
		miterX := b.X + mx*miterLen
		miterY := b.Y + my*miterLen
		out = append(
			out,
			b.X, b.Y,
			b.X+n1x*halfW, b.Y+n1y*halfW,
			miterX, miterY,
		)
		out = append(
			out,
			b.X, b.Y,
			miterX, miterY,
			b.X+n2x*halfW, b.Y+n2y*halfW,
		)
	}
	return out
}

// addCap adds cap geometry at a path endpoint.
// prevDir is the point before the endpoint (used for direction).
func addCap(out []float32, from, to geo.Position, halfW float32, cap canvas.LineCap) []float32 {
	nx, ny := segmentNormal(from, to)

	switch cap {
	case canvas.LineCapSquare:
		// Extend the endpoint by halfW in the direction from→to.
		dx := to.X - from.X
		dy := to.Y - from.Y
		length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if length == 0 {
			return out
		}
		ex := dx / length * halfW
		ey := dy / length * halfW
		return appendQuad(
			out,
			to.X+nx*halfW, to.Y+ny*halfW,
			to.X+nx*halfW+ex, to.Y+ny*halfW+ey,
			to.X-nx*halfW+ex, to.Y-ny*halfW+ey,
			to.X-nx*halfW, to.Y-ny*halfW,
		)
	case canvas.LineCapRound:
		// Semicircle at the endpoint.
		dir := float32(math.Atan2(float64(ny), float64(nx)))
		segments := 8
		da := float32(math.Pi) / float32(segments)
		prevX := to.X + halfW*float32(math.Cos(float64(dir)))
		prevY := to.Y + halfW*float32(math.Sin(float64(dir)))
		for i := 1; i <= segments; i++ {
			a := dir + da*float32(i)
			cx := to.X + halfW*float32(math.Cos(float64(a)))
			cy := to.Y + halfW*float32(math.Sin(float64(a)))
			out = append(out, to.X, to.Y, prevX, prevY, cx, cy)
			prevX, prevY = cx, cy
		}
		return out
	default: // butt
		return out
	}
}

// applyDashPattern splits a polyline into dashed segments.
func applyDashPattern(pts []geo.Position, pattern []float32, offset float32) [][]geo.Position {
	if len(pts) < 2 || len(pattern) == 0 {
		return [][]geo.Position{pts}
	}

	// Compute total pattern length and normalize offset.
	var patternLen float32
	for _, d := range pattern {
		patternLen += d
	}
	if patternLen <= 0 {
		return [][]geo.Position{pts}
	}

	// Normalize offset to be within [0, patternLen).
	offset = float32(math.Mod(float64(offset), float64(patternLen)))
	if offset < 0 {
		offset += patternLen
	}

	// Find starting position in pattern.
	patIdx := 0
	remaining := pattern[0]
	for offset > 0 {
		if offset < remaining {
			remaining -= offset
			break
		}
		offset -= remaining
		patIdx = (patIdx + 1) % len(pattern)
		remaining = pattern[patIdx]
	}
	drawing := patIdx%2 == 0 // even indices are dashes, odd are gaps

	var result [][]geo.Position
	var current []geo.Position

	for i := 0; i < len(pts)-1; i++ {
		seg := pts[i+1]
		from := pts[i]

		dx := seg.X - from.X
		dy := seg.Y - from.Y
		segLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if segLen == 0 {
			continue
		}

		consumed := float32(0)
		for consumed < segLen {
			advance := remaining
			if consumed+advance > segLen {
				advance = segLen - consumed
			}

			t := (consumed + advance) / segLen
			pt := geo.NewPos(from.X+dx*t, from.Y+dy*t)

			if drawing {
				if len(current) == 0 {
					// Start of dash segment.
					tStart := consumed / segLen
					current = append(current, geo.NewPos(from.X+dx*tStart, from.Y+dy*tStart))
				}
				current = append(current, pt)
			}

			consumed += advance
			remaining -= advance

			if remaining <= 0 {
				if drawing && len(current) >= 2 {
					result = append(result, current)
					current = nil
				}
				patIdx = (patIdx + 1) % len(pattern)
				remaining = pattern[patIdx]
				drawing = patIdx%2 == 0
			}
		}
	}

	if drawing && len(current) >= 2 {
		result = append(result, current)
	}
	return result
}

// ExpandFillFringe generates a 1-pixel-wide alpha-fading strip along subpath
// boundaries. Each vertex is (x, y, alpha) — inner edge vertices have alpha=1,
// outer edge vertices have alpha=0. This provides anti-aliased fill edges.
func ExpandFillFringe(subpaths [][]geo.Position, fringeWidth float32) []float32 {
	var out []float32
	for _, sp := range subpaths {
		n := len(sp)
		if n < 2 {
			continue
		}
		closed := sp[0].X == sp[n-1].X && sp[0].Y == sp[n-1].Y
		if closed {
			n-- // exclude closing duplicate
		}
		if n < 2 {
			continue
		}

		for i := 0; i < n; i++ {
			var prev, curr, next int
			if closed {
				prev = (i + n - 1) % n
				curr = i
				next = (i + 1) % n
			} else {
				curr = i
				if i == 0 {
					prev = 0
				} else {
					prev = i - 1
				}
				if i == n-1 {
					next = n - 1
				} else {
					next = i + 1
				}
			}

			// Average the normals of adjacent segments for a smooth fringe.
			n1x, n1y := segmentNormal(sp[prev], sp[curr])
			n2x, n2y := segmentNormal(sp[curr], sp[next])
			if prev == curr {
				n1x, n1y = n2x, n2y
			}
			if next == curr {
				n2x, n2y = n1x, n1y
			}
			nx := (n1x + n2x) * 0.5
			ny := (n1y + n2y) * 0.5
			mLen := float32(math.Sqrt(float64(nx*nx + ny*ny)))
			if mLen > 1e-6 {
				nx /= mLen
				ny /= mLen
			}

			// Inner (alpha=1) and outer (alpha=0) vertices for this point.
			innerX := sp[curr].X
			innerY := sp[curr].Y
			outerX := sp[curr].X + nx*fringeWidth
			outerY := sp[curr].Y + ny*fringeWidth

			// Connect to next point's fringe to form a quad strip.
			var nextIdx int
			if closed {
				nextIdx = (i + 1) % n
			} else {
				nextIdx = i + 1
				if nextIdx >= n {
					continue
				}
			}

			nn1x, nn1y := segmentNormal(sp[curr], sp[nextIdx])
			var nn2x, nn2y float32
			if closed {
				nnext := (nextIdx + 1) % n
				nn2x, nn2y = segmentNormal(sp[nextIdx], sp[nnext])
			} else if nextIdx < n-1 {
				nn2x, nn2y = segmentNormal(sp[nextIdx], sp[nextIdx+1])
			} else {
				nn2x, nn2y = nn1x, nn1y
			}
			nnx := (nn1x + nn2x) * 0.5
			nny := (nn1y + nn2y) * 0.5
			nnLen := float32(math.Sqrt(float64(nnx*nnx + nny*nny)))
			if nnLen > 1e-6 {
				nnx /= nnLen
				nny /= nnLen
			}

			nextInnerX := sp[nextIdx].X
			nextInnerY := sp[nextIdx].Y
			nextOuterX := sp[nextIdx].X + nnx*fringeWidth
			nextOuterY := sp[nextIdx].Y + nny*fringeWidth

			// Two triangles forming the fringe quad: (inner, outer, nextOuter), (inner, nextOuter, nextInner)
			out = append(
				out,
				innerX, innerY, 1, outerX, outerY, 0, nextOuterX, nextOuterY, 0,
				innerX, innerY, 1, nextOuterX, nextOuterY, 0, nextInnerX, nextInnerY, 1,
			)
		}
	}
	return out
}

// ExpandStrokeAA generates stroke geometry with alpha fringe on both edges.
// Core stroke vertices have alpha=1, outer fringe vertices have alpha=0.
// Returns (x, y, alpha) triples.
func ExpandStrokeAA(subpaths [][]geo.Position, width float32, cap canvas.LineCap, join canvas.LineJoin,
	miterLimit float32, dashPattern []float32, dashOffset float32, fringeWidth float32,
) []float32 {
	// Generate the core stroke geometry with alpha=1.
	core := ExpandStroke(subpaths, width, cap, join, miterLimit, dashPattern, dashOffset)
	coreAA := make([]float32, 0, len(core)/2*3)
	for i := 0; i < len(core); i += 2 {
		coreAA = append(coreAA, core[i], core[i+1], 1)
	}

	// Generate the outer fringe from a slightly wider stroke minus the core.
	// This is the simple approach: expand the stroke with (width + 2*fringe),
	// set those vertices to alpha=0, and let the overlap with the core
	// produce a smooth transition.
	outerWidth := width + fringeWidth*2
	outer := ExpandStroke(subpaths, outerWidth, cap, join, miterLimit, dashPattern, dashOffset)
	outerAA := make([]float32, 0, len(outer)/2*3)
	for i := 0; i < len(outer); i += 2 {
		outerAA = append(outerAA, outer[i], outer[i+1], 0)
	}

	return append(outerAA, coreAA...)
}
