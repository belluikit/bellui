// Package ycbcr converts Y'CbCr video samples to RGB by a frame's own colour
// matrix and range. canvas.Raster uses it on the processor and the GPU painters
// hand its coefficients to their shaders, so every painter shows a frame in the
// same colours (#1107).
package ycbcr

import "image"

// The colour matrices, numbered as canvas.YCbCrMatrix numbers them.
const (
	BT601 = iota
	BT709
	BT2020
)

// lumaWeights returns Kr and Kb, the red and blue weights of luma, for a
// matrix. An unknown matrix gets BT.601's, the matrix of standard-definition
// video and of JPEG.
func lumaWeights(matrix int) (kr, kb float64) {
	switch matrix {
	case BT709:
		return 0.2126, 0.0722
	case BT2020:
		return 0.2627, 0.0593
	}
	return 0.299, 0.114
}

// Coefficients returns the affine map from Y'CbCr samples normalised to [0, 1]
// (a byte over 255, which is what a GPU's unorm texture reads) to R'G'B' in
// [0, 1]. Row i gives channel i (red, green, blue) as
// row[0]*y + row[1]*cb + row[2]*cr + row[3].
//
// Limited ("TV") range puts black at 16 and white at 235 for luma, and spans
// 16 to 240 for chroma. Full ("PC") range uses every byte.
func Coefficients(matrix int, fullRange bool) [3][4]float32 {
	kr, kb := lumaWeights(matrix)
	kg := 1 - kr - kb

	// y' = ay*y + by and c' = ac*c + bc, with y' in [0, 1] and c' in
	// [-0.5, 0.5].
	ay, by := 255.0/219, -16.0/219
	ac, bc := 255.0/224, -128.0/224
	if fullRange {
		ay, by = 1, 0
		ac, bc = 1, -128.0/255
	}

	rCr := 2 * (1 - kr)
	bCb := 2 * (1 - kb)
	gCb := -2 * (1 - kb) * kb / kg
	gCr := -2 * (1 - kr) * kr / kg

	return [3][4]float32{
		{float32(ay), 0, float32(rCr * ac), float32(by + rCr*bc)},
		{float32(ay), float32(gCb * ac), float32(gCr * ac), float32(by + (gCb+gCr)*bc)},
		{float32(ay), float32(bCb * ac), 0, float32(by + bCb*bc)},
	}
}

// ToRGBA converts src into a new opaque RGBA image of the same size, by the
// given matrix and range. Each chroma sample covers the pixels its subsample
// ratio gives it, as image.YCbCr.At reads them.
func ToRGBA(src *image.YCbCr, matrix int, fullRange bool) *image.RGBA {
	b := src.Rect
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))

	// Fixed point, 16 fractional bits: a byte sample times a coefficient
	// gives the channel as a byte.
	var k [3][4]int32
	for i, row := range Coefficients(matrix, fullRange) {
		for j := 0; j < 3; j++ {
			k[i][j] = int32(row[j]*65536 + roundHalf(row[j]))
		}
		k[i][3] = int32(row[3]*255*65536+roundHalf(row[3])) + 1<<15
	}

	for y := b.Min.Y; y < b.Max.Y; y++ {
		out := dst.Pix[(y-b.Min.Y)*dst.Stride:]
		for x := b.Min.X; x < b.Max.X; x++ {
			yy := int32(src.Y[src.YOffset(x, y)])
			ci := src.COffset(x, y)
			cb, cr := int32(src.Cb[ci]), int32(src.Cr[ci])
			o := (x - b.Min.X) * 4
			out[o] = clampByte(k[0][0]*yy + k[0][2]*cr + k[0][3])
			out[o+1] = clampByte(k[1][0]*yy + k[1][1]*cb + k[1][2]*cr + k[1][3])
			out[o+2] = clampByte(k[2][0]*yy + k[2][1]*cb + k[2][3])
			out[o+3] = 0xff
		}
	}
	return dst
}

// roundHalf is the half a coefficient needs to round, rather than truncate,
// into fixed point.
func roundHalf(v float32) float32 {
	if v < 0 {
		return -0.5
	}
	return 0.5
}

func clampByte(v int32) uint8 {
	v >>= 16
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// ChromaSize returns the size of img's Cb and Cr planes, as image.YCbCr lays
// them out for its subsample ratio.
func ChromaSize(img *image.YCbCr) (w, h int) {
	r := img.Rect
	w, h = r.Dx(), r.Dy()
	halfW := (r.Max.X+1)/2 - r.Min.X/2
	halfH := (r.Max.Y+1)/2 - r.Min.Y/2
	quarterW := (r.Max.X+3)/4 - r.Min.X/4
	switch img.SubsampleRatio {
	case image.YCbCrSubsampleRatio422:
		return halfW, h
	case image.YCbCrSubsampleRatio420:
		return halfW, halfH
	case image.YCbCrSubsampleRatio440:
		return w, halfH
	case image.YCbCrSubsampleRatio411:
		return quarterW, h
	case image.YCbCrSubsampleRatio410:
		return quarterW, halfH
	}
	return w, h
}
