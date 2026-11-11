package rasterx

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

func pt(x, y float64) fixed.Point26_6 {
	return fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
}

// TestScannerGV_SizedToPath pins #1137: a path is rasterized within its own
// extent, so a small path costs its own pixels, not the image's.
func TestScannerGV_SizedToPath(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2000, 2000))
	s := NewScannerGV(2000, 2000, img, img.Bounds())
	s.SetColor(color.Black)
	s.Start(pt(1000.5, 1000.5))
	s.Line(pt(1002.5, 1000.5))
	s.Line(pt(1002.5, 1002.5))
	s.Line(pt(1000.5, 1002.5))
	s.Line(pt(1000.5, 1000.5))
	s.Draw()

	if got := s.r.Size(); got.X > 4 || got.Y > 4 {
		t.Errorf("rasterizer for a 2 px square is %v, want at most 4x4", got)
	}
	if _, _, _, a := img.At(1001, 1001).RGBA(); a != 0xffff {
		t.Errorf("inside the square alpha = %#x, want opaque", a)
	}
	if _, _, _, a := img.At(999, 1001).RGBA(); a != 0 {
		t.Errorf("left of the square alpha = %#x, want none", a)
	}
}

// TestScannerGV_MatchesWholeImage draws paths inside, across each edge of
// and outside the image, and compares each with what a rasterizer the size
// of the whole image draws, as the scanner did before #1137. Below 512 px the
// rasterizer's math is the same either way, so the masks must be identical.
// Each path is closed, as the Filler closes every path it hands the scanner.
func TestScannerGV_MatchesWholeImage(t *testing.T) {
	const w, h = 120, 90
	paths := map[string][]fixed.Point26_6{
		"inside":         {pt(10.3, 10.7), pt(40.2, 12.1), pt(25.5, 50.9)},
		"across left":    {pt(-20.5, 30.2), pt(30.1, 20.6), pt(10.4, 60.3)},
		"across top":     {pt(50.5, -15.2), pt(80.1, 20.6), pt(30.4, 25.3)},
		"across right":   {pt(100.5, 30.2), pt(150.1, 40.6), pt(110.4, 70.3)},
		"across bottom":  {pt(40.5, 70.2), pt(90.1, 120.6), pt(20.4, 110.3)},
		"covers all":     {pt(-10, -10), pt(200, -10), pt(200, 200), pt(-10, 200)},
		"outside":        {pt(-50, -50), pt(-10, -50), pt(-30, -10)},
		"tiny":           {pt(60.25, 45.25), pt(60.75, 45.25), pt(60.5, 45.75)},
		"on a pixel row": {pt(5, 5), pt(115, 5), pt(115, 6), pt(5, 6)},
	}
	for name, path := range paths {
		path = append(path, path[0])
		t.Run(name, func(t *testing.T) {
			got := image.NewRGBA(image.Rect(0, 0, w, h))
			s := NewScannerGV(w, h, got, got.Bounds())
			s.SetColor(ColorFunc(func(x, y int) color.Color { return color.NRGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xff} }))
			s.Start(path[0])
			for _, p := range path[1:] {
				s.Line(p)
			}
			s.Draw()

			want := image.NewRGBA(image.Rect(0, 0, w, h))
			var r vector.Rasterizer
			r.Reset(w, h)
			r.MoveTo(float32(path[0].X)/64, float32(path[0].Y)/64)
			for _, p := range path[1:] {
				r.LineTo(float32(p.X)/64, float32(p.Y)/64)
			}
			r.Draw(want, want.Bounds(), s.Source, image.Point{})

			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if g, e := got.RGBAAt(x, y), want.RGBAAt(x, y); g != e {
						t.Fatalf("pixel (%d, %d) = %v, want %v", x, y, g, e)
					}
				}
			}
		})
	}
}
