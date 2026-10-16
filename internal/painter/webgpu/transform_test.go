//go:build !gl

package webgpu

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestRectCoordsTransform covers pre-rebrand tracker #233/#960: under a non-trivial world transform
// the textured quad (rectCoords) carries the transform on its corners, but
// anchored about the object's own origin — the worldTransform the paint walk
// hands the painter already contains Translate(pos), so the object must
// scale/rotate about its top-left, not the canvas origin (pre-rebrand tracker #960). With no
// transform it stays on the pixel-snapped axis-aligned path.
func TestRectCoordsTransform(t *testing.T) {
	const fw, fh = 200, 100
	frame := geo.NewSize(fw, fh)
	size := geo.NewSize(40, 20)
	pos := geo.NewPos(50, 30)

	// anchoredNDC: the correct NDC of a canvas-space corner under a transform
	// whose linear part is L, anchored at the object origin `pos` — i.e. the
	// corner maps to pos + L·(corner - pos). This is what the software painter
	// draws (worldTransform split into translation=pos + linear=L).
	anchoredNDC := func(l geo.Matrix, cx, cy float32) (float32, float32) {
		lx, ly := l.TransformVector(cx-pos.X, cy-pos.Y)
		tx, ty := pos.X+lx, pos.Y+ly
		return -1 + (tx/fw)*2, 1 - (ty/fh)*2
	}
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

	t.Run("no transform uses untransformed path", func(t *testing.T) {
		p := &painter{pixScale: 1, groupOpacity: 1, transform: geo.Identity()}
		pts, _ := p.rectCoords(size, pos, frame, canvas.ImageFillStretch, 0, 0)
		// vertex 0 = top-left at (pos.X, pos.Y+H); vertex 3 = (pos.X+W, pos.Y).
		wantTLx := float32(-1 + pos.X/fw*2)
		wantTLy := float32(1 - (pos.Y+size.Height)/fh*2)
		if !approx(pts[0], wantTLx) || !approx(pts[1], wantTLy) {
			t.Fatalf("TL = (%v,%v), want (%v,%v)", pts[0], pts[1], wantTLx, wantTLy)
		}
	})

	t.Run("scale transforms corners about the object origin", func(t *testing.T) {
		// A realistic leaf worldTransform: Translate(pos)·Scale(2,3).
		lin := geo.NewScale(2, 3)
		wt := geo.NewTranslation(pos.X, pos.Y).Multiply(lin)
		p := &painter{pixScale: 1, groupOpacity: 1, transform: wt}
		if !p.hasTransform() {
			t.Fatal("hasTransform() should be true for a scale matrix")
		}
		pts, _ := p.rectCoords(size, pos, frame, canvas.ImageFillStretch, 0, 0)

		// The object's top-left must stay anchored at pos (scale about top-left),
		// not double-translated to Translate(pos)·Scale·pos.
		wantTLx, wantTLy := anchoredNDC(lin, pos.X, pos.Y+size.Height)
		if !approx(pts[0], wantTLx) || !approx(pts[1], wantTLy) {
			t.Fatalf("TL = (%v,%v), want anchored (%v,%v)", pts[0], pts[1], wantTLx, wantTLy)
		}

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
	})

	t.Run("rotation transforms corners about the object origin", func(t *testing.T) {
		lin := geo.NewRotation(math.Pi / 6) // 30°
		wt := geo.NewTranslation(pos.X, pos.Y).Multiply(lin)
		p := &painter{pixScale: 1, groupOpacity: 1, transform: wt}
		pts, _ := p.rectCoords(size, pos, frame, canvas.ImageFillStretch, 0, 0)
		// A rotation must move the top-left corner off the axis-aligned position…
		flatX := float32(-1 + pos.X/fw*2)
		if approx(pts[0], flatX) {
			t.Errorf("rotated TL x = %v unexpectedly equals axis-aligned %v", pts[0], flatX)
		}
		// …but the TL corner (which sits at the object origin) rotates in place, so
		// it stays anchored at pos.
		wantX, wantY := anchoredNDC(lin, pos.X, pos.Y+size.Height)
		if !approx(pts[0], wantX) || !approx(pts[1], wantY) {
			t.Errorf("rotated TL = (%v,%v), want anchored (%v,%v)", pts[0], pts[1], wantX, wantY)
		}
	})
}

// TestAnchorMatchesSoftwareModel checks the anchor() helper against the software
// painter's transform model: worldTransform split into its translation (used as
// the draw position) and its linear part (applied about that position). For a
// leaf worldTransform Translate(pos)·L, anchor(pos) applied to an absolute
// corner must equal pos + L·(corner - pos).
func TestAnchorMatchesSoftwareModel(t *testing.T) {
	pos := geo.NewPos(120, 260)
	cases := []geo.Matrix{
		geo.NewScale(1.5, 1.5),
		geo.NewScale(2, 3),
		geo.NewRotation(math.Pi / 4),
		geo.NewScale(2, 2).Multiply(geo.NewRotation(math.Pi / 3)),
	}
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

	for _, lin := range cases {
		wt := geo.NewTranslation(pos.X, pos.Y).Multiply(lin)
		p := &painter{pixScale: 1, groupOpacity: 1, transform: wt}
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
