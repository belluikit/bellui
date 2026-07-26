package painter_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/painter"
	intTest "github.com/belluikit/bellui/internal/test"
	"github.com/belluikit/bellui/test"
)

func TestCachedFontFace(t *testing.T) {
	for name, tt := range map[string]struct {
		style bellui.TextStyle
		runes string
	}{
		"symbol font": {
			bellui.TextStyle{
				Symbol: true,
			},
			"←↑→↓↖↘↵↵⇞⇟⇥⇧⌃⌘⌥⌦⌫⎋␣⌃⌥⇧⌘",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := painter.CachedFontFace(tt.style, nil, nil)
			for _, r := range tt.runes {
				f := got.Fonts.ResolveFace(r)
				assert.NotNil(t, f, "symbol Font should include: %c", r)
			}
		})
	}

	// check the wide symbol rune
	symbol := canvas.NewText("⌘", color.Black)
	symbol.TextStyle.Symbol = true
	assert.True(t, symbol.MinSize().Width > 10)
}

func TestDrawString(t *testing.T) {
	for name, tt := range map[string]struct {
		color    color.Color
		style    bellui.TextStyle
		size     float32
		string   string
		tabWidth int
		want     string
	}{
		"regular": {
			color:    color.Black,
			style:    bellui.TextStyle{},
			size:     40,
			string:   "Hello\tworld!",
			tabWidth: 7,
			want:     "hello_TAB_world_regular_size_40_height_50_tab_width_7.png",
		},
		"bold italic": {
			color:    color.NRGBA{R: 255, A: 255},
			style:    bellui.TextStyle{Bold: true, Italic: true},
			size:     27.42,
			string:   "Hello\tworld!",
			tabWidth: 3,
			want:     "hello_TAB_world_bold_italic_size_27.42_height_42_tab_width_3.png",
		},
		"missing glyphs": {
			color:    color.Black,
			style:    bellui.TextStyle{},
			size:     40,
			string:   "Missing: स",
			tabWidth: 4,
			want:     "missing_glyph.png",
		},
	} {
		t.Run(name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 300, 100))
			f := painter.CachedFontFace(tt.style, nil, nil)

			fontMap := &intTest.FontMap{f.Fonts.ResolveFace(' ')} // first (ascii) font
			painter.DrawString(img, tt.string, tt.color, fontMap, tt.size, 1, bellui.TextStyle{TabWidth: tt.tabWidth})
			test.AssertImageMatches(t, "font/"+tt.want, img)
		})
	}
}

// TestDrawString_Underline exercises the drawUnderline path of DrawString.
// It renders the same text with and without the Underline style and asserts
// that the underlined render adds a horizontal line below the glyphs: some row
// that is fully transparent without the underline gains a wide contiguous run
// of drawn pixels with it. This is font-robust (it does not depend on the exact
// glyph shapes, only that an underline stroke is added).
func TestDrawString_Underline(t *testing.T) {
	const w, h = 300, 100
	f := painter.CachedFontFace(bellui.TextStyle{}, nil, nil)
	fontMap := &intTest.FontMap{f.Fonts.ResolveFace(' ')}

	render := func(style bellui.TextStyle) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		painter.DrawString(img, "Underlined", color.Black, fontMap, 40, 1, style)
		return img
	}
	plain := render(bellui.TextStyle{})
	under := render(bellui.TextStyle{Underline: true})

	drawn := func(img *image.NRGBA, x, y int) bool { _, _, _, a := img.At(x, y).RGBA(); return a > 0 }
	widestRun := func(img *image.NRGBA, y int) int {
		best, cur := 0, 0
		for x := 0; x < w; x++ {
			if drawn(img, x, y) {
				cur++
				if cur > best {
					best = cur
				}
			} else {
				cur = 0
			}
		}
		return best
	}

	// Find a row empty in the plain render but carrying a long horizontal stroke
	// once underlined — that stroke is the underline.
	found := false
	for y := 0; y < h; y++ {
		if widestRun(plain, y) == 0 && widestRun(under, y) >= 40 {
			found = true
			break
		}
	}
	assert.True(t, found, "underline style should add a horizontal stroke below the text that is absent without it")
}

