package painter

import (
	"testing"

	"github.com/belluikit/typesetting/di"
	"github.com/belluikit/typesetting/shaping"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

func TestResolveBaseDirection(t *testing.T) {
	assert.Equal(t, di.DirectionLTR, resolveBaseDirection(bellui.TextStyle{Direction: bellui.TextDirectionLTR}))
	assert.Equal(t, di.DirectionRTL, resolveBaseDirection(bellui.TextStyle{Direction: bellui.TextDirectionRTL}))

	// Explicit override wins over the locale.
	lang.SetLocaleForTests(t, "ar")
	assert.Equal(t, di.DirectionLTR, resolveBaseDirection(bellui.TextStyle{Direction: bellui.TextDirectionLTR}))

	// Auto follows the active locale.
	assert.Equal(t, di.DirectionRTL, resolveBaseDirection(bellui.TextStyle{}), "Auto should be RTL under an Arabic locale")
	lang.SetLocaleForTests(t, "en-US")
	assert.Equal(t, di.DirectionLTR, resolveBaseDirection(bellui.TextStyle{}), "Auto should be LTR under an English locale")
}

// TestResolveBaseDirectionFollowsLayoutOverride verifies that text base
// direction is driven by the same ambient UI direction as layout mirroring: an
// explicit SetLayoutDirection override seeds Auto text even under a contrary
// locale, while an explicit TextStyle.Direction still wins.
func TestResolveBaseDirectionFollowsLayoutOverride(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US") // LTR locale
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.Equal(t, di.DirectionRTL, resolveBaseDirection(bellui.TextStyle{}),
		"Auto text should follow the RTL layout override even under an LTR locale")
	assert.Equal(t, di.DirectionLTR, resolveBaseDirection(bellui.TextStyle{Direction: bellui.TextDirectionLTR}),
		"explicit TextDirectionLTR still wins over the layout override")

	bellui.SetLayoutDirection(bellui.DirectionLTR)
	assert.Equal(t, di.DirectionLTR, resolveBaseDirection(bellui.TextStyle{}),
		"Auto text should follow the LTR layout override")
}

// runsFor shapes s through the normal walkString path and returns the per-run
// shaping outputs (which carry the direction actually handed to HarfBuzz).
func runsFor(t *testing.T, s string, style bellui.TextStyle) []shaping.Output {
	t.Helper()
	face := CachedFontFace(style, nil, nil)
	var runs []shaping.Output
	adv := float32(0)
	walkString(face.Fonts, s, float32ToFixed266(14), style, &adv, 1,
		func(out shaping.Output, _ float32) {
			runs = append(runs, out)
		})
	return runs
}

func hasRTLRun(runs []shaping.Output) bool {
	for _, r := range runs {
		if r.Direction.Progression() == di.TowardTopLeft {
			return true
		}
	}
	return false
}

// TestShapingDetectsRTL verifies the core Phase 1 outcome: Arabic and Hebrew
// text is shaped as right-to-left (so it joins and reorders), while Latin stays
// left-to-right — independent of the base direction. Glyph coverage is font
// dependent and deliberately not asserted here.
func TestShapingDetectsRTL(t *testing.T) {
	lang.SetLocaleForTests(t, "en-US")

	arabic := runsFor(t, "مرحبا", bellui.TextStyle{})
	assert.True(t, hasRTLRun(arabic), "Arabic should produce an RTL run even under an LTR locale")

	hebrew := runsFor(t, "שלום", bellui.TextStyle{})
	assert.True(t, hasRTLRun(hebrew), "Hebrew should produce an RTL run")

	latin := runsFor(t, "hello", bellui.TextStyle{})
	assert.False(t, hasRTLRun(latin), "Latin should stay LTR")
}

// TestBaseDirectionAffectsSegmentation verifies that the base direction is
// actually plumbed through: a mixed Latin/Arabic string segments differently
// under an RTL base (neutral characters bind to the RTL context) than under an
// LTR base.
func TestBaseDirectionAffectsSegmentation(t *testing.T) {
	const mixed = "hello مرحبا world"
	ltr := runsFor(t, mixed, bellui.TextStyle{Direction: bellui.TextDirectionLTR})
	rtl := runsFor(t, mixed, bellui.TextStyle{Direction: bellui.TextDirectionRTL})

	assert.NotEqual(t, len(ltr), len(rtl),
		"LTR and RTL base directions should segment mixed text differently")
	assert.True(t, hasRTLRun(ltr), "mixed text always contains an RTL (Arabic) run")
	assert.True(t, hasRTLRun(rtl), "mixed text always contains an RTL (Arabic) run")
}

// TestMeasureRTLStable confirms RTL text measures to a stable, positive width
// (a single run's advance is independent of base direction).
func TestMeasureRTLStable(t *testing.T) {
	face := CachedFontFace(bellui.TextStyle{}, nil, nil)
	sizeLTR, _ := MeasureString(face.Fonts, "مرحبا", 14, bellui.TextStyle{Direction: bellui.TextDirectionLTR})
	sizeRTL, _ := MeasureString(face.Fonts, "مرحبا", 14, bellui.TextStyle{Direction: bellui.TextDirectionRTL})

	assert.Greater(t, sizeLTR.Width, float32(0))
	assert.Equal(t, sizeLTR.Width, sizeRTL.Width, "single-run advance is direction independent")
}
