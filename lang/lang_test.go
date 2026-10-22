package lang

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

func TestAddTranslations(t *testing.T) {
	err := AddTranslations(bellui.NewStaticResource("en.pxf", []byte(`messages = {
		"Test": "Match"
	}`)))
	if assert.NoError(t, err) {
		setupLang("en")
		assert.Equal(t, "Match", L("Test"))
	}

	err = AddTranslationsForLocale([]byte(`messages = {
		"Test2": "Match2"
	}`), "fr")
	if assert.NoError(t, err) {
		setupLang("fr")
		assert.Equal(t, "Match2", L("Test2"))
	}
}

func TestLocalize_Default(t *testing.T) {
	fallback := closestSupportedLocale([]string{"xx"})
	assert.Equal(t, bellui.Locale("en"), fallback[0:2])
}

func TestLocalize_Fallback(t *testing.T) {
	assert.Equal(t, "Missing", L("Missing"))
}

func TestLocalize_Struct(t *testing.T) {
	type data struct {
		Str string
	}

	assert.Equal(t, "Hello World!", L("Hello {Str}!", data{Str: "World"}))
	assert.Equal(t, "Hello World!", L("Hello {{.Str}}!", data{Str: "World"}), "the Go template form still fills")
}

func TestLocalize_Map(t *testing.T) {
	assert.Equal(t, "Hello World!", L("Hello {Str}!", map[string]string{"Str": "World"}))
	assert.Equal(t, "Hello World!", L("Hello {{.Str}}!", map[string]string{"Str": "World"}), "the Go template form still fills")
}

func TestLocalizePlural_Fallback(t *testing.T) {
	require.NoError(t, AddTranslations(bellui.NewStaticResource("en.pxf", []byte(`messages = {
		"Apple": {
			one = "Apple"
			other = "Apples"
		}
	}`))))

	setupLang("en")
	assert.Equal(t, "Missing", N("Missing", 1))
	assert.Equal(t, "Apple", N("Apple", 1))
	assert.Equal(t, "Apples", N("Apple", 2))
}

func TestLocalizeKey_Fallback(t *testing.T) {
	require.NoError(t, AddTranslations(bellui.NewStaticResource("en.pxf", []byte(`messages = {
		"appleID": "Apple Matched"
	}`))))

	setupLang("en")
	assert.Equal(t, "Apple", X("appleIDMissing", "Apple"))
	assert.Equal(t, "Apple Matched", X("appleID", "Apple"))
}

func TestLocalizePluralKey_Fallback(t *testing.T) {
	require.NoError(t, AddTranslations(bellui.NewStaticResource("en.pxf", []byte(`messages = {
		"appleID": {
			one = "Apple"
			other = "Apples"
		}
	}`))))

	setupLang("en")
	assert.Equal(t, "Missing", XN("appleIDMissing", "Missing", 1))
	assert.Equal(t, "Apple", XN("appleID", "Apple", 1))
	assert.Equal(t, "Apples", XN("appleID", "Apple", 2))
}

func TestLoadBuiltInTranslations(t *testing.T) {
	assert.NoError(t, AddTranslationsFS(translations, "translations"))
	setupLang("en")
	assert.Equal(t, "OK", X("OK", "FAIL"))
}

func TestSetLocale(t *testing.T) {
	// Restore the system locale after the test so state does not leak.
	t.Cleanup(func() { SetLocale("") })

	// An override is honoured by CurrentLocale and the direction resolver, with
	// the full region/script tag retained.
	SetLocale("ar-EG")
	assert.Equal(t, "ar-EG", CurrentLocale().String())
	assert.True(t, IsRTL(), "Arabic locale override should be RTL")

	SetLocale("fr-FR")
	assert.Equal(t, "fr-FR", CurrentLocale().String())
	assert.False(t, IsRTL(), "French locale override should be LTR")

	// An empty locale reverts to the system locale.
	SetLocale("")
	assert.Equal(t, SystemLocale(), CurrentLocale())
}

func TestSetLocale_RefreshHook(t *testing.T) {
	SetLocale("en") // known starting point
	t.Cleanup(func() {
		RegisterLocaleChangeRefresh(nil)
		SetLocale("")
	})

	var refreshes int
	RegisterLocaleChangeRefresh(func() { refreshes++ })

	SetLocale("fr-FR")
	assert.Equal(t, 1, refreshes, "a real locale change should refresh once")

	SetLocale("fr-FR")
	assert.Equal(t, 1, refreshes, "setting the same locale should not refresh")

	SetLocale("ar-EG")
	assert.Equal(t, 2, refreshes, "changing to another locale should refresh")
}

func TestAddLocaleChangeListener(t *testing.T) {
	SetLocale("en") // known starting point
	t.Cleanup(func() { SetLocale("") })

	var calls int
	remove := AddLocaleChangeListener(func() { calls++ })

	SetLocale("fr-FR")
	assert.Equal(t, 1, calls, "a real locale change should notify the listener")

	SetLocale("fr-FR")
	assert.Equal(t, 1, calls, "setting the same locale should not notify")

	SetLocale("ar-EG")
	assert.Equal(t, 2, calls, "changing to another locale should notify")

	remove()
	SetLocale("fr-FR")
	assert.Equal(t, 2, calls, "a removed listener should not be notified")
}

func TestUpdateLocalizer_EmptyLocaleList(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("a C-locale environment only yields an empty locale list where locales come from env vars")
	}
	// Registered before t.Setenv's cleanups so it runs after the environment
	// is restored (cleanups are LIFO), re-seating the real locale.
	t.Cleanup(updateLocalizer)
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANGUAGE", "")

	updateLocalizer() // panicked before the empty-list guard

	assert.Equal(t, "en", CurrentLocale().LanguageString())
}
