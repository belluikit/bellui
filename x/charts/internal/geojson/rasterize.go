package geojson

import (
	"image"
	"image/color"
	"sort"
)

// edge is one non-horizontal polygon edge, oriented top-to-bottom.
type edge struct {
	yTop, yBot int
	xTop, xBot int
}

// xAt returns the edge's x at scanline y, using the same integer arithmetic the
// original per-scanline scan used so the fill is byte-identical.
func (e edge) xAt(y int) int {
	return e.xTop + (y-e.yTop)*(e.xBot-e.xTop)/(e.yBot-e.yTop)
}

// Rasterizer fills polygons on an NRGBA image using a scanline sweep, reusing
// its working buffers across calls. The zero value is ready to use.
//
// Reuse matters because the choropleth rasterises hundreds of rings per view:
// the buffers here were previously a fresh allocation per *scanline*.
// Rasterizer is not safe for concurrent use; give each goroutine its own.
type Rasterizer struct {
	edges  []edge
	active []int32
	xs     []int
}

// RasterizePolygon fills a polygon on an NRGBA image using scanline
// rasterization. Points are in pixel coordinates.
//
// It sweeps an active-edge list rather than testing every edge against every
// scanline. The old form was O(scanlines x points), which on a country-sized
// ring — thousands of points spanning hundreds of scanlines — dominated the
// first rasterisation of a new view (belluikit/bellui#40).
func RasterizePolygon(img *image.NRGBA, points [][2]int, c color.NRGBA) {
	var r Rasterizer
	r.Polygon(img, points, c)
}

// Polygon fills a polygon, reusing the rasterizer's buffers.
func (r *Rasterizer) Polygon(img *image.NRGBA, points [][2]int, c color.NRGBA) {
	if len(points) < 3 {
		return
	}

	bounds := img.Bounds()

	minY, maxY := points[0][1], points[0][1]
	for _, p := range points {
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}
	if minY < bounds.Min.Y {
		minY = bounds.Min.Y
	}
	if maxY >= bounds.Max.Y {
		maxY = bounds.Max.Y - 1
	}
	if minY > maxY {
		return
	}

	// Collect the non-horizontal edges, each oriented top-to-bottom, then order
	// them by their top so the sweep can admit them with a single cursor.
	r.edges = r.edges[:0]
	n := len(points)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		y1, y2 := points[i][1], points[j][1]
		x1, x2 := points[i][0], points[j][0]
		if y1 == y2 {
			continue // horizontal edges contribute no crossing
		}
		if y1 > y2 {
			y1, y2 = y2, y1
			x1, x2 = x2, x1
		}
		if y2 <= minY || y1 > maxY {
			continue // never crosses a scanline we will visit
		}
		r.edges = append(r.edges, edge{yTop: y1, yBot: y2, xTop: x1, xBot: x2})
	}
	if len(r.edges) == 0 {
		return
	}
	sort.Slice(r.edges, func(a, b int) bool { return r.edges[a].yTop < r.edges[b].yTop })

	r.active = r.active[:0]
	next := 0
	for y := minY; y <= maxY; y++ {
		// Admit edges that start at or above this scanline.
		for next < len(r.edges) && r.edges[next].yTop <= y {
			r.active = append(r.active, int32(next))
			next++
		}

		// Retire edges that end at or above it, and collect the crossings of
		// those that remain. The original test was `y >= y1 && y < y2`.
		r.xs = r.xs[:0]
		live := r.active[:0]
		for _, idx := range r.active {
			e := r.edges[idx]
			if e.yBot <= y {
				continue
			}
			live = append(live, idx)
			r.xs = append(r.xs, e.xAt(y))
		}
		r.active = live
		if len(r.xs) < 2 {
			continue
		}
		sort.Ints(r.xs)

		for k := 0; k+1 < len(r.xs); k += 2 {
			fillSpan(img, bounds, y, r.xs[k], r.xs[k+1], c)
		}
	}
}

// fillSpan paints the inclusive horizontal run [xStart, xEnd] on scanline y,
// writing straight into the pixel buffer rather than through per-pixel
// coordinate arithmetic and bounds checks.
func fillSpan(img *image.NRGBA, bounds image.Rectangle, y, xStart, xEnd int, c color.NRGBA) {
	if xStart < bounds.Min.X {
		xStart = bounds.Min.X
	}
	if xEnd >= bounds.Max.X {
		xEnd = bounds.Max.X - 1
	}
	if xStart > xEnd {
		return
	}

	base := (y-bounds.Min.Y)*img.Stride + (xStart-bounds.Min.X)*4
	row := img.Pix[base : base+(xEnd-xStart+1)*4]
	for i := 0; i < len(row); i += 4 {
		row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, c.A
	}
}
