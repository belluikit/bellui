package software_test

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestPainter_drawTransformed_clipped pins #1190: an object scaled past its
// clipping container's bounds is cut at them, as an untransformed one is.
func TestPainter_drawTransformed_clipped(t *testing.T) {
	r := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
	r.Resize(geo.NewSize(100, 100))
	// 200×200, centred on the box.
	r.SetTransform(geo.NewTranslation(50, 50).Multiply(geo.NewScale(2, 2)).Multiply(geo.NewTranslation(-50, -50)))
	box := container.NewWithoutLayout(r)
	box.ClipToBounds = true
	box.Resize(geo.NewSize(100, 100))
	box.Move(geo.NewPos(100, 100))

	c := test.NewCanvas()
	c.SetPadded(false)
	c.SetContent(container.NewWithoutLayout(box))
	c.Resize(geo.NewSize(300, 300))
	img := software.NewPainter().Paint(c)

	w, h := redExtent(img)
	assert.Equal(t, 100, w, "the scaled rect is cut at the box's left and right")
	assert.Equal(t, 100, h, "the scaled rect is cut at the box's top and bottom")
	_, _, _, a := img.At(150, 150).RGBA()
	assert.NotZero(t, a, "the box itself is drawn")
	_, _, _, a = img.At(60, 150).RGBA()
	assert.Zero(t, a, "nothing is drawn left of the box")
}

// inkBox returns the bounding box and count of img's visibly drawn pixels.
func inkBox(img image.Image) (image.Rectangle, int) {
	b := img.Bounds()
	box, n := image.Rectangle{}, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x1000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
				n++
			}
		}
	}
	return box, n
}

// TestPainter_drawTransformed_paintExtent pins #1192: an object drawn through
// drawTransformed was rendered into a layer its own size, so a shadow drew
// nothing and a stroke, a thick line or an overflowing text was cut at the
// object's bounds. A scale too small to move a pixel takes that path, and must
// draw what the untransformed object draws, at every canvas scale.
func TestPainter_drawTransformed_paintExtent(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	cases := map[string]func() core.CanvasObject{
		"shadow": func() core.CanvasObject { return canvas.NewBoxShadow(color.Black, 8) },
		"offset shadow": func() core.CanvasObject {
			s := canvas.NewBoxShadow(color.Black, 4)
			s.OffsetX, s.OffsetY = 6, -5
			return s
		},
		"stroked rectangle": func() core.CanvasObject {
			r := canvas.NewRectangle(red)
			r.StrokeColor, r.StrokeWidth = red, 8
			return r
		},
		"stroked circle": func() core.CanvasObject {
			r := canvas.NewCircle(red)
			r.StrokeColor, r.StrokeWidth = red, 8
			return r
		},
		"line": func() core.CanvasObject { l := canvas.NewLine(red); l.StrokeWidth = 8; return l },
		"overflowing text": func() core.CanvasObject {
			return canvas.NewText("Wider than its box", red)
		},
		"scene3d": func() core.CanvasObject { s := canvas.NewScene3D(); s.Background = red; return s },
	}
	paint := func(obj core.CanvasObject, scale float32) image.Image {
		c := test.NewTransparentCanvasWithPainter(software.NewPainter())
		c.SetPadded(false)
		c.SetScale(scale)
		c.SetContent(container.NewWithoutLayout(obj))
		c.Resize(geo.NewSize(200, 200))
		return c.Capture()
	}
	for name, mk := range cases {
		for _, scale := range []float32{1, 1.5, 2} {
			t.Run(fmt.Sprintf("%s at %v", name, scale), func(t *testing.T) {
				plainObj, transObj := mk(), mk()
				for _, o := range []core.CanvasObject{plainObj, transObj} {
					o.Resize(geo.NewSize(40, 40))
					o.Move(geo.NewPos(80, 80))
				}
				transObj.(core.Transformable).SetTransform(geo.NewScale(1.0001, 1))

				plain, plainN := inkBox(paint(plainObj, scale))
				trans, transN := inkBox(paint(transObj, scale))
				require.NotZero(t, plainN, "the untransformed object drew nothing, so the check would prove nothing")
				assert.InDelta(t, plain.Min.X, trans.Min.X, 1, "left edge: plain %v, transformed %v", plain, trans)
				assert.InDelta(t, plain.Min.Y, trans.Min.Y, 1, "top edge: plain %v, transformed %v", plain, trans)
				assert.InDelta(t, plain.Max.X, trans.Max.X, 1, "right edge: plain %v, transformed %v", plain, trans)
				assert.InDelta(t, plain.Max.Y, trans.Max.Y, 1, "bottom edge: plain %v, transformed %v", plain, trans)
				assert.InEpsilon(t, plainN, transN, 0.05, "drawn pixels: plain %d, transformed %d", plainN, transN)
			})
		}
	}
}

// TestPainter_drawTransformed_rotatedShadow: a shadow under a quarter turn is
// drawn turned, its blur included (#1192).
func TestPainter_drawTransformed_rotatedShadow(t *testing.T) {
	s := canvas.NewBoxShadow(color.Black, 4) // pads by 8 on each side
	s.Resize(geo.NewSize(80, 20))
	s.Move(geo.NewPos(60, 90))
	s.SetTransform(geo.NewRotationAround(math.Pi/2, 40, 10)) // about its centre: 20×80 at (90, 60)

	c := test.NewTransparentCanvasWithPainter(software.NewPainter())
	c.SetPadded(false)
	c.SetContent(container.NewWithoutLayout(s))
	c.Resize(geo.NewSize(200, 200))
	img := c.Capture()

	box, _ := inkBox(img)
	assert.Greater(t, box.Dy(), box.Dx()+40, "the shadow is turned upright: %v", box)
	assert.Less(t, box.Min.X, 90-2, "the blur spreads left of the turned box: %v", box)
	assert.Greater(t, box.Max.X, 110+2, "the blur spreads right of the turned box: %v", box)
	if a := alphaAt(img, 100, 100); a < 0.99 {
		t.Errorf("alpha %.3f in the middle of the turned shadow; want it solid", a)
	}
	if a := alphaAt(img, 90, 100); a < 0.3 || a > 0.7 {
		t.Errorf("alpha %.3f on the turned box's edge; want the blur's half", a)
	}
}
