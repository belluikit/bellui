package lang

import (
	"github.com/jeandeaual/go-locale"
	"golang.org/x/text/language"

	"github.com/belluikit/bellui"
)

// SystemLocale returns the primary locale on the current system.
// This may refer to a language that Bellui does not have translations for.
func SystemLocale() bellui.Locale {
	loc, err := locale.GetLocale()
	if err != nil {
		bellui.LogError("Failed to look up user locale", err)
	}
	if len(loc) < 2 {
		loc = "en"
	}

	tag, err := language.Parse(loc)
	if err != nil {
		bellui.LogError("Error parsing user locale "+loc, err)
	}
	return localeFromTag(tag)
}

// CurrentLocale returns the active locale used by the translation and
// formatting helpers. Unlike [SystemLocale], which always reports the operating
// system locale, this honours an app- or test-level override (see
// [SetLocaleForTests]); with no override active it reflects the system locale.
//
// It is the same locale source that [IsRTL] (and therefore the default text base
// direction) consults, so text shaping, direction and formatting stay in step.
//
// Since: 2.8
func CurrentLocale() bellui.Locale {
	tag := getCurrentTag()
	if tag == (language.Tag{}) { // no override and not yet initialised
		return SystemLocale()
	}
	return localeFromTag(tag)
}

func closestSupportedLocale(locs []string) bellui.Locale {
	// translated is mutated by addLanguage; build the matcher under the read
	// lock so a concurrent AddTranslations cannot race the slice.
	tagMu.RLock()
	matcher := language.NewMatcher(translated)
	tagMu.RUnlock()

	tags := make([]language.Tag, len(locs))
	for i, loc := range locs {
		tag, err := language.Parse(loc)
		if err != nil {
			bellui.LogError("Error parsing user locale "+loc, err)
		}
		tags[i] = tag
	}
	best, _, _ := matcher.Match(tags...)
	return localeFromTag(best)
}

func localeFromTag(in language.Tag) bellui.Locale {
	b, s, r := in.Raw()
	ret := b.String()

	if r.String() != "ZZ" {
		ret += "-" + r.String()

		if s.String() != "Zzzz" {
			ret += "-" + s.String()
		}
	}

	return bellui.Locale(ret)
}
