//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
)

// TestBoxShadowEdgeProfile reads back a box shadow across its right edge: it
// falls off as the box blurred by a Gaussian of sigma BlurRadius/2 (#442), in
// steps no steeper than that Gaussian's, so a wide blur shows no bands. Both
// painters divided the blur radius by the texture width twice, which left the
// shadow a hard-edged box.
//
// A blur wide enough to halve the texture four times or more is the least
// exact: how much halving blurs depends on where the edges fall on the halved
// texels (paint.PlanBlur), which moves sigma by up to about a tenth.
func TestBoxShadowEdgeProfile(t *testing.T) {
	for _, tc := range []struct {
		radius        float32
		sigmaTol, tol float64
	}{
		{3, 0.08, 0.02},  // not halved
		{12, 0.08, 0.03}, // halved twice
		{36, 0.08, 0.03}, // DialogLevel; halved three times
		{60, 0.15, 0.08}, // halved four times
	} {
		radius := tc.radius
		spread := 2 * radius
		size := geo.NewSize(120, 80)
		w := int(size.Width + 2*spread + 40)
		h := int(size.Height + 2*spread + 40)
		o := newOffscreenPainterSized(t, w, h)

		shadow := canvas.NewBoxShadow(color.NRGBA{A: 255}, radius)
		shadow.Resize(size)
		pos := geo.NewPos(spread+20, spread+20)
		img := o.frame(t, func() {
			o.drawBoxShadow(shadow, pos, geo.NewSize(float32(w), float32(h)))
		})

		// Opaque black over the blue clear leaves blue at 1 - coverage.
		y := int(pos.Y + size.Height/2)
		coverage := func(x int) float64 {
			return 1 - float64(pixel(img, x, y).B)/255
		}
		// The coverage of the box blurred by sigma: per axis, the share of a
		// Gaussian around the pixel's centre that falls inside the box.
		top, bottom := float64(pos.Y), float64(pos.Y+size.Height)
		left, edge := float64(pos.X), float64(pos.X+size.Width)
		blurred := func(x int, sigma float64) float64 {
			inside := func(c, lo, hi float64) float64 {
				return 0.5 * (math.Erf((hi-c)/(sigma*math.Sqrt2)) - math.Erf((lo-c)/(sigma*math.Sqrt2)))
			}
			return inside(float64(x)+0.5, left, edge) * inside(float64(y)+0.5, top, bottom)
		}

		sigma := float64(paint.BlurSigma(radius))
		from, to := int(edge-3*sigma), int(edge+3*sigma)
		worst, fit, fitErr := 0.0, 0.0, math.Inf(1)
		for x := from; x < to; x++ {
			worst = max(worst, math.Abs(coverage(x)-blurred(x, sigma)))
		}
		for s := sigma / 2; s < sigma*2; s += sigma / 200 {
			e := 0.0
			for x := from; x < to; x++ {
				d := coverage(x) - blurred(x, s)
				e += d * d
			}
			if e < fitErr {
				fit, fitErr = s, e
			}
		}
		assert.InDelta(t, 1, fit/sigma, tc.sigmaTol, "radius %v: the sigma the edge fits", radius)
		assert.Less(t, worst, tc.tol, "radius %v: the edge's largest difference from the blurred box", radius)

		steepest := 1 / (sigma * math.Sqrt(2*math.Pi)) // the Gaussian's slope at the edge
		jump := 0.0
		for x := from; x < to; x++ {
			jump = max(jump, math.Abs(coverage(x+1)-coverage(x)))
		}
		assert.Less(t, jump, 1.3*steepest+2.0/255, "radius %v: no bands", radius)

		for x := int(edge + sigma); x < int(edge+4.2*sigma) && testing.Verbose(); x++ {
			t.Logf("TAIL r=%v d=%.2fσ got %.4f want %.4f", radius, (float64(x)+0.5-edge)/sigma, coverage(x), blurred(x, sigma))
		}
		// The tail fades out: a kernel that ends on a heavy tap stops it on a
		// step, a faint line around the shadow.
		for x := int(edge + sigma); x < int(edge+4*sigma); x++ {
			step := math.Abs(coverage(x+1) - coverage(x))
			slope := math.Abs(blurred(x+1, sigma) - blurred(x, sigma))
			if !assert.Less(t, step, 2*slope+1.5/255, "radius %v: the tail %.1fσ from the edge", radius, (float64(x)-edge)/sigma) {
				break
			}
		}
	}
}

// TestBackdropBlurEdgeProfile reads back a backdrop blur over a hard edge: it
// blurs by the same sigma, BlurRadius/2, as a box shadow (#442). It took the
// radius as the distance between taps, which blurred by 1.7 radii.
func TestBackdropBlurEdgeProfile(t *testing.T) {
	const w, h, radius = 160, 60, 12
	o := newOffscreenPainterSized(t, w, h)
	red := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
	red.Resize(geo.NewSize(w/2, h))
	blur := canvas.NewBackdropBlur(radius)
	blur.Resize(geo.NewSize(w, h))
	frame := geo.NewSize(w, h)
	img := o.frame(t, func() {
		o.drawRectangle(red, geo.Position{}, frame)
		o.drawBackdropBlur(blur, geo.Position{}, frame)
	})

	// Red over the blue clear, left of the edge: red is the share of it.
	sigma := float64(paint.BlurSigma(radius))
	edge := float64(w / 2)
	worst := 0.0
	for x := int(edge - 2*sigma); x < int(edge+2*sigma); x++ {
		want := 0.5 * math.Erfc((float64(x)+0.5-edge)/(sigma*math.Sqrt2))
		worst = max(worst, math.Abs(float64(pixel(img, x, h/2).R)/255-want))
	}
	assert.Less(t, worst, 0.03, "the edge falls off as a Gaussian of sigma radius/2")
}
