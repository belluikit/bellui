//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestDrawsDoNotAllocate pins that drawing allocates nothing once the frame's
// buffers have grown: each draw's vertices and uniform block go through the
// painter's scratch into the frame arena, and a rounded clip's stencil stamp is
// not a canvas object (#447).
func TestDrawsDoNotAllocate(t *testing.T) {
	for _, batched := range []bool{false, true} {
		name := "unbatched"
		if batched {
			name = "batched"
		}
		t.Run(name, func(t *testing.T) {
			prev := batchEnabled
			batchEnabled = batched
			defer func() { batchEnabled = prev }()

			o := newOffscreenPainter(t)
			src, err := o.dev.CreateRenderTexture(8, 8)
			require.NoError(t, err)
			t.Cleanup(src.Release)

			frame := geo.NewSquareSize(clipDim)
			rect := canvas.NewRectangle(clipRed)
			rect.Resize(geo.NewSize(20, 10))
			rounded := canvas.NewRectangle(clipRed)
			rounded.CornerRadius = 4
			rounded.StrokeColor = color.White
			rounded.StrokeWidth = 2
			rounded.Resize(geo.NewSize(20, 10))
			circle := canvas.NewCircle(clipRed)
			circle.Resize(geo.NewSquareSize(10))
			line := canvas.NewLine(clipRed)
			line.Position2 = geo.NewPos(20, 20)
			line.StrokeWidth = 2

			var allocs float64
			o.frame(t, func() {
				allocs = testing.AllocsPerRun(50, func() {
					// Start each run where a frame starts, so the arena and
					// the textured draws' bind groups are reused, not grown.
					o.dev.arena.reset()
					o.dev.texBindInUse = 0

					o.drawRectangle(rect, geo.NewPos(2, 2), frame)
					o.drawRectangle(rounded, geo.NewPos(30, 2), frame)
					o.drawCircle(circle, geo.NewPos(2, 20), frame)
					o.drawLine(line, geo.NewPos(30, 20), frame)
					o.StartRoundedClipping(geo.NewPos(4, 40), geo.NewSize(40, 20), 6)
					o.drawRectangle(rect, geo.NewPos(4, 40), frame)
					o.StopRoundedClipping()
					o.blitTextureAlpha(src.View(), geo.NewSquareSize(8), geo.NewPos(50, 50), frame, 1)
				})
			})
			assert.Zero(t, allocs, "allocations per run of draws")
		})
	}
}
