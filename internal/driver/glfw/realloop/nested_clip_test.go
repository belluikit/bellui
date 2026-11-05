//go:build darwin && !ci

package realloop_test

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// TestNestedRoundedClipsDraw pins #1012 in a real window, under WebGPU and
// under -tags gl: a 200x200 rounded clip (radius 40) holds a clip of its own,
// then a red fill larger than itself. Once the inner clip has ended the outer
// one still rounds its corners; before, the outer's corners read red. An
// inner clip sticking out past the outer one is cut by it. Points are read in
// the outer clip's own coordinates.
func TestNestedRoundedClipsDraw(t *testing.T) {
	red := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		return r>>8 > 200 && g>>8 < 60 && b>>8 < 60
	}
	for _, tc := range []struct {
		name string
		// inner returns what goes inside the outer clip, before the red fill.
		inner func() core.CanvasObject
		// redAt lists points, besides the centre, that must read red.
		redAt [][2]float32
	}{
		{"rounded clip, then a fill", func() core.CanvasObject {
			blue := canvas.NewRectangle(color.NRGBA{B: 255, A: 255})
			blue.Resize(geo.NewSize(60, 60))
			c := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 10, Objects: []core.CanvasObject{blue}}
			c.Move(geo.NewPos(70, 70))
			c.Resize(geo.NewSize(60, 60))
			return c
		}, nil},
		{"rounded clip sticking out past the outer", func() core.CanvasObject {
			// Square, so only the outer clip can round the corner at (2,2).
			fill := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
			fill.Resize(geo.NewSize(120, 120))
			c := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 1, Objects: []core.CanvasObject{fill}}
			c.Move(geo.NewPos(-20, -20))
			c.Resize(geo.NewSize(120, 120))
			return c
		}, [][2]float32{{60, 60}}},
		{"path clip, then a fill", func() core.CanvasObject {
			blue := canvas.NewRectangle(color.NRGBA{B: 255, A: 255})
			blue.Resize(geo.NewSize(60, 60))
			w := newPathClipWidget(blue)
			w.Move(geo.NewPos(70, 70))
			w.Resize(geo.NewSize(60, 60))
			return w
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fill := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
			fill.Move(geo.NewPos(-60, -60))
			fill.Resize(geo.NewSize(320, 320))
			outer := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 40, Objects: []core.CanvasObject{tc.inner(), fill}}
			outer.Move(geo.NewPos(50, 50))
			outer.Resize(geo.NewSize(200, 200))

			img, scale, origin := shootClip(t, outer)
			at := func(x, y float32) color.Color {
				b := img.Bounds()
				return img.At(b.Min.X+int((origin.X+x)*scale), b.Min.Y+int((origin.Y+y)*scale))
			}
			if c := at(100, 100); !red(c) {
				t.Fatalf("centre reads %v, want red: the test cannot see the fill", c)
			}
			for _, p := range tc.redAt {
				if c := at(p[0], p[1]); !red(c) {
					t.Errorf("(%g,%g) reads %v, want red", p[0], p[1], c)
				}
			}
			for _, p := range [][2]float32{{2, 2}, {197, 2}, {2, 197}, {197, 197}} {
				if c := at(p[0], p[1]); red(c) {
					t.Errorf("the outer clip's corner (%g,%g) reads %v: it stopped rounding after the inner clip", p[0], p[1], c)
				}
			}
		})
	}
}

// shootClip shows content in a 300x300 window and returns its capture, the
// pixel scale, and where content sits in the canvas.
func shootClip(t *testing.T, content core.CanvasObject) (image.Image, float32, geo.Position) {
	t.Helper()
	var w bellui.Window
	bellui.DoAndWait(func() {
		w = testApp.NewWindow("nested clips")
		w.SetContent(container.NewWithoutLayout(content))
		w.Resize(geo.NewSize(300, 300))
		w.Show()
	})
	defer bellui.DoAndWait(w.Hide) // closing the last window quits the app
	time.Sleep(500 * time.Millisecond)
	var img image.Image
	var scale float32
	var origin geo.Position
	bellui.DoAndWait(func() {
		img = w.Canvas().Capture()
		scale = float32(img.Bounds().Dx()) / w.Canvas().Size().Width
		origin = testApp.Driver().AbsolutePositionForObject(content)
	})
	return img, scale, origin
}

// pathClipWidget clips its one object to a triangle through its renderer's
// ClipPath.
type pathClipWidget struct {
	widget.BaseWidget
	fill core.CanvasObject
	path *canvas.Path
}

func newPathClipWidget(fill core.CanvasObject) *pathClipWidget {
	w := &pathClipWidget{fill: fill, path: canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(60, 0)
		p.LineTo(0, 60)
		p.Close()
	})}
	w.ExtendBaseWidget(w)
	return w
}

func (w *pathClipWidget) CreateRenderer() core.WidgetRenderer {
	return &pathClipRenderer{w: w}
}

type pathClipRenderer struct{ w *pathClipWidget }

func (r *pathClipRenderer) Destroy()                     {}
func (r *pathClipRenderer) Layout(geo.Size)              {}
func (r *pathClipRenderer) MinSize() geo.Size            { return geo.Size{} }
func (r *pathClipRenderer) Objects() []core.CanvasObject { return []core.CanvasObject{r.w.fill} }
func (r *pathClipRenderer) Refresh()                     {}
func (r *pathClipRenderer) ClipPath() any                { return r.w.path }
