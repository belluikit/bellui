package colorprofile

import (
	"encoding/binary"
	"math"
)

type curveKind uint8

const (
	curveIdentity curveKind = iota
	curveGamma
	curveTable
	curveParametric
)

// curve is an ICC tone curve (curveType or parametricCurveType): it maps an
// encoded channel value in [0,1] to linear light in [0,1].
type curve struct {
	kind  curveKind
	fn    uint16     // parametric function type, 0-4
	p     [7]float64 // gamma for curveGamma; g, a, b, c, d, e, f for parametric
	table []uint16   // samples spread evenly over [0,1] for curveTable
}

func parseCurve(t []byte) (curve, error) {
	if len(t) < 12 {
		return curve{}, errMalformed
	}
	switch string(t[:4]) {
	case "curv":
		n := int(binary.BigEndian.Uint32(t[8:]))
		switch {
		case n == 0:
			return curve{kind: curveIdentity}, nil
		case n == 1 && len(t) >= 14:
			return curve{kind: curveGamma, p: [7]float64{float64(binary.BigEndian.Uint16(t[12:])) / 256}}, nil
		case n > 1 && len(t) >= 12+2*n:
			tab := make([]uint16, n)
			for i := range tab {
				tab[i] = binary.BigEndian.Uint16(t[12+2*i:])
			}
			return curve{kind: curveTable, table: tab}, nil
		}
	case "para":
		fn := binary.BigEndian.Uint16(t[8:])
		count := [...]int{1, 3, 4, 5, 7}
		if int(fn) >= len(count) || len(t) < 12+4*count[fn] {
			break
		}
		c := curve{kind: curveParametric, fn: fn}
		for i := range count[fn] {
			c.p[i] = s15f16(t[12+4*i:])
		}
		return c, nil
	}
	return curve{}, errMalformed
}

// eval linearises x, an encoded value in [0,1].
func (c curve) eval(x float64) float64 {
	switch c.kind {
	case curveGamma:
		return math.Pow(x, c.p[0])
	case curveTable:
		pos := x * float64(len(c.table)-1)
		i := int(pos)
		if i >= len(c.table)-1 {
			return float64(c.table[len(c.table)-1]) / 65535
		}
		f := pos - float64(i)
		return (float64(c.table[i])*(1-f) + float64(c.table[i+1])*f) / 65535
	case curveParametric:
		return clamp01(c.parametric(x))
	}
	return x
}

func (c curve) parametric(x float64) float64 {
	g, a, b, cc, d, e, f := c.p[0], c.p[1], c.p[2], c.p[3], c.p[4], c.p[5], c.p[6]
	pow := func(v float64) float64 {
		if v <= 0 {
			return 0
		}
		return math.Pow(v, g)
	}
	switch c.fn {
	case 0:
		return pow(x)
	case 1:
		if a != 0 && x >= -b/a {
			return pow(a*x + b)
		}
		return 0
	case 2:
		if a != 0 && x >= -b/a {
			return pow(a*x+b) + cc
		}
		return cc
	case 3:
		if x >= d {
			return pow(a*x + b)
		}
		return cc * x
	case 4:
		if x >= d {
			return pow(a*x+b) + e
		}
		return cc*x + f
	}
	return x
}

// srgbEncode is sRGB's encoding: linear light to the encoded value.
func srgbEncode(l float64) float64 {
	if l <= 0.0031308 {
		return 12.92 * l
	}
	return 1.055*math.Pow(l, 1/2.4) - 0.055
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
