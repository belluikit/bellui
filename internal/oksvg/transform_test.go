package oksvg_test

import (
	"image"
	"strings"
	"testing"

	. "github.com/belluikit/bellui/internal/oksvg"
	. "github.com/belluikit/bellui/internal/rasterx"
)

// drawTransformed draws a full-size 24×24 square inside a group carrying the
// given transform, and returns the image.
func drawTransformed(t *testing.T, transform string) *image.RGBA {
	t.Helper()
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">` +
		`<g transform="` + transform + `"><path d="M0 0h24v24H0z"/></g></svg>`
	icon, err := ReadIconStream(strings.NewReader(src))
	if err != nil {
		t.Fatalf("%s: %v", transform, err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 24, 24))
	icon.SetTarget(0, 0, 24, 24)
	icon.Draw(NewDasher(24, 24, NewScannerGV(24, 24, img, img.Bounds())), 1)
	return img
}

func paintedPixels(img *image.RGBA) int {
	n := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			n++
		}
	}
	return n
}

// A one-argument scale(s) scales both axes by s, as the SVG spec says (sy
// defaults to sx). It used to scale y by 0, so the shape vanished (#1474).
func TestScaleOneArgumentScalesBothAxes(t *testing.T) {
	for _, c := range []struct{ one, two string }{
		{"scale(0.5)", "scale(0.5 0.5)"},
		{"scale(.5)", "scale(.5, .5)"},
		{"translate(12 12) scale(0.5)", "translate(12 12) scale(0.5 0.5)"},
	} {
		one, two := drawTransformed(t, c.one), drawTransformed(t, c.two)
		if n := paintedPixels(one); n == 0 {
			t.Errorf("%s drew nothing", c.one)
		}
		if string(one.Pix) != string(two.Pix) {
			t.Errorf("%s drew %d pixels, %s drew %d; want the same image",
				c.one, paintedPixels(one), c.two, paintedPixels(two))
		}
	}
}
