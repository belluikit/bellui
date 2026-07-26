package clipboard

import (
	"encoding/binary"
	"image"
	"image/draw"
)

// biCompression values understood by the DIB decoder.
const (
	biRGB       = 0
	biBitfields = 3

	infoHeaderSize = 40  // BITMAPINFOHEADER
	v4HeaderSize   = 108 // BITMAPV4HEADER
	v5HeaderSize   = 124 // BITMAPV5HEADER
)

// EncodeDIB converts an image to a CF_DIB clipboard payload: a
// BITMAPINFOHEADER followed by bottom-up 32bpp BGRA pixel rows. The alpha
// channel is stored in the reserved byte, which modern consumers honour.
func EncodeDIB(img image.Image) []byte {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	nrgba, ok := img.(*image.NRGBA)
	if !ok || nrgba.Bounds() != bounds {
		nrgba = image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(nrgba, nrgba.Bounds(), img, bounds.Min, draw.Src)
	}

	stride := w * 4 // 32bpp rows are always 4-byte aligned
	data := make([]byte, infoHeaderSize+stride*h)
	binary.LittleEndian.PutUint32(data[0:], infoHeaderSize)
	binary.LittleEndian.PutUint32(data[4:], uint32(w))
	binary.LittleEndian.PutUint32(data[8:], uint32(h)) // positive: bottom-up
	binary.LittleEndian.PutUint16(data[12:], 1)        // planes
	binary.LittleEndian.PutUint16(data[14:], 32)       // bit count
	binary.LittleEndian.PutUint32(data[16:], biRGB)
	binary.LittleEndian.PutUint32(data[20:], uint32(stride*h)) // image size

	for y := 0; y < h; y++ {
		src := nrgba.Pix[(h-1-y)*nrgba.Stride:]
		dst := data[infoHeaderSize+y*stride:]
		for x := 0; x < w; x++ {
			dst[x*4+0] = src[x*4+2] // B
			dst[x*4+1] = src[x*4+1] // G
			dst[x*4+2] = src[x*4+0] // R
			dst[x*4+3] = src[x*4+3] // A
		}
	}
	return data
}

// DecodeDIB converts a CF_DIB / CF_DIBV5 clipboard payload to an image.
// It understands BITMAPINFOHEADER, BITMAPV4HEADER and BITMAPV5HEADER with
// 24/32bpp BI_RGB or 32bpp BI_BITFIELDS pixel formats, bottom-up or top-down
// rows. It returns (nil, false) for anything else or a truncated payload.
func DecodeDIB(data []byte) (image.Image, bool) {
	if len(data) < infoHeaderSize {
		return nil, false
	}
	headerSize := int(binary.LittleEndian.Uint32(data[0:]))
	if headerSize != infoHeaderSize && headerSize != v4HeaderSize && headerSize != v5HeaderSize {
		return nil, false
	}
	if len(data) < headerSize {
		return nil, false
	}
	w := int(int32(binary.LittleEndian.Uint32(data[4:])))
	h := int(int32(binary.LittleEndian.Uint32(data[8:])))
	bitCount := int(binary.LittleEndian.Uint16(data[14:]))
	compression := binary.LittleEndian.Uint32(data[16:])
	clrUsed := int(binary.LittleEndian.Uint32(data[32:]))

	topDown := false
	if h < 0 {
		topDown = true
		h = -h
	}
	if w <= 0 || h <= 0 {
		return nil, false
	}

	// Masks for BI_BITFIELDS; the alpha mask only exists in V4/V5 headers.
	var redMask, greenMask, blueMask, alphaMask uint32
	pixelOffset := headerSize
	switch {
	case compression == biRGB && (bitCount == 24 || bitCount == 32):
	case compression == biBitfields && bitCount == 32:
		if headerSize == infoHeaderSize {
			// The three colour masks follow the header.
			if len(data) < headerSize+12 {
				return nil, false
			}
			redMask = binary.LittleEndian.Uint32(data[40:])
			greenMask = binary.LittleEndian.Uint32(data[44:])
			blueMask = binary.LittleEndian.Uint32(data[48:])
			pixelOffset += 12
		} else {
			redMask = binary.LittleEndian.Uint32(data[40:])
			greenMask = binary.LittleEndian.Uint32(data[44:])
			blueMask = binary.LittleEndian.Uint32(data[48:])
			alphaMask = binary.LittleEndian.Uint32(data[52:])
		}
		if redMask == 0 || greenMask == 0 || blueMask == 0 {
			return nil, false
		}
	default:
		return nil, false
	}
	pixelOffset += clrUsed * 4 // optional palette entries even at >8bpp

	stride := (w*bitCount/8 + 3) &^ 3
	if pixelOffset < 0 || len(data) < pixelOffset+stride*h {
		return nil, false
	}
	pixels := data[pixelOffset:]

	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	if compression == biBitfields {
		decodeBitfieldRows(img, pixels, w, h, stride, topDown, redMask, greenMask, blueMask, alphaMask)
		return img, true
	}

	bytesPerPixel := bitCount / 8
	opaque := true
	for y := 0; y < h; y++ {
		row := pixels[srcRow(y, h, topDown)*stride:]
		dst := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			px := row[x*bytesPerPixel:]
			dst[x*4+0] = px[2] // R
			dst[x*4+1] = px[1] // G
			dst[x*4+2] = px[0] // B
			if bitCount == 32 {
				dst[x*4+3] = px[3]
				if px[3] != 0 {
					opaque = false
				}
			} else {
				dst[x*4+3] = 0xff
			}
		}
	}
	// A fully zero alpha channel means the writer left the reserved byte
	// untouched, so the image is really opaque.
	if bitCount == 32 && opaque {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 0xff
		}
	}
	return img, true
}

