//go:build darwin && !ci

package realloop_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// TestPathInsideRoundedClipDraws pins #1015 in a real window, under WebGPU
// and under -tags gl: a filled path, and a path clip's red content, larger
// than the 200x200 rounded clip (radius 40) they sit in. The clip's corners
// read the background. Before, a path counted its winding over the clip's
// depth in the stencil, and was confined to the path only. Points are read in
// the clip's own coordinates.
func TestPathInsideRoundedClipDraws(t *testing.T) {
	red := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		return r>>8 > 200 && g>>8 < 60 && b>>8 < 60
	}
	// big is a square from (-60,-60) to (260,260), past the clip on every side.
	big := func() *canvas.Path {
		return canvas.NewPath(func(p *canvas.Path) {
			p.MoveTo(-60, -60)
			p.LineTo(260, -60)
			p.LineTo(260, 260)
			p.LineTo(-60, 260)
			p.Close()
		})
	}
	for _, tc := range []struct {
		name    string
		content func() core.CanvasObject
	}{
		{"path fill", func() core.CanvasObject {
			p := big()
			p.FillColor = color.NRGBA{R: 255, A: 255}
			p.Resize(geo.NewSize(200, 200))
			return p
		}},
		{"path clip's content", func() core.CanvasObject {
			fill := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
			fill.Move(geo.NewPos(-60, -60))
			fill.Resize(geo.NewSize(320, 320))
			w := newPathClipWidget(fill)
			w.path = big()
			w.Resize(geo.NewSize(200, 200))
			return w
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clip := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 40, Objects: []core.CanvasObject{tc.content()}}
			clip.Move(geo.NewPos(50, 50))
			clip.Resize(geo.NewSize(200, 200))

			img, scale, origin := shootClip(t, clip)
			at := func(x, y float32) color.Color {
				b := img.Bounds()
				return img.At(b.Min.X+int((origin.X+x)*scale), b.Min.Y+int((origin.Y+y)*scale))
			}
			for _, p := range [][2]float32{{100, 100}, {100, 2}, {2, 100}} {
				if c := at(p[0], p[1]); !red(c) {
					t.Fatalf("(%g,%g) reads %v, want red: the test cannot see the path", p[0], p[1], c)
				}
			}
			for _, p := range [][2]float32{{2, 2}, {197, 2}, {2, 197}, {197, 197}} {
				if c := at(p[0], p[1]); red(c) {
					t.Errorf("the clip's corner (%g,%g) reads %v: the path was not cut by the rounded clip", p[0], p[1], c)
				}
			}
		})
	}
}

// TestRoundedClipInsidePathClipDraws pins the other nesting of #1015 in a real
// window, under WebGPU and under -tags gl: a path clip holding the left half
// of a 200x200 square holds a rounded clip larger than itself, and a red fill
// larger still. The right half reads the background. Before, the rounded clip
// reset the stencil and the path clip was gone.
func TestRoundedClipInsidePathClipDraws(t *testing.T) {
	red := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		return r>>8 > 200 && g>>8 < 60 && b>>8 < 60
	}
	fill := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
	fill.Move(geo.NewPos(-60, -60))
	fill.Resize(geo.NewSize(320, 320))
	rounded := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 10, Objects: []core.CanvasObject{fill}}
	rounded.Resize(geo.NewSize(200, 200))
	w := newPathClipWidget(rounded)
	w.path = canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(100, 0)
		p.LineTo(100, 200)
		p.LineTo(0, 200)
		p.Close()
	})
	w.Move(geo.NewPos(50, 50))
	w.Resize(geo.NewSize(200, 200))

	img, scale, origin := shootClip(t, w)
	at := func(x, y float32) color.Color {
		b := img.Bounds()
		return img.At(b.Min.X+int((origin.X+x)*scale), b.Min.Y+int((origin.Y+y)*scale))
	}
	if c := at(50, 100); !red(c) {
		t.Fatalf("(50,100) reads %v, want red: the test cannot see the fill", c)
	}
	for _, p := range [][2]float32{{150, 100}, {110, 20}, {190, 180}} {
		if c := at(p[0], p[1]); red(c) {
			t.Errorf("(%g,%g), outside the path clip, reads %v: the rounded clip dropped it", p[0], p[1], c)
		}
	}
}
