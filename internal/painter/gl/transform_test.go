//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (!windows || !ci)

package gl

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestTransformedRectCoordsAnchored covers #960 in the GL painter: a textured
// quad under a non-trivial world transform scales/rotates about the object's own
// origin, not the canvas origin. The worldTransform the paint walk supplies
// already contains Translate(pos), so applying it to the object's absolute
// corners would double-count the translation and push the object off its anchor.
func TestTransformedRectCoordsAnchored(t *testing.T) {
	const fw, fh = 200, 100
	frame := geo.NewSize(fw, fh)
	size := geo.NewSize(40, 20)
	pos := geo.NewPos(50, 30)

	anchoredNDC := func(l geo.Matrix, cx, cy float32) (float32, float32) {
		lx, ly := l.TransformVector(cx-pos.X, cy-pos.Y)
		tx, ty := pos.X+lx, pos.Y+ly
		return -1 + (tx/fw)*2, 1 - (ty/fh)*2
	}
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

	lin := geo.NewScale(2, 3)
	wt := geo.NewTranslation(pos.X, pos.Y).Multiply(lin)
	p := &painter{pixScale: 1, currentTransform: wt}
	if !p.hasTransform() {
		t.Fatal("hasTransform() should be true for a scale matrix")
	}
	pts, _ := p.transformedRectCoords(size, pos, frame, canvas.ImageFillStretch, 0, 0)

	// Quad corner order: TL=(x, y+H), BL=(x, y), TR=(x+W, y+H), BR=(x+W, y).
	corners := [][2]float32{
		{pos.X, pos.Y + size.Height},
		{pos.X, pos.Y},
		{pos.X + size.Width, pos.Y + size.Height},
		{pos.X + size.Width, pos.Y},
	}
	for i, c := range corners {
		wantX, wantY := anchoredNDC(lin, c[0], c[1])
		gotX, gotY := pts[i*5], pts[i*5+1]
		if !approx(gotX, wantX) || !approx(gotY, wantY) {
			t.Errorf("corner %d = (%v,%v), want anchored (%v,%v)", i, gotX, gotY, wantX, wantY)
		}
	}
}

// TestAnchorMatchesSoftwareModel checks anchor() against the software painter's
// transform model (worldTransform split into translation=pos + linear part).
func TestAnchorMatchesSoftwareModel(t *testing.T) {
	pos := geo.NewPos(120, 260)
	cases := []geo.Matrix{
		geo.NewScale(1.5, 1.5),
		geo.NewRotation(math.Pi / 4),
		geo.NewScale(2, 2).Multiply(geo.NewRotation(math.Pi / 3)),
	}
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

	for _, lin := range cases {
		wt := geo.NewTranslation(pos.X, pos.Y).Multiply(lin)
		p := &painter{pixScale: 1, currentTransform: wt}
		a := p.anchor(pos)
		for _, off := range [][2]float32{{0, 0}, {40, 0}, {0, 20}, {40, 20}} {
			cx, cy := pos.X+off[0], pos.Y+off[1]
			gotX, gotY := a.TransformPoint(cx, cy)
			lx, ly := lin.TransformVector(off[0], off[1])
			wantX, wantY := pos.X+lx, pos.Y+ly
			if !approx(gotX, wantX) || !approx(gotY, wantY) {
				t.Errorf("anchor corner off %v = (%v,%v), want (%v,%v)", off, gotX, gotY, wantX, wantY)
			}
		}
	}
}
