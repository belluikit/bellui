package oksvg_test

import (
	"image"
	"math"
	"strings"
	"testing"

	. "github.com/belluikit/bellui/internal/oksvg"
	. "github.com/belluikit/bellui/internal/rasterx"
)

// drawStroked draws src at 100×100 and returns the image.
func drawStroked(t *testing.T, src string) *image.RGBA {
	t.Helper()
	icon, err := ReadIconStream(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	icon.SetTarget(0, 0, 100, 100)
	icon.Draw(NewDasher(100, 100, NewScannerGV(100, 100, img, img.Bounds())), 1)
	return img
}

// inked counts the pixels at least half covered along a row (dx=1) or a
// column (dy=1) starting at (x, y).
func inked(img *image.RGBA, x, y, dx, dy int) int {
	n := 0
	for ; x < 100 && y < 100; x, y = x+dx, y+dy {
		if img.RGBAAt(x, y).A >= 128 {
			n++
		}
	}
	return n
}

// A stroke under a non-uniform transform is stretched with the shape, as a
// browser draws it: an 8-unit stroke under scale(0.5 2) is 4 px wide on the
// vertical sides and 16 px on the horizontal ones (#1295). It used to be one
// width, the geometric mean, 8 px on every side.
func TestStrokeStretchesWithNonUniformTransform(t *testing.T) {
	// The rectangle lands on x 10..60 and y 20..80.
	img := drawStroked(t, `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">`+
		`<g transform="scale(0.5 2)"><path d="M20 10H120V40H20Z" fill="none" stroke="#000" stroke-width="8"/></g></svg>`)

	// Row y=50 crosses the two vertical sides, 4 px each.
	if got := inked(img, 0, 50, 1, 0); got < 7 || got > 9 {
		t.Errorf("vertical sides: %d px across both, want 8 (4 each)", got)
	}
	// Column x=35 crosses the two horizontal sides, 16 px each.
	if got := inked(img, 35, 0, 0, 1); got < 31 || got > 33 {
		t.Errorf("horizontal sides: %d px across both, want 32 (16 each)", got)
	}
}

// A thin stroke under a strong stretch is still drawn. The stroke is built at
// the transform's larger scale and then shrunk along the other axis. Built at
// the smaller scale instead, a 0.25-unit stroke under scale(0.1 8) is 0.025
// px wide, which rounds to nothing in the dasher's 1/64 px units, and the
// whole curve vanished.
func TestThinStrokeSurvivesStrongStretch(t *testing.T) {
	// A circle of radius 5 at (500, 6.25) becomes an ellipse at (50, 50)
	// with semi-axes 0.5 and 40; the stroke is 2 px tall at the top and
	// bottom.
	img := drawStroked(t, `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">`+
		`<g transform="scale(0.1 8)"><path d="M505 6.25A5 5 0 1 1 495 6.25A5 5 0 1 1 505 6.25Z" fill="none" stroke="#000" stroke-width="0.25"/></g></svg>`)
	missed := 0
	for deg := 0; deg < 360; deg++ {
		a := float64(deg) * math.Pi / 180
		x, y := 0.1*(500+5*math.Cos(a)), 8*(6.25+5*math.Sin(a))
		if img.RGBAAt(int(x), int(y)).A == 0 {
			missed++
		}
	}
	if missed > 0 {
		t.Errorf("%d of 360 points on the ellipse's centre line are not inked", missed)
	}
}

// A uniform scale strokes as before: the width scales by the same factor on
// every side (#1292).
func TestStrokeUnderUniformScale(t *testing.T) {
	// The rectangle lands on x 20..80 and y 20..80; the stroke is 8 px.
	img := drawStroked(t, `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">`+
		`<g transform="scale(2 2)"><path d="M10 10H40V40H10Z" fill="none" stroke="#000" stroke-width="4"/></g></svg>`)
	if got := inked(img, 0, 50, 1, 0); got < 15 || got > 17 {
		t.Errorf("vertical sides: %d px across both, want 16 (8 each)", got)
	}
	if got := inked(img, 50, 0, 0, 1); got < 15 || got > 17 {
		t.Errorf("horizontal sides: %d px across both, want 16 (8 each)", got)
	}
}
