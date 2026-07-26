package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLSelect_SelectorHit(t *testing.T) {
	setupLang("en")
	data := map[string]any{"Name": "Alex"}
	assert.Equal(t, "Alex updated her profile", LSelect("profile_updated", "female", data))
	assert.Equal(t, "Alex updated his profile", LSelect("profile_updated", "male", data))
	assert.Equal(t, "Alex updated their profile", LSelect("profile_updated", "other", data))
}

func TestLSelect_UnknownSelectorFallsBackToOther(t *testing.T) {
	setupLang("en")
	data := map[string]any{"Name": "Sam"}
	// "nonbinary" has no variant key, so it falls back to ".other".
	assert.Equal(t, "Sam updated their profile", LSelect("profile_updated", "nonbinary", data))
	// Empty selector goes straight to ".other".
	assert.Equal(t, "Sam updated their profile", LSelect("profile_updated", "", data))
}

func TestLSelect_MissingPrefixReturnsPrefix(t *testing.T) {
	setupLang("en")
	// No catalog keys at all for this prefix: neither ".female" nor ".other"
	// exist, so the prefix itself is the last-resort fallback.
	assert.Equal(t, "no_such_message", LSelect("no_such_message", "female"))
}

func TestLSelect_Locale(t *testing.T) {
	data := map[string]any{"Name": "Alex"}

	setupLang("de")
	assert.Equal(t, "Alex hat ihr Profil aktualisiert", LSelect("profile_updated", "female", data))
	assert.Equal(t, "Alex hat sein Profil aktualisiert", LSelect("profile_updated", "male", data))

	// Same call, different active locale — reacts like the other formatters.
	setupLang("en")
	assert.Equal(t, "Alex updated his profile", LSelect("profile_updated", "male", data))
}

func TestLSelectPlural_ComposesSelectorAndCount(t *testing.T) {
	setupLang("en")
	one := map[string]any{"Count": 1}
	many := map[string]any{"Count": 3}

	assert.Equal(t, "She shared 1 item", LSelectPlural("items_shared", "female", 1, one))
	assert.Equal(t, "She shared 3 items", LSelectPlural("items_shared", "female", 3, many))
	assert.Equal(t, "He shared 1 item", LSelectPlural("items_shared", "male", 1, one))
	assert.Equal(t, "They shared 3 items", LSelectPlural("items_shared", "other", 3, many))
}

func TestLSelectPlural_UnknownSelectorFallsBackToOther(t *testing.T) {
	setupLang("en")
	assert.Equal(t, "They shared 2 items", LSelectPlural("items_shared", "nonbinary", 2, map[string]any{"Count": 2}))
}

func TestLSelectPlural_Locale(t *testing.T) {
	setupLang("de")
	assert.Equal(t, "Sie hat 1 Element geteilt", LSelectPlural("items_shared", "female", 1, map[string]any{"Count": 1}))
	assert.Equal(t, "Er hat 5 Elemente geteilt", LSelectPlural("items_shared", "male", 5, map[string]any{"Count": 5}))
}
