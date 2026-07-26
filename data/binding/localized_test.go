package binding

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
)

func TestNewLocalizedString(t *testing.T) {
	test.NewApp()
	lang.SetLocale("en")
	t.Cleanup(func() { lang.SetLocale("") })

	s := NewLocalizedString("Cancel")

	// Resolves once at construction so Get works before being bound.
	v, err := s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Cancel", v)

	// While bound (has a listener), it re-derives on a live locale change.
	s.AddListener(NewDataListener(func() {}))

	lang.SetLocale("fr-FR")
	v, err = s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Annuler", v)

	lang.SetLocale("en")
	v, err = s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Cancel", v)
}

func TestNewLocalizedString_TemplateData(t *testing.T) {
	test.NewApp()
	lang.SetLocale("en")
	t.Cleanup(func() { lang.SetLocale("") })

	// Unknown key falls back to the key, with template data filled in, and the
	// data is re-applied on every re-derivation rather than being baked in.
	s := NewLocalizedString("Hello {{.Name}}", map[string]any{"Name": "Ada"})
	s.AddListener(NewDataListener(func() {}))

	v, err := s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Hello Ada", v)

	lang.SetLocale("fr-FR")
	v, err = s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Hello Ada", v)
}

// TestNewLocalizedString_UnsubscribesWhenUnbound verifies that once the last
// listener is removed the binding no longer tracks locale changes, so it does
// not leak a lang subscription for a transient binding.
func TestNewLocalizedString_UnsubscribesWhenUnbound(t *testing.T) {
	test.NewApp()
	lang.SetLocale("en")
	t.Cleanup(func() { lang.SetLocale("") })

	s := NewLocalizedString("Cancel")
	l := NewDataListener(func() {})
	s.AddListener(l)
	s.RemoveListener(l)

	// No listeners remain, so a locale change must not update the value.
	lang.SetLocale("fr-FR")
	v, err := s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Cancel", v)
}

func TestNewLocalizedStringWithKey(t *testing.T) {
	test.NewApp()
	lang.SetLocale("en")
	t.Cleanup(func() { lang.SetLocale("") })

	// A known key resolves via the catalogue, not the fallback, and re-translates.
	s := NewLocalizedStringWithKey("Cancel", "ignored fallback")
	s.AddListener(NewDataListener(func() {}))

	v, err := s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Cancel", v)

	lang.SetLocale("fr-FR")
	v, err = s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Annuler", v)

	// An unknown key uses the explicit fallback.
	u := NewLocalizedStringWithKey("no.such.key.xyz", "Fallback text")
	uv, err := u.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Fallback text", uv)
}

func TestNewLocalizedPlural(t *testing.T) {
	test.NewApp()
	assert.NoError(t, lang.AddTranslationsForLocale(
		[]byte(`{"TestApples":{"one":"{{.Count}} apple","other":"{{.Count}} apples"}}`), "en",
	))
	assert.NoError(t, lang.AddTranslationsForLocale(
		[]byte(`{"TestApples":{"one":"{{.Count}} pomme","other":"{{.Count}} pommes"}}`), "fr",
	))
	lang.SetLocale("en")
	t.Cleanup(func() { lang.SetLocale("") })

	count := NewInt()
	assert.NoError(t, count.Set(1))

	// Singular at construction, with the live count exposed to the template.
	s := NewLocalizedPlural("TestApples", count)
	v, err := s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "1 apple", v)

	s.AddListener(NewDataListener(func() {}))

	// Changing the count selects the plural form; a synchronous locale change
	// re-derives and reads the latest count, exercising both inputs together.
	assert.NoError(t, count.Set(3))
	lang.SetLocale("fr-FR")
	v, err = s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "3 pommes", v)
}

func TestNewLocalizedPluralWithKey(t *testing.T) {
	test.NewApp()
	assert.NoError(t, lang.AddTranslationsForLocale(
		[]byte(`{"plural.cart":{"one":"{{.Count}} item","other":"{{.Count}} items"}}`), "en",
	))
	lang.SetLocale("en")
	t.Cleanup(func() { lang.SetLocale("") })

	count := NewInt()
	assert.NoError(t, count.Set(2))

	s := NewLocalizedPluralWithKey("plural.cart", "{{.Count}} thing(s)", count)
	v, err := s.Get()
	assert.NoError(t, err)
	assert.Equal(t, "2 items", v)

	// Unknown key falls back to the supplied template, still pluralised by count.
	u := NewLocalizedPluralWithKey("plural.missing", "{{.Count}} widget", count)
	uv, err := u.Get()
	assert.NoError(t, err)
	assert.Equal(t, "2 widget", uv)
}
