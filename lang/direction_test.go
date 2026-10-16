package lang

import (
	"testing"

	"golang.org/x/text/language"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/uidir"
)

func TestIsRTLLocale(t *testing.T) {
	rtl := []string{"ar", "ar-EG", "he", "he-IL", "fa", "fa-IR", "ur", "ur-PK", "ckb"}
	for _, l := range rtl {
		assert.True(t, IsRTLLocale(bellui.Locale(l)), "expected %q to be RTL", l)
	}

	ltr := []string{"en", "en-US", "fr", "fr-FR", "ru", "zh", "ja", "sr-Cyrl", "", "xx"}
	for _, l := range ltr {
		assert.False(t, IsRTLLocale(bellui.Locale(l)), "expected %q to be LTR", l)
	}
}

func TestIsRTL(t *testing.T) {
	SetLocaleForTests(t, "ar")
	assert.True(t, IsRTL(), "Arabic locale should be RTL")

	SetLocaleForTests(t, "he-IL")
	assert.True(t, IsRTL(), "Hebrew locale should be RTL")

	SetLocaleForTests(t, "en-US")
	assert.False(t, IsRTL(), "English locale should be LTR")
}

// TestIsRTLColdStart covers the pre-initialisation state where currentTag is
// the zero (undefined) tag. IsRTL must then follow the system locale rather
// than mis-report und as LTR (pre-rebrand tracker #74). The assertion is system-locale-derived (so
// it holds on any host); without the fix IsRTL would ignore the system locale
// and always return false on this branch.
func TestIsRTLColdStart(t *testing.T) {
	prev := getCurrentTag()
	t.Cleanup(func() { setCurrentTag(prev) })

	setCurrentTag(language.Tag{}) // simulate "lang not initialised yet"
	assert.Equal(t, IsRTLLocale(SystemLocale()), IsRTL(),
		"with no active locale, IsRTL should follow the system locale, not return false for und")
}

func TestIsRTLDirection(t *testing.T) {
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	// Auto defers to the active locale.
	SetLocaleForTests(t, "en-US")
	bellui.SetLayoutDirection(bellui.DirectionAuto)
	assert.False(t, IsRTLDirection(), "auto + LTR locale should be LTR")

	SetLocaleForTests(t, "ar")
	assert.True(t, IsRTLDirection(), "auto + RTL locale should be RTL")

	// Explicit overrides win regardless of locale.
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	assert.False(t, IsRTLDirection(), "explicit LTR override wins over RTL locale")

	SetLocaleForTests(t, "en-US")
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.True(t, IsRTLDirection(), "explicit RTL override wins over LTR locale")
}

func TestIsRTLDirection_AmbientScopeWins(t *testing.T) {
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	SetLocaleForTests(t, "en-US")
	bellui.SetLayoutDirection(bellui.DirectionRTL) // global is RTL
	assert.True(t, IsRTLDirection())

	// An LTR scope flips the resolution for the duration of the push.
	uidir.Push(bellui.DirectionLTR)
	assert.False(t, IsRTLDirection(), "ambient LTR scope wins over global RTL")
	uidir.Pop(bellui.DirectionLTR)

	assert.True(t, IsRTLDirection(), "global RTL restored after pop")

	// A scope overrides even an LTR global / locale.
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	uidir.Push(bellui.DirectionRTL)
	assert.True(t, IsRTLDirection(), "ambient RTL scope wins over global LTR")
	uidir.Pop(bellui.DirectionRTL)
	assert.False(t, IsRTLDirection())
}

func TestResolveAlign(t *testing.T) {
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	bellui.SetLayoutDirection(bellui.DirectionLTR)
	assert.Equal(t, bellui.TextAlignLeading, ResolveAlign(bellui.TextAlignLeading))
	assert.Equal(t, bellui.TextAlignTrailing, ResolveAlign(bellui.TextAlignTrailing))
	assert.Equal(t, bellui.TextAlignCenter, ResolveAlign(bellui.TextAlignCenter))

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.Equal(t, bellui.TextAlignTrailing, ResolveAlign(bellui.TextAlignLeading), "leading flips to trailing under RTL")
	assert.Equal(t, bellui.TextAlignLeading, ResolveAlign(bellui.TextAlignTrailing), "trailing flips to leading under RTL")
	assert.Equal(t, bellui.TextAlignCenter, ResolveAlign(bellui.TextAlignCenter), "center is unchanged under RTL")

	// ResolveAlign is its own inverse, so a double application is identity.
	assert.Equal(t, bellui.TextAlignLeading, ResolveAlign(ResolveAlign(bellui.TextAlignLeading)))
}
