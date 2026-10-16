//go:build (gl || (!darwin && !linux && !windows && !wasm)) && (!windows || !ci)

package gl

import (
	"math"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
)

// TestAlignedTextPos_AnchorsOnBoxOrigin covers pre-rebrand tracker #1086 (follow-up to pre-rebrand tracker #960) in the
// GL painter: a center/trailing-aligned canvas.Text whose container is larger
// than its content, under a non-trivial world transform, must scale/rotate about
// the text-box origin — NOT about the alignment-shifted glyph position.
//
// The alignment offset is a *local* offset inside the box, so it must be carried
// through the transform's linear part L: the draw position becomes
// boxOrigin + L·alignOffset. This matches the software painter (which renders the
// aligned bitmap and then scales the whole thing about the box origin wt.TX/TY).
func TestAlignedTextPos_AnchorsOnBoxOrigin(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	box := geo.NewPos(50, 30)
	content := geo.NewSize(40, 20)    // MinSize of the text
	container := geo.NewSize(140, 60) // oversized container
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

	// alignOffset for each alignment (matches the software painter's in-bitmap
	// placement): center = half the slack in both axes, trailing = full x slack.
	centerOff := geo.NewPos((container.Width-content.Width)/2, (container.Height-content.Height)/2)
	trailOff := geo.NewPos(container.Width-content.Width, (container.Height-content.Height)/2)

	cases := []struct {
		name  string
		align bellui.TextAlign
		off   geo.Position
	}{
		{"center", bellui.TextAlignCenter, centerOff},
		{"trailing", bellui.TextAlignTrailing, trailOff},
	}

	t.Run("no transform adds the offset verbatim", func(t *testing.T) {
		p := &painter{pixScale: 1, currentTransform: geo.Identity()}
		for _, tc := range cases {
			got := p.alignedTextPos(box, tc.align, content, container)
			wantX, wantY := box.X+tc.off.X, box.Y+tc.off.Y
			if !approx(got.X, wantX) || !approx(got.Y, wantY) {
				t.Errorf("%s: pos = (%v,%v), want (%v,%v)", tc.name, got.X, got.Y, wantX, wantY)
			}
		}
	})

	// A realistic leaf worldTransform: Translate(box)·L. The painter is handed
	// the full worldTransform; hasTransform() keys off its linear part.
	for _, lin := range []geo.Matrix{
		geo.NewScale(1.5, 2),
		geo.NewScale(2, 2).Multiply(geo.NewRotation(math.Pi / 5)),
	} {
		wt := geo.NewTranslation(box.X, box.Y).Multiply(lin)
		p := &painter{pixScale: 1, currentTransform: wt}
		if !p.hasTransform() {
			t.Fatal("hasTransform() should be true for a scale/rotate matrix")
		}
		for _, tc := range cases {
			got := p.alignedTextPos(box, tc.align, content, container)
			// The alignment offset is carried through L and re-anchored at box.
			lx, ly := lin.TransformVector(tc.off.X, tc.off.Y)
			wantX, wantY := box.X+lx, box.Y+ly
			if !approx(got.X, wantX) || !approx(got.Y, wantY) {
				t.Errorf("%s: aligned pos = (%v,%v), want box-origin-anchored (%v,%v)",
					tc.name, got.X, got.Y, wantX, wantY)
			}
		}
	}
}

// TestAlignedText_GlyphCornersMatchSoftwareModel closes the loop: feeding the
// box-origin-anchored draw position into transformedRectCoords must land every
// glyph-quad corner exactly where the software painter puts it —
// boxOrigin + L·(alignOffset + localCorner) — for center/trailing-aligned text
// in an oversized container (pre-rebrand tracker #1086).
func TestAlignedText_GlyphCornersMatchSoftwareModel(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	const fw, fh = 300, 200
	frame := geo.NewSize(fw, fh)
	box := geo.NewPos(50, 30)
	content := geo.NewSize(40, 20)
	container := geo.NewSize(140, 60)
	approx := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

	// softwareNDC: the NDC the software painter produces for a local corner
	// (relative to the content box) with the given alignment offset — the whole
	// aligned content scaled about the box origin.
	softwareNDC := func(l geo.Matrix, alignOff geo.Position, lx, ly float32) (float32, float32) {
		vx, vy := l.TransformVector(alignOff.X+lx, alignOff.Y+ly)
		tx, ty := box.X+vx, box.Y+vy
		return -1 + (tx/fw)*2, 1 - (ty/fh)*2
	}

	centerOff := geo.NewPos((container.Width-content.Width)/2, (container.Height-content.Height)/2)

	lin := geo.NewScale(1.5, 2)
	wt := geo.NewTranslation(box.X, box.Y).Multiply(lin)
	p := &painter{pixScale: 1, currentTransform: wt}

	drawPos := p.alignedTextPos(box, bellui.TextAlignCenter, content, container)
	pts, _ := p.transformedRectCoords(content, drawPos, frame, canvas.ImageFillStretch, 0, 0)

	// Quad corner order: TL=(0,H), BL=(0,0), TR=(W,H), BR=(W,0) in local coords.
	corners := [][2]float32{
		{0, content.Height},
		{0, 0},
		{content.Width, content.Height},
		{content.Width, 0},
	}
	for i, c := range corners {
		wantX, wantY := softwareNDC(lin, centerOff, c[0], c[1])
		gotX, gotY := pts[i*5], pts[i*5+1]
		if !approx(gotX, wantX) || !approx(gotY, wantY) {
			t.Errorf("corner %d = (%v,%v), want software-model (%v,%v)", i, gotX, gotY, wantX, wantY)
		}
	}
}
