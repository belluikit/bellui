package colorprofile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// gridImage holds 729 colours, every mix of nine levels per channel.
func gridImage() *image.NRGBA {
	levels := []uint8{0, 32, 64, 96, 128, 160, 192, 224, 255}
	img := image.NewNRGBA(image.Rect(0, 0, 27, 27))
	i := 0
	for _, r := range levels {
		for _, g := range levels {
			for _, b := range levels {
				img.SetNRGBA(i%27, i/27, color.NRGBA{r, g, b, 255})
				i++
			}
		}
	}
	return img
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// The conversion matches an independent colour engine. testdata holds grids
// tagged with three profiles these tests build (Display P3, Adobe RGB, and
// Rec. 2020 primaries with a sampled 1024-point curve) and the same grids
// converted to sRGB by macOS ColorSync (sips --matchTo the sRGB profile,
// 2026-10-26), stored as plain pixels. Every channel agrees within one 8-bit
// step (#802).
func TestToSRGB_MatchesColorSync(t *testing.T) {
	for _, name := range []string{"p3", "adobergb", "rec2020table"} {
		t.Run(name, func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join("testdata", "grid-"+name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "grid-"+name+".colorsync-srgb.png"))
			if err != nil {
				t.Fatal(err)
			}
			p, ok, err := FromImageFile(in)
			if !ok || err != nil {
				t.Fatalf("no profile read: ok=%v err=%v", ok, err)
			}
			src := decodePNG(t, in)
			got := ToSRGB(src, p)
			ref := decodePNG(t, want)

			changed := 0
			for y := 0; y < 27; y++ {
				for x := 0; x < 27; x++ {
					g := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA)
					r := color.NRGBAModel.Convert(ref.At(x, y)).(color.NRGBA)
					s := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
					if r != s {
						changed++
					}
					if absDiff(g.R, r.R) > 1 || absDiff(g.G, r.G) > 1 || absDiff(g.B, r.B) > 1 || g.A != r.A {
						t.Errorf("pixel %v: converted to %v, ColorSync gives %v", s, g, r)
					}
				}
			}
			if changed < 600 {
				t.Errorf("ColorSync changed only %d of 729 colours: the fixture does not exercise the conversion", changed)
			}
		})
	}
}

// A JPEG's profile may be split over several APP2 segments, in any order.
func TestFromImageFile_JPEGProfileInSegments(t *testing.T) {
	icc := rec2020Table.icc() // over 2 KB, so 512-byte segments split it
	data := withJPEGProfile(gridImage(), icc, 512)
	p, ok, err := FromImageFile(data)
	if !ok || err != nil {
		t.Fatalf("no profile read: ok=%v err=%v", ok, err)
	}
	want, _ := ParseICC(icc)
	if p.toXYZ != want.toXYZ {
		t.Errorf("primaries %v, want %v", p.toXYZ, want.toXYZ)
	}
}

// What a PNG declares besides iCCP: sRGB, and gAMA with or without cHRM.
func TestFromImageFile_PNGChunks(t *testing.T) {
	gama := func(g uint32) []byte { return pngChunk("gAMA", binary.BigEndian.AppendUint32(nil, g)) }
	chrm := func(xy ...float64) []byte {
		var b []byte
		for _, v := range xy {
			b = binary.BigEndian.AppendUint32(b, uint32(v*100000+0.5))
		}
		return pngChunk("cHRM", b)
	}
	img := gridImage()
	for _, c := range []struct {
		name   string
		chunks [][]byte
		want   bool
	}{
		{"none", nil, false},
		{"sRGB chunk", [][]byte{pngChunk("sRGB", []byte{0})}, false},
		{"sRGB chunk wins over a P3 iCCP after it", [][]byte{pngChunk("sRGB", []byte{0}), iccpChunk(p3.icc())}, false},
		{"gAMA 1/2.2 alone is sRGB", [][]byte{gama(45455)}, false},
		{"gAMA 1.0 is linear", [][]byte{gama(100000)}, true},
		{"gAMA with P3 cHRM", [][]byte{gama(45455), chrm(0.3127, 0.3290, 0.680, 0.320, 0.265, 0.690, 0.150, 0.060)}, true},
		{"an embedded sRGB profile changes nothing", [][]byte{iccpChunk(srgbTest.icc())}, false},
		{"Display P3", [][]byte{iccpChunk(p3.icc())}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, ok, err := FromImageFile(withPNGChunks(img, c.chunks...))
			if err != nil {
				t.Fatal(err)
			}
			if ok != c.want {
				t.Errorf("profile read = %v, want %v", ok, c.want)
			}
		})
	}
}