// TestDrawString_Strikethrough exercises the drawStrikethrough path: the same
// text rendered with Strikethrough gains a wide horizontal stroke through the
// glyph body, positioned above the baseline (unlike the underline). Robust to
// glyph shape — it only checks that a long contiguous stroke appears.
func TestDrawString_Strikethrough(t *testing.T) {
	const w, h = 300, 100
	f := painter.CachedFontFace(bellui.TextStyle{}, nil, nil)
	fontMap := &intTest.FontMap{f.Fonts.ResolveFace(' ')}

	render := func(style bellui.TextStyle) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		painter.DrawString(img, "Struck", color.Black, fontMap, 40, 1, style)
		return img
	}
	plain := render(bellui.TextStyle{})
	struck := render(bellui.TextStyle{Strikethrough: true})
	under := render(bellui.TextStyle{Underline: true})

	drawn := func(img *image.NRGBA, x, y int) bool { _, _, _, a := img.At(x, y).RGBA(); return a > 0 }
	widestRun := func(img *image.NRGBA, y int) int {
		best, cur := 0, 0
		for x := 0; x < w; x++ {
			if drawn(img, x, y) {
				cur++
				if cur > best {
					best = cur
				}
			} else {
				cur = 0
			}
		}
		return best
	}

	// The strikethrough adds a long stroke on a row that gains a lot of width
	// relative to the plain render (the plain glyphs never span most of the
	// text width on a single row).
	strikeRow := -1
	for y := 0; y < h; y++ {
		if widestRun(struck, y)-widestRun(plain, y) >= 60 {
			strikeRow = y
			break
		}
	}
	assert.GreaterOrEqual(t, strikeRow, 0, "strikethrough should add a horizontal stroke through the text")

	// The strike sits above where the underline lands.
	underRow := -1
	for y := 0; y < h; y++ {
		if widestRun(plain, y) == 0 && widestRun(under, y) >= 40 {
			underRow = y
			break
		}
	}
	if underRow >= 0 {
		assert.Less(t, strikeRow, underRow, "strikethrough should sit above the underline")
	}
}

func TestMeasureString(t *testing.T) {
	for name, tt := range map[string]struct {
		style    bellui.TextStyle
		size     float32
		string   string
		tabWidth int
		want     float32
	}{
		"regular": {
			style:    bellui.TextStyle{},
			size:     40,
			string:   "Hello\tworld!",
			tabWidth: 7,
			want:     249.04688,
		},
		"bold italic": {
			style:    bellui.TextStyle{Bold: true, Italic: true},
			size:     27.42,
			string:   "Hello\tworld!",
			tabWidth: 3,
			want:     165.01562,
		},
		"missing glyph": {
			style:    bellui.TextStyle{},
			size:     40,
			string:   "Missing: स",
			tabWidth: 4,
			want:     195.3125,
		},
	} {
		t.Run(name, func(t *testing.T) {
			faces := painter.CachedFontFace(tt.style, nil, nil)
			fontMap := &intTest.FontMap{faces.Fonts.ResolveFace(' ')} // first (ascii) font
			got, _ := painter.MeasureString(fontMap, tt.string, tt.size, bellui.TextStyle{TabWidth: tt.tabWidth})
			assert.Equal(t, tt.want, got.Width)
		})
	}
}

func TestRenderedTextSize(t *testing.T) {
	size1, baseline1 := painter.RenderedTextSize("Hello World!", 20, bellui.TextStyle{}, nil)
	size2, baseline2 := painter.RenderedTextSize("\rH\re\rl\rl\ro\r \rW\ro\rr\rl\rd\r!\r", 20, bellui.TextStyle{}, nil)
	assert.Equal(t, int(size1.Width), int(size2.Width))
	assert.Equal(t, size1.Height, size2.Height)
	assert.Equal(t, baseline1, baseline2)
}

