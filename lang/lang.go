// Package lang introduces a translation and localisation API for Bellui applications
//
// Since 2.5
package lang

import (
	"embed"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"text/template"

	"github.com/jeandeaual/go-locale"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/belluikit/bellui"

	"golang.org/x/text/language"
)

var (
	// L is a shortcut to localize a string, similar to the gettext "_" function.
	// More info available on the `Localize` function.
	L = Localize

	// N is a shortcut to localize a string with plural forms, similar to the ngettext function.
	// More info available on the `LocalizePlural` function.
	N = LocalizePlural

	// X is a shortcut to get the localization of a string with specified key, similar to pgettext.
	// More info available on the `LocalizeKey` function.
	X = LocalizeKey

	// XN is a shortcut to get the localization plural form of a string with specified key, similar to npgettext.
	// More info available on the `LocalizePluralKey` function.
	XN = LocalizePluralKey

	bundle     *i18n.Bundle
	localizer  *i18n.Localizer
	currentTag language.Tag // resolved language tag for formatting functions
	tagMu      sync.RWMutex
	setupOnce  sync.Once

	//go:embed translations
	translations embed.FS
	translated   []language.Tag
)

// Localize asks the translation engine to translate a string, this behaves like the gettext "_" function.
// The string can be templated and the template data can be passed as a struct with exported fields,
// or as a map of string keys to any suitable value.
func Localize(in string, data ...any) string {
	return LocalizeKey(in, in, data...)
}

// LocalizeKey asks the translation engine for the translation with specific ID.
// If it cannot be found then the fallback will be used.
// The string can be templated and the template data can be passed as a struct with exported fields,
// or as a map of string keys to any suitable value.
func LocalizeKey(key, fallback string, data ...any) string {
	var d0 any
	if len(data) > 0 {
		d0 = data[0]
	}

	ret, err := localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    key,
			Other: fallback,
		},
		TemplateData: d0,
	})
	if err != nil {
		bellui.LogError("Translation failure", err)
		return fallbackWithData(key, fallback, d0)
	}
	return ret
}

// LocalizePlural asks the translation engine to translate a string from one of a number of plural forms.
// This behaves like the ngettext function, with the `count` parameter determining the plurality looked up.
// The string can be templated and the template data can be passed as a struct with exported fields,
// or as a map of string keys to any suitable value.
func LocalizePlural(in string, count int, data ...any) string {
	return LocalizePluralKey(in, in, count, data...)
}

// LocalizePluralKey asks the translation engine for the translation with specific ID in plural form.
// This behaves like the npgettext function, with the `count` parameter determining the plurality looked up.
// If it cannot be found then the fallback will be used.
// The string can be templated and the template data can be passed as a struct with exported fields,
// or as a map of string keys to any suitable value.
func LocalizePluralKey(key, fallback string, count int, data ...any) string {
	var d0 any
	if len(data) > 0 {
		d0 = data[0]
	}

	ret, err := localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    key,
			Other: fallback,
		},
		PluralCount:  count,
		TemplateData: d0,
	})
	if err != nil {
		bellui.LogError("Translation failure", err)
		return fallbackWithData(key, fallback, d0)
	}
	return ret
}

// AddTranslations allows an app to load a bundle of translations.
// The language that this relates to will be inferred from the resource name, for example "fr.json".
// The data should be in json format.
func AddTranslations(r bellui.Resource) error {
	defer updateLocalizer()
	return addLanguage(r.Content(), r.Name())
}

// AddTranslationsForLocale allows an app to load a bundle of translations for a specified locale.
// The data should be in json format.
func AddTranslationsForLocale(data []byte, l bellui.Locale) error {
	defer updateLocalizer()
	return addLanguage(data, l.String()+".json")
}

// AddTranslationsFS supports adding all translations in one calling using an `embed.FS` setup.
// The `dir` parameter specifies the name or path of the directory containing translation files
// inside this embedded filesystem.
// Each file should be a json file with the name following pattern [prefix.]lang.json.
func AddTranslationsFS(fs embed.FS, dir string) (retErr error) {
	files, err := fs.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		name := f.Name()
		data, err := fs.ReadFile(dir + "/" + name)
		if err != nil {
			if retErr == nil {
				retErr = err
			}
			continue
		}

		err = addLanguage(data, name)
		if err != nil {
			if retErr == nil {
				retErr = err
			}
			continue
		}
	}

	updateLocalizer()

	return retErr
}

func addLanguage(data []byte, name string) error {
	// Guard the shared bundle and translated slice against concurrent readers
	// (localize, newLocalizer, closestSupportedLocale) now that SetLocale is a
	// public runtime API.
	tagMu.Lock()
	defer tagMu.Unlock()

	f, err := bundle.ParseMessageFileBytes(data, name)
	if err != nil {
		return err
	}

	translated = append(translated, f.Tag)
	return nil
}

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	translated = []language.Tag{language.Make("en")} // the first item in this list will be the fallback if none match
	err := AddTranslationsFS(translations, "translations")
	if err != nil {
		bellui.LogError("Error occurred loading built-in translations", err)
	}
}

func fallbackWithData(key, fallback string, data any) string {
	t, err := template.New(key).Parse(fallback)
	if err != nil {
		log.Println("Could not parse fallback template")
		return fallback
	}
	str := &strings.Builder{}
	_ = t.Execute(str, data)
	return str.String()
}

