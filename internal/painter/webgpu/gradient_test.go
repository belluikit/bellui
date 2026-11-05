//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// paintGradient draws o at (10, 10) with the gradient shader or with the CPU
// texture reference.
func (b *batchPainter) paintGradient(t *testing.T, o core.CanvasObject, shader bool) *image.RGBA {
	t.Helper()
	prev := gpuGradients
	gpuGradients = shader
	defer func() { gpuGradients = prev }()
	return b.frame(t, false, func() { b.Paint(o, geo.NewPos(10, 10), batchFrame()) })
}

// TestGradientShaderMatchesGenerate pins #956: the gradient shader draws what
// the CPU texture (canvas Generate) draws, for both gradient kinds, every
// linear angle case, centre offsets and multi-stop gradients.
func TestGradientShaderMatchesGenerate(t *testing.T) {
	b := newBatchPainter(t)
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	soft := color.NRGBA{R: 255, G: 255, B: 255, A: 0x24}

	cases := map[string]func() core.CanvasObject{}
	for _, a := range []float64{0, 45, 90, 135, 180, 225, 270, 315, 30} {
		cases["linear angle "+fmtAngle(a)] = func() core.CanvasObject {
			g := canvas.NewLinearGradient(red, blue, a)
			g.Resize(geo.NewSize(90, 60))
			return g
		}
	}
	cases["radial"] = func() core.CanvasObject {
		g := canvas.NewRadialGradient(red, blue)
		g.Resize(geo.NewSize(80, 80))
		return g
	}
	cases["radial offset"] = func() core.CanvasObject {
		g := canvas.NewRadialGradient(red, blue)
		g.CenterOffsetX, g.CenterOffsetY = 0.2, -0.3
		g.Resize(geo.NewSize(90, 60))
		return g
	}
	cases["radial translucent to clear"] = func() core.CanvasObject {
		g := canvas.NewRadialGradient(soft, color.Transparent)
		g.Resize(geo.NewSize(80, 80))
		return g
	}
	cases["linear three stops"] = func() core.CanvasObject {
		g := canvas.NewLinearGradient(nil, nil, 270)
		g.Stops = []canvas.GradientStop{{Color: red, Position: 0}, {Color: color.NRGBA{G: 255, A: 255}, Position: 0.3}, {Color: blue, Position: 1}}
		g.Resize(geo.NewSize(90, 40))
		return g
	}

	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			ref := b.paintGradient(t, mk(), false)
			got := b.paintGradient(t, mk(), true)
			require.Greater(t, ink(ref), 100, "the reference drew nothing")
			if n := diffPixels(ref, got, 3); n > 0 {
				x, y := firstDiff(ref, got, 3)
				t.Errorf("%d pixels differ from Generate by more than 3/255; first at (%d,%d): reference %v, shader %v",
					n, x, y, ref.RGBAAt(x, y), got.RGBAAt(x, y))
			}
		})
	}
}

func fmtAngle(a float64) string {
	return string(rune('0'+int(a)/100%10)) + string(rune('0'+int(a)/10%10)) + string(rune('0'+int(a)%10))
}

func firstDiff(a, b *image.RGBA, tol int) (int, int) {
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p, q := a.RGBAAt(x, y), b.RGBAAt(x, y)
			for _, d := range []int{int(p.R) - int(q.R), int(p.G) - int(q.G), int(p.B) - int(q.B)} {
				if d > tol || -d > tol {
					return x, y
				}
			}
		}
	}
	return -1, -1
}
