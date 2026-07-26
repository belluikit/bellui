package clipboard

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	img.SetNRGBA(2, 0, color.NRGBA{B: 255, A: 255})
	img.SetNRGBA(0, 1, color.NRGBA{R: 10, G: 20, B: 30, A: 128})
	img.SetNRGBA(1, 1, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	img.SetNRGBA(2, 1, color.NRGBA{A: 255})
	return img
}

func TestEncodeDIB_RoundTrip(t *testing.T) {
	src := testImage()
	decoded, ok := DecodeDIB(EncodeDIB(src))
	require.True(t, ok)

	got, isNRGBA := decoded.(*image.NRGBA)
	require.True(t, isNRGBA)
	assert.Equal(t, src.Bounds(), got.Bounds())
	assert.Equal(t, src.Pix, got.Pix)
}

func TestEncodeDIB_SubImageAndNonNRGBA(t *testing.T) {
	rgba := image.NewRGBA(image.Rect(0, 0, 4, 4))
	rgba.SetRGBA(2, 2, color.RGBA{R: 255, A: 255})
	sub := rgba.SubImage(image.Rect(1, 1, 4, 4))

	decoded, ok := DecodeDIB(EncodeDIB(sub))
	require.True(t, ok)
	assert.Equal(t, image.Rect(0, 0, 3, 3), decoded.Bounds())
	assert.Equal(t, color.NRGBA{R: 255, A: 255}, decoded.(*image.NRGBA).NRGBAAt(1, 1))
}

// build24bpp writes a bottom-up 24bpp BI_RGB DIB with one red pixel at (0,0)
// and odd width so rows need padding to a 4-byte boundary.
func TestDecodeDIB_24bpp(t *testing.T) {
	w, h := 3, 2
	stride := (w*3 + 3) &^ 3
	data := make([]byte, infoHeaderSize+stride*h)
	binary.LittleEndian.PutUint32(data[0:], infoHeaderSize)
	binary.LittleEndian.PutUint32(data[4:], uint32(w))
	binary.LittleEndian.PutUint32(data[8:], uint32(h))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 24)
	// (0,0) is in the LAST stored row (bottom-up): BGR = red
	data[infoHeaderSize+(h-1)*stride+2] = 255

	img, ok := DecodeDIB(data)
	require.True(t, ok)
	got := img.(*image.NRGBA)
	assert.Equal(t, color.NRGBA{R: 255, A: 255}, got.NRGBAAt(0, 0))
	assert.Equal(t, color.NRGBA{A: 255}, got.NRGBAAt(2, 1))
}

func TestDecodeDIB_TopDown32bppZeroAlphaIsOpaque(t *testing.T) {
	w, h := 2, 2
	data := make([]byte, infoHeaderSize+w*4*h)
	binary.LittleEndian.PutUint32(data[0:], infoHeaderSize)
	binary.LittleEndian.PutUint32(data[4:], uint32(w))
	binary.LittleEndian.PutUint32(data[8:], uint32(-int32(h))) // top-down
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	// (1,0): BGRX with X=0 — a writer that ignores the reserved byte
	copy(data[infoHeaderSize+4:], []byte{50, 100, 200, 0})

	img, ok := DecodeDIB(data)
	require.True(t, ok)
	got := img.(*image.NRGBA)
	assert.Equal(t, color.NRGBA{R: 200, G: 100, B: 50, A: 255}, got.NRGBAAt(1, 0), "zero alpha channel must decode opaque")
}

func TestDecodeDIB_V5Bitfields(t *testing.T) {
	w, h := 2, 1
	data := make([]byte, v5HeaderSize+w*4*h)
	binary.LittleEndian.PutUint32(data[0:], v5HeaderSize)
	binary.LittleEndian.PutUint32(data[4:], uint32(w))
	binary.LittleEndian.PutUint32(data[8:], uint32(h))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	binary.LittleEndian.PutUint32(data[16:], biBitfields)
	binary.LittleEndian.PutUint32(data[40:], 0x00ff0000)             // red
	binary.LittleEndian.PutUint32(data[44:], 0x0000ff00)             // green
	binary.LittleEndian.PutUint32(data[48:], 0x000000ff)             // blue
	binary.LittleEndian.PutUint32(data[52:], 0xff000000)             // alpha
	binary.LittleEndian.PutUint32(data[v5HeaderSize:], 0x80ff8040)   // ARGB half-alpha
	binary.LittleEndian.PutUint32(data[v5HeaderSize+4:], 0xff0000ff) // opaque blue

	img, ok := DecodeDIB(data)
	require.True(t, ok)
	got := img.(*image.NRGBA)
	assert.Equal(t, color.NRGBA{R: 255, G: 128, B: 64, A: 128}, got.NRGBAAt(0, 0))
	assert.Equal(t, color.NRGBA{B: 255, A: 255}, got.NRGBAAt(1, 0))
}

func TestDecodeDIB_InfoHeaderBitfieldsMasksFollowHeader(t *testing.T) {
	w, h := 1, 1
	data := make([]byte, infoHeaderSize+12+w*4*h)
	binary.LittleEndian.PutUint32(data[0:], infoHeaderSize)
	binary.LittleEndian.PutUint32(data[4:], uint32(w))
	binary.LittleEndian.PutUint32(data[8:], uint32(h))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	binary.LittleEndian.PutUint32(data[16:], biBitfields)
	binary.LittleEndian.PutUint32(data[40:], 0x00ff0000)
	binary.LittleEndian.PutUint32(data[44:], 0x0000ff00)
	binary.LittleEndian.PutUint32(data[48:], 0x000000ff)
	binary.LittleEndian.PutUint32(data[infoHeaderSize+12:], 0x00123456)

	img, ok := DecodeDIB(data)
	require.True(t, ok)
	// no alpha mask in a BITMAPINFOHEADER bitfields DIB -> opaque
	assert.Equal(t, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 255}, img.(*image.NRGBA).NRGBAAt(0, 0))
}

func TestDecodeDIB_Invalid(t *testing.T) {
	valid := EncodeDIB(testImage())

	truncatedHeader := valid[:20]
	truncatedPixels := valid[:len(valid)-1]

	badBpp := make([]byte, len(valid))
	copy(badBpp, valid)
	binary.LittleEndian.PutUint16(badBpp[14:], 8)

	badCompression := make([]byte, len(valid))
	copy(badCompression, valid)
	binary.LittleEndian.PutUint32(badCompression[16:], 1) // BI_RLE8

	for name, data := range map[string][]byte{
		"empty":            nil,
		"truncated header": truncatedHeader,
		"truncated pixels": truncatedPixels,
		"palette bpp":      badBpp,
		"rle compression":  badCompression,
	} {
		_, ok := DecodeDIB(data)
		assert.False(t, ok, name)
	}
}
