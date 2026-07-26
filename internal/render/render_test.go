package render_test

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"

	"github.com/belluikit/bellui/internal/render"
	"github.com/belluikit/typesetting/font"
	"github.com/belluikit/typesetting/shaping"

	"golang.org/x/image/math/fixed"

	ot "github.com/go-text/typesetting-utils/opentype"
)

func Test_Render(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 425, 250))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	data, _ := os.Open("testdata/NotoSans-Regular.ttf")
	f1, _ := font.ParseTTF(data)

	r := &render.Renderer{
		FontSize: 48,
		Color:    color.Black,
	}
	str := "Hello! ± ज्या"
	r.DrawString(str, img, f1)
	r.DrawStringAt(str, img, 0, 100, f1)

	r.PixScale = 2
	r.Color = color.Gray{Y: 0xcc}
	r.DrawStringAt("baseline", img, 0, 180, f1)

	data, _ = os.Open("testdata/NotoSans-Bold.ttf")
	f2, _ := font.ParseTTF(data)
	r.FontSize = 36
	r.Color = color.NRGBA{R: 0xcc, G: 0, B: 0x33, A: 0x99}
	x := r.DrawStringAt("Red", img, 60, 140, f2)
	r.DrawStringAt("Bold", img, x, 140, f2)

	// from https://github.com/adobe-fonts/emojione-color, MIT license
	data, _ = os.Open("testdata/EmojiOneColor.otf")
	f3, _ := font.ParseTTF(data)
	r.FontSize = 36
	r.DrawStringAt("🚀🖥️", img, 270, 80, f3)

	data, _ = os.Open("testdata/Greybeard-22px.ttf")
	f4, _ := font.ParseTTF(data)
	r.FontSize = 22
	r.Color = color.NRGBA{R: 0xcc, G: 0x66, B: 0x33, A: 0xcc}
	r.DrawStringAt("\uE0A2░", img, 366, 164, f4)

	content, _ := ot.Files.ReadFile("bitmap/cherry-10-r.otb")
	f5, _ := font.ParseTTF(bytes.NewReader(content))
	(&render.Renderer{FontSize: 10, PixScale: 1, Color: color.Black}).DrawStringAt("Hello, world!", img, 6, 10, f5)

	str = "Hello ज्या 😀! 🎁 fin."
	rs := []rune(str)
	sh := &shaping.HarfbuzzShaper{}
	in := shaping.Input{
		Text:     rs,
		RunStart: 0,
		RunEnd:   len(rs),
		Size:     fixed.I(int(r.FontSize)),
	}
	seg := shaping.Segmenter{}
	runs := seg.Split(in, fixedFontmap([]*font.Face{f1, f2, f3}))

	line := make(shaping.Line, len(runs))
	for i, run := range runs {
		line[i] = sh.Shape(run)
	}

	x = 0
	r.Color = color.NRGBA{R: 0x33, G: 0x99, B: 0x33, A: 0xcc}
	for _, run := range line {
		x = r.DrawShapedRunAt(run, img, x, 232)
	}

	w, _ := os.Create("testdata/out.png")
	png.Encode(w, img)
	w.Close()
}

func TestRender_PixScaleAdvance(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 350, 180))

	data, _ := os.Open("testdata/NotoSans-Regular.ttf")
	f, _ := font.ParseTTF(data)

	r := &render.Renderer{
		FontSize: 48,
		Color:    color.Black,
	}
	str := "Testing"
	adv0 := r.DrawString(str, img, f)

	r.PixScale = 1 // instead of the zero value
	adv1 := r.DrawString(str, img, f)
	if adv0 != adv1 {
		t.Error("unscaled font did not advance as default")
	}

	r.PixScale = 2
	adv2 := r.DrawString(str, img, f)
	if adv2 <= int(float32(adv1)*1.9) || adv2 >= int(float32(adv1)*2.1) {
		t.Error("scaled font did not advance proportionately")
	}
}

