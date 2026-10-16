package lang

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// LSelect resolves a message whose wording depends on a non-count selector such
// as grammatical gender or formality — the ICU MessageFormat "select" case that
// CLDR plural rules cannot express (e.g. "{name} updated her/his/their profile").
//
// It is the "blessed" multi-key pattern: author one catalog key per variant,
// suffixed with the selector, plus a mandatory ".other" variant used as the
// fallback. LSelect looks up keyPrefix+"."+selector, falling back to
// keyPrefix+".other", and finally to keyPrefix itself. Templating and locale
// reactivity come from the underlying LocalizeKey machinery.
//
//	// catalog: profile_updated.female / .male / .other
//	lang.LSelect("profile_updated", gender, map[string]any{"Name": name})
//
// Since: 1.0
func LSelect(keyPrefix, selector string, data ...any) string {
	var d0 any
	if len(data) > 0 {
		d0 = data[0]
	}

	if selector != "" {
		if s, ok := lookupMessage(keyPrefix+"."+selector, d0); ok {
			return s
		}
	}

	// Fall back to the mandatory ".other" variant, then to the prefix itself.
	return LocalizeKey(keyPrefix+".other", keyPrefix, data...)
}

// LSelectPlural composes a selector variant with CLDR plural forms: it picks the
// keyPrefix+"."+selector message (falling back to keyPrefix+".other") and then
// resolves the correct plural form within it using count. Each variant key is a
// plural message (one/other/…), so gender and plurality compose as key suffixes
// without an app-side switch.
//
//	// catalog: items_updated.female = {one, other}, .male = {…}, .other = {…}
//	lang.LSelectPlural("items_updated", gender, n, map[string]any{"Count": n})
//
// Since: 1.0
func LSelectPlural(keyPrefix, selector string, count int, data ...any) string {
	var d0 any
	if len(data) > 0 {
		d0 = data[0]
	}

	if selector != "" {
		if s, ok := lookupPluralMessage(keyPrefix+"."+selector, count, d0); ok {
			return s
		}
	}

	return LocalizePluralKey(keyPrefix+".other", keyPrefix, count, data...)
}

// lookupMessage probes the catalog for an exact key, reporting whether it exists
// so callers can chain fallbacks. Unlike LocalizeKey it does not substitute a
// literal fallback for a missing key.
func lookupMessage(key string, data any) (string, bool) {
	ret, err := localize(&i18n.LocalizeConfig{MessageID: key, TemplateData: data})
	if err != nil {
		return "", false
	}
	return ret, true
}

// lookupPluralMessage is lookupMessage for a plural key at the given count.
func lookupPluralMessage(key string, count int, data any) (string, bool) {
	ret, err := localize(&i18n.LocalizeConfig{MessageID: key, PluralCount: count, TemplateData: data})
	if err != nil {
		return "", false
	}
	return ret, true
}
