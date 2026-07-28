package geojson

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"testing"
)

// rasterizePolygonReference is the original per-scanline implementation, kept as
// a correctness oracle for the active-edge sweep that replaced it. Every change
// to RasterizePolygon must stay byte-identical to this.
func rasterizePolygonReference(img *image.NRGBA, points [][2]int, c color.NRGBA) {
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

	n := len(points)
	for y := minY; y <= maxY; y++ {
		var intersections []int

		for i := 0; i < n; i++ {
			j := (i + 1) % n
			y1, y2 := points[i][1], points[j][1]
			x1, x2 := points[i][0], points[j][0]

			if y1 == y2 {
				continue
			}
			if y1 > y2 {
				y1, y2 = y2, y1
				x1, x2 = x2, x1
			}
			if y >= y1 && y < y2 {
				intersections = append(intersections, x1+(y-y1)*(x2-x1)/(y2-y1))
			}
		}

		sort.Ints(intersections)

		for k := 0; k+1 < len(intersections); k += 2 {
			xStart, xEnd := intersections[k], intersections[k+1]
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

func regularPolygon(cx, cy, r float64, points int) [][2]int {
	ring := make([][2]int, points)
	for i := range ring {
		a := 2 * math.Pi * float64(i) / float64(points)
		ring[i] = [2]int{int(cx + r*math.Cos(a)), int(cy + r*math.Sin(a))}
	}
	return ring
}

func assertSameRaster(t *testing.T, name string, w, h int, ring [][2]int) {
	t.Helper()
	c := color.NRGBA{R: 10, G: 200, B: 30, A: 255}

	got := image.NewNRGBA(image.Rect(0, 0, w, h))
	RasterizePolygon(got, ring, c)

	want := image.NewNRGBA(image.Rect(0, 0, w, h))
	rasterizePolygonReference(want, ring, c)

	for i := range want.Pix {
		if got.Pix[i] != want.Pix[i] {
			px := (i % want.Stride) / 4
			py := i / want.Stride
			t.Fatalf("%s: pixel (%d,%d) differs from the reference rasterizer", name, px, py)
		}
	}
}

func TestRasterizePolygon_MatchesReference(t *testing.T) {
	assertSameRaster(t, "square", 64, 64, [][2]int{{8, 8}, {40, 8}, {40, 40}, {8, 40}})
	assertSameRaster(t, "triangle", 64, 64, [][2]int{{4, 60}, {32, 4}, {60, 60}})
	assertSameRaster(t, "circle", 200, 200, regularPolygon(100, 100, 80, 512))
	assertSameRaster(t, "concave", 64, 64, [][2]int{{2, 2}, {60, 2}, {60, 60}, {32, 20}, {2, 60}})

	// Clipping on all four sides, which the fill path clamps by hand.
	assertSameRaster(t, "overflow", 64, 64, [][2]int{{-40, -40}, {120, -40}, {120, 120}, {-40, 120}})
	assertSameRaster(t, "left", 64, 64, [][2]int{{-80, 10}, {-20, 10}, {-20, 50}, {-80, 50}})
	assertSameRaster(t, "right", 64, 64, [][2]int{{100, 10}, {160, 10}, {160, 50}, {100, 50}})
	assertSameRaster(t, "above", 64, 64, [][2]int{{10, -80}, {50, -80}, {50, -20}, {10, -20}})
	assertSameRaster(t, "below", 64, 64, [][2]int{{10, 100}, {50, 100}, {50, 160}, {10, 160}})

	// Degenerate inputs must be no-ops, not panics.
	assertSameRaster(t, "collinear", 64, 64, [][2]int{{5, 20}, {30, 20}, {55, 20}})
	assertSameRaster(t, "two points", 64, 64, [][2]int{{5, 5}, {30, 30}})
	assertSameRaster(t, "empty", 64, 64, nil)
}

// Random rings are where a sweep diverges from a full rescan if the active-edge
// bookkeeping is wrong — self-intersections, coincident vertices, spans that
// begin above the clip and end below it.
func TestRasterizePolygon_MatchesReference_Random(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 300; i++ {
		n := 3 + rng.Intn(30)
		ring := make([][2]int, n)
		for j := range ring {
			ring[j] = [2]int{rng.Intn(160) - 40, rng.Intn(160) - 40}
		}
		assertSameRaster(t, "random", 80, 80, ring)
	}
}

// The rasterizer reuses its buffers across calls; a second polygon must not see
// the first one's state.
func TestRasterizer_ReuseAcrossPolygons(t *testing.T) {
	c := color.NRGBA{R: 1, G: 2, B: 3, A: 255}
	rings := [][][2]int{
		regularPolygon(50, 50, 30, 64),
		{{5, 5}, {20, 5}, {20, 20}, {5, 20}},
		regularPolygon(60, 60, 10, 8),
	}

	var r Rasterizer
	got := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	want := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for _, ring := range rings {
		r.Polygon(got, ring, c)
		rasterizePolygonReference(want, ring, c)
	}

	for i := range want.Pix {
		if got.Pix[i] != want.Pix[i] {
			t.Fatalf("reused rasterizer diverged at byte %d", i)
		}
	}
}

func benchRing() [][2]int { return regularPolygon(450, 300, 280, 2000) }

func BenchmarkRasterizePolygon(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, 900, 600))
	ring := benchRing()
	c := color.NRGBA{R: 1, G: 2, B: 3, A: 255}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RasterizePolygon(img, ring, c)
	}
}

// BenchmarkRasterizePolygonReference is the cost before the active-edge sweep.
func BenchmarkRasterizePolygonReference(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, 900, 600))
	ring := benchRing()
	c := color.NRGBA{R: 1, G: 2, B: 3, A: 255}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rasterizePolygonReference(img, ring, c)
	}
}
