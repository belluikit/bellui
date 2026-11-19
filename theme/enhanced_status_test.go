package theme

import (
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/bellui"
)

// Warning is amber and success green in both variants, at MD3's
// custom-colour tones, with on-colours that read on them; neither comes
// from the primary-derived scheme (#1338). Success was the scheme's
// tertiary, pink with the default blue, and warning a red-orange salmon.
func Test_EnhancedTheme_StatusColors(t *testing.T) {
	th := DefaultTheme()
	hueDist := func(a, b float64) float64 {
		d := math.Abs(a - b)
		return math.Min(d, 360-d)
	}
	for _, tc := range []struct {
		name, on bellui.ThemeColorName
		hue      float64 // the builtin constant's: #FF9800, #43F436
	}{
		{ColorNameWarning, ColorNameForegroundOnWarning, HCTFromColor(colorDarkWarning).Hue},
		{ColorNameSuccess, ColorNameForegroundOnSuccess, HCTFromColor(colorDarkSuccess).Hue},
	} {
		for _, v := range []bellui.ThemeVariant{VariantLight, VariantDark} {
			c := th.Color(tc.name, v).(color.NRGBA)
			on := th.Color(tc.on, v).(color.NRGBA)
			h := HCTFromColor(c)
			// The tone's gamut clip moves the hue a little; the old
			// values were 30° (warning) and 200° (success) away.
			if d := hueDist(h.Hue, tc.hue); d > 10 {
				t.Errorf("%s variant %d = %v, hue %.0f, want near %.0f", tc.name, v, c, h.Hue, tc.hue)
			}
			wantTone, onTone := 80.0, 20.0
			if v == VariantLight {
				wantTone, onTone = 40, 100
			}
			if math.Abs(h.Tone-wantTone) > 2 {
				t.Errorf("%s variant %d tone %.1f, want %.0f", tc.name, v, h.Tone, wantTone)
			}
			if got := HCTFromColor(on).Tone; math.Abs(got-onTone) > 2 {
				t.Errorf("%s variant %d tone %.1f, want %.0f", tc.on, v, got, onTone)
			}
		}
	}
}

// A theme wrapping its own MD3 scheme (an app's primary) falls back to its
// base for warning and success: the scheme has no say in them (#1338).
func Test_ColorFromMD3Scheme_NoStatusColors(t *testing.T) {
	for _, primary := range []color.NRGBA{{R: 0xE5, G: 0x39, B: 0x35, A: 0xff}, {R: 0x15, G: 0x65, B: 0xC0, A: 0xff}} {
		p := CorePalettesFromPrimary(primary)
		for _, s := range []*MD3Scheme{LightSchemeFromPalettes(p), DarkSchemeFromPalettes(p)} {
			for _, n := range []bellui.ThemeColorName{ColorNameWarning, ColorNameForegroundOnWarning, ColorNameSuccess, ColorNameForegroundOnSuccess} {
				if c := ColorFromMD3Scheme(s, n, VariantDark); c != nil {
					t.Errorf("primary %v: ColorFromMD3Scheme(%s) = %v, want nil", primary, n, c)
				}
			}
		}
	}
}
