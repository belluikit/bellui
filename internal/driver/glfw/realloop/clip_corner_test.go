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

// TestContainerClipCornerRadiusDraws pins #991 (player#268) in a real window: the
// software test painter has no rounded clipping, so only GL or WebGPU shows
// it. A red rectangle larger than a 200x200 clipping container fills it,
// except, with ClipCornerRadius 40, its corners; with radius 0 (the control)
// the corners are red too, so the test can tell them apart. Points are read
// in the clip's own coordinates.
//
// It runs under -tags gl too: the GL painter draws into an offscreen canvas
// FBO, which had no stencil buffer, so no rounded clip clipped (#990). The
// widget path (a renderer's RoundedClipCornerRadius, as Card and Entry use)
// is checked the same way.
func TestContainerClipCornerRadiusDraws(t *testing.T) {
	for _, tc := range []struct {
		name string
		// clip returns the 200x200 clipping object with the red fill inside
		// it, and the object to place in the window.
		clip func(radius float32, fill core.CanvasObject) (clip, content core.CanvasObject)
	}{
		{"container", func(radius float32, fill core.CanvasObject) (core.CanvasObject, core.CanvasObject) {
			c := &bellui.Container{ClipToBounds: true, ClipCornerRadius: radius, Objects: []core.CanvasObject{fill}}
			return c, c
		}},
		{"widget", func(radius float32, fill core.CanvasObject) (core.CanvasObject, core.CanvasObject) {
			w := newRoundClipWidget(radius, fill)
			return w, w
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testClipCornerRadiusDraws(t, tc.clip)
		})
	}
}

func testClipCornerRadiusDraws(t *testing.T, build func(float32, core.CanvasObject) (core.CanvasObject, core.CanvasObject)) {
	red := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		return r>>8 > 200 && g>>8 < 60 && b>>8 < 60
	}
	// shoot returns the window's capture, its pixel scale, and where the clip
	// sits in the canvas (the window insets its content).
	shoot := func(radius float32) (image.Image, float32, geo.Position) {
		fill := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		fill.Move(geo.NewPos(-60, -60))
		fill.Resize(geo.NewSize(320, 320))
		clip, content := build(radius, fill)
		content.Move(geo.NewPos(50, 50))
		content.Resize(geo.NewSize(200, 200))
		var w bellui.Window
		bellui.DoAndWait(func() {
			w = testApp.NewWindow("clip corners")
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
			origin = testApp.Driver().AbsolutePositionForObject(clip)
		})
		return img, scale, origin
	}
	// at reads the pixel at (x, y) in the clip's own coordinates.
	at := func(img image.Image, scale float32, origin geo.Position, x, y float32) color.Color {
		b := img.Bounds()
		return img.At(b.Min.X+int((origin.X+x)*scale), b.Min.Y+int((origin.Y+y)*scale))
	}

	square, sq, so := shoot(0)
	for _, p := range [][2]float32{{100, 100}, {2, 2}} {
		if c := at(square, sq, so, p[0], p[1]); !red(c) {
			t.Fatalf("control: a square clip at %v reads %v at (%g,%g), want red: the test cannot see the clip", so, c, p[0], p[1])
		}
	}
	rounded, rs, ro := shoot(40)
	for _, p := range []struct {
		name    string
		x, y    float32
		wantRed bool
	}{
		{"centre", 100, 100, true},
		{"top edge middle", 100, 2, true},
		{"top-left corner", 2, 2, false},
		{"bottom-right corner", 197, 197, false},
		{"outside the clip", -10, 100, false},
	} {
		if got := at(rounded, rs, ro, p.x, p.y); red(got) != p.wantRed {
			t.Errorf("radius 40, %s (%g,%g): %v, want red=%v", p.name, p.x, p.y, got, p.wantRed)
		}
	}
}

// roundClipWidget clips its one object to its bounds with rounded corners,
// the way Card and Entry do: through its renderer's IsClip and
// RoundedClipCornerRadius. A radius of 0 makes it a square clip.
type roundClipWidget struct {
	widget.BaseWidget
	radius float32
	fill   core.CanvasObject
}

func newRoundClipWidget(radius float32, fill core.CanvasObject) *roundClipWidget {
	w := &roundClipWidget{radius: radius, fill: fill}
	w.ExtendBaseWidget(w)
	return w
}

func (w *roundClipWidget) CreateRenderer() core.WidgetRenderer {
	return &roundClipRenderer{w: w}
}

type roundClipRenderer struct{ w *roundClipWidget }

func (r *roundClipRenderer) Destroy()                         {}
func (r *roundClipRenderer) Layout(geo.Size)                  {} // the fill keeps its own place
func (r *roundClipRenderer) MinSize() geo.Size                { return geo.Size{} }
func (r *roundClipRenderer) Objects() []core.CanvasObject     { return []core.CanvasObject{r.w.fill} }
func (r *roundClipRenderer) Refresh()                         {}
func (r *roundClipRenderer) IsClip()                          {}
func (r *roundClipRenderer) RoundedClipCornerRadius() float32 { return r.w.radius }
