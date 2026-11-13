//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestClipCutsATransformedDraw is #1190's check on this painter: a rectangle
// scaled past its clipping container's bounds is cut at them. The software
// painter drew it whole.
func TestClipCutsATransformedDraw(t *testing.T) {
	b := newBatchPainter(t)
	t.Cleanup(func() { b.transform = geo.Identity() })

	const box, side = 24, 48 // the clip is 24..72 on both axes
	img := b.frame(t, false, func() {
		b.StartClipping(geo.NewPos(box, box), geo.NewSquareSize(side))
		r := canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		r.Resize(geo.NewSquareSize(side))
		// Scaled 2× about its centre (48, 48): its origin lands on (0, 0)
		// and it covers the whole 96² target.
		b.transform = geo.NewScale(2, 2)
		b.Paint(r, geo.Position{}, geo.NewSquareSize(batchDim))
		b.transform = geo.Identity()
		b.StopClipping()
	})

	for y := 0; y < batchDim; y++ {
		for x := 0; x < batchDim; x++ {
			inside := x >= box && x < box+side && y >= box && y < box+side
			got := img.RGBAAt(x, y)
			if inside {
				require.Truef(t, got.R > 200, "pixel (%d,%d) inside the clip is %v: not drawn", x, y, got)
			} else {
				require.Truef(t, got.R < 50, "pixel (%d,%d) outside the clip is %v: drawn past it", x, y, got)
			}
		}
	}
}
