package lang

import (
	"testing"

	"github.com/jeandeaual/go-locale"
	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, []string{"en-US"},
		toStrings(parseUserLocales([]string{"en-US"})))

	// Java Locale.toString() forms normalize: underscores, and the trailing
	// "#..." script/extension marker (#271).
	assert.Equal(t, []string{"en-US", "zh-CN", "de-DE"},
		toStrings(parseUserLocales([]string{"en_US", "zh_CN_#Hans", "de_DE_#u-co-phonebk"})))

	// Unparseable entries are dropped rather than passed to the matcher as
	// und tags — the entries behind them must keep winning. ColorOS has
	// delivered a list holding an empty string (#271).
	assert.Equal(t, []string{"nb-NO"},
		toStrings(parseUserLocales([]string{"", "nb-NO"})))

	assert.Empty(t, parseUserLocales([]string{"", "utter junk!!"}))
}

func TestClosestSupportedLocale_SkipsMalformedEntries(t *testing.T) {
	// A malformed leading entry must not cost the locale the parseable
	// entries behind it (#271).
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
