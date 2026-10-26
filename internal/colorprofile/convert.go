package colorprofile

import (
	"image"
	"image/color"
	"runtime"
	"sync"
)

// encodeSteps is the resolution of the linear-to-sRGB table. sRGB's steepest
// slope is 12.92, so a step of 1/16384 moves the 8-bit output by at most 0.2.
const encodeSteps = 1 << 14

var encodeTable = sync.OnceValue(func() *[encodeSteps + 1]uint8 {
	var t [encodeSteps + 1]uint8
	for i := range t {
		t[i] = uint8(srgbEncode(float64(i)/encodeSteps)*255 + 0.5)
	}
	return &t
})

// converter maps 8-bit encoded RGB in a source profile to 8-bit sRGB.
type converter struct {
	lin [3][256]float32 // per channel, encoded 8-bit value to linear light
	m   [3][3]float32   // source linear RGB to sRGB linear RGB
	enc *[encodeSteps + 1]uint8
}

func newConverter(p *Profile) *converter {
	c := &converter{enc: encodeTable()}
	for ch := range 3 {
		for v := range 256 {
			c.lin[ch][v] = float32(p.trc[ch].eval(float64(v) / 255))
		}
	}
	toSRGB, _ := invert(srgbToXYZ)
	m := mul(toSRGB, p.toXYZ)
	for r := range 3 {
		for col := range 3 {
			c.m[r][col] = float32(m[r][col])
		}
	}
	return c
}

func (c *converter) rgb(r, g, b uint8) (uint8, uint8, uint8) {
	lr, lg, lb := c.lin[0][r], c.lin[1][g], c.lin[2][b]
	return c.out(c.m[0][0]*lr + c.m[0][1]*lg + c.m[0][2]*lb),
		c.out(c.m[1][0]*lr + c.m[1][1]*lg + c.m[1][2]*lb),
		c.out(c.m[2][0]*lr + c.m[2][1]*lg + c.m[2][2]*lb)
}

// out clips linear v to [0,1], sRGB's gamut, and encodes it.
func (c *converter) out(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return c.enc[int(v*encodeSteps+0.5)]
}

// ToSRGB returns img converted from profile p to sRGB. A palette image keeps
// its pixels and gets a converted palette; anything else becomes an
// *image.NRGBA, at 8 bits per channel, the precision Bellui draws at.
func ToSRGB(img image.Image, p *Profile) image.Image {
	c := newConverter(p)
	if pal, ok := img.(*image.Paletted); ok {
		out := *pal
		out.Palette = make(color.Palette, len(pal.Palette))
		for i, col := range pal.Palette {
			n := color.NRGBAModel.Convert(col).(color.NRGBA)
			n.R, n.G, n.B = c.rgb(n.R, n.G, n.B)
			out.Palette[i] = n
		}
		return &out
	}

	b := img.Bounds()
	dst := image.NewNRGBA(b)
	rows := func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			o := dst.PixOffset(b.Min.X, y)
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := pixel(img, x, y)
				dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2] = c.rgb(r, g, bl)
				dst.Pix[o+3] = a
				o += 4
			}
		}
	}
	// Split the rows across the CPUs: a 12-megapixel photo is 12 million
	// table lookups and matrix products.
	workers := min(runtime.GOMAXPROCS(0), max(b.Dy()/64, 1))
	var wg sync.WaitGroup
	for w := range workers {
		y0 := b.Min.Y + b.Dy()*w/workers
		y1 := b.Min.Y + b.Dy()*(w+1)/workers
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows(y0, y1)
		}()
	}
	wg.Wait()
	return dst
}

// pixel reads one pixel as non-premultiplied 8-bit RGBA, with fast paths for
// the types Go's PNG and JPEG decoders return.
func pixel(img image.Image, x, y int) (r, g, b, a uint8) {
	switch im := img.(type) {
	case *image.NRGBA:
		i := im.PixOffset(x, y)
		return im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]
	case *image.YCbCr:
		yi, ci := im.YOffset(x, y), im.COffset(x, y)
		r, g, b := color.YCbCrToRGB(im.Y[yi], im.Cb[ci], im.Cr[ci])
		return r, g, b, 255
	case *image.RGBA:
		i := im.PixOffset(x, y)
		if a := im.Pix[i+3]; a != 255 {
			n := color.NRGBAModel.Convert(color.RGBA{im.Pix[i], im.Pix[i+1], im.Pix[i+2], a}).(color.NRGBA)
			return n.R, n.G, n.B, n.A
		}
		return im.Pix[i], im.Pix[i+1], im.Pix[i+2], 255
	}
	n := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return n.R, n.G, n.B, n.A
}
