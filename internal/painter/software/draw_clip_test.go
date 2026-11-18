package software_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// A rasterised shape inside a clip that holds all it draws, stroke
// included, draws exactly as it does with no clip (#1330). The raster
// carries padding beyond the stroke (its width + 2), so a clip margin of 3
// cuts the padding's top and left but none of the stroke, and the source
// point must move by what was cut. It moved only for a shape off the
// canvas's top or left, so the shape was drawn shifted right and down, its
// right and bottom edges cut off.
func TestPainter_clippedShapeDrawsInPlace(t *testing.T) {
	test.ApplyTheme(t, test.Theme())
	stroke := color.NRGBA{R: 0xd0, A: 0xff}
	for name, make := range map[string]func() core.CanvasObject{
		"rectangle stroke": func() core.CanvasObject {
			r := canvas.NewRectangle(color.NRGBA{B: 0xd0, A: 0xff})
			r.StrokeColor, r.StrokeWidth, r.CornerRadius = stroke, 2, 12
			return r
		},
		"circle": func() core.CanvasObject {
			c := canvas.NewCircle(color.NRGBA{B: 0xd0, A: 0xff})
			c.StrokeColor, c.StrokeWidth = stroke, 2
			return c
		},
		"arc": func() core.CanvasObject {
			a := canvas.NewArc(0, 270, 0.4, color.NRGBA{B: 0xd0, A: 0xff})
			a.StrokeColor, a.StrokeWidth = stroke, 2
			return a
		},
		"line": func() core.CanvasObject {
			l := canvas.NewLine(stroke)
			l.StrokeWidth = 2
			return l
		},
		"polygon": func() core.CanvasObject {
			p := canvas.NewPolygon(3, stroke)
			p.StrokeColor, p.StrokeWidth = color.NRGBA{G: 0xd0, A: 0xff}, 2
			return p
		},
		"path": func() core.CanvasObject {
			p := &canvas.Path{StrokeColor: stroke, StrokeWidth: 2}
			p.MoveTo(0, 0)
			p.LineTo(60, 20)
			p.LineTo(0, 40)
			p.Close()
			return p
		},
	} {
		t.Run(name, func(t *testing.T) {
			const margin = 3
			pos, size := geo.NewPos(20, 20), geo.NewSize(60, 40)
			paint := func(clipped bool) image.Image {
				obj := make()
				obj.Resize(size)
				var placed core.CanvasObject = obj
				obj.Move(pos)
				if clipped {
					obj.Move(geo.NewPos(margin, margin))
					placed = container.NewClip(container.NewWithoutLayout(obj))
					placed.Move(pos.SubtractXY(margin, margin))
					placed.Resize(size.AddWidthHeight(2*margin, 2*margin))
				}
				c := test.NewCanvas()
				c.SetScale(3)
				c.SetContent(container.NewWithoutLayout(placed))
				c.Resize(geo.NewSize(100, 80))
				return software.NewPainter().Paint(c)
			}
			free, clipped := paint(false), paint(true)

			// The clip's rectangle, in pixels.
			box := image.Rect(3*(20-margin), 3*(20-margin), 3*(80+margin), 3*(60+margin))
			drawn, differ := 0, 0
			var first image.Point
			for y := box.Min.Y; y < box.Max.Y; y++ {
				for x := box.Min.X; x < box.Max.X; x++ {
					a, b := free.At(x, y), clipped.At(x, y)
					if !sameRGBA(a, free.At(0, 0)) {
						drawn++
					}
					if !sameRGBA(a, b) {
						if differ == 0 {
							first = image.Pt(x, y)
						}
						differ++
					}
				}
			}
			if drawn == 0 {
				t.Fatal("the shape drew nothing inside the clip's rectangle, so the comparison proves nothing")
			}
			if differ > 0 {
				t.Errorf("%d of %d pixels inside the clip differ from the unclipped shape, the first at %v: %v clipped, %v unclipped",
					differ, box.Dx()*box.Dy(), first, clipped.At(first.X, first.Y), free.At(first.X, first.Y))
			}
		})
	}
}

func sameRGBA(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
