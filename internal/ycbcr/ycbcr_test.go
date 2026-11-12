package ycbcr_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/ycbcr"
	"github.com/belluikit/bellui/internal/ycbcr/ycbcrtest"
)

func TestToRGBAMatchesTheColourBars(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bars   []ycbcrtest.Bar
		matrix int
		full   bool
	}{
		{"BT.601 limited", ycbcrtest.BT601Limited, ycbcr.BT601, false},
		{"BT.709 limited", ycbcrtest.BT709Limited, ycbcr.BT709, false},
		{"BT.709 full", ycbcrtest.BT709Full, ycbcr.BT709, true},
		{"BT.2020 limited", ycbcrtest.BT2020Limited, ycbcr.BT2020, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ycbcr.ToRGBA(ycbcrtest.Frame(tc.bars, 4, 4), tc.matrix, tc.full)
			ycbcrtest.AssertBars(t, tc.bars, 4, got, 2)
		})
	}
}

// The matrix is what tells BT.601 from BT.709 apart: the same samples decoded
// by the wrong one are visibly off, so the test above would catch a painter
// that ignores it.
func TestTheWrongMatrixMissesTheBars(t *testing.T) {
	got := ycbcr.ToRGBA(ycbcrtest.Frame(ycbcrtest.BT709Limited, 4, 4), ycbcr.BT601, false)
	yellow := got.RGBAAt(4+2, 2)
	off := absDiff(yellow.R, 191) + absDiff(yellow.G, 191) + absDiff(yellow.B, 0)
	assert.Greater(t, off, 15, "BT.709 yellow decoded as BT.601: %v", yellow)
}

// A frame that does not start at the origin is read from its own rectangle,
// and a 4:4:4 frame reads one chroma sample per pixel.
func TestToRGBAReadsTheFramesOwnLayout(t *testing.T) {
	full := ycbcrtest.Frame(ycbcrtest.BT601Limited, 4, 4)
	sub := full.SubImage(image.Rect(4, 0, 12, 4)).(*image.YCbCr)
	got := ycbcr.ToRGBA(sub, ycbcr.BT601, false)
	assert.Equal(t, image.Rect(0, 0, 8, 4), got.Rect)
	ycbcrtest.AssertBars(t, ycbcrtest.BT601Limited[1:3], 4, got, 2)

	img := image.NewYCbCr(image.Rect(0, 0, 2, 1), image.YCbCrSubsampleRatio444)
	img.Y[0], img.Cb[0], img.Cr[0] = 65, 100, 212
	img.Y[1], img.Cb[1], img.Cr[1] = 35, 212, 114
	got = ycbcr.ToRGBA(img, ycbcr.BT601, false)
	assert.InDelta(t, 191, int(got.RGBAAt(0, 0).R), 2)
	assert.InDelta(t, 191, int(got.RGBAAt(1, 0).B), 2)
}

// ChromaSize agrees with the planes image.NewYCbCr allocates, for every ratio
// and for odd sizes, which round the chroma plane up.
func TestChromaSizeMatchesTheAllocatedPlanes(t *testing.T) {
	for _, ratio := range []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420,
		image.YCbCrSubsampleRatio440, image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	} {
		for _, r := range []image.Rectangle{image.Rect(0, 0, 8, 6), image.Rect(0, 0, 7, 5), image.Rect(1, 1, 8, 6)} {
			img := image.NewYCbCr(r, ratio)
			w, h := ycbcr.ChromaSize(img)
			assert.Equalf(t, len(img.Cb), w*h, "%v %v: %dx%d", ratio, r, w, h)
			assert.Equalf(t, img.CStride, w, "%v %v", ratio, r)
		}
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
