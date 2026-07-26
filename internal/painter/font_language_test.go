package painter

import (
	"testing"

	"github.com/belluikit/typesetting/language"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
)

// TestResolveLanguage checks the per-text language override resolves to the
// TextStyle.Language tag when set, and otherwise falls back to the active
// locale (issues #22, #69).
func TestResolveLanguage(t *testing.T) {
	// The zero value follows the active (override-aware) locale, so a locale
	// override drives shaping language — not just the OS locale (#69).
	lang.SetLocaleForTests(t, "ru-RU")
	assert.Equal(t, language.NewLanguage("ru-RU"), resolveLanguage(bellui.TextStyle{}),
		"empty Language should follow the active locale override")

	// A non-empty Language overrides the locale and is canonicalized as a
	// BCP 47 tag (lowercased, '_' mapped to '-').
	assert.Equal(t, language.NewLanguage("sr-Cyrl"), resolveLanguage(bellui.TextStyle{Language: "sr-Cyrl"}),
		"explicit Language should override the locale")
	assert.Equal(t, language.NewLanguage("sr-cyrl"), resolveLanguage(bellui.TextStyle{Language: "sr_Cyrl"}),
		"Language should be canonicalized to a BCP 47 tag")

	// The override does not depend on the locale.
	lang.SetLocaleForTests(t, "en-US")
	assert.Equal(t, language.NewLanguage("sr-Cyrl"), resolveLanguage(bellui.TextStyle{Language: "sr-Cyrl"}),
		"override should resolve to its own tag regardless of the locale")
	assert.Equal(t, language.NewLanguage("en-US"), resolveLanguage(bellui.TextStyle{}),
		"empty Language should track the new active locale")
}
