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