// SetLocaleForTests overrides the current locale for testing purposes.
// It is exported so that widget tests in other packages can test
// locale-dependent behavior. It should not be called from production code.
//
// The previous locale is automatically restored via t.Cleanup when the
// test (or subtest) finishes, preventing locale state from leaking
// between tests.
func SetLocaleForTests(t interface{ Cleanup(func()) }, locale string) {
	prevTag := getCurrentTag()
	prevLocalizer := getLocalizer()
	t.Cleanup(func() {
		setLocalizer(prevLocalizer)
		setCurrentTag(prevTag)
	})
	setupLang(locale)
}

// SetLocale overrides the active locale used for translations, date and number
// formatting, and text direction. Pass a BCP 47 locale tag such as "ar",
// "fr-FR" or "sr-Cyrl-RS"; pass an empty locale to revert to the system locale.
//
// Translations resolve to the closest supported language, while the full tag
// (including region and script) is retained for formatting, [CurrentLocale],
// [IsRTL] and text shaping. Unlike [SystemLocale], subsequent calls to those
// helpers reflect this override.
//
// When the active locale actually changes and an app is running, every running
// window is re-laid-out and repainted — re-applying locale-derived formatting
// and direction and re-rendering each widget's current text — the way a theme
// change is applied. This follows the same threading rules as
// [bellui.Settings.SetTheme], so call it on the main goroutine. If no app is
// running the new locale simply takes effect for UI built or refreshed
// afterwards.
//
// Note that the relayout repaints existing text but does not re-evaluate the
// [Localize]/[L], [LocalizeKey]/[X] and plural calls that produced it: a widget
// built from a static lang.L("Cancel") keeps the string captured at build time.
// For text that should re-translate on a live language switch, drive it from
// [github.com/belluikit/bellui/data/binding.NewLocalizedString] (or another
// [AddLocaleChangeListener] subscriber) instead of a pre-resolved string.
//
// Since: 2.8
func SetLocale(l bellui.Locale) {
	prev := getCurrentTag()

	if l == "" { // revert to the system locale
		updateLocalizer()
	} else {
		setupOnce.Do(initRuntime)
		loc := l.String()
		setupLang(closestSupportedLocale([]string{loc}).LanguageString())
		// Retain the full locale (including region/script) as the current tag so
		// formatting and direction honour it, matching updateLocalizer.
		if tag, err := language.Parse(loc); err == nil {
			setCurrentTag(tag)
		}
	}

	if getCurrentTag() != prev {
		refreshLocaleChange()
	}
}

// getCurrentTag returns the current language tag under the read lock.
func getCurrentTag() language.Tag {
	tagMu.RLock()
	defer tagMu.RUnlock()
	return currentTag
}

// setCurrentTag sets the current language tag under the write lock.
func setCurrentTag(tag language.Tag) {
	tagMu.Lock()
	defer tagMu.Unlock()
	currentTag = tag
}

// localize runs a translation under the read lock. tagMu guards both the
// localizer pointer (reassigned by SetLocale) and the shared bundle that the
// localizer reads while resolving a message (mutated by AddTranslations), so the
// lock is held for the whole call. It is a read lock: concurrent translations
// proceed in parallel and only block while a writer holds tagMu, which is rare.
func localize(cfg *i18n.LocalizeConfig) (string, error) {
	tagMu.RLock()
	defer tagMu.RUnlock()
	return localizer.Localize(cfg)
}

// getLocalizer returns the active localizer under the read lock.
func getLocalizer() *i18n.Localizer {
	tagMu.RLock()
	defer tagMu.RUnlock()
	return localizer
}

// setLocalizer swaps the active localizer under the write lock.
func setLocalizer(l *i18n.Localizer) {
	tagMu.Lock()
	defer tagMu.Unlock()
	localizer = l
}

// newLocalizer builds a localizer for lang under the read lock, since
// i18n.NewLocalizer reads the shared bundle.
func newLocalizer(lang string) *i18n.Localizer {
	tagMu.RLock()
	defer tagMu.RUnlock()
	return i18n.NewLocalizer(bundle, lang)
}

// A utility for setting up languages - available to unit tests for overriding system
func setupLang(lang string) {
	setLocalizer(newLocalizer(lang))
	tag, _ := language.Parse(lang)
	setCurrentTag(tag)
}

// updateLocalizer Finds the closest translation from the user's locale list and sets it up
func updateLocalizer() {
	setupOnce.Do(initRuntime)

	all, err := locale.GetLocales()
	if err != nil {
		bellui.LogError("Failed to load user locales", err)
	}
	tags := parseUserLocales(all)
	if len(tags) == 0 {
		// GetLocales reports an empty list without an error in a C/POSIX
		// locale environment (e.g. LANG=C; #1148), and a vendor list whose
		// every entry fails to parse (ColorOS has delivered [""], #271)
		// degrades the same way: English, keeping tags[0] below valid.
		tags = []language.Tag{language.English}
	}
	setupLang(closestSupportedTag(tags).LanguageString())
	// Override currentTag with the full system locale (including region) so
	// that formatting functions can use the region for CLDR lookups — the
	// first entry that parsed, not blindly the first entry reported: a
	// malformed leading entry otherwise silently degrades formatting,
	// collation and direction to the matcher's language-only tag (#271).
	setCurrentTag(tags[0])
}