func TestRenderedTextSize_FontWeight(t *testing.T) {
	normalSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{}, nil)
	boldSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{Bold: true}, nil)
	weightBoldSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}, nil)

	// Bold and Weight=700 should produce the same width.
	assert.Equal(t, int(boldSize.Width), int(weightBoldSize.Width))
	// Bold text is typically wider than normal.
	assert.Greater(t, boldSize.Width, normalSize.Width)
}

func TestRenderedTextSize_LineHeight(t *testing.T) {
	normalSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{}, nil)
	tallerSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{LineHeightScale: bellui.Ptr(float32(1.5))}, nil)

	// 1.5x line height should make the line taller.
	assert.Greater(t, tallerSize.Height, normalSize.Height)
	// Width should not change.
	assert.Equal(t, normalSize.Width, tallerSize.Width)
}

func TestRenderedTextSize_LetterSpacing(t *testing.T) {
	normalSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{}, nil)
	spacedSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{LetterSpacing: bellui.Ptr(float32(2))}, nil)
	tightSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{LetterSpacing: bellui.Ptr(float32(-1))}, nil)

	// Positive letter spacing should make text wider.
	assert.Greater(t, spacedSize.Width, normalSize.Width)
	// Negative letter spacing should make text narrower.
	assert.Less(t, tightSize.Width, normalSize.Width)
}

func TestRenderedTextSize_WeightSemiBoldThreshold(t *testing.T) {
	// The bold theme-font variant is selected when the resolved weight reaches
	// WeightSemiBold (600) — see font.go (resolvedWeight >= bellui.WeightSemiBold).
	// Verify the boundary exactly: 600 crosses it (bold face), 599 stays below it
	// (regular face). The 500/700 cases are covered elsewhere; 600 is the hinge.
	belowSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{Weight: bellui.Ptr(bellui.TextWeight(599))}, nil)
	semiBoldSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightSemiBold)}, nil)
	boldSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{Bold: true}, nil)

	// Exactly 600 crosses the threshold, selecting the same bold face as Bold:true.
	assert.Equal(t, int(boldSize.Width), int(semiBoldSize.Width))
	// 599 stays below the threshold and uses the (narrower) regular face.
	assert.Less(t, belowSize.Width, semiBoldSize.Width)
}

func TestRenderedTextSize_WeightMedium(t *testing.T) {
	normalSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{}, nil)
	mediumSize, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightMedium)}, nil)

	// Medium weight (500) should produce valid dimensions.
	assert.Greater(t, mediumSize.Width, float32(0))
	assert.Greater(t, mediumSize.Height, float32(0))
	// Height should be the same as normal (same font metrics).
	assert.Equal(t, normalSize.Height, mediumSize.Height)
}

func TestMeasureStyledRuns_SingleRun(t *testing.T) {
	runs := []canvas.TextRun{
		{Text: "Hello World"},
	}
	size, advances := painter.MeasureStyledRuns(runs, 20)
	assert.Greater(t, size.Width, float32(0))
	assert.Greater(t, size.Height, float32(0))
	assert.Len(t, advances, 1)
	assert.Greater(t, advances[0], float32(0))
}

func TestMeasureStyledRuns_MultipleRuns(t *testing.T) {
	runs := []canvas.TextRun{
		{Text: "Hello"},
		{Text: " "},
		{Text: "World"},
	}
	size, advances := painter.MeasureStyledRuns(runs, 20)
	assert.Greater(t, size.Width, float32(0))
	assert.Greater(t, size.Height, float32(0))
	assert.Len(t, advances, 3)

	// Total width should equal sum of advances.
	total := advances[0] + advances[1] + advances[2]
	assert.InDelta(t, total, size.Width, 0.01)
}

