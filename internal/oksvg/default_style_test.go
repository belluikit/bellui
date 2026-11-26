package oksvg_test

import (
	"image"
	"strings"
	"testing"

	. "github.com/belluikit/bellui/internal/oksvg"
	. "github.com/belluikit/bellui/internal/rasterx"
)

// drawDefault draws one path, given its d and its stroke attributes, on a
// 100×100 viewBox at 100×100 px.
func drawDefault(t *testing.T, d, attrs string) *image.RGBA {
	t.Helper()
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">` +
		`<path d="` + d + `" fill="none" stroke="#000" ` + attrs + `/></svg>`
	icon, err := ReadIconStream(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	icon.SetTarget(0, 0, 100, 100)
	icon.Draw(NewDasher(100, 100, NewScannerGV(100, 100, img, img.Bounds())), 1)
	return img
}

// A stroke with no stroke-linejoin has mitred corners, SVG's initial value,
// as a browser draws them. It used to be bevelled, which cut off each outer
// corner (#1478).
func TestDefaultJoinIsMiter(t *testing.T) {
	// The stroke's outer edge runs along x and y = 16 and 84. A bevel cuts
	// the corner pixels off; a miter fills them.
	img := drawDefault(t, "M20 20H80V80H20Z", `stroke-width="8"`)
	for _, p := range []image.Point{{16, 16}, {83, 16}, {16, 83}, {83, 83}} {
		if a := img.RGBAAt(p.X, p.Y).A; a < 128 {
			t.Errorf("corner pixel %v has alpha %d, want it inked by a mitred join", p, a)
		}
	}
}

// A corner sharper than the miter limit falls back to a bevel, as in SVG,
// rather than being clipped at the limit.
func TestDefaultJoinBevelsPastMiterLimit(t *testing.T) {
	// The apex at (50, 10) turns by about 166°, so a miter would reach 8
	// stroke widths past it, double the default limit of 4. A bevel stops
	// within a pixel of the apex; a miter clipped at the limit (miter-clip)
	// reaches y = 2.
	img := drawDefault(t, "M40 90L50 10L60 90", `stroke-width="4"`)
	for y := 0; y < 8; y++ {
		if a := img.RGBAAt(50, y).A; a != 0 {
			t.Errorf("pixel (50, %d) above the apex has alpha %d, want a bevel that stops at the apex", y, a)
		}
	}
}

// A stroke with no stroke-width is 1 unit wide, SVG's initial value, as a
// browser draws it. It used to be 2 (#1478).
func TestDefaultStrokeWidthIsOne(t *testing.T) {
	img := drawDefault(t, "M10 50H90", ``)
	// The total coverage down a column is the stroke's width in pixels.
	coverage := 0.0
	for y := 0; y < 100; y++ {
		coverage += float64(img.RGBAAt(50, y).A) / 255
	}
	if coverage < 0.9 || coverage > 1.1 {
		t.Errorf("stroke covers %.2f px down a column, want 1", coverage)
	}
}
