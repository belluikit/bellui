package software_test

import (
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

// TestPainter_WorldTransformMovesObjects pins #997 for the software painter:
// an object is drawn where its world transform puts its origin, translation
// included. Each case reads one point where the transform puts the red
// rectangle and one where it would be without the transform's translation.
func TestPainter_WorldTransformMovesObjects(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	isRed := func(c color.Color) bool {
		r, g, b, a := c.RGBA()
		return a > 0x8000 && r > 0xC000 && g < 0x4000 && b < 0x4000
	}
	rect := func(x, y, w, h float32) *canvas.Rectangle {
		r := canvas.NewRectangle(red)
		r.Move(geo.NewPos(x, y))
		r.Resize(geo.NewSize(w, h))
		return r
	}

	for name, tc := range map[string]struct {
		scene     func() core.CanvasObject
		drawn, no geo.Position
	}{
		"translation only": {
			scene: func() core.CanvasObject {
				r := rect(20, 10, 20, 20)
				r.SetTransform(geo.NewTranslation(100, 0))
				return r
			},
			drawn: geo.NewPos(130, 20), no: geo.NewPos(30, 20),
		},
		"rotation about a pivot": {
			// Half a turn about its centre covers the same box; about its
			// origin it would cover (10, 30)-(50, 50).
			scene: func() core.CanvasObject {
				r := rect(50, 50, 40, 20)
				r.SetTransform(geo.NewRotationAround(math.Pi, 20, 10))
				return r
			},
			drawn: geo.NewPos(85, 65), no: geo.NewPos(15, 35),
		},
		"transformed parent, offset child": {
			// A quarter turn about the parent's centre takes the child's
			// centre from (85, 15) to (85, 85) in the parent; about its own
			// origin it would stay at (120, 50)-(130, 60) in the canvas.
			scene: func() core.CanvasObject {
				box := newTiltBox(geo.NewRotationAround(math.Pi/2, 50, 50), rect(80, 10, 10, 10))
				box.Move(geo.NewPos(40, 40))
				box.Resize(geo.NewSize(100, 100))
				return box
			},
			drawn: geo.NewPos(125, 125), no: geo.NewPos(125, 55),
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := test.NewCanvas()
			c.SetPadded(false)
			c.SetContent(container.NewWithoutLayout(tc.scene()))
			c.Resize(geo.NewSize(200, 200))
			img := software.NewPainter().Paint(c)

			assert.True(t, isRed(img.At(int(tc.drawn.X), int(tc.drawn.Y))), "red where the transform puts it, %v", tc.drawn)
			assert.False(t, isRed(img.At(int(tc.no.X), int(tc.no.Y))), "nothing where it would be without its translation, %v", tc.no)
		})
	}
}
