package lang

import (
	"fmt"
	"strings"

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

	tag := language.English
	if parsed := parseUserLocales([]string{loc}); len(parsed) > 0 {
		tag = parsed[0]
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
	return closestSupportedTag(parseUserLocales(locs))
}

func closestSupportedTag(tags []language.Tag) bellui.Locale {
	// translated is mutated by addLanguage; build the matcher under the read
	// lock so a concurrent AddTranslations cannot race the slice.
	tagMu.RLock()
	matcher := language.NewMatcher(translated)
	tagMu.RUnlock()

	best, _, _ := matcher.Match(tags...)
	return localeFromTag(best)
}

// parseUserLocales parses the locale strings the OS reports into language
// tags, dropping entries that cannot be made well-formed. Systems hand out
// non-BCP-47 forms — Java Locale.toString() underscores ("en_US") and its
// trailing "#..." script/extension marker ("zh_CN_#Hans"), and go-locale
// turns a failed Android JNI lookup into a list holding one empty string
// (#271: the "tag is not well-formed at every start" on the OPD2403 bench
// was exactly that, a "no current JVM" failure during package init) — so
// entries are normalized before parsing, and one that still fails is logged
// with the raw string %q-quoted (an empty or whitespace entry stays visible
// in the log, the on-device capture instrument) and skipped rather than
// handed to the matcher as an und tag: a bad entry must not cost the locale
// the parseable entries behind it.
func parseUserLocales(locs []string) []language.Tag {
	tags := make([]language.Tag, 0, len(locs))
	for _, loc := range locs {
		norm := strings.TrimSpace(loc)
		if i := strings.IndexByte(norm, '#'); i >= 0 {
			norm = strings.TrimRight(norm[:i], "-_ ")
		}
		norm = strings.ReplaceAll(norm, "_", "-")
		tag, err := language.Parse(norm)
		if err != nil {
			bellui.LogError(fmt.Sprintf("Error parsing user locale %q", loc), err)
			continue
		}
		tags = append(tags, tag)
	}
	return tags
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
