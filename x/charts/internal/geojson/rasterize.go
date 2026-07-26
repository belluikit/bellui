package geojson

import (
	"image"
	"image/color"
	"sort"
)

// RasterizePolygon fills a polygon on an NRGBA image using scanline rasterization.
// Points are in pixel coordinates.
func RasterizePolygon(img *image.NRGBA, points [][2]int, c color.NRGBA) {
	if len(points) < 3 {
		return
	}

	bounds := img.Bounds()

	// Find Y range
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

	n := len(points)

	// Scanline fill
	for y := minY; y <= maxY; y++ {
		var intersections []int

		for i := 0; i < n; i++ {
			j := (i + 1) % n
			y1, y2 := points[i][1], points[j][1]
			x1, x2 := points[i][0], points[j][0]

			if y1 == y2 {
				continue // skip horizontal edges
			}

			// Ensure y1 < y2
			if y1 > y2 {
				y1, y2 = y2, y1
				x1, x2 = x2, x1
			}

			if y >= y1 && y < y2 {
				// Compute X intersection
				x := x1 + (y-y1)*(x2-x1)/(y2-y1)
				intersections = append(intersections, x)
			}
		}

		sort.Ints(intersections)

		// Fill between pairs
		for k := 0; k+1 < len(intersections); k += 2 {
			xStart := intersections[k]
			xEnd := intersections[k+1]
			if xStart < bounds.Min.X {
				xStart = bounds.Min.X
			}
			if xEnd >= bounds.Max.X {
				xEnd = bounds.Max.X - 1
			}
			for x := xStart; x <= xEnd; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}
