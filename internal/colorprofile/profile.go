// Package colorprofile reads the colour profile an image file declares and
// converts its pixels to sRGB, which is what Bellui draws in (#802).
//
// It handles RGB matrix/TRC profiles: three primaries and a tone curve per
// channel, the shape of Display P3, Adobe RGB, ProPhoto RGB, sRGB and nearly
// every camera and display profile. Lookup-table profiles (A2B0 without the
// primaries), CMYK, greyscale and Lab profiles are not read: such an image is
// drawn as if it were sRGB, as it was before.
//
// The conversion is relative colorimetric. Both the source profile's primaries
// and sRGB's are adapted to the D50 white of the profile connection space, so
// white maps to white, and a colour outside sRGB is clipped per channel.
package colorprofile

import (
	"encoding/binary"
	"errors"
	"math"
)

// Profile is an RGB matrix/TRC colour profile: what a pixel's R, G and B mean.
type Profile struct {
	// toXYZ maps linear RGB to CIE XYZ (D50); its columns are the primaries.
	toXYZ [3][3]float64
	// trc linearises each channel, from encoded [0,1] to linear [0,1].
	trc [3]curve
}

var (
	// ErrUnsupported is returned for a valid profile Bellui does not convert:
	// not RGB, a Lab connection space, or no primaries and tone curves.
	ErrUnsupported = errors.New("colorprofile: unsupported profile")
	errMalformed   = errors.New("colorprofile: malformed profile")
)

// sRGB's primaries, adapted to D50 (Bradford), as in the ICC sRGB profiles.
var srgbToXYZ = [3][3]float64{
	{0.4360747, 0.3850649, 0.1430804},
	{0.2225045, 0.7168786, 0.0606169},
	{0.0139322, 0.0971045, 0.7141733},
}

// sRGB is the profile images are converted to, and the one assumed when an
// image declares none.
var sRGB = &Profile{toXYZ: srgbToXYZ, trc: [3]curve{srgbCurve, srgbCurve, srgbCurve}}

// srgbCurve is sRGB's tone curve as an ICC parametric curve (type 3).
var srgbCurve = curve{kind: curveParametric, fn: 3, p: [7]float64{2.4, 1 / 1.055, 0.055 / 1.055, 1 / 12.92, 0.04045}}

// ParseICC reads an ICC profile. It returns ErrUnsupported for a valid profile
// that is not an RGB matrix/TRC one.
func ParseICC(data []byte) (*Profile, error) {
	if len(data) < 132 || string(data[36:40]) != "acsp" {
		return nil, errMalformed
	}
	if string(data[16:20]) != "RGB " || string(data[20:24]) != "XYZ " {
		return nil, ErrUnsupported
	}
	tags := map[string][]byte{}
	n := int(binary.BigEndian.Uint32(data[128:]))
	if n > (len(data)-132)/12 {
		return nil, errMalformed
	}
	for i := 0; i < n; i++ {
		e := data[132+12*i:]
		off, size := int(binary.BigEndian.Uint32(e[4:])), int(binary.BigEndian.Uint32(e[8:]))
		if off < 0 || size < 0 || off > len(data) || size > len(data)-off {
			return nil, errMalformed
		}
		tags[string(e[:4])] = data[off : off+size]
	}

	p := &Profile{}
	for col, sig := range []string{"rXYZ", "gXYZ", "bXYZ"} {
		t, ok := tags[sig]
		if !ok {
			return nil, ErrUnsupported
		}
		xyz, err := parseXYZ(t)
		if err != nil {
			return nil, err
		}
		for row := range 3 {
			p.toXYZ[row][col] = xyz[row]
		}
	}
	for ch, sig := range []string{"rTRC", "gTRC", "bTRC"} {
		t, ok := tags[sig]
		if !ok {
			return nil, ErrUnsupported
		}
		c, err := parseCurve(t)
		if err != nil {
			return nil, err
		}
		p.trc[ch] = c
	}
	if _, err := invert(p.toXYZ); err != nil {
		return nil, errMalformed // primaries that span no colour space
	}
	return p, nil
}

// FromChromaticities builds a profile from a white point and primaries given
// as CIE xy chromaticities, and a pure power-law decoding exponent, as a PNG's
// cHRM and gAMA chunks declare them.
func FromChromaticities(white, red, green, blue [2]float64, gamma float64) (*Profile, error) {
	m, err := primariesToXYZ(white, red, green, blue)
	if err != nil {
		return nil, err
	}
	// Adapt from the declared white to D50, as an ICC profile's primaries are.
	m = mul(bradford(xyToXYZ(white), d50), m)
	c := curve{kind: curveGamma, p: [7]float64{gamma}}
	return &Profile{toXYZ: m, trc: [3]curve{c, c, c}}, nil
}

