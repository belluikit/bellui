package lang

import (
	"testing"

	"github.com/jeandeaual/go-locale"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestParseUserLocales(t *testing.T) {
	toStrings := func(tags []language.Tag) []string {
		out := make([]string, len(tags))
		for i, tag := range tags {
			out[i] = tag.String()
		}
		return out
	}

	// Well-formed entries pass through untouched; surrounding whitespace is
	// trimmed rather than costing the entry.
	assert.Equal(t, []string{"en-US", "en-GB"},
		toStrings(parseUserLocales([]string{"en-US", "  en-GB  "})))

	// Java Locale.toString() forms normalize: underscores, and the trailing
	// "#..." marker — whose script is folded back into the tag while its
	// Unicode extension is dropped (#271).
	assert.Equal(t, []string{"en-US", "zh-Hans-CN", "sr-Latn-RS", "de-DE"},
		toStrings(parseUserLocales([]string{"en_US", "zh_CN_#Hans", "sr_RS_#Latn", "de_DE_#u-co-phonebk"})))

	// An entry that is not well-formed still contributes whatever part of the
	// tag parsed — it must not pass the locale to the entry behind it.
	assert.Equal(t, []string{"th-TH", "nb-NO"},
		toStrings(parseUserLocales([]string{"th_TH_TH_#u-nu-thai", "nb-NO"})))

	// Entries carrying nothing at all are dropped rather than passed on as
	// und tags — an und currentTag aliases the "not yet initialised" zero
	// Tag. A failed Android lookup arrives as a list holding one empty
	// string (#271).
	assert.Equal(t, []string{"nb-NO"},
		toStrings(parseUserLocales([]string{"", "nb-NO"})))

	assert.Empty(t, parseUserLocales([]string{"", "   ", "utter junk!!", "und", "root"}))
	assert.Empty(t, parseUserLocales(nil))
}

func TestApplyUserLocales_MalformedLeadingEntry(t *testing.T) {
	SetLocaleForTests(t, "en") // restores the localizer and tag on cleanup

	// The region-carrying tag that drives CLDR formatting, collation and
	// direction comes from the first entry that parsed. Before #271 a
	// malformed leading entry skipped the override entirely, silently
	// degrading it to the matcher's language-only tag.
	applyUserLocales([]string{"", "fr-FR"})
	assert.Equal(t, "fr-FR", getCurrentTag().String())
	assert.Equal(t, "fr-FR", CurrentLocale().String())

	// Nothing parseable at all degrades to English, like the C/POSIX empty
	// list (pre-rebrand tracker #1148) — not to an und tag.
	applyUserLocales([]string{""})
	assert.Equal(t, "en", getCurrentTag().String())
}

func TestClosestSupportedLocale_SkipsMalformedEntries(t *testing.T) {
	// A malformed leading entry must not cost the locale the parseable
	// entries behind it (#271). This pins the contract rather than guarding a
	// regression: language.NewMatcher already skipped und candidates, so the
	// translation lookup survived a bad entry even before #271 — it was
	// currentTag that degraded (see TestApplyUserLocales_MalformedLeadingEntry).
	assert.Equal(t, "fr", closestSupportedLocale([]string{"", "fr-FR"}).LanguageString())

	// Nothing parseable at all degrades to the matcher default rather than
	// poisoning it with und tags.
	assert.Equal(t, "en", closestSupportedLocale([]string{""}).LanguageString())
}

func TestSystemLocale(t *testing.T) {
	info, err := locale.GetLocale()
	if err != nil {
		// something not testable
		t.Log("Unable to run locale test because", err)
		return
	}

	if len(info) < 2 {
		info = "en_US"
	}

	loc := SystemLocale()
	assert.Equal(t, info[:2], loc.String()[:2])
}

func TestCurrentLocale(t *testing.T) {
	// With an active override, CurrentLocale reflects it (unlike SystemLocale).
	SetLocaleForTests(t, "sr-RS")
	assert.Equal(t, "sr-RS", CurrentLocale().String())
	assert.True(t, IsRTL() == IsRTLLocale(CurrentLocale()),
		"CurrentLocale should agree with the source IsRTL uses")

	SetLocaleForTests(t, "ar")
	assert.Equal(t, "ar", CurrentLocale().String())
}

// A match the matcher has no confidence in is a guess from a shared script, so
// the app's default language wins over it, unless the user's region speaks the
// guessed language (#342).
func TestClosestSupportedLocale_NoConfidenceFallsBackToDefault(t *testing.T) {
	tagMu.Lock()
	saved := translated
	translated = []language.Tag{language.English, language.Arabic, language.Russian, language.SimplifiedChinese}
	tagMu.Unlock()
	t.Cleanup(func() {
		tagMu.Lock()
		translated = saved
		tagMu.Unlock()
	})

	for user, want := range map[string]string{
		"fa-IR":    "en", // was ar: Persian is not Arabic
		"fa":       "en",
		"ug-CN":    "en", // was ar
		"sr-RS":    "en", // was ru
		"bg":       "en", // was ru
		"ce-RU":    "ru", // Chechen users in Russia read Russian
		"sah":      "ru",
		"yue-Hans": "zh", // simplified-script Cantonese reads as Simplified Chinese
		"ar-EG":    "ar", // a real match is untouched
		"kk":       "ru", // as is a high-confidence one
		"pt-BR":    "en",
	} {
		assert.Equal(t, want, closestSupportedLocale([]string{user}).LanguageString(), user)
	}
	assert.Equal(t, "ar", closestSupportedLocale([]string{"fa-IR", "ar-EG"}).LanguageString(),
		"a later entry that really matches still wins")
}

// The issue's case end to end: an app that ships Arabic but not Persian shows
// Persian users its default language, not Arabic (#342).
func TestPersianUserOfAnArabicApp_GetsTheDefaultLanguage(t *testing.T) {
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = { "greeting342": "مرحبا" }`), "ar"))

	SetLocaleForTests(t, "fa-IR")
	assert.Equal(t, "greeting342", L("greeting342"))

	SetLocaleForTests(t, "ar-EG")
	assert.Equal(t, "مرحبا", L("greeting342"))
}
