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

// TestClipScissorsAgreeOnSharedEdges pins #975. A partial repaint clears the
// dirty region under one scissor, then redraws a ClipToBounds container under
// the container's clip intersected with that region. Where the two rects share
// an edge in dp, they must share it in pixels: a row the clear covers and the
// redraw does not keeps the cleared colour, the dark line Daysong's cover
// showed on a Pixel 9a (scale 2.625).
func TestClipScissorsAgreeOnSharedEdges(t *testing.T) {
	b := newBatchPainter(t)
	const scale = 2.625
	b.pixScale, b.texScale = scale, scale
	t.Cleanup(func() { b.pixScale, b.texScale = 1, 1 })

	full := geo.NewSquareSize(batchDim / scale)
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	fill := func(c color.Color) {
		r := canvas.NewRectangle(c)
		r.Resize(full)
		b.Paint(r, geo.Position{}, full)
	}

	// The dirty region and the container's clip within it share their bottom
	// edge (11.4 dp): rounding y and h separately, the region would end at
	// pixel row 30 and the clip at row 29.
	region := struct {
		pos  geo.Position
		size geo.Size
	}{geo.NewPos(0, 0), geo.NewSize(full.Width, 11.4)}
	clip := struct {
		pos  geo.Position
		size geo.Size
	}{geo.NewPos(0, 1.7), geo.NewSize(full.Width, 9.7)}

	img := b.frame(t, false, func() {
		fill(red) // last frame's pixels, retained
		b.StartClipping(region.pos, region.size)
		fill(blue) // the partial repaint's clear
		b.StopClipping()
		b.StartClipping(clip.pos, clip.size)
		fill(red) // the clipped container, redrawn
		b.StopClipping()
	})

	// Every pixel whose centre lies inside the clip was redrawn.
	y0, y1 := clip.pos.Y*scale, (clip.pos.Y+clip.size.Height)*scale
	for y := 0; y < batchDim; y++ {
		if c := float32(y) + 0.5; c < y0 || c >= y1 {
			continue
		}
		for x := 0; x < batchDim; x++ {
			got := img.RGBAAt(x, y)
			require.Truef(t, got.R > 200 && got.B < 50,
				"pixel (%d,%d), centre %.3f dp inside the clip, is %v: cleared and not redrawn", x, y, (float32(y)+0.5)/scale, got)
		}
	}
}