// IsSRGB reports whether p draws the same as sRGB to within a fraction of an
// 8-bit step, so converting through it would change nothing visible.
func (p *Profile) IsSRGB() bool {
	for r := range 3 {
		for c := range 3 {
			if math.Abs(p.toXYZ[r][c]-srgbToXYZ[r][c]) > 0.002 {
				return false
			}
		}
	}
	for ch := range 3 {
		for i := 0; i <= 32; i++ {
			x := float64(i) / 32
			if math.Abs(p.trc[ch].eval(x)-srgbCurve.eval(x)) > 0.5/255 {
				return false
			}
		}
	}
	return true
}

func parseXYZ(t []byte) ([3]float64, error) {
	if len(t) < 20 || string(t[:4]) != "XYZ " {
		return [3]float64{}, errMalformed
	}
	return [3]float64{s15f16(t[8:]), s15f16(t[12:]), s15f16(t[16:])}, nil
}

func s15f16(b []byte) float64 { return float64(int32(binary.BigEndian.Uint32(b))) / 65536 }

var d50 = [3]float64{0.9642, 1, 0.8249}

func xyToXYZ(xy [2]float64) [3]float64 {
	return [3]float64{xy[0] / xy[1], 1, (1 - xy[0] - xy[1]) / xy[1]}
}

// primariesToXYZ is the matrix from linear RGB to XYZ for the given
// chromaticities, scaled so RGB white is the white point with Y = 1.
func primariesToXYZ(white, r, g, b [2]float64) ([3][3]float64, error) {
	var m [3][3]float64
	for col, xy := range [][2]float64{r, g, b} {
		if xy[1] <= 0 {
			return m, errMalformed
		}
		v := xyToXYZ(xy)
		for row := range 3 {
			m[row][col] = v[row]
		}
	}
	if white[1] <= 0 {
		return m, errMalformed
	}
	inv, err := invert(m)
	if err != nil {
		return m, err
	}
	s := apply(inv, xyToXYZ(white))
	for row := range 3 {
		for col := range 3 {
			m[row][col] *= s[col]
		}
	}
	return m, nil
}

// bradford is the chromatic adaptation from white src to white dst.
func bradford(src, dst [3]float64) [3][3]float64 {
	b := [3][3]float64{
		{0.8951, 0.2664, -0.1614},
		{-0.7502, 1.7135, 0.0367},
		{0.0389, -0.0685, 1.0296},
	}
	bi, _ := invert(b)
	s, d := apply(b, src), apply(b, dst)
	scale := [3][3]float64{{d[0] / s[0]}, {0, d[1] / s[1]}, {0, 0, d[2] / s[2]}}
	return mul(bi, mul(scale, b))
}

func apply(m [3][3]float64, v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func mul(a, b [3][3]float64) (m [3][3]float64) {
	for r := range 3 {
		for c := range 3 {
			m[r][c] = a[r][0]*b[0][c] + a[r][1]*b[1][c] + a[r][2]*b[2][c]
		}
	}
	return m
}

func invert(m [3][3]float64) ([3][3]float64, error) {
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	if math.Abs(det) < 1e-12 {
		return m, errMalformed
	}
	var r [3][3]float64
	r[0][0] = (m[1][1]*m[2][2] - m[1][2]*m[2][1]) / det
	r[0][1] = (m[0][2]*m[2][1] - m[0][1]*m[2][2]) / det
	r[0][2] = (m[0][1]*m[1][2] - m[0][2]*m[1][1]) / det
	r[1][0] = (m[1][2]*m[2][0] - m[1][0]*m[2][2]) / det
	r[1][1] = (m[0][0]*m[2][2] - m[0][2]*m[2][0]) / det
	r[1][2] = (m[0][2]*m[1][0] - m[0][0]*m[1][2]) / det
	r[2][0] = (m[1][0]*m[2][1] - m[1][1]*m[2][0]) / det
	r[2][1] = (m[0][1]*m[2][0] - m[0][0]*m[2][1]) / det
	r[2][2] = (m[0][0]*m[1][1] - m[0][1]*m[1][0]) / det
	return r, nil
}
