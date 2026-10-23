package lang

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When the user's language is one bellui ships a catalog for (fr) but the app
// has no catalog for it, a keyed message falls back to the app's catalog in
// the default language (English) without logging a failure. #339 found N,
// X, XN and LSelect returned the raw message ID and logged "Translation
// failure": go-i18n hands back the English message together with a
// not-found error, and the error discarded the message.
func TestLocalize_FallsBackToDefaultLanguageCatalog(t *testing.T) {
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = {
		"fallback339.files": { one = "{Count} file" other = "{Count} files" }
		"fallback339.greet": "Hello {Name}"
		"fallback339.updated.female": "{Name} updated her profile"
		"fallback339.updated.other": "{Name} updated their profile"
	}`), "en"))
	SetLocaleForTests(t, "fr")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	data := map[string]any{"Count": 2, "Name": "Ada"}
	assert.Equal(t, "2 files", N("fallback339.files", 2, data))
	assert.Equal(t, "2 files", XN("fallback339.files", "fallback", 2, data))
	assert.Equal(t, "Hello Ada", X("fallback339.greet", "fallback", data))
	assert.Equal(t, "Ada updated her profile", LSelect("fallback339.updated", "female", data))
	assert.Equal(t, "Ada updated their profile", LSelect("fallback339.updated", "male", data))

	// A message no catalog has still resolves to its source text, quietly:
	// an untranslated string is not a failure.
	assert.Equal(t, "Not in any catalog 339", L("Not in any catalog 339"))

	assert.NotContains(t, buf.String(), "Translation failure")
}

// A fallback whose template fails to execute is still a failure. go-i18n
// reports the fallback's not-found error ahead of the execution error, so
// treating every fallback as a success returned "" with no log.
func TestLocalize_FailedFallbackStillFails(t *testing.T) {
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = {
		"fail688.items": { one = "{Count} item" }
	}`), "en"))
	SetLocaleForTests(t, "fr")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// English has no "other" form for 3, so nothing renders.
	assert.Equal(t, "fail688.items", N("fail688.items", 3, map[string]any{"Count": 3}))
	assert.Contains(t, buf.String(), "Translation failure")
}

// In a partly translated app the user's own ".other" variant beats a selector
// variant that exists only in the default language: LSelect answers in the
// user's language, as it did before default-language fallbacks counted as
// found.
func TestLSelect_PrefersOwnLanguageOtherOverFallbackVariant(t *testing.T) {
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = {
		"partial688.updated.female": "{Name} updated her profile"
		"partial688.updated.other": "{Name} updated their profile"
		"partial688.items.female": { one = "{Name} added her item" other = "{Name} added her items" }
		"partial688.items.other": { one = "{Name} added an item" other = "{Name} added items" }
	}`), "en"))
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = {
		"partial688.updated.other": "{Name} a mis à jour son profil"
		"partial688.items.other": { one = "{Name} a ajouté un élément" other = "{Name} a ajouté des éléments" }
	}`), "fr"))
	SetLocaleForTests(t, "fr")

	data := map[string]any{"Name": "Ada"}
	assert.Equal(t, "Ada a mis à jour son profil", LSelect("partial688.updated", "female", data))
	assert.Equal(t, "Ada a ajouté des éléments", LSelectPlural("partial688.items", "female", 2, data))
}
