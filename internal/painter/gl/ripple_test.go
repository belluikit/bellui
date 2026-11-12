//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !android && !ios && !mobile && !wasm && !no_glfw

package gl

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// A ripple covering its whole size, rounded on its left side only as a
// segmented button's first segment is (#1156), leaves its left corners clear
// and fills its right ones. A swapped radius order fails here.
func TestRippleClipsEachCorner(t *testing.T) {
	o := newOffscreenPainter(t)
	r := canvas.NewRipple(color.NRGBA{R: 0xff, A: 0xff})
	r.Resize(geo.NewSize(40, 40))
	r.Center = geo.NewPos(20, 20)
	r.Radius = 80
	r.Opacity = 1
	r.TopLeftCornerRadius, r.BottomLeftCornerRadius = 20, 20

	img := o.paint(r) // drawn at (10, 10)
	for _, c := range []struct {
		name   string
		x, y   int
		filled bool
	}{
		{"top-left", 11, 11, false},
		{"bottom-left", 11, 48, false},
		{"top-right", 48, 11, true},
		{"bottom-right", 48, 48, true},
		{"left edge middle", 11, 30, true},
	} {
		red := img.RGBAAt(c.x, c.y).R
		if c.filled && red < 250 {
			t.Errorf("%s red = %d, want the ripple there", c.name, red)
		} else if !c.filled && red > 5 {
			t.Errorf("%s red = %d, want it clipped away", c.name, red)
		}
	}
}
