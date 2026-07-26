package theme

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HCT_RoundTrip(t *testing.T) {
	colors := []color.NRGBA{
		{R: 0x67, G: 0x50, B: 0xA4, A: 0xff}, // MD3 baseline primary
		{R: 0xff, G: 0x00, B: 0x00, A: 0xff}, // pure red
		{R: 0x00, G: 0xff, B: 0x00, A: 0xff}, // pure green
		{R: 0x00, G: 0x00, B: 0xff, A: 0xff}, // pure blue
		{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // white
		{R: 0x00, G: 0x00, B: 0x00, A: 0xff}, // black
		{R: 0x80, G: 0x80, B: 0x80, A: 0xff}, // mid gray
		{R: 0x29, G: 0x6F, B: 0xF6, A: 0xff}, // Bellui blue primary
	}

	for _, c := range colors {
		hct := HCTFromColor(c)
		back := hct.ToColor()

		// Allow ±2 per channel due to floating point precision
		assert.InDelta(t, int(c.R), int(back.R), 3, "R mismatch for %v (hct=%v)", c, hct)
		assert.InDelta(t, int(c.G), int(back.G), 3, "G mismatch for %v (hct=%v)", c, hct)
		assert.InDelta(t, int(c.B), int(back.B), 3, "B mismatch for %v (hct=%v)", c, hct)
	}
}

func Test_HCT_ToneRange(t *testing.T) {
	hct := HCTFromColor(color.NRGBA{R: 0x67, G: 0x50, B: 0xA4, A: 0xff})

	// Tone should be between 0 and 100
	assert.Greater(t, hct.Tone, 0.0)
	assert.Less(t, hct.Tone, 100.0)

	// Chroma should be positive
	assert.Greater(t, hct.Chroma, 0.0)

	// Hue should be 0–360
	assert.GreaterOrEqual(t, hct.Hue, 0.0)
	assert.Less(t, hct.Hue, 360.0)
}

func Test_TonalPalette_Tones(t *testing.T) {
	tp := NewTonalPalette(270, 40)

	tone0 := tp.Tone(0)
	tone50 := tp.Tone(50)
	tone100 := tp.Tone(100)

	// Tone 0 should be black
	assert.Equal(t, uint8(0), tone0.R)
	assert.Equal(t, uint8(0), tone0.G)
	assert.Equal(t, uint8(0), tone0.B)

	// Tone 100 should be white
	assert.Equal(t, uint8(255), tone100.R)
	assert.Equal(t, uint8(255), tone100.G)
	assert.Equal(t, uint8(255), tone100.B)

	// Tone 50 should be mid-range
	hct50 := HCTFromColor(tone50)
	assert.InDelta(t, 50, hct50.Tone, 5)
}

func Test_TonalPalette_MonotoneTone(t *testing.T) {
	tp := NewTonalPalette(200, 30)

	// Each tone should be lighter than the previous
	tones := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 95, 99, 100}
	prevLuminance := -1.0
	for _, tone := range tones {
		c := tp.Tone(tone)
		lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
		assert.Greater(t, lum, prevLuminance, "tone %d should be lighter than previous", tone)
		prevLuminance = lum
	}
}

func Test_CorePalettesFromPrimary(t *testing.T) {
	primary := color.NRGBA{R: 0x29, G: 0x6F, B: 0xF6, A: 0xff} // Bellui blue
	palettes := CorePalettesFromPrimary(primary)

	assert.NotNil(t, palettes.Primary)
	assert.NotNil(t, palettes.Secondary)
	assert.NotNil(t, palettes.Tertiary)
	assert.NotNil(t, palettes.Error)
	assert.NotNil(t, palettes.Neutral)
	assert.NotNil(t, palettes.NeutralVariant)

	// Primary should preserve the hue
	primaryHCT := HCTFromColor(primary)
	assert.InDelta(t, primaryHCT.Hue, palettes.Primary.hue, 1)

	// Secondary should have same hue but lower chroma
	assert.InDelta(t, primaryHCT.Hue, palettes.Secondary.hue, 1)
	assert.Less(t, palettes.Secondary.chroma, palettes.Primary.chroma)

	// Tertiary should have shifted hue
	expectedTertiaryHue := math.Mod(primaryHCT.Hue+60, 360)
	assert.InDelta(t, expectedTertiaryHue, palettes.Tertiary.hue, 1)

	// Error should be fixed red-ish hue
	assert.InDelta(t, 25.0, palettes.Error.hue, 1)

	// Neutral should have very low chroma
	assert.LessOrEqual(t, palettes.Neutral.chroma, 5.0)
}
