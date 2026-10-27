package colorprofile

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"math"
	"sort"
)

// maxICC bounds the profile read from a file, so a corrupt or hostile length
// cannot make the reader allocate without limit. Real RGB profiles are a few
// kilobytes; the largest LUT profiles are a few megabytes.
const maxICC = 4 << 20

// FromImageFile returns the profile an encoded PNG or JPEG declares. ok is
// false when it declares none, declares sRGB, or declares one Bellui does not
// convert (ErrUnsupported in err): the image is then drawn as sRGB.
func FromImageFile(data []byte) (p *Profile, ok bool, err error) {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		p, err = fromPNG(data)
	case bytes.HasPrefix(data, []byte{0xff, 0xd8}):
		p, err = fromJPEG(data)
	}
	if p == nil || err != nil || p.IsSRGB() {
		return nil, false, err
	}
	return p, true, nil
}

// fromPNG reads iCCP, sRGB, gAMA and cHRM, which the PNG specification puts
// before the image data. An sRGB or iCCP chunk wins over gAMA and cHRM.
func fromPNG(data []byte) (*Profile, error) {
	var gama, chrm []byte
	for off := 8; off+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		if n < 0 || n > len(data)-off-12 {
			return nil, errMalformed
		}
		body := data[off+8 : off+8+n]
		switch typ {
		case "sRGB":
			return nil, nil
		case "iCCP":
			return pngICC(body)
		case "gAMA":
			gama = body
		case "cHRM":
			chrm = body
		case "IDAT", "IEND":
			return pngGamma(gama, chrm)
		}
		off += 12 + n
	}
	return pngGamma(gama, chrm)
}

// pngICC decodes an iCCP chunk: a name, a NUL, a compression method (0,
// zlib) and the compressed profile.
func pngICC(body []byte) (*Profile, error) {
	nul := bytes.IndexByte(body, 0)
	if nul < 0 || nul+2 > len(body) || body[nul+1] != 0 {
		return nil, errMalformed
	}
	zr, err := zlib.NewReader(bytes.NewReader(body[nul+2:]))
	if err != nil {
		return nil, errMalformed
	}
	icc, err := io.ReadAll(io.LimitReader(zr, maxICC+1))
	if err != nil || len(icc) > maxICC {
		return nil, errMalformed
	}
	return ParseICC(icc)
}

// pngGamma builds a profile from gAMA and cHRM. Missing chromaticities are
// sRGB's. A gAMA of 1/2.2 with no cHRM is how most encoders say "sRGB", so it
// is taken as sRGB rather than as a pure 2.2 curve, as browsers do.
func pngGamma(gama, chrm []byte) (*Profile, error) {
	if len(gama) != 4 {
		return nil, nil
	}
	g := float64(binary.BigEndian.Uint32(gama)) / 100000
	if g <= 0 {
		return nil, errMalformed
	}
	white, red, green, blue := [2]float64{0.3127, 0.3290}, [2]float64{0.64, 0.33}, [2]float64{0.30, 0.60}, [2]float64{0.15, 0.06}
	if len(chrm) == 32 {
		v := func(i int) [2]float64 {
			return [2]float64{
				float64(binary.BigEndian.Uint32(chrm[8*i:])) / 100000,
				float64(binary.BigEndian.Uint32(chrm[8*i+4:])) / 100000,
			}
		}
		white, red, green, blue = v(0), v(1), v(2), v(3)
	} else if math.Abs(g-1/2.2) < 0.005 {
		return nil, nil
	}
	return FromChromaticities(white, red, green, blue, 1/g)
}

// fromJPEG gathers the ICC profile from APP2 "ICC_PROFILE" segments, which
// split a profile larger than a segment into numbered chunks.
func fromJPEG(data []byte) (*Profile, error) {
	const sig = "ICC_PROFILE\x00"
	type chunk struct {
		seq  int
		data []byte
	}
	var chunks []chunk
	total := 0
	for off := 2; off+4 <= len(data); {
		if data[off] != 0xff {
			return nil, errMalformed
		}
		marker := data[off+1]
		if marker == 0xd8 || (marker >= 0xd0 && marker <= 0xd7) || marker == 0x01 || marker == 0xff {
			off += 2 // standalone markers, and fill bytes
			if marker == 0xff {
				off--
			}
			continue
		}
		if marker == 0xda || marker == 0xd9 { // start of scan: no more metadata
			break
		}
		n := int(binary.BigEndian.Uint16(data[off+2:]))
		if n < 2 || off+2+n > len(data) {
			return nil, errMalformed
		}
		seg := data[off+4 : off+2+n]
		if marker == 0xe2 && len(seg) > len(sig)+2 && string(seg[:len(sig)]) == sig {
			chunks = append(chunks, chunk{seq: int(seg[len(sig)]), data: seg[len(sig)+2:]})
			if total += len(seg); total > maxICC {
				return nil, errMalformed
			}
		}
		off += 2 + n
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	sort.SliceStable(chunks, func(i, j int) bool { return chunks[i].seq < chunks[j].seq })
	var icc []byte
	for _, c := range chunks {
		icc = append(icc, c.data...)
	}
	return ParseICC(icc)
}
