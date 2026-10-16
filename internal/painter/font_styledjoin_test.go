package painter

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/lang"
)

// joinWord is three Arabic beh, a dual-joining letter, so the word connects
// cursively across any split point — the seam glyphs take medial forms that are
// distinct (and narrower) from the isolated/initial/final forms a run shaped on
// its own would use. pre-rebrand Issue pre-rebrand tracker #26.
const joinWord = "ببب"

// TestStyledLineVisualLayoutCrossRunJoining verifies that an Arabic word split
// across two same-face style runs is shaped with cursive joining preserved, so
// the split layout matches shaping the whole word as a single run (pre-rebrand issue #26).
func TestStyledLineVisualLayoutCrossRunJoining(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	rs := []rune(joinWord)

	whole := StyledLineVisualLayout([]StyledRun{{Runes: rs, TextSize: 14}})
	split := StyledLineVisualLayout([]StyledRun{
		{Runes: rs[:1], TextSize: 14},
		{Runes: rs[1:], TextSize: 14},
	})

	// Each half shaped as its own line gets no cross-run joining. If their
	// combined width already equals the whole word, this font does not join at
	// the seam and the test cannot distinguish the fix — skip.
	isoA := StyledLineVisualLayout([]StyledRun{{Runes: rs[:1], TextSize: 14}})
	isoB := StyledLineVisualLayout([]StyledRun{{Runes: rs[1:], TextSize: 14}})
	if whole.Width == 0 || math.Abs(float64(isoA.Width+isoB.Width-whole.Width)) < 0.5 {
		t.Skip("font does not join this Arabic word on this platform; cannot distinguish joining")
	}

	assert.InDelta(t, whole.Width, split.Width, 0.01,
		"split same-face RTL runs must shape (and so measure) as the joined whole word")
	assert.Equal(t, whole.Carets, split.Carets,
		"joined split must produce the same caret table as the whole word")
}

// TestDrawStyledRunsCrossRunJoining is an end-to-end check that the same word,
// split across two same-colour same-face runs, renders pixel-identically to the
// whole word — i.e. DrawStyledRuns shapes the runs together (pre-rebrand issue #26).
func TestDrawStyledRunsCrossRunJoining(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	rs := []rune(joinWord)
	black := color.NRGBA{A: 255}

	whole := image.NewRGBA(image.Rect(0, 0, 200, 40))
	DrawStyledRuns(whole, []canvas.TextRun{{Text: joinWord, Color: black}}, color.Black, 20, 1)

	split := image.NewRGBA(image.Rect(0, 0, 200, 40))
	DrawStyledRuns(split, []canvas.TextRun{
		{Text: string(rs[:1]), Color: black},
		{Text: string(rs[1:]), Color: black},
	}, color.Black, 20, 1)

	if imageEmpty(whole) {
		t.Skip("font lacks Arabic glyph coverage on this platform; skipping pixel assertion")
	}
	assert.True(t, imagesEqual(whole, split),
		"a same-colour word split across two same-face runs must render identically to the whole word")
}

// TestGroupStyledRunsForDraw covers the grouping that drives cross-run joining:
// adjacent same-face RTL runs merge (carrying per-rune colour), while pure-LTR
// neighbours, face changes and default-colour fallback behave as documented.
func TestGroupStyledRunsForDraw(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}

	rtl := groupStyledRunsForDraw([]canvas.TextRun{
		{Text: "بب", Color: red},
		{Text: "بب", Color: blue},
	}, color.Black, 14)
	assert.Len(t, rtl, 1, "adjacent same-face RTL runs must merge into one group")
	assert.Equal(t, []color.Color{red, red, blue, blue}, rtl[0].colors,
		"each rune keeps its originating run's colour")

	ltr := groupStyledRunsForDraw([]canvas.TextRun{
		{Text: "ab", Color: red},
		{Text: "cd", Color: blue},
	}, color.Black, 14)
	assert.Len(t, ltr, 2, "pure-LTR neighbours keep the legacy one-group-per-run path")

	face := groupStyledRunsForDraw([]canvas.TextRun{
		{Text: "بب", Color: red},
		{Text: "بب", Color: blue, TextStyle: bellui.TextStyle{Bold: true}},
	}, color.Black, 14)
	assert.Len(t, face, 2, "a face change must start a new group even for RTL")

	def := groupStyledRunsForDraw([]canvas.TextRun{{Text: "بب"}}, red, 14)
	assert.Equal(t, []color.Color{red, red}, def[0].colors,
		"runs without an explicit colour fall back to the default colour")
}

func imageEmpty(img *image.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				return false
			}
		}
	}
	return true
}

func imagesEqual(a, b *image.RGBA) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	return bytes.Equal(a.Pix, b.Pix)
}
