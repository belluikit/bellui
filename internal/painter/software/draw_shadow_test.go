package software_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// paintShadow paints s alone on a transparent 300×200 canvas.
func paintShadow(t *testing.T, s *canvas.BoxShadow) image.Image {
	t.Helper()
	c := test.NewTransparentCanvasWithPainter(software.NewPainter())
	c.SetPadded(false)
	c.SetContent(container.NewWithoutLayout(s))
	c.Resize(geo.NewSize(300, 200))
	return c.Capture()
}

func alphaAt(img image.Image, x, y int) float64 {
	return float64(color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A) / 0xff
}

// TestPainter_paintBoxShadow pins #876: the software painter drew no
// canvas.BoxShadow at all. Across a straight edge far from the others, a shape
// blurred by a Gaussian of sigma s is the normal CDF of the distance over s,
// which is what the GPU painters' 9-tap kernel approximates (#442).
func TestPainter_paintBoxShadow(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 8) // sigma 4
	s.Move(geo.NewPos(100, 50))
	s.Resize(geo.NewSize(100, 100))
	img := paintShadow(t, s)

	const sigma = 4.0
	for x := 80; x <= 120; x++ {
		d := float64(x) + 0.5 - 100 // pixel centre to the left edge
		want := 0.5 * math.Erfc(-d/(sigma*math.Sqrt2))
		if got := alphaAt(img, x, 100); math.Abs(got-want) > 3.0/255 {
			t.Errorf("alpha at x=%d, %.1f px from the edge: %.3f; want %.3f", x, d, got, want)
		}
	}
	if a := alphaAt(img, 150, 100); a < 0.99 {
		t.Errorf("alpha %.3f under the middle of the shadow; want it solid", a)
	}
	// Past the padding (twice the blur radius) nothing is drawn.
	if a := alphaAt(img, 100-17, 100); a != 0 {
		t.Errorf("alpha %.3f outside the shadow's padded box; want none", a)
	}
}

// TestPainter_paintBoxShadowOffsetAndColor: the offset moves the shadow, and a
// translucent colour keeps its hue and scales the alpha.
func TestPainter_paintBoxShadowOffsetAndColor(t *testing.T) {
	s := canvas.NewBoxShadow(color.NRGBA{R: 0x20, G: 0x40, B: 0x80, A: 0x80}, 4)
	s.Move(geo.NewPos(100, 50))
	s.Resize(geo.NewSize(100, 100))
	s.OffsetX, s.OffsetY = 10, 6
	img := paintShadow(t, s)

	mid := color.NRGBAModel.Convert(img.At(160, 106)).(color.NRGBA)
	if mid.R != 0x20 || mid.G != 0x40 || mid.B != 0x80 || mid.A < 0x7e || mid.A > 0x81 {
		t.Errorf("middle of the shadow is %v; want the colour at its own alpha", mid)
	}
	// The left edge has moved to x=110: half coverage there, none to its left.
	if a := alphaAt(img, 109, 106) + alphaAt(img, 110, 106); math.Abs(a/2-0.25) > 0.03 {
		t.Errorf("alpha across x=110 averages %.3f; want the edge's half of 0.5", a/2)
	}
	if a := alphaAt(img, 100, 106); a > 0.05 {
		t.Errorf("alpha %.3f at the unshifted edge; want the shadow moved off it", a)
	}
}

// TestPainter_paintBoxShadowNoBlur: as on the GPU painters, a shadow with no
// blur draws nothing.
func TestPainter_paintBoxShadowNoBlur(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 0)
	s.Move(geo.NewPos(100, 50))
	s.Resize(geo.NewSize(100, 100))
	if a := alphaAt(paintShadow(t, s), 150, 100); a != 0 {
		t.Errorf("alpha %.3f from a shadow with no blur; want nothing drawn", a)
	}
}