// A linear (gAMA 1.0) image is converted: mid-grey 128 is 0.5 linear light,
// which sRGB encodes as 188.
func TestToSRGB_LinearGamma(t *testing.T) {
	data := withPNGChunks(gridImage(), pngChunk("gAMA", binary.BigEndian.AppendUint32(nil, 100000)))
	p, ok, _ := FromImageFile(data)
	if !ok {
		t.Fatal("no profile read")
	}
	grey := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	grey.SetNRGBA(0, 0, color.NRGBA{128, 128, 128, 200})
	got := ToSRGB(grey, p).(*image.NRGBA).NRGBAAt(0, 0)
	if got != (color.NRGBA{188, 188, 188, 200}) {
		t.Errorf("linear grey 128 converted to %v, want {188 188 188 200}", got)
	}
}

// Profiles Bellui does not convert are reported, not guessed at; broken ones
// are errors and never panic.
func TestParseICC_Unsupported(t *testing.T) {
	icc := p3.icc()
	cmyk := append([]byte(nil), icc...)
	copy(cmyk[16:], "CMYK")
	if _, err := ParseICC(cmyk); !errors.Is(err, ErrUnsupported) {
		t.Errorf("CMYK profile: %v, want ErrUnsupported", err)
	}
	lab := append([]byte(nil), icc...)
	copy(lab[20:], "Lab ")
	if _, err := ParseICC(lab); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Lab connection space: %v, want ErrUnsupported", err)
	}
	noPrimaries := append([]byte(nil), icc...)
	copy(noPrimaries[132+12:], "A2B0") // rename rXYZ, the second tag
	if _, err := ParseICC(noPrimaries); !errors.Is(err, ErrUnsupported) {
		t.Errorf("profile without rXYZ: %v, want ErrUnsupported", err)
	}
	for _, n := range []int{0, 100, 131, 140, len(icc) - 5} {
		if _, err := ParseICC(icc[:n]); err == nil {
			t.Errorf("profile truncated to %d bytes parsed", n)
		}
	}
}

// A palette image keeps its pixels and gets a converted palette.
func TestToSRGB_Paletted(t *testing.T) {
	p, _ := ParseICC(p3.icc())
	pal := image.NewPaletted(image.Rect(0, 0, 2, 1), color.Palette{color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 0, 0}})
	pal.SetColorIndex(1, 0, 1)
	out, ok := ToSRGB(pal, p).(*image.Paletted)
	if !ok {
		t.Fatalf("got %T, want *image.Paletted", ToSRGB(pal, p))
	}
	if out.ColorIndexAt(0, 0) != 0 || out.ColorIndexAt(1, 0) != 1 {
		t.Errorf("pixel indexes changed: %v", out.Pix)
	}
	if got := out.Palette[0].(color.NRGBA); got != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("P3 red is %v in sRGB, want it clipped to {255 0 0 255}", got)
	}
	if got := pal.Palette[0].(color.NRGBA); got != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("the source palette was modified: %v", got)
	}
}

// The readers face files the app did not make: no input may panic or
// allocate without bound.
func FuzzFromImageFile(f *testing.F) {
	img := gridImage()
	f.Add(withPNGChunks(img, iccpChunk(p3.icc())))
	f.Add(withPNGChunks(img, pngChunk("gAMA", []byte{0, 0, 0xb1, 0x8f}), pngChunk("cHRM", make([]byte, 32))))
	f.Add(withJPEGProfile(img, rec2020Table.icc(), 300))
	f.Add(p3.icc())
	f.Fuzz(func(t *testing.T, data []byte) {
		if p, ok, _ := FromImageFile(data); ok {
			ToSRGB(img, p)
		}
		ParseICC(data)
	})
}
