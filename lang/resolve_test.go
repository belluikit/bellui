package lang

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/belluikit/bellui"
)

// snapshotLocaleState restores the package locale state (tag, localizer and
// override flag) when the test finishes, mirroring SetLocaleForTests without
// changing the locale itself.
func snapshotLocaleState(t *testing.T) {
	prevTag := getCurrentTag()
	prevLocalizer := getLocalizer()
	prevOverride := localeOverride.Load()
	t.Cleanup(func() {
		setLocalizer(prevLocalizer)
		setCurrentTag(prevTag)
		localeOverride.Store(prevOverride)
	})
}

func TestResolveSystemLocale_RespectsOverride(t *testing.T) {
	snapshotLocaleState(t)

	SetLocale("de")
	fired := false
	remove := AddLocaleChangeListener(func() { fired = true })
	defer remove()

	ResolveSystemLocale()

	assert.Equal(t, bellui.Locale("de"), CurrentLocale())
	assert.False(t, fired)
}

func TestResolveSystemLocale_AppliesAfterFailedInit(t *testing.T) {
	snapshotLocaleState(t)

	// Learn what a working lookup reports on this host, then seat a different —
	// but well-formed — tag to stand in for the Android cold-start state (#282):
	// no override active, and a fallback tag left behind by a lookup that ran
	// before the JVM attached. It has to parse: an ill-formed string lands as
	// language.Und, which CurrentLocale treats as "not initialised yet" rather
	// than as the stuck fallback this reproduces.
	localeOverride.Store(false)
	ResolveSystemLocale()
	system := getCurrentTag()

	stale := language.MustParse("zu") // Zulu; swapped below on the off chance it is the host locale
	if stale.String() == system.String() {
		stale = language.MustParse("is")
	}
	setupLang(stale.String())
	require.Equal(t, stale.String(), getCurrentTag().String(), "the stale tag must parse")

	fired := false
	remove := AddLocaleChangeListener(func() { fired = true })
	defer remove()

	ResolveSystemLocale()

	assert.True(t, fired, "a changed resolution must notify locale listeners")
	assert.Equal(t, system.String(), getCurrentTag().String(), "the re-resolve must seat the system tag")

	// With the system locale unchanged, a second resolve must not re-fire.
	fired = false
	ResolveSystemLocale()
	assert.False(t, fired)
}

func TestSetLocale_TogglesOverride(t *testing.T) {
	snapshotLocaleState(t)

	SetLocale("de")
	assert.True(t, localeOverride.Load())

	SetLocale("")
	assert.False(t, localeOverride.Load())
}

func TestSetLocaleForTests_RestoresOverride(t *testing.T) {
	snapshotLocaleState(t)
	localeOverride.Store(false)

	t.Run("inner", func(t *testing.T) {
		SetLocaleForTests(t, "fr")
		assert.True(t, localeOverride.Load())

		// A driver-triggered re-resolve must not clobber the test locale.
		ResolveSystemLocale()
		assert.Equal(t, bellui.Locale("fr"), CurrentLocale())
	})

	assert.False(t, localeOverride.Load(), "cleanup must restore the override flag")
}

func TestUpdateLocalizer_KeepsOverrideOnAddTranslations(t *testing.T) {
	snapshotLocaleState(t)

	SetLocale("fr")
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = {
		"resolve-test-key": "clé"
	}`), "fr"))

	// The updateLocalizer run triggered by AddTranslations* must keep the
	// explicit override — it used to revert to the system locale (#282) — while
	// still picking up the newly added messages.
	assert.Equal(t, bellui.Locale("fr"), CurrentLocale())
	assert.Equal(t, "clé", L("resolve-test-key"))
}

// TestNoLookupBeforeTheJVM pins #892: on Android the locales cannot be read
// until JNI_OnLoad hands over the JVM, after package init. A lookup made
// earlier failed and logged "Failed to load user locales" on every launch.
// Until the system can be asked, nothing asks it and nothing is logged; once
// it can, ResolveSystemLocale seats the system's locale.
func TestNoLookupBeforeTheJVM(t *testing.T) {
	snapshotLocaleState(t)
	localeOverride.Store(false)
	oldReady, oldGet := systemLocalesReady, getLocales
	t.Cleanup(func() { systemLocalesReady, getLocales = oldReady, oldGet })

	ready, lookups := false, 0
	systemLocalesReady = func() bool { return ready }
	getLocales = func() ([]string, error) {
		lookups++
		if !ready {
			return []string{""}, errors.New("no current JVM")
		}
		return []string{"pt-BR"}, nil
	}
	var out bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(prev) })

	updateLocalizer() // what package init and an early AddTranslations do
	assert.Zero(t, lookups, "the system was asked before it could answer")
	assert.NotContains(t, out.String(), "Failed to load user locales")
	assert.Equal(t, "en", getCurrentTag().String())

	ready = true
	ResolveSystemLocale() // what the driver does once the JVM is attached
	assert.Equal(t, 1, lookups)
	assert.Equal(t, "pt-BR", getCurrentTag().String())
	assert.Empty(t, out.String())
}
