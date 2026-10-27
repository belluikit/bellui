package colorprofile

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"math"
)

// The tests build their own profiles and files rather than shipping
// third-party ICC profiles in testdata.

// testProfile describes an RGB matrix/TRC profile to encode.
type testProfile struct {
	white, red, green, blue [2]float64 // xy chromaticities
	trc                     []byte     // one curv or para tag body, shared by R, G and B
}

func s15(v float64) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(int32(math.Round(v*65536))))
	return b
}

func xyzTag(v [3]float64) []byte {
	t := append([]byte("XYZ \x00\x00\x00\x00"), s15(v[0])...)
	t = append(t, s15(v[1])...)
	return append(t, s15(v[2])...)
}

// paraTag is a parametricCurveType of function type fn.
func paraTag(fn uint16, params ...float64) []byte {
	t := []byte("para\x00\x00\x00\x00")
	t = binary.BigEndian.AppendUint16(t, fn)
	t = append(t, 0, 0)
	for _, p := range params {
		t = append(t, s15(p)...)
	}
	return t
}

// gammaTag is a curveType holding a single u8Fixed8 gamma.
func gammaTag(g float64) []byte {
	t := []byte("curv\x00\x00\x00\x00\x00\x00\x00\x01")
	return binary.BigEndian.AppendUint16(t, uint16(math.Round(g*256)))
}

// tableTag is a curveType of n samples of f.
func tableTag(n int, f func(float64) float64) []byte {
	t := []byte("curv\x00\x00\x00\x00")
	t = binary.BigEndian.AppendUint32(t, uint32(n))
	for i := range n {
		t = binary.BigEndian.AppendUint16(t, uint16(math.Round(f(float64(i)/float64(n-1))*65535)))
	}
	return t
}

var (
	srgbPara = paraTag(3, 2.4, 1/1.055, 0.055/1.055, 1/12.92, 0.04045)
	d65      = [2]float64{0.3127, 0.3290}
	p3       = testProfile{white: d65, red: [2]float64{0.680, 0.320}, green: [2]float64{0.265, 0.690}, blue: [2]float64{0.150, 0.060}, trc: srgbPara}
	adobeRGB = testProfile{white: d65, red: [2]float64{0.64, 0.33}, green: [2]float64{0.21, 0.71}, blue: [2]float64{0.15, 0.06}, trc: gammaTag(563.0 / 256)}
	// rec2020Table has wide primaries and a sampled curve, to exercise
	// curveTable.
	rec2020Table = testProfile{white: d65, red: [2]float64{0.708, 0.292}, green: [2]float64{0.170, 0.797}, blue: [2]float64{0.131, 0.046}, trc: tableTag(1024, func(x float64) float64 { return math.Pow(x, 2.4) })}
	srgbTest     = testProfile{white: d65, red: [2]float64{0.64, 0.33}, green: [2]float64{0.30, 0.60}, blue: [2]float64{0.15, 0.06}, trc: srgbPara}
)

// icc encodes p as a version 4 display profile. Its primaries are adapted to
// D50 with Bradford, as the ICC specification requires.
func (p testProfile) icc() []byte {
	m, err := primariesToXYZ(p.white, p.red, p.green, p.blue)
	if err != nil {
		panic(err)
	}
	m = mul(bradford(xyToXYZ(p.white), d50), m)
	col := func(c int) [3]float64 { return [3]float64{m[0][c], m[1][c], m[2][c]} }
	tags := []struct {
		sig  string
		body []byte
	}{
		{"wtpt", xyzTag(d50)},
		{"rXYZ", xyzTag(col(0))},
		{"gXYZ", xyzTag(col(1))},
		{"bXYZ", xyzTag(col(2))},
		{"rTRC", p.trc},
		{"gTRC", p.trc},
		{"bTRC", p.trc},
	}
	header := make([]byte, 128)
	copy(header[8:], []byte{4, 0x30, 0, 0})
	copy(header[12:], "mntr")
	copy(header[16:], "RGB ")
	copy(header[20:], "XYZ ")
	copy(header[36:], "acsp")
	copy(header[68:], s15(d50[0]))
	copy(header[72:], s15(d50[1]))
	copy(header[76:], s15(d50[2]))
	table := binary.BigEndian.AppendUint32(nil, uint32(len(tags)))
	var data []byte
	off := 128 + 4 + 12*len(tags)
	for _, t := range tags {
		for len(t.body)%4 != 0 {
			t.body = append(t.body, 0)
		}
		table = append(table, t.sig...)
		table = binary.BigEndian.AppendUint32(table, uint32(off+len(data)))
		table = binary.BigEndian.AppendUint32(table, uint32(len(t.body)))
		data = append(data, t.body...)
	}
	out := append(append(header, table...), data...)
	binary.BigEndian.PutUint32(out, uint32(len(out)))
	return out
}

// pngChunk encodes one PNG chunk with its CRC.
func pngChunk(typ string, body []byte) []byte {
	c := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	c = append(c, typ...)
	c = append(c, body...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

// withPNGChunks encodes img as a PNG with extra chunks inserted after IHDR.
func withPNGChunks(img image.Image, chunks ...[]byte) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	b := buf.Bytes()
	ihdrEnd := 8 + 12 + int(binary.BigEndian.Uint32(b[8:]))
	out := append([]byte(nil), b[:ihdrEnd]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, b[ihdrEnd:]...)
}

// iccpChunk is an iCCP chunk carrying profile.
func iccpChunk(profile []byte) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write(profile)
	w.Close()
	return pngChunk("iCCP", append([]byte("test\x00\x00"), z.Bytes()...))
}

// withJPEGProfile encodes img as a JPEG carrying profile in APP2 segments of
// at most segMax bytes of profile each, written in reverse order to check the
// reader sorts them.
func withJPEGProfile(img image.Image, profile []byte, segMax int) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		panic(err)
	}
	var parts [][]byte
	for len(profile) > 0 {
		n := min(segMax, len(profile))
		parts = append(parts, profile[:n])
		profile = profile[n:]
	}
	var segs []byte
	for i := len(parts) - 1; i >= 0; i-- {
		body := append([]byte("ICC_PROFILE\x00"), byte(i+1), byte(len(parts)))
		body = append(body, parts[i]...)
		segs = append(segs, 0xff, 0xe2)
		segs = binary.BigEndian.AppendUint16(segs, uint16(len(body)+2))
		segs = append(segs, body...)
	}
	b := buf.Bytes()
	return append(append(append([]byte(nil), b[:2]...), segs...), b[2:]...)
}
