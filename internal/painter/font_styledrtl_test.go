package painter

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/lang"
)

// TestStyledRunVisualOrderLTR confirms pure-LTR runs keep their logical order.
func TestStyledRunVisualOrderLTR(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	runs := []canvas.TextRun{{Text: "abc"}, {Text: "def"}, {Text: "ghi"}}
	assert.Equal(t, []int{0, 1, 2}, styledRunVisualOrder(runs))
}

// TestStyledRunVisualOrderEmptySkipped confirms empty runs are dropped from the
// draw order.
func TestStyledRunVisualOrderEmptySkipped(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	runs := []canvas.TextRun{{Text: "abc"}, {Text: ""}, {Text: "def"}}
	assert.Equal(t, []int{0, 2}, styledRunVisualOrder(runs))
}

// TestStyledRunVisualOrderRTLReversed confirms that a sequence of RTL runs is
// reversed for drawing: logically [A,B,C] of Arabic renders C,B,A left-to-right.
func TestStyledRunVisualOrderRTLReversed(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	// Three separate Arabic words as distinct runs (e.g. different colors).
	runs := []canvas.TextRun{{Text: "مرحبا"}, {Text: "بك"}, {Text: "هنا"}}
	order := styledRunVisualOrder(runs)
	assert.Equal(t, []int{2, 1, 0}, order,
		"a run of RTL spans must be drawn right-to-left (reversed)")
}

// TestStyledRunVisualOrderMixedBase confirms an LTR base with an embedded RTL
// run keeps the LTR runs in place and orders the embedded RTL run correctly.
func TestStyledRunVisualOrderMixedBase(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")
	// "abc" [Arabic] "def" — base LTR, single embedded RTL run stays put.
	runs := []canvas.TextRun{{Text: "abc "}, {Text: "مرحبا"}, {Text: " def"}}
	order := styledRunVisualOrder(runs)
	assert.Equal(t, []int{0, 1, 2}, order,
		"single embedded RTL run in LTR base keeps logical run order")
}

// TestDrawStyledRunsRTLPlacement is an end-to-end check that DrawStyledRuns
// places a reversed RTL run sequence such that the logically-last run's pixels
// appear on the left. We color each run and look for its color's horizontal
// extent in the rendered image.
func TestDrawStyledRunsRTLPlacement(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	// This end-to-end placement check needs a real Arabic face from the system
	// scan. When the scan is stubbed (e.g. -tags test) there are no system fonts,
	// so the Arabic runs render as same-width tofu boxes with no meaningful
	// horizontal placement — skip rather than assert on that.
	if len(SystemFontFamilies()) == 0 {
		t.Skip("no system fonts available; RTL placement needs a real Arabic face")
	}

	red := color.NRGBA{R: 220, A: 255}
	blue := color.NRGBA{B: 220, A: 255}
	// Two Arabic runs; logically red then blue. Visually blue (logical last)
	// must be left of red.
	runs := []canvas.TextRun{
		{Text: "مرحبا", Color: red},
		{Text: "هنا", Color: blue},
	}

	img := image.NewRGBA(image.Rect(0, 0, 200, 40))
	DrawStyledRuns(img, runs, color.Black, 20, 1)

	redMin := minXWithHue(img, true)
	blueMax := maxXWithHue(img, false)
	if redMin < 0 || blueMax < 0 {
		t.Skip("font lacks Arabic glyph coverage on this platform; skipping pixel assertion")
	}
	assert.Less(t, blueMax, redMin+5,
		"logically-last RTL run (blue) should render to the left of the first (red)")
}

// minXWithHue returns the leftmost x with a reddish (red==true) or bluish pixel,
// or -1 if none.
func minXWithHue(img *image.RGBA, red bool) int {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if hueMatch(img.RGBAAt(x, y), red) {
				return x
			}
		}
	}
	return -1
}

func maxXWithHue(img *image.RGBA, red bool) int {
	b := img.Bounds()
	for x := b.Max.X - 1; x >= b.Min.X; x-- {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if hueMatch(img.RGBAAt(x, y), red) {
				return x
			}
		}
	}
	return -1
}

func hueMatch(c color.RGBA, red bool) bool {
	if c.A < 40 {
		return false
	}
	if red {
		return c.R > 120 && c.B < 80
	}
	return c.B > 120 && c.R < 80
}
