//go:build darwin && !ci

package realloop_test

import (
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// tiltBox is a widget with a transform, holding its objects where they are put.
type tiltBox struct {
	widget.BaseWidget
	m    geo.Matrix
	objs []core.CanvasObject
}

func newTiltBox(m geo.Matrix, objs ...core.CanvasObject) *tiltBox {
	b := &tiltBox{m: m, objs: objs}
	b.ExtendBaseWidget(b)
	return b
}

func (b *tiltBox) Transform() geo.Matrix     { return b.m }
func (b *tiltBox) SetTransform(m geo.Matrix) { b.m = m }

func (b *tiltBox) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout(b.objs...))
}

// rotatedClip is a 60x60 clip holding content, at (20, 20) in a 100x100 box
// at (40, 40) turned an eighth about its centre.
func rotatedClip(content core.CanvasObject) core.CanvasObject {
	clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{content}}
	clip.Move(geo.NewPos(20, 20))
	clip.Resize(geo.NewSize(60, 60))
	box := newTiltBox(geo.NewRotationAround(math.Pi/4, 50, 50), clip)
	box.Move(geo.NewPos(40, 40))
	box.Resize(geo.NewSize(100, 100))
	return box
}

// TestWorldTransformDraws pins #997 on the GPU painters, WebGPU by default and
// GL under -tags gl: an object is drawn where its world transform puts it,
// translation included. Each case reads one point where the transform puts
// the red rectangle and one where the painters drew it before: without its
// translation, or for a parent's child, turned about its own origin, or past
// the edge of a clip that stayed behind. The first three scenes are the
// software painter's (TestPainter_WorldTransformMovesObjects).
func TestWorldTransformDraws(t *testing.T) {
	red := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		return r>>8 > 200 && g>>8 < 60 && b>>8 < 60
	}
	rect := func(x, y, w, h float32) *canvas.Rectangle {
		r := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		r.Move(geo.NewPos(x, y))
		r.Resize(geo.NewSize(w, h))
		return r
	}

	for _, tc := range []struct {
		name      string
		scene     func() core.CanvasObject
		drawn, no geo.Position
	}{
		{
			name: "translation only",
			scene: func() core.CanvasObject {
				r := rect(20, 10, 20, 20)
				r.SetTransform(geo.NewTranslation(100, 0))
				return r
			},
			drawn: geo.NewPos(130, 20), no: geo.NewPos(30, 20),
		},
		{
			name: "rotation about a pivot",
			scene: func() core.CanvasObject {
				r := rect(50, 50, 40, 20)
				r.SetTransform(geo.NewRotationAround(math.Pi, 20, 10))
				return r
			},
			drawn: geo.NewPos(85, 65), no: geo.NewPos(15, 35),
		},
		{
			name: "transformed parent, offset child",
			scene: func() core.CanvasObject {
				box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), rect(80, 10, 10, 10))
				box.Move(geo.NewPos(40, 40))
				box.Resize(geo.NewSize(100, 100))
				return box
			},
			drawn: geo.NewPos(125, 125), no: geo.NewPos(115, 55),
		},
		{
			// The same scene with the child in a RepaintBoundary: under a
			// rotation the boundary draws its content directly, turned with
			// its parent (#1003). Its texture, blitted axis-aligned from the
			// rotated corner, would cover (130, 120)-(140, 130) instead.
			name: "boundary under a rotation",
			scene: func() core.CanvasObject {
				b := container.NewRepaintBoundary(rect(0, 0, 10, 10))
				b.Move(geo.NewPos(80, 10))
				b.Resize(geo.NewSize(10, 10))
				box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), b)
				box.Move(geo.NewPos(40, 40))
				box.Resize(geo.NewSize(100, 100))
				return box
			},
			drawn: geo.NewPos(125, 125), no: geo.NewPos(135, 125),
		},
		{
			// A clipping container moved by its parent's transform clips its
			// larger child where it is drawn: (110, 10)-(150, 50).
			name: "clip under a transform",
			scene: func() core.CanvasObject {
				clip := &bellui.Container{ClipToBounds: true, Objects: []core.CanvasObject{rect(0, 0, 80, 80)}}
				clip.Move(geo.NewPos(10, 10))
				clip.Resize(geo.NewSize(40, 40))
				box := newTiltBox(geo.NewTranslation(100, 0), clip)
				box.Resize(geo.NewSize(60, 60))
				return box
			},
			drawn: geo.NewPos(130, 30), no: geo.NewPos(165, 30),
		},
		{
			// A clip turned an eighth about its centre clips its larger child
			// to the turned square, a diamond about (90, 90), not to the box
			// around it, (47.6, 47.6)-(132.4, 132.4) (#1003). (52, 52) is in
			// that box's corner, outside the diamond.
			name:  "clip under a rotation",
			scene: func() core.CanvasObject { return rotatedClip(rect(-50, -50, 160, 160)) },
			drawn: geo.NewPos(90, 90), no: geo.NewPos(52, 52),
		},
		{
			// The same inside a rounded clip: the turned clip's stencil nests
			// in the rounded one's (#1012).
			name: "clip under a rotation, in a rounded clip",
			scene: func() core.CanvasObject {
				outer := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 10, Objects: []core.CanvasObject{rotatedClip(rect(-50, -50, 160, 160))}}
				outer.Resize(geo.NewSize(200, 200))
				return outer
			},
			drawn: geo.NewPos(90, 90), no: geo.NewPos(52, 52),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := container.NewWithoutLayout(tc.scene())
			var w bellui.Window
			bellui.DoAndWait(func() {
				w = testApp.NewWindow("world transform")
				w.SetPadded(false)
				w.SetContent(root)
				w.Resize(geo.NewSize(200, 200))
				w.Show()
			})
			defer bellui.DoAndWait(w.Hide) // closing the last window quits the app
			time.Sleep(500 * time.Millisecond)
			img, scale, origin := capture(t, w, root)
			at := func(p geo.Position) color.Color {
				b := img.Bounds()
				return img.At(b.Min.X+int((origin.X+p.X)*scale), b.Min.Y+int((origin.Y+p.Y)*scale))
			}
			if c := at(tc.drawn); !red(c) {
				t.Errorf("at %v, where the transform puts it: %v, want red", tc.drawn, c)
			}
			if c := at(tc.no); red(c) {
				t.Errorf("at %v, where it must not be drawn: %v, want not red", tc.no, c)
			}
		})
	}
}
