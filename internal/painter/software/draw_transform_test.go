package software_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// redExtent scans img and returns the width and height of the bounding box of
// clearly-red pixels (the rectangle colour), or (0, 0) if none are found.
func redExtent(img image.Image) (w, h int) {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0x8000 && r > 0xC000 && g < 0x4000 && bl < 0x4000 {
				found = true
				minX, maxX = min(minX, x), max(maxX, x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	if !found {
		return 0, 0
	}
	return maxX - minX + 1, maxY - minY + 1
}

// TestPainter_drawTransformed exercises the software painter's drawTransformed
// path (reached when an object implements core.Transformable with a non-identity
// transform). A tall, thin rectangle rotated 90° about its centre must swap its
// footprint: the rendered red region becomes wide-and-short instead of
// narrow-and-tall. This is robust to anti-aliasing and exact pivot arithmetic —
// it only asserts that the rotation was actually applied by the painter.
func TestPainter_drawTransformed(t *testing.T) {
	newRect := func(transform bool) *canvas.Rectangle {
		r := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		r.Resize(geo.NewSize(20, 60)) // narrow (20) and tall (60)
		r.Move(geo.NewPos(40, 20))
		if transform {
			r.SetTransform(geo.NewRotationAround(math.Pi/2, 10, 30)) // 90° about local centre
		}
		return r
	}

	paint := func(r *canvas.Rectangle) image.Image {
		c := test.NewCanvas()
		c.SetPadded(false)
		c.SetContent(container.NewWithoutLayout(r))
		c.Resize(geo.NewSize(100, 100))
		return software.NewPainter().Paint(c)
	}

	plainW, plainH := redExtent(paint(newRect(false)))
	rotW, rotH := redExtent(paint(newRect(true)))

	// Sanity: the un-transformed rectangle renders as narrow-and-tall.
	assert.Greater(t, plainH, plainW, "control: unrotated rect should be taller than wide")

	// After a 90° rotation the footprint swaps to wide-and-short — proof that
	// drawTransformed applied the transform rather than drawing the object as-is.
	assert.Greater(t, rotW, plainW+15, "rotated rect should be much wider than the unrotated one")
	assert.Greater(t, plainH, rotH+15, "rotated rect should be much shorter than the unrotated one")
}

// drawnPixels counts pixels that are not fully transparent and differ from a
// flat background — enough to confirm an object rendered through the transform.
func drawnPixels(img image.Image) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x2000 {
				n++
			}
		}
	}
	return n
}

// TestPainter_drawTransformed_primitives walks the object type switch in
// drawTransformed: each supported primitive, given a rotation, must render some
// pixels through the transformed path without panicking.
func TestPainter_drawTransformed_primitives(t *testing.T) {
	rot := geo.NewRotation(math.Pi / 4) // 45°: guarantees a non-trivial transform for all shapes
	red := color.NRGBA{R: 255, A: 255}

	cases := map[string]func() core.CanvasObject{
		"rectangle": func() core.CanvasObject { return canvas.NewRectangle(red) },
		"circle":    func() core.CanvasObject { return canvas.NewCircle(red) },
		"line":      func() core.CanvasObject { l := canvas.NewLine(red); l.StrokeWidth = 4; return l },
		"text":      func() core.CanvasObject { return canvas.NewText("Xy", red) },
		"image":     func() core.CanvasObject { return canvas.NewImageFromImage(makeTestImage(40, 40)) },
	}

	for name, make := range cases {
		t.Run(name, func(t *testing.T) {
			obj := make()
			obj.Resize(geo.NewSize(40, 40))
			obj.Move(geo.NewPos(30, 30))
			if tr, ok := obj.(interface{ SetTransform(geo.Matrix) }); ok {
				tr.SetTransform(rot)
			}

			c := test.NewCanvas()
			c.SetPadded(false)
			c.SetContent(container.NewWithoutLayout(obj))
			c.Resize(geo.NewSize(120, 120))

			img := software.NewPainter().Paint(c)
			assert.Greater(t, drawnPixels(img), 0, "%s should render through drawTransformed", name)
		})
	}
}