func TestMeasureStyledRuns_EmptyRuns(t *testing.T) {
	runs := []canvas.TextRun{
		{Text: ""},
		{Text: "Hello"},
		{Text: ""},
	}
	size, advances := painter.MeasureStyledRuns(runs, 20)
	assert.Greater(t, size.Width, float32(0))
	assert.Len(t, advances, 3)
	assert.Equal(t, float32(0), advances[0])
	assert.Greater(t, advances[1], float32(0))
	assert.Equal(t, float32(0), advances[2])
}

func TestMeasureStyledRuns_NilRuns(t *testing.T) {
	size, advances := painter.MeasureStyledRuns(nil, 20)
	assert.Equal(t, float32(0), size.Width)
	assert.Equal(t, float32(0), size.Height)
	assert.Len(t, advances, 0)
}

func TestMeasureStyledRuns_DifferentSizes(t *testing.T) {
	small := []canvas.TextRun{{Text: "Hello", TextSize: 10}}
	large := []canvas.TextRun{{Text: "Hello", TextSize: 30}}

	smallSize, _ := painter.MeasureStyledRuns(small, 14)
	largeSize, _ := painter.MeasureStyledRuns(large, 14)

	// Larger text size should produce wider and taller output.
	assert.Greater(t, largeSize.Width, smallSize.Width)
	assert.Greater(t, largeSize.Height, smallSize.Height)
}

func TestMeasureStyledRuns_DefaultSizeFallback(t *testing.T) {
	// Run with TextSize 0 should use the defaultSize.
	withDefault := []canvas.TextRun{{Text: "Hello", TextSize: 0}}
	explicit := []canvas.TextRun{{Text: "Hello", TextSize: 20}}

	size1, _ := painter.MeasureStyledRuns(withDefault, 20)
	size2, _ := painter.MeasureStyledRuns(explicit, 20)

	assert.InDelta(t, size2.Width, size1.Width, 0.01)
	assert.InDelta(t, size2.Height, size1.Height, 0.01)
}

func TestDrawStyledRuns_NoPanic(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 50))
	runs := []canvas.TextRun{
		{Text: "Hello"},
		{Text: " World"},
	}
	assert.NotPanics(t, func() {
		painter.DrawStyledRuns(img, runs, color.Black, 20, 1)
	})
}

func TestDrawStyledRuns_WritesPixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 50))
	runs := []canvas.TextRun{
		{Text: "Hello World"},
	}
	painter.DrawStyledRuns(img, runs, color.Black, 20, 1)

	// At least some pixels should be non-zero (text was drawn).
	hasContent := false
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				hasContent = true
				break
			}
		}
		if hasContent {
			break
		}
	}
	assert.True(t, hasContent, "DrawStyledRuns should produce visible output")
}

func TestDrawStyledRuns_EmptyRuns(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 50))
	runs := []canvas.TextRun{{Text: ""}}
	// Should not panic or write anything.
	assert.NotPanics(t, func() {
		painter.DrawStyledRuns(img, runs, color.Black, 20, 1)
	})
}

func TestDrawStyledRuns_ColorFallback(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 50))
	runs := []canvas.TextRun{
		{Text: "Default"}, // nil Color → defaultColor
		{Text: "Red", Color: color.NRGBA{R: 255, A: 255}}, // explicit
	}
	assert.NotPanics(t, func() {
		painter.DrawStyledRuns(img, runs, color.White, 20, 1)
	})
}

func TestClearFontCache(t *testing.T) {
	// ClearFontCache should not panic even when cache is empty.
	assert.NotPanics(t, func() {
		painter.ClearFontCache()
	})

	// Ensure fonts still work after cache clear.
	size, _ := painter.RenderedTextSize("Hello", 20, bellui.TextStyle{}, nil)
	assert.Greater(t, size.Width, float32(0))
}
