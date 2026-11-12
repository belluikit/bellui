// Package ycbcrtest holds the colour-bar frames that pin how each painter
// converts a Y'CbCr video frame to RGB (#1107).
package ycbcrtest

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Bar is one colour bar: its Y'CbCr samples and the RGB they stand for.
type Bar struct {
	Y, Cb, Cr uint8
	RGB       color.RGBA
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 0xff} }

// The 75% colour bars, then black and white, as FFmpeg's swscale encodes
// them from those RGB values (scale=out_color_matrix=…:out_range=…, flags
// accurate_rnd+full_chroma_int, to yuv444p). The limited-range sets match the
// published BT.601 and BT.709 bar tables; BT.601 yellow is 161 here and 162
// there, a rounding difference.
var (
	BT601Limited = []Bar{
		{180, 128, 128, rgb(191, 191, 191)},
		{161, 44, 142, rgb(191, 191, 0)},
		{131, 156, 44, rgb(0, 191, 191)},
		{112, 72, 58, rgb(0, 191, 0)},
		{84, 184, 198, rgb(191, 0, 191)},
		{65, 100, 212, rgb(191, 0, 0)},
		{35, 212, 114, rgb(0, 0, 191)},
		{16, 128, 128, rgb(0, 0, 0)},
		{235, 128, 128, rgb(255, 255, 255)},
	}
	BT709Limited = []Bar{
		{180, 128, 128, rgb(191, 191, 191)},
		{168, 44, 136, rgb(191, 191, 0)},
		{145, 147, 44, rgb(0, 191, 191)},
		{133, 63, 52, rgb(0, 191, 0)},
		{63, 193, 204, rgb(191, 0, 191)},
		{51, 109, 212, rgb(191, 0, 0)},
		{28, 212, 120, rgb(0, 0, 191)},
		{16, 128, 128, rgb(0, 0, 0)},
		{235, 128, 128, rgb(255, 255, 255)},
	}
	BT709Full = []Bar{
		{191, 128, 128, rgb(191, 191, 191)},
		{177, 32, 136, rgb(191, 191, 0)},
		{150, 149, 32, rgb(0, 191, 191)},
		{137, 54, 41, rgb(0, 191, 0)},
		{54, 201, 214, rgb(191, 0, 191)},
		{41, 106, 223, rgb(191, 0, 0)},
		{14, 223, 119, rgb(0, 0, 191)},
		{0, 128, 128, rgb(0, 0, 0)},
		{255, 128, 128, rgb(255, 255, 255)},
	}
	BT2020Limited = []Bar{
		{170, 44, 135, rgb(191, 191, 0)},
		{137, 151, 44, rgb(0, 191, 191)},
		{127, 68, 51, rgb(0, 191, 0)},
		{69, 188, 205, rgb(191, 0, 191)},
		{59, 105, 212, rgb(191, 0, 0)},
		{26, 212, 121, rgb(0, 0, 191)},
	}
)

// Frame lays bars out side by side, each barW pixels wide and h high, in a
// 4:2:0 frame. With barW even no chroma sample straddles two bars.
func Frame(bars []Bar, barW, h int) *image.YCbCr {
	img := image.NewYCbCr(image.Rect(0, 0, len(bars)*barW, h), image.YCbCrSubsampleRatio420)
	for y := 0; y < h; y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			b := bars[x/barW]
			img.Y[img.YOffset(x, y)] = b.Y
			ci := img.COffset(x, y)
			img.Cb[ci], img.Cr[ci] = b.Cb, b.Cr
		}
	}
	return img
}

// AssertBars checks the middle of each bar of got, drawn from Frame(bars,
// barW, h) at its own size, against the bar's RGB, within delta per channel.
// A GPU painter samples chroma with linear filtering, so only a bar's edges
// blend with its neighbour.
func AssertBars(t *testing.T, bars []Bar, barW int, got image.Image, delta int) {
	t.Helper()
	b := got.Bounds()
	for i, bar := range bars {
		c := color.RGBAModel.Convert(got.At(b.Min.X+i*barW+barW/2, b.Min.Y+b.Dy()/2)).(color.RGBA)
		ok := assert.InDeltaf(t, int(bar.RGB.R), int(c.R), float64(delta), "bar %d red", i) &&
			assert.InDeltaf(t, int(bar.RGB.G), int(c.G), float64(delta), "bar %d green", i) &&
			assert.InDeltaf(t, int(bar.RGB.B), int(c.B), float64(delta), "bar %d blue", i) &&
			assert.Equalf(t, uint8(0xff), c.A, "bar %d is opaque", i)
		if !ok {
			t.Logf("bar %d: got %v, want %v", i, c, bar.RGB)
		}
	}
}

// Reversed returns bars in reverse order. Every set above encodes the same
// colours, so a test that checks a second frame needs its bars in another
// order: otherwise a painter still showing the first frame passes.
func Reversed(bars []Bar) []Bar {
	out := make([]Bar, len(bars))
	for i, b := range bars {
		out[len(bars)-1-i] = b
	}
	return out
}