func TestRenderHindi(t *testing.T) {
	text := "नमस्ते"
	r := &render.Renderer{
		FontSize: 30,
		Color:    color.Black,
	}

	img := image.NewNRGBA(image.Rect(0, 0, 120, 50))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	data, _ := os.Open("testdata/NotoSans-Regular.ttf")
	face, _ := font.ParseTTF(data)

	r.DrawString(text, img, face)

	w, _ := os.Create("testdata/out_hindi.png")
	png.Encode(w, img)
	w.Close()
}

type fixedFontmap []*font.Face

// ResolveFace panics if the slice is empty
func (ff fixedFontmap) ResolveFace(r rune) *font.Face {
	for _, f := range ff {
		if _, has := f.NominalGlyph(r); has {
			return f
		}
	}
	return ff[0]
}

func TestBitmapBaseline(t *testing.T) {
	text := "\U0001F615\U0001F618\U0001F616"
	r := &render.Renderer{
		FontSize: 40,
		Color:    color.Black,
	}

	img := image.NewNRGBA(image.Rect(0, 0, 150, 100))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	data, _ := ot.Files.ReadFile("bitmap/NotoColorEmoji.ttf")
	face, _ := font.ParseTTF(bytes.NewReader(data))

	r.DrawString(text, img, face)

	// w, _ := os.Create("testdata/bitmap_emoji.png")
	// png.Encode(w, img)
	// w.Close()

	// compare against the reference
	assertImageMatchesReference(t, img, "testdata/bitmap_emoji.png")
}

// assertImageMatchesReference compares a rendered image against a reference
// PNG, tolerating small per-pixel differences: antialiasing coverage varies
// slightly between CPU architectures (e.g. FMA rounding on arm64 vs amd64),
// so a byte-exact comparison would be flaky across platforms.
func assertImageMatchesReference(t *testing.T, img image.Image, path string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening reference image: %v", err)
	}
	defer f.Close()
	ref, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding reference image: %v", err)
	}

	if ref.Bounds() != img.Bounds() {
		t.Errorf("unexpected image bounds: got %v, want %v", img.Bounds(), ref.Bounds())
		return
	}
	// Out of the 16-bit channel range; ~8x the largest difference observed
	// between architectures, well below genuine rendering changes.
	const tolerance = 0x0800
	for y := ref.Bounds().Min.Y; y < ref.Bounds().Max.Y; y++ {
		for x := ref.Bounds().Min.X; x < ref.Bounds().Max.X; x++ {
			rr, rg, rb, ra := ref.At(x, y).RGBA()
			gr, gg, gb, ga := img.At(x, y).RGBA()
			if delta(rr, gr) > tolerance || delta(rg, gg) > tolerance ||
				delta(rb, gb) > tolerance || delta(ra, ga) > tolerance {
				t.Errorf("unexpected image output: pixel (%d, %d) is %v, want %v", x, y, img.At(x, y), ref.At(x, y))
				return
			}
		}
	}
}

func delta(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestMixedLTR_RTL(t *testing.T) {
	s := "وَرَسُولِهِ وَإِنْ تُبْتُمْ فَلَكُمْ رُءُوسُ hello أَمْوَالِكُمْ لَا تَظْلِمُونَ وَلَا تُظْلَمُونَ ( 279 ) وَإِنْ كَانَ ذُو عُسْرَةٍ فَنَظِرَةٌ إِلَى مَيْسَرَةٍ"

	data, _ := ot.Files.ReadFile("common/NotoSansArabic.ttf")
	face, _ := font.ParseTTF(bytes.NewReader(data))

	img := image.NewNRGBA(image.Rect(0, 0, 800, 40))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	r := render.Renderer{FontSize: 15, Color: color.Black}
	r.DrawString(s, img, face)

	// w, _ := os.Create("testdata/mixed_ltr_rtl.png")
	// _ = png.Encode(w, img)
	// w.Close()

	// compare against the reference
	assertImageMatchesReference(t, img, "testdata/mixed_ltr_rtl.png")
}
