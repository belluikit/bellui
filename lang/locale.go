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
		// A failed lookup still returns a value, which is not a locale;
		// parsing it would only log a second, misleading error — the same
		// rule updateLocalizer applies to GetLocales (#271).
		loc = ""
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
// Since: 1.0
func CurrentLocale() bellui.Locale {
	tag := getCurrentTag()
	if tag == (language.Tag{}) { // no override and not yet initialised
		return SystemLocale()
	}
	return localeFromTag(tag)
}

func closestSupportedLocale(locs []string) bellui.Locale {
	return closestSupportedLocaleForTags(parseUserLocales(locs))
}

func closestSupportedLocaleForTags(tags []language.Tag) bellui.Locale {
	// translated is mutated by addLanguage; build the matcher under the read
	// lock so a concurrent AddTranslations cannot race the slice.
	tagMu.RLock()
	supported := translated
	matcher := language.NewMatcher(supported)
	tagMu.RUnlock()

	best, idx, conf := matcher.Match(tags...)
	if conf == language.No && !spokenInUserRegion(matcher, idx, tags) {
		return localeFromTag(supported[0])
	}
	return localeFromTag(best)
}

// spokenInUserRegion reports whether the supported language at idx is the main
// language of the region of one of the user's locales. A match the matcher has
// no confidence in is only a guess from a shared script — Persian gets Arabic,
// Serbian gets Russian — so the app's default language is the better answer
// (#342). The guess is kept where the region itself speaks it: Chechen or Yakut
// users in Russia read Russian, and Cantonese written in simplified script
// reads as Simplified Chinese.
func spokenInUserRegion(m language.Matcher, idx int, tags []language.Tag) bool {
	for _, tag := range tags {
		region, _ := tag.Region() // guessed from the language when the tag names none
		local, err := language.Compose(region)
		if err != nil {
			continue
		}
		if _, i, conf := m.Match(local); i == idx && conf >= language.High {
			return true
		}
	}
	return false
}

// parseUserLocales parses the locale strings the OS reports into language
// tags, keeping every entry that names a language and dropping the rest.
// Entries are normalized first (see normalizeLocale) and one that is still
// not well-formed is logged with the raw string %q-quoted — an empty or
// whitespace entry stays visible in the log, the on-device capture
// instrument (#271). Logging it is not the same as losing it:
// [language.Parse] hands back whatever part of the tag it could read
// alongside the error, so Java's "th_TH_TH_#u-nu-thai" still contributes
// th-TH rather than passing the locale to the entry behind it.
//
// Only an entry that comes through as und is dropped, which covers the list
// holding one empty string that go-locale returns from a failed Android JNI
// lookup (#271: the "tag is not well-formed at every start" on the OPD2403
// bench was exactly that, a "no current JVM" failure during package init),
// junk, and a literal "und"/"root". An und tag names nothing to match on and
// is the zero [language.Tag] that [CurrentLocale] reads as "not yet
// initialised", so it must never reach the matcher or currentTag.
func parseUserLocales(locs []string) []language.Tag {
	tags := make([]language.Tag, 0, len(locs))
	for _, loc := range locs {
		tag, err := language.Parse(normalizeLocale(loc))
		if err != nil {
			bellui.LogError(fmt.Sprintf("Error parsing user locale %q", loc), err)
		}
		if tag == language.Und {
			continue
		}
		tags = append(tags, tag)
	}
	return tags
}

// normalizeLocale rewrites the vendor forms an OS reports into BCP 47: the
// Java Locale.toString() separator ("en_US" — which x/text already tolerates,
// so this is belt and braces) and its trailing marker carrying the script and
// any Unicode extension ("zh_CN_#Hans", "de_DE_#u-co-phonebk"), which x/text
// does not accept. The script is folded back into the tag rather than dropped
// with the marker: losing it would resolve sr_RS_#Latn to Serbian's Cyrillic
// default. The extension is discarded — nothing in the toolkit reads it.
func normalizeLocale(loc string) string {
	norm := strings.ReplaceAll(strings.TrimSpace(loc), "_", "-")
	marker := strings.IndexByte(norm, '#')
	if marker < 0 {
		return norm
	}

	base := strings.TrimRight(norm[:marker], "- ")
	script, _, _ := strings.Cut(norm[marker+1:], "-")
	if base == "" || len(script) != 4 { // no script, just an extension ("u-co-phonebk")
		return base
	}

	// BCP 47 places the script directly after the primary language subtag.
	lang, rest, hasRest := strings.Cut(base, "-")
	withScript := lang + "-" + script
	if hasRest {
		withScript += "-" + rest
	}
	if _, err := language.Parse(withScript); err != nil {
		return base // the marker did not hold a script after all
	}
	return withScript
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