func decodeBitfieldRows(img *image.NRGBA, pixels []byte, w, h, stride int, topDown bool, redMask, greenMask, blueMask, alphaMask uint32) {
	rShift, rBits := maskParams(redMask)
	gShift, gBits := maskParams(greenMask)
	bShift, bBits := maskParams(blueMask)
	aShift, aBits := maskParams(alphaMask)

	for y := 0; y < h; y++ {
		row := pixels[srcRow(y, h, topDown)*stride:]
		dst := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			v := binary.LittleEndian.Uint32(row[x*4:])
			dst[x*4+0] = scaleComponent(v&redMask>>rShift, rBits)
			dst[x*4+1] = scaleComponent(v&greenMask>>gShift, gBits)
			dst[x*4+2] = scaleComponent(v&blueMask>>bShift, bBits)
			if alphaMask == 0 {
				dst[x*4+3] = 0xff
			} else {
				dst[x*4+3] = scaleComponent(v&alphaMask>>aShift, aBits)
			}
		}
	}
}

// srcRow maps a destination row to the DIB row index, honouring bottom-up
// (default) vs top-down storage.
func srcRow(y, h int, topDown bool) int {
	if topDown {
		return y
	}
	return h - 1 - y
}

// maskParams returns the shift and bit width of a contiguous channel mask.
func maskParams(mask uint32) (shift, bits int) {
	if mask == 0 {
		return 0, 0
	}
	for mask&1 == 0 {
		mask >>= 1
		shift++
	}
	for mask&1 == 1 {
		mask >>= 1
		bits++
	}
	return shift, bits
}

// scaleComponent expands a channel value of the given bit width to 8 bits.
func scaleComponent(v uint32, bits int) uint8 {
	switch {
	case bits == 0:
		return 0
	case bits == 8:
		return uint8(v)
	case bits > 8:
		return uint8(v >> (bits - 8))
	default:
		// Replicate the bit pattern to fill 8 bits (e.g. 5-bit 31 -> 255).
		out := v << (8 - bits)
		for filled := bits; filled < 8; filled *= 2 {
			out |= out >> filled
		}
		return uint8(out)
	}
}
